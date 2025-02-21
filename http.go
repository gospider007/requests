package requests

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/gospider007/tools"
	"golang.org/x/net/http/httpguts"
)

type httpTask struct {
	req          *http.Request
	res          *http.Response
	orderHeaders []string
	err          error

	ctx context.Context
	cnl context.CancelFunc

	writeCtx context.Context
	writeCnl context.CancelFunc

	readCtx context.Context
	readCnl context.CancelCauseFunc
}
type conn2 struct {
	err       error
	tasks     chan *httpTask
	conn      net.Conn
	r         *bufio.Reader
	w         *bufio.Writer
	closeFunc func(error)
	ctx       context.Context

	closeCtx context.Context
	closeCnl context.CancelCauseFunc

	keepMsgNotice    chan struct{}
	keepEnableNotice chan struct{}
	keepDisNotice    chan struct{}

	keepCloseCtx context.Context
	keepCloseCnl context.CancelFunc

	readCtx context.Context
}
type httpBody struct {
	r *io.PipeReader
}

func (obj *httpBody) Read(b []byte) (i int, err error) {
	return obj.r.Read(b)
}
func (obj *httpBody) Close() error {
	return obj.r.Close()
}

func newConn2(ctx context.Context, con net.Conn, closeFunc func(error)) *conn2 {
	closeCtx, closeCnl := context.WithCancelCause(ctx)
	keepCloseCtx, keepCloseCnl := context.WithCancel(closeCtx)
	c := &conn2{
		closeCtx:         closeCtx,
		closeCnl:         closeCnl,
		ctx:              ctx,
		conn:             con,
		closeFunc:        closeFunc,
		r:                bufio.NewReader(con),
		w:                bufio.NewWriter(con),
		tasks:            make(chan *httpTask),
		keepMsgNotice:    make(chan struct{}),
		keepEnableNotice: make(chan struct{}),
		keepDisNotice:    make(chan struct{}),

		keepCloseCtx: keepCloseCtx,
		keepCloseCnl: keepCloseCnl,
	}
	go c.run()
	go c.CheckTCPAliveSafe()
	return c
}

func (obj *conn2) CheckTCPAliveSafe() {
	for {
		select {
		case <-obj.ctx.Done():
			return
		case <-obj.closeCtx.Done():
			return
		case <-obj.keepDisNotice:
			obj.keepSendMsg()
		case <-obj.keepCloseCtx.Done():
			return
		case <-obj.keepEnableNotice:
			obj.keepSendMsg()
			if obj.CheckTCPAliveSafeEnable() {
				return
			}
		}
	}
}
func (obj *conn2) CheckTCPAliveSafeEnable() (closed bool) {
	select {
	case <-obj.ctx.Done():
		return true
	case <-obj.keepCloseCtx.Done():
		return true
	case <-obj.keepDisNotice:
		obj.keepSendMsg()
		return false
	case <-obj.readCtx.Done():
	}
	for {
		select {
		case <-obj.ctx.Done():
			return true
		case <-obj.closeCtx.Done():
			return true
		case <-obj.keepCloseCtx.Done():
			return true
		case <-obj.keepDisNotice:
			obj.keepSendMsg()
			return false
		case <-time.After(time.Second * 30):
			err := obj.conn.SetReadDeadline(time.Now().Add(2 * time.Millisecond))
			if err != nil {
				obj.CloseWithError(err)
				return true
			}
			if _, err = obj.r.Peek(1); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					err = nil
				} else {
					obj.CloseWithError(err)
					return true
				}
			}
			if err = obj.conn.SetReadDeadline(time.Time{}); err != nil {
				obj.CloseWithError(err)
				return true
			}
		}
	}
}

func (obj *conn2) keepSendMsg() {
	select {
	case obj.keepMsgNotice <- struct{}{}:
	case <-obj.ctx.Done():
		return
	case <-obj.closeCtx.Done():
		return
	case <-obj.keepCloseCtx.Done():
		return
	}
}
func (obj *conn2) keepRecvMsg() {
	select {
	case <-obj.keepMsgNotice:
	case <-obj.ctx.Done():
		return
	case <-obj.closeCtx.Done():
		return
	case <-obj.keepCloseCtx.Done():
		return
	}
}
func (obj *conn2) keepSendEnable(task *httpTask) {
	obj.readCtx = task.readCtx
	select {
	case obj.keepEnableNotice <- struct{}{}:
	case <-obj.ctx.Done():
		return
	case <-obj.closeCtx.Done():
		return
	case <-obj.keepCloseCtx.Done():
		return
	}

	obj.keepRecvMsg()
}
func (obj *conn2) keepSendDisable() {
	select {
	case obj.keepDisNotice <- struct{}{}:
	case <-obj.ctx.Done():
		return
	case <-obj.closeCtx.Done():
		return
	case <-obj.keepCloseCtx.Done():
		return
	}

	obj.keepRecvMsg()
}
func (obj *conn2) keepSendClose() {
	obj.keepCloseCnl()
}

var errLastTaskRuning = errors.New("last task is running")

