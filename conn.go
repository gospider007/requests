package requests

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gospider007/http1"
	"github.com/gospider007/tools"
)

func (obj *Response) doRequest(conn http1.Conn) (err error) {
	var response *http.Response
	if obj.option.ResponseHeaderTimeout > 0 {
		ctx, cnl := context.WithTimeout(obj.Context(), obj.option.ResponseHeaderTimeout)
		response, err = conn.DoRequest(ctx, obj.request, &http1.Option{OrderHeaders: obj.option.orderHeaders.Data()})
		cnl()
	} else {
		response, err = conn.DoRequest(obj.Context(), obj.request, &http1.Option{OrderHeaders: obj.option.orderHeaders.Data()})
	}
	if err != nil {
		err = tools.WrapError(err, "roundTrip error")
	} else {
		obj.response = response
		obj.response.Request = obj.request
	}
	if obj.option.Logger != nil {
		obj.option.Logger(Log{
			Id:   obj.requestId,
			Time: time.Now(),
			Type: LogType_ResponseHeader,
			Msg:  "response header",
		})
	}
	return
}

func newSSHConn(sshCon net.Conn, rawCon net.Conn) *sshConn {
	return &sshConn{sshCon: sshCon, rawCon: rawCon}
}

type sshConn struct {
	sshCon net.Conn
	rawCon net.Conn
}

func (obj *sshConn) Read(b []byte) (n int, err error) {
	return obj.sshCon.Read(b)
}

func (obj *sshConn) Write(b []byte) (n int, err error) {
	return obj.sshCon.Write(b)
}

func (obj *sshConn) Close() error {
	return obj.sshCon.Close()
}
func (obj *sshConn) LocalAddr() net.Addr {
	return obj.sshCon.LocalAddr()
}
func (obj *sshConn) RemoteAddr() net.Addr {
	return obj.sshCon.RemoteAddr()
}
func (obj *sshConn) SetDeadline(deadline time.Time) error {
	return obj.rawCon.SetDeadline(deadline)
}
func (obj *sshConn) SetReadDeadline(deadline time.Time) error {
	return obj.rawCon.SetReadDeadline(deadline)
}

func (obj *sshConn) SetWriteDeadline(deadline time.Time) error {
	return obj.rawCon.SetWriteDeadline(deadline)
}
