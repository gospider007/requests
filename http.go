package requests

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gospider007/tools"
	"golang.org/x/net/http/httpguts"
)

type httpTask struct {
	req          *http.Request
	res          *http.Response
	orderHeaders []interface {
		Key() string
		Val() any
	}
	err error

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
	defer func() {
		obj.CloseWithError(err)
	}()
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
			if task.res != nil {
				task.res.Request = task.req
			}
			if task.res != nil && task.res.Body != nil && task.err == nil {
				rawBody := task.res.Body
				pr, pw := io.Pipe()
				go func() {
					var readErr error
					defer func() {
						task.readCnl(readErr)
					}()
					_, readErr = io.Copy(pw, rawBody)
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
							if task.res.StatusCode == 101 || strings.Contains(task.res.Header.Get("Content-Type"), "text/event-stream") {
								obj.keepSendClose()
								select {
								case <-obj.ctx.Done():
									return
								case <-obj.closeCtx.Done():
									return
								}
							}
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
func (obj *conn2) DoRequest(req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) (*http.Response, context.Context, error) {
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
	default:
		return nil, nil, errLastTaskRuning
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

type websocketConn struct {
	r   io.Reader
	w   io.WriteCloser
	cnl context.CancelCauseFunc
}

func (obj *websocketConn) Read(p []byte) (n int, err error) {
	return obj.r.Read(p)
}
func (obj *websocketConn) Write(p []byte) (n int, err error) {
	return obj.w.Write(p)
}
func (obj *websocketConn) Close() error {
	obj.cnl(nil)
	return obj.w.Close()
}

func (obj *conn2) Stream() io.ReadWriteCloser {
	obj.keepSendClose()
	return &websocketConn{
		cnl: obj.closeCnl,
		r:   obj.r,
		w:   obj.conn,
	}
}
func (obj *conn2) httpWrite(task *httpTask, rawHeaders http.Header) {
	defer func() {
		if task.err != nil {
			obj.CloseWithError(tools.WrapError(task.err, "failed to send request body"))
		}
		task.writeCnl()
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
	contentL, chunked := tools.GetContentLength(task.req)
	if contentL >= 0 {
		rawHeaders.Set("Content-Length", fmt.Sprint(contentL))
	} else if chunked {
		rawHeaders.Set("Transfer-Encoding", "chunked")
	}
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
	for _, kv := range tools.NewHeadersWithH1(task.orderHeaders, rawHeaders) {
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
	if chunked {
		chunkedWriter := newChunkedWriter(obj.w)
		if _, task.err = io.Copy(chunkedWriter, task.req.Body); task.err != nil {
			return
		}
		if task.err = chunkedWriter.Close(); task.err != nil {
			return
		}
	} else {
		if _, task.err = io.Copy(obj.w, task.req.Body); task.err != nil {
			return
		}
	}
	task.err = obj.w.Flush()
}

func newChunkedWriter(w *bufio.Writer) io.WriteCloser {
	return &chunkedWriter{w}
}

type chunkedWriter struct {
	w *bufio.Writer
}

func (cw *chunkedWriter) Write(data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, nil
	}
	if _, err = fmt.Fprintf(cw.w, "%x\r\n", len(data)); err != nil {
		return 0, err
	}
	if _, err = cw.w.Write(data); err != nil {
		return
	}
	if _, err = io.WriteString(cw.w, "\r\n"); err != nil {
		return
	}
	return len(data), cw.w.Flush()
}

func (cw *chunkedWriter) Close() error {
	_, err := io.WriteString(cw.w, "0\r\n\r\n")
	return err
}
