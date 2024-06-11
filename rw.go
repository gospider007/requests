package requests

import (
	"errors"
	"io"
)

type readWriteCloser struct {
	body io.ReadCloser
	conn *connecotr
}

func (obj *readWriteCloser) Conn() *connecotr {
	return obj.conn
}
func (obj *readWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *readWriteCloser) InPool() bool {
	return obj.conn.inPool
}
func (obj *readWriteCloser) Proxy() string {
	return obj.conn.proxy
}

var ErrgospiderBodyClose = errors.New("gospider body close error")

func (obj *readWriteCloser) Close() (err error) {
	if !obj.InPool() {
		obj.ForceCloseConn()
	} else {
		err = obj.body.Close() //reuse conn
		obj.conn.bodyCnl(ErrgospiderBodyClose)
	}
	return
}

// safe close conn
func (obj *readWriteCloser) CloseConn() {
	if !obj.InPool() {
		obj.ForceCloseConn()
	} else {
		obj.conn.bodyCnl(errors.New("readWriterCloser close conn"))
		obj.conn.closeCnl(errors.New("readWriterCloser close conn"))
	}
}

// force close conn
func (obj *readWriteCloser) ForceCloseConn() {
	obj.conn.Close()
}
