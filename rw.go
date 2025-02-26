package requests

import (
	"context"
	"errors"
	"io"
)

type readWriteCloser struct {
	body io.ReadCloser
	conn *connecotr
}

func (obj *readWriteCloser) connStream() io.ReadWriteCloser {
	return obj.conn.Conn.Stream()
}
func (obj *readWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *readWriteCloser) Proxys() []Address {
	return obj.conn.proxys
}

func (obj *readWriteCloser) ConnCloseCtx() context.Context {
	return obj.conn.Conn.CloseCtx()
}
func (obj *readWriteCloser) CloseWithError(err error) error {
	if err != nil {
		obj.conn.CloseWithError(err)
	}
	return obj.body.Close() //reuse conn
}
func (obj *readWriteCloser) Close() error {
	return obj.CloseWithError(nil)
}

// safe close conn
func (obj *readWriteCloser) CloseConn() {
	obj.conn.forceCnl(errors.New("readWriterCloser close conn"))
	obj.conn.CloseWithError(errConnectionForceClosed)
}
