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

	writeCtx context.Context
	writeCnl context.CancelFunc

	readCtx context.Context
	readCnl context.CancelCauseFunc
}
type clientConn struct {
	err       error
	task      *httpTask
	conn      net.Conn
	r         *bufio.Reader
	w         *bufio.Writer
	closeFunc func(error)
	ctx       context.Context

	closeCtx context.Context
	closeCnl context.CancelCauseFunc
}

func newClientConn(ctx context.Context, con net.Conn, closeFunc func(error)) *clientConn {
	closeCtx, closeCnl := context.WithCancelCause(ctx)
	reader, writer := io.Pipe()
	c := &clientConn{
		closeCtx:  closeCtx,
		closeCnl:  closeCnl,
		ctx:       ctx,
		conn:      con,
		closeFunc: closeFunc,
		r:         bufio.NewReader(reader),
		w:         bufio.NewWriter(con),
	}
	go func() {
		_, err := io.Copy(writer, con)
		writer.CloseWithError(err)
		c.CloseWithError(err)
	}()
	return c
}

var errLastTaskRuning = errors.New("last task is running")

func (obj *clientConn) send() {
	go obj.httpWrite(obj.task, obj.task.req.Header.Clone())
	obj.task.res, obj.task.err = http.ReadResponse(obj.r, obj.task.req)
	if obj.task.res == nil || obj.task.err != nil || obj.task.res.Body == nil {
		obj.task.readCnl(nil)
		return
	}
	rawBody := obj.task.res.Body
	pr, pw := io.Pipe()
	go func() {
		var readErr error
		defer func() {
			obj.task.readCnl(readErr)
		}()
		_, readErr = io.Copy(pw, rawBody)
		pw.CloseWithError(readErr)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			obj.task.err = tools.WrapError(readErr, "failed to read response body")
		} else {
			readErr = nil
		}
		if readErr != nil {
			obj.CloseWithError(readErr)
		} else {
			select {
			case <-obj.task.writeCtx.Done():
				if obj.task.res.StatusCode == 101 || strings.Contains(obj.task.res.Header.Get("Content-Type"), "text/event-stream") {
					select {
					case <-obj.ctx.Done():
						return
					case <-obj.closeCtx.Done():
						return
					}
				}
			default:
				readErr = tools.WrapError(errLastTaskRuning, errors.New("last task not write done with read done"))
				obj.task.err = readErr
				obj.CloseWithError(readErr)
				return
			}
		}
	}()
	obj.task.res.Body = pr
}

func (obj *clientConn) CloseCtx() context.Context {
	return obj.closeCtx
}

func (obj *clientConn) Close() error {
	return obj.CloseWithError(nil)
}
func (obj *clientConn) CloseWithError(err error) error {
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
func (obj *clientConn) initTask(req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) {
	readCtx, readCnl := context.WithCancelCause(obj.closeCtx)
	writeCtx, writeCnl := context.WithCancel(obj.closeCtx)
	obj.task = &httpTask{
		readCtx:      readCtx,
		readCnl:      readCnl,
		writeCtx:     writeCtx,
		writeCnl:     writeCnl,
		req:          req,
		orderHeaders: orderHeaders,
	}
}
func (obj *clientConn) DoRequest(req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) (*http.Response, context.Context, error) {
	select {
	case <-obj.ctx.Done():
		return nil, obj.task.readCtx, obj.ctx.Err()
	case <-obj.closeCtx.Done():
		return nil, obj.task.readCtx, obj.closeCtx.Err()
	default:
	}
	if obj.task != nil {
		select {
		case <-obj.task.writeCtx.Done():
		case <-obj.ctx.Done():
			return nil, nil, obj.ctx.Err()
		case <-obj.closeCtx.Done():
			return nil, nil, obj.closeCtx.Err()
		default:
			return nil, obj.task.readCtx, errLastTaskRuning
		}
		select {
		case <-obj.task.readCtx.Done():
		case <-obj.ctx.Done():
			return nil, nil, obj.ctx.Err()
		case <-obj.closeCtx.Done():
			return nil, nil, obj.closeCtx.Err()
		default:
			return nil, obj.task.readCtx, errLastTaskRuning
		}
	} else {
		select {
		case <-obj.ctx.Done():
			return nil, nil, obj.ctx.Err()
		case <-obj.closeCtx.Done():
			return nil, nil, obj.closeCtx.Err()
		default:
			return nil, obj.task.readCtx, errLastTaskRuning
		}
	}
	obj.initTask(req, orderHeaders)
	obj.send()
	if obj.task.err != nil {
		obj.CloseWithError(obj.task.err)
	}
	return obj.task.res, obj.task.readCtx, obj.task.err
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
	// i, err := obj.w.Write(p)
	// log.Print(err, "  write error  ", i, p)
	// return i, err
	return obj.w.Write(p)
}
func (obj *websocketConn) Close() error {
	obj.cnl(nil)
	return obj.w.Close()
}

func (obj *clientConn) Stream() io.ReadWriteCloser {
	return &websocketConn{
		cnl: obj.closeCnl,
		r:   obj.r,
		w:   obj.conn,
	}
}
func (obj *clientConn) httpWrite(task *httpTask, rawHeaders http.Header) {
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
