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
func (obj *readWriteCloser) Proxys() []*url.URL {
	if l := len(obj.conn.proxys); l > 0 {
		proxys := make([]*url.URL, l)
		for i, proxy := range obj.conn.proxys {
			proxys[i] = cloneUrl(proxy)
		}
	}
	return obj.conn.proxys
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
