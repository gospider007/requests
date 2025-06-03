package requests

import (
	"errors"
	"io"
	"log"
)

type wrapBody struct {
	rawBody io.ReadCloser
	conn    *connecotr
}

func (obj *wrapBody) connStream() io.ReadWriteCloser {
	return obj.conn.Conn.Stream()
}
func (obj *wrapBody) Read(p []byte) (n int, err error) {
	return obj.rawBody.Read(p)
}
func (obj *wrapBody) Proxys() []Address {
	return obj.conn.proxys
}

func (obj *wrapBody) CloseWithError(err error) error {
	if err != nil {
		obj.conn.CloseWithError(err)
	}
	return obj.rawBody.Close() //reuse conn
}
func (obj *wrapBody) Close() error {
	return obj.CloseWithError(nil)
}

// safe close conn
func (obj *wrapBody) CloseConn() {
	log.Print("111")
	obj.conn.forceCnl(errors.New("readWriterCloser close conn"))
	log.Print("222")
	obj.conn.CloseWithError(errConnectionForceClosed)
	log.Print("333")
}
