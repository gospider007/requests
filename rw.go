package requests

import (
	"io"
	"net"
)

type ReadWriteCloser struct {
	body io.ReadCloser
	conn *Connecotr
}

func (obj *ReadWriteCloser) Conn() net.Conn {
	return obj.conn
}
func (obj *ReadWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *ReadWriteCloser) Close() (err error) {
	err = obj.body.Close()
	if !obj.InPool() {
		obj.ForceDelete()
	} else {
		obj.conn.bodyCnl()
	}
	return
}
func (obj *ReadWriteCloser) InPool() bool {
	return obj.conn.isPool
}
func (obj *ReadWriteCloser) Proxy() string {
	return obj.conn.key.proxy
}
func (obj *ReadWriteCloser) Ja3() string {
	return obj.conn.key.ja3
}
func (obj *ReadWriteCloser) H2Ja3() string {
	return obj.conn.key.h2Ja3
}

// safe close conn
func (obj *ReadWriteCloser) Delete() {
	obj.conn.closeCnl()
}

// force close conn
func (obj *ReadWriteCloser) ForceDelete() {
	obj.conn.Close()
}
