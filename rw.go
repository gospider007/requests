package requests

import (
	"errors"
	"io"

	"github.com/gospider007/http1"
	"github.com/gospider007/tools"
)

type wrapBody struct {
	rawBody *http1.ClientBody
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

func (obj *wrapBody) Close() error {
	return obj.CloseWithError(nil)
}

func (obj *wrapBody) CloseWithError(err error) error {
	if err != nil && err != tools.ErrNoErr {
		obj.conn.CloseWithError(err)
	}
	return obj.rawBody.CloseWithError(err)
}

// safe close conn
func (obj *wrapBody) CloseConn() {
	obj.conn.forceCnl(errors.New("readWriterCloser close conn"))
	obj.conn.CloseWithError(errConnectionForceClosed)
}
