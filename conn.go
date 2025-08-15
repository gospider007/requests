package requests

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gospider007/http1"
	"github.com/gospider007/tools"
)

func taskMain(conn http1.Conn, task *reqTask) (err error) {
	defer func() {
		if err != nil && task.reqCtx.option.ErrCallBack != nil {
			task.reqCtx.err = err
			if err2 := task.reqCtx.option.ErrCallBack(task.reqCtx); err2 != nil {
				task.disRetry = true
				err = err2
			}
		}
		if err != nil {
			task.cnl(err)
		} else {
			task.cnl(tools.ErrNoErr)
			if bodyContext := conn.BodyContext(); bodyContext != nil {
				select {
				case <-task.reqCtx.Context().Done():
					if context.Cause(task.reqCtx.Context()) != tools.ErrNoErr {
						err = context.Cause(task.reqCtx.Context())
					}
				case <-bodyContext.Done():
					if context.Cause(bodyContext) != tools.ErrNoErr {
						err = context.Cause(bodyContext)
					}
				}
			}
		}
		if err != nil {
			conn.CloseWithError(tools.WrapError(err, "taskMain close with error"))
		}
	}()
	select {
	case <-conn.Context().Done(): //force conn close
		err = context.Cause(conn.Context())
		return
	default:
	}
	var response *http.Response
	if task.reqCtx.option.ResponseHeaderTimeout > 0 {
		ctx, cnl := context.WithTimeout(task.reqCtx.Context(), task.reqCtx.option.ResponseHeaderTimeout)
		response, err = conn.DoRequest(ctx, task.reqCtx.request, &http1.Option{OrderHeaders: task.reqCtx.option.orderHeaders.Data()})
		cnl()
	} else {
		response, err = conn.DoRequest(task.reqCtx.Context(), task.reqCtx.request, &http1.Option{OrderHeaders: task.reqCtx.option.orderHeaders.Data()})
	}
	if err != nil {
		err = tools.WrapError(err, "roundTrip error")
	} else {
		task.reqCtx.response = response
		task.reqCtx.response.Request = task.reqCtx.request
	}
	if task.reqCtx.option.Logger != nil {
		task.reqCtx.option.Logger(Log{
			Id:   task.reqCtx.requestId,
			Time: time.Now(),
			Type: LogType_ResponseHeader,
			Msg:  "response header",
		})
	}
	return
}

func taskM(conn http1.Conn, task *reqTask) error {
	err := taskMain(conn, task)
	if err != nil {
		return err
	}
	if task.reqCtx.response != nil && task.reqCtx.response.Close {
		return tools.ErrNoErr
	}
	return err
}
func rwMain(conn http1.Conn, task *reqTask, tasks chan *reqTask) (err error) {
	defer func() {
		if err != nil && err != tools.ErrNoErr {
			conn.CloseWithError(tools.WrapError(err, "rwMain close with error"))
		}
	}()
	if err = taskM(conn, task); err != nil {
		return
	}
	for {
		select {
		case <-conn.Context().Done(): //force close conn
			return errors.New("connecotr force close")
		case task := <-tasks: //recv task
			if task == nil {
				return errors.New("task is nil")
			}
			err = taskM(conn, task)
			if err != nil {
				return
			}
		}
	}
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
