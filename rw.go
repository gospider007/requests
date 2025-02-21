package requests

import (
	"context"
	"errors"
	"io"
	"sync/atomic"

	"github.com/gospider007/tools"
)

type readWriteCloser struct {
	body     io.ReadCloser
	err      error
	conn     *connecotr
	isClosed atomic.Bool
}

func (obj *readWriteCloser) connStream() io.ReadWriteCloser {
	return obj.conn.Conn.Stream()
}
func (obj *readWriteCloser) Read(p []byte) (n int, err error) {
	if obj.isClosed.Load() {
		return 0, obj.err
	}
	i, err := obj.body.Read(p)
	if err != nil {
		obj.err = err
		if err == io.EOF {
			obj.Close()
		}
	}
	return i, err
}
func (obj *readWriteCloser) Proxys() []Address {
	return obj.conn.proxys
}

func (obj *readWriteCloser) Close() (err error) {
	return obj.CloseWithError(nil)
}
func (obj *readWriteCloser) ConnCloseCtx() context.Context {
	return obj.conn.Conn.CloseCtx()
}
func (obj *readWriteCloser) CloseWithError(err error) error {
	if err == nil {
		obj.err = io.EOF
	} else {
		err = tools.WrapError(obj.err, err)
		obj.err = err
	}
	obj.isClosed.Store(true)
	// obj.conn.bodyCnl(err)
	return obj.body.Close() //reuse conn
}

// safe close conn
func (obj *readWriteCloser) CloseConn() {
	// obj.conn.bodyCnl(errors.New("readWriterCloser close conn"))
	obj.conn.safeCnl(errors.New("readWriterCloser close conn"))
}

// force close conn
func (obj *readWriteCloser) ForceCloseConn() {
	obj.conn.CloseWithError(errConnectionForceClosed)
}
