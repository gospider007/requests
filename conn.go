package requests

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/gospider007/http1"
	"github.com/gospider007/tools"
)

var maxRetryCount = 5

type connecotr struct {
	forceCtx context.Context //force close
	forceCnl context.CancelCauseFunc
	Conn     http1.Conn
	c        net.Conn
	proxys   []Address
}

func (obj *connecotr) withCancel(forceCtx context.Context) {
	obj.forceCtx, obj.forceCnl = context.WithCancelCause(forceCtx)
}
func (obj *connecotr) Close() error {
	return obj.CloseWithError(errors.New("connecotr Close close"))
}
func (obj *connecotr) CloseWithError(err error) error {
	err = obj.Conn.CloseWithError(err)
	if obj.c != nil {
		return obj.c.Close()
	}
	return err
}

func (obj *connecotr) wrapBody(task *reqTask) {
	body := new(wrapBody)
	body.rawBody = task.reqCtx.response.Body.(*http1.ClientBody)
	body.conn = obj
	task.reqCtx.response.Body = body
	task.reqCtx.response.Request = task.reqCtx.request
}

func (obj *connecotr) httpReq(task *reqTask, done chan struct{}) (err error) {
	defer close(done)
	response, bodyCtx, derr := obj.Conn.DoRequest(task.reqCtx.request, &http1.Option{OrderHeaders: task.reqCtx.option.orderHeaders.Data()})
	if derr != nil {
		err = tools.WrapError(derr, "roundTrip error")
		return
	}
	task.reqCtx.response = response
	task.bodyCtx = bodyCtx
	obj.wrapBody(task)
	return
}

func (obj *connecotr) taskMain(task *reqTask) (err error) {
	defer func() {
		if err != nil && task.reqCtx.option.ErrCallBack != nil {
			task.reqCtx.err = err
			if err2 := task.reqCtx.option.ErrCallBack(task.reqCtx); err2 != nil {
				task.isNotice = false
				task.disRetry = true
				err = err2
			}
		}
		if err == nil {
			task.cnl(tools.ErrNoErr)
		} else {
			task.cnl(err)
		}
		if err == nil && task.reqCtx.response != nil && task.reqCtx.response.Body != nil && task.bodyCtx != nil {
			select {
			case <-obj.forceCtx.Done():
				err = context.Cause(obj.forceCtx)
			case <-task.reqCtx.Context().Done():
				if context.Cause(task.reqCtx.Context()) != tools.ErrNoErr {
					err = context.Cause(task.reqCtx.Context())
				}
				if err == nil && task.reqCtx.response.StatusCode == 101 {
					select {
					case <-obj.forceCtx.Done():
						err = context.Cause(obj.forceCtx)
					case <-task.bodyCtx.Done():
						if context.Cause(task.bodyCtx) != tools.ErrNoErr {
							err = context.Cause(task.bodyCtx)
						}
					}
				}
			case <-task.bodyCtx.Done():
				if context.Cause(task.bodyCtx) != tools.ErrNoErr {
					err = context.Cause(task.bodyCtx)
				}
			}
		}
		if err != nil {
			obj.CloseWithError(tools.WrapError(err, "taskMain close with error"))
		}
	}()
	select {
	case <-obj.forceCtx.Done(): //force conn close
		err = context.Cause(obj.forceCtx)
		task.enableRetry = true
		task.isNotice = true
		return
	default:
	}
	done := make(chan struct{})
	go func() {
		err = obj.httpReq(task, done)
	}()
	select {
	case <-obj.forceCtx.Done(): //force conn close
		err = tools.WrapError(context.Cause(obj.forceCtx), "taskMain delete ctx error: ")
	case <-time.After(task.reqCtx.option.ResponseHeaderTimeout):
		err = errors.New("ResponseHeaderTimeout error: ")
	case <-task.ctx.Done():
		err = context.Cause(task.ctx)
	case <-done:
		if err == nil && task.reqCtx.response == nil {
			err = context.Cause(task.ctx)
			if err == nil {
				err = errors.New("body done response is nil")
			}
		}
		if task.reqCtx.option.Logger != nil {
			task.reqCtx.option.Logger(Log{
				Id:   task.reqCtx.requestId,
				Time: time.Now(),
				Type: LogType_ResponseHeader,
				Msg:  "response header",
			})
		}
	}
	return
}

func (obj *connecotr) rwMain(ctx context.Context, done chan struct{}, tasks chan *reqTask) (err error) {
	obj.withCancel(ctx)
	defer func() {
		if err != nil && err != tools.ErrNoErr {
			obj.CloseWithError(tools.WrapError(err, "rwMain close with error"))
		}
	}()
	close(done)
	for {
		select {
		case <-obj.forceCtx.Done(): //force close conn
			return errors.New("connecotr force close")
		case task := <-tasks: //recv task
			if task == nil {
				return errors.New("task is nil")
			}
			err = obj.taskMain(task)
			if err != nil {
				return
			}
			if task.reqCtx.response != nil && task.reqCtx.response.StatusCode == 101 {
				return tools.ErrNoErr
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