func (obj *conn2) run() (err error) {
	defer obj.CloseWithError(err)
	for {
		select {
		case <-obj.ctx.Done():
			return
		case <-obj.closeCtx.Done():
			return
		case task := <-obj.tasks:
			obj.keepSendDisable()
			go obj.httpWrite(task, task.req.Header.Clone())
			task.res, task.err = http.ReadResponse(obj.r, nil)
			if task.res != nil && task.res.Body != nil {
				rawBody := task.res.Body
				pr, pw := io.Pipe()
				go func() {
					var readErr error
					defer task.readCnl(readErr)
					_, readErr = io.Copy(pw, rawBody)
					log.Print(readErr)
					pw.CloseWithError(readErr)
					if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
						task.err = tools.WrapError(readErr, "failed to read response body")
					} else {
						readErr = nil
					}
					if readErr != nil {
						obj.CloseWithError(readErr)
					} else {
						select {
						case <-task.writeCtx.Done():
						default:
							readErr = tools.WrapError(errLastTaskRuning, errors.New("last task not write done with read done"))
							task.err = readErr
							obj.CloseWithError(readErr)
							return
						}
					}
				}()
				task.res.Body = &httpBody{r: pr}
			} else {
				task.readCnl(nil)
			}
			task.cnl()
			if task.res == nil || task.err != nil {
				return
			}
			obj.keepSendEnable(task)
		}
	}
}
func (obj *conn2) CloseCtx() context.Context {
	return obj.closeCtx
}

func (obj *conn2) Close() error {
	return obj.CloseWithError(nil)
}
func (obj *conn2) CloseWithError(err error) error {
	obj.closeCnl(err)
	if err == nil {
		obj.err = tools.WrapError(obj.err, "connecotr closeWithError close")
	} else {
		obj.err = tools.WrapError(err, "connecotr closeWithError close")
	}
	if obj.closeFunc != nil {
		obj.closeFunc(obj.err)
	}
	return obj.conn.Close()
}
func (obj *conn2) DoRequest(req *http.Request, orderHeaders []string) (*http.Response, context.Context, error) {
	readCtx, readCnl := context.WithCancelCause(obj.closeCtx)
	writeCtx, writeCnl := context.WithCancel(obj.closeCtx)
	ctx, cnl := context.WithCancel(req.Context())
	task := &httpTask{
		readCtx: readCtx,
		readCnl: readCnl,

		writeCtx: writeCtx,
		writeCnl: writeCnl,

		req:          req,
		orderHeaders: orderHeaders,
		ctx:          ctx,
		cnl:          cnl,
	}
	select {
	case <-obj.ctx.Done():
		return nil, task.readCtx, obj.ctx.Err()
	case <-obj.closeCtx.Done():
		return nil, task.readCtx, obj.closeCtx.Err()
	case obj.tasks <- task:
	}
	select {
	case <-obj.ctx.Done():
		return nil, task.readCtx, obj.ctx.Err()
	case <-obj.closeCtx.Done():
		return nil, task.readCtx, obj.closeCtx.Err()
	case <-task.ctx.Done():
	}
	if task.err != nil {
		obj.CloseWithError(task.err)
	}
	return task.res, task.readCtx, task.err
}
func (obj *conn2) Stream() io.ReadWriteCloser {
	obj.keepSendClose()
	return obj.conn
}
func (obj *conn2) httpWrite(task *httpTask, rawHeaders http.Header) {
	defer task.writeCnl()
	defer func() {
		if task.err != nil {
			obj.CloseWithError(tools.WrapError(task.err, "failed to send request body"))
		}
	}()

	host := task.req.Host
	if host == "" {
		host = task.req.URL.Host
	}
	host, task.err = httpguts.PunycodeHostPort(host)
	if task.err != nil {
		return
	}
	host = removeZone(host)

	if rawHeaders.Get("Host") == "" {
		rawHeaders.Set("Host", host)
	}
	if rawHeaders.Get("Connection") == "" {
		rawHeaders.Set("Connection", "keep-alive")
	}
	if rawHeaders.Get("User-Agent") == "" {
		rawHeaders.Set("User-Agent", tools.UserAgent)
	}
	if rawHeaders.Get("Content-Length") == "" && task.req.ContentLength != 0 && shouldSendContentLength(task.req) {
		rawHeaders.Set("Content-Length", fmt.Sprint(task.req.ContentLength))
	}
	writeHeaders := [][2]string{}
	for k, vs := range rawHeaders {
		for _, v := range vs {
			writeHeaders = append(writeHeaders, [2]string{k, v})
		}
	}
	sort.Slice(writeHeaders, func(x, y int) bool {
		xI := slices.Index(task.orderHeaders, writeHeaders[x][0])
		yI := slices.Index(task.orderHeaders, writeHeaders[y][0])
		if xI < 0 {
			return false
		}
		if yI < 0 {
			return true
		}
		if xI <= yI {
			return true
		}
		return false
	})
	ruri := task.req.URL.RequestURI()
	if task.req.Method == "CONNECT" && task.req.URL.Path == "" {
		if task.req.URL.Opaque != "" {
			ruri = task.req.URL.Opaque
		} else {
			ruri = host
		}
	}
	if _, task.err = obj.w.WriteString(fmt.Sprintf("%s %s %s\r\n", task.req.Method, ruri, task.req.Proto)); task.err != nil {
		return
	}
	for _, kv := range writeHeaders {
		if _, task.err = obj.w.WriteString(fmt.Sprintf("%s: %s\r\n", kv[0], kv[1])); task.err != nil {
			return
		}
	}
	if _, task.err = obj.w.WriteString("\r\n"); task.err != nil {
		return
	}
	if task.req.Body == nil {
		task.err = obj.w.Flush()
		return
	}
	if _, task.err = io.Copy(obj.w, task.req.Body); task.err != nil {
		return
	}
	task.err = obj.w.Flush()
}
