package requests

import (
	"errors"
	"io"
	"net/http"

	"github.com/gospider007/tools"
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

func (obj *wrapBody) Close() error {
	return obj.CloseWithError(nil)
}

func (obj *wrapBody) CloseWithError(err error) error {
	if err != nil && err != tools.ErrNoErr {
		obj.conn.CloseWithError(err)
	}
	if obj.rawBody == nil || obj.rawBody == http.NoBody {
		return nil
	}
	return obj.rawBody.(interface {
		CloseWithError(error) error
	}).CloseWithError(err)
}

// safe close conn
func (obj *wrapBody) CloseConn() {
	obj.conn.forceCnl(errors.New("readWriterCloser close conn"))
	obj.conn.CloseWithError(errConnectionForceClosed)
}
