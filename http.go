package requests

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gospider007/tools"
	"golang.org/x/net/http/httpguts"
)

type rsp struct {
	r   *http.Response
	ctx context.Context
	err error
}

type clientConn struct {
	conn      net.Conn
	r         *bufio.Reader
	w         *bufio.Writer
	closeFunc func(error)
	ctx       context.Context
	cnl       context.CancelCauseFunc
	rsps      chan *rsp
}

func NewClientConn(con net.Conn, closeFunc func(error)) *clientConn {
	ctx, cnl := context.WithCancelCause(context.TODO())
	c := &clientConn{
		ctx:       ctx,
		cnl:       cnl,
		conn:      con,
		closeFunc: closeFunc,
		rsps:      make(chan *rsp),
		r:         bufio.NewReader(con),
		w:         bufio.NewWriter(con),
	}
	go c.read()
	return c
}
func (obj *clientConn) read() {
	var err error
	var res *http.Response
	defer obj.CloseWithError(err)
	for {
		res, err = http.ReadResponse(obj.r, nil)
		if res == nil && err == nil {
			err = errors.New("response is nil")
		}
		if err != nil {
			select {
			case obj.rsps <- &rsp{res, nil, err}:
			case <-obj.ctx.Done():
				return
			}
			return
		}
		if res.StatusCode == 101 {
			select {
			case obj.rsps <- &rsp{res, obj.ctx, err}:
			case <-obj.ctx.Done():
				return
			}
			<-obj.ctx.Done()
			return
		} else if res == nil || res.Body == nil || res.Body == http.NoBody {
			select {
			case obj.rsps <- &rsp{res, nil, err}:
			case <-obj.ctx.Done():
				return
			}
		} else {
			ctx, cnl := context.WithCancelCause(obj.ctx)
			res.Body = &clientBody{res.Body, cnl}
			select {
			case obj.rsps <- &rsp{res, ctx, err}:
			case <-obj.ctx.Done():
				return
			}
			<-ctx.Done()
		}
		select {
		case <-obj.ctx.Done():
			return
		default:
		}
	}
}

func (obj *clientConn) httpWrite(req *http.Request, rawHeaders http.Header, orderHeaders []interface {
	Key() string
	Val() any
}) (err error) {
	defer func() {
		if err != nil {
			obj.CloseWithError(tools.WrapError(err, "failed to send request body"))
		}
	}()
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	host, err = httpguts.PunycodeHostPort(host)
	if err != nil {
		return
	}
	host = removeZone(host)
	if rawHeaders.Get("Host") == "" {
		rawHeaders.Set("Host", host)
	}
	contentL, chunked := tools.GetContentLength(req)
	if contentL >= 0 {
		rawHeaders.Set("Content-Length", fmt.Sprint(contentL))
	} else if chunked {
		rawHeaders.Set("Transfer-Encoding", "chunked")
	}
	ruri := req.URL.RequestURI()
	if req.Method == "CONNECT" && req.URL.Path == "" {
		if req.URL.Opaque != "" {
			ruri = req.URL.Opaque
		} else {
			ruri = host
		}
	}
	if _, err = obj.w.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, ruri, req.Proto)); err != nil {
		return
	}
	for _, kv := range tools.NewHeadersWithH1(orderHeaders, rawHeaders) {
		if _, err = obj.w.WriteString(fmt.Sprintf("%s: %s\r\n", kv[0], kv[1])); err != nil {
			return
		}
	}
	if _, err = obj.w.WriteString("\r\n"); err != nil {
		return
	}
	if req.Body == nil {
		err = obj.w.Flush()
		return
	}
	if chunked {
		chunkedWriter := newChunkedWriter(obj.w)
		if _, err = tools.Copy(chunkedWriter, req.Body); err != nil {
			return
		}
		if err = chunkedWriter.Close(); err != nil {
			return
		}
	} else {
		if _, err = tools.Copy(obj.w, req.Body); err != nil {
			return
		}
	}
	err = obj.w.Flush()
	return
}

type clientBody struct {
	r   io.Reader
	cnl context.CancelCauseFunc
}

func (obj *clientBody) Read(p []byte) (n int, err error) {
	return obj.r.Read(p)
}
func (obj *clientBody) Close() error {
	return obj.CloseWithError(nil)
}
func (obj *clientBody) CloseWithError(err error) error {
	obj.cnl(err)
	return nil
}

func (obj *clientConn) DoRequest(req *http.Request, orderHeaders []interface {
	Key() string
	Val() any
}) (res *http.Response, ctx context.Context, err error) {
	defer func() {
		if err != nil {
			obj.CloseWithError(tools.WrapError(err, "failed to send request"))
		}
	}()
	var writeErr error
	writeDone := make(chan struct{})
	go func() {
		writeErr = obj.httpWrite(req, req.Header.Clone(), orderHeaders)
		close(writeDone)
	}()
	select {
	case <-writeDone:
		if writeErr != nil {
			return nil, nil, writeErr
		}
		select {
		case <-req.Context().Done():
			return nil, nil, req.Context().Err()
		case <-obj.ctx.Done():
			return nil, nil, obj.ctx.Err()
		case rsp := <-obj.rsps:
			return rsp.r, rsp.ctx, rsp.err
		}
	case <-req.Context().Done():
		return nil, nil, req.Context().Err()
	case <-obj.ctx.Done():
		return nil, nil, obj.ctx.Err()
	case rsp := <-obj.rsps:
		return rsp.r, rsp.ctx, rsp.err
	}
}

func (obj *clientConn) Close() error {
	return obj.CloseWithError(nil)
}
func (obj *clientConn) CloseWithError(err error) error {
	if obj.closeFunc != nil {
		obj.closeFunc(err)
	}
	obj.cnl(err)
	return obj.conn.Close()
}

func (obj *clientConn) Stream() io.ReadWriteCloser {
	return &websocketConn{
		cnl: obj.cnl,
		r:   obj.r,
		w:   obj.conn,
	}
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
