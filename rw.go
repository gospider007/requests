package requests

import (
	"errors"
	"io"
	"net"
)

type readWriteCloser struct {
	body io.ReadCloser
	conn *connecotr
}

func (obj *readWriteCloser) Conn() net.Conn {
	return obj.conn.Conn.(net.Conn)
}
func (obj *readWriteCloser) Read(p []byte) (n int, err error) {
	i, err := obj.body.Read(p)
	if err == io.EOF {
		obj.Close()
	}
	return i, err
}
func (obj *readWriteCloser) Proxys() []Address {
	return obj.conn.proxys
}

var errGospiderBodyClose = errors.New("gospider body close error")

func (obj *readWriteCloser) Close() (err error) {
	obj.conn.bodyCnl(errGospiderBodyClose)
	return obj.body.Close() //reuse conn
}

// safe close conn
func (obj *readWriteCloser) CloseConn() {
	obj.conn.bodyCnl(errors.New("readWriterCloser close conn"))
	obj.conn.safeCnl(errors.New("readWriterCloser close conn"))
}

// force close conn
func (obj *readWriteCloser) ForceCloseConn() {
	obj.conn.CloseWithError(errConnectionForceClosed)
}
