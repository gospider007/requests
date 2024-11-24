package requests

import (
	"errors"
	"io"
	"net/url"
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
func (obj *readWriteCloser) Proxy() *url.URL {
	if obj.conn.proxy == nil {
		return nil
	}
	return cloneUrl(obj.conn.proxy)
}

var ErrgospiderBodyClose = errors.New("gospider body close error")

func (obj *readWriteCloser) Close() (err error) {
	if !obj.InPool() {
		obj.ForceCloseConn()
	} else {
		obj.conn.bodyCnl(ErrgospiderBodyClose)
		err = obj.body.Close() //reuse conn
	}
	return
}

// safe close conn
func (obj *readWriteCloser) CloseConn() {
	if !obj.InPool() {
		obj.ForceCloseConn()
	} else {
		obj.conn.bodyCnl(errors.New("readWriterCloser close conn"))
		obj.conn.safeCnl(errors.New("readWriterCloser close conn"))
	}
}

// force close conn
func (obj *readWriteCloser) ForceCloseConn() {
	obj.conn.closeWithError(errConnectionForceClosed)
}
