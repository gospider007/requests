package requests

import (
	"io"
	"net"
)

type readWriteCloser struct {
	body io.ReadCloser
	conn *connecotr
}

func (obj *readWriteCloser) Conn() net.Conn {
	return obj.conn
}
func (obj *readWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *readWriteCloser) Close() (err error) {
	err = obj.body.Close()
	if !obj.InPool() {
		obj.ForceDelete()
	} else {
		obj.conn.bodyCnl()
	}
	return
}
func (obj *readWriteCloser) InPool() bool {
	return obj.conn.isPool
}
func (obj *readWriteCloser) Proxy() string {
	return obj.conn.key.proxy
}
func (obj *readWriteCloser) Ja3() string {
	return obj.conn.key.ja3
}
func (obj *readWriteCloser) H2Ja3() string {
	return obj.conn.key.h2Ja3
}

// safe close conn
func (obj *readWriteCloser) Delete() {
	obj.conn.closeCnl()
}

// force close conn
func (obj *readWriteCloser) ForceDelete() {
	obj.conn.Close()
}
