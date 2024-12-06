package requests

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gospider007/http2"
	"github.com/gospider007/http3"
	"github.com/gospider007/tools"
)

type connecotr struct {
	parentForceCtx context.Context //parent force close
	forceCtx       context.Context //force close
	forceCnl       context.CancelCauseFunc
	safeCtx        context.Context //safe close
	safeCnl        context.CancelCauseFunc

	bodyCtx context.Context //body close
	bodyCnl context.CancelCauseFunc

	rawConn   net.Conn
	h2RawConn *http2.Http2ClientConn
	h3RawConn http3.RoundTripper
	proxys    []*url.URL
	r         *bufio.Reader
	w         *bufio.Writer
	pr        *pipCon
	inPool    bool
}

func (obj *connecotr) withCancel(forceCtx context.Context, safeCtx context.Context) {
	obj.parentForceCtx = forceCtx
	obj.forceCtx, obj.forceCnl = context.WithCancelCause(forceCtx)
	obj.safeCtx, obj.safeCnl = context.WithCancelCause(safeCtx)
}
func (obj *connecotr) Close() error {
	return obj.closeWithError(errors.New("connecotr Close close"))
}
func (obj *connecotr) closeWithError(err error) error {
	if err == nil {
		err = errors.New("connecotr closeWithError close")
	} else {
		err = tools.WrapError(err, "connecotr closeWithError close")
	}
	obj.forceCnl(err)
	if obj.pr != nil {
		obj.pr.Close(err)
	}
	if obj.h2RawConn != nil {
		obj.h2RawConn.Close()
	}
	if obj.h3RawConn != nil {
		return obj.h3RawConn.Close(err.Error())
	}
	return obj.rawConn.Close()
}
func (obj *connecotr) read() (err error) {
	if obj.pr != nil {
		return nil
	}
	var pw *pipCon
	obj.pr, pw = pipe(obj.forceCtx)
	if _, err = io.Copy(pw, obj.rawConn); err == nil {
		err = io.EOF
	}
	pw.Close(err)
	obj.closeWithError(err)
	return
}
func (obj *connecotr) Read(b []byte) (i int, err error) {
	if obj.pr == nil {
		return obj.rawConn.Read(b)
	}
	return obj.pr.Read(b)
}
func (obj *connecotr) Write(b []byte) (int, error) {
	return obj.rawConn.Write(b)
}
func (obj *connecotr) LocalAddr() net.Addr {
	return obj.rawConn.LocalAddr()
}
func (obj *connecotr) RemoteAddr() net.Addr {
	return obj.rawConn.RemoteAddr()
}
func (obj *connecotr) SetDeadline(t time.Time) error {
	return obj.rawConn.SetDeadline(t)
}
func (obj *connecotr) SetReadDeadline(t time.Time) error {
	return obj.rawConn.SetReadDeadline(t)
}
func (obj *connecotr) SetWriteDeadline(t time.Time) error {
	return obj.rawConn.SetWriteDeadline(t)
}

func (obj *connecotr) wrapBody(task *reqTask) {
	body := new(readWriteCloser)
	obj.bodyCtx, obj.bodyCnl = context.WithCancelCause(task.req.Context())
	body.body = task.res.Body
	body.conn = obj
	task.res.Body = body
}
func (obj *connecotr) http1Req(task *reqTask, done chan struct{}) {
	if task.err = httpWrite(task.req, obj.w, task.orderHeaders); task.err == nil {
		task.res, task.err = http.ReadResponse(obj.r, task.req)
		if task.err != nil {
			task.err = tools.WrapError(task.err, "http1 read error")
		} else if task.res == nil {
			task.err = errors.New("response is nil")
		} else {
			obj.wrapBody(task)
		}
	}
	close(done)
}

func (obj *connecotr) http2Req(task *reqTask, done chan struct{}) {
	if task.res, task.err = obj.h2RawConn.DoRequest(task.req, task.orderHeaders2); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	} else if task.err != nil {
		task.err = tools.WrapError(task.err, "http2 roundTrip error")
	}
	close(done)
}
func (obj *connecotr) http3Req(task *reqTask, done chan struct{}) {
	if task.res, task.err = obj.h3RawConn.RoundTrip(task.req); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	} else if task.err != nil {
		task.err = tools.WrapError(task.err, "http2 roundTrip error")
	}
	close(done)
}
func (obj *connecotr) waitBodyClose() error {
	select {
	case <-obj.bodyCtx.Done(): //wait body close
		if err := context.Cause(obj.bodyCtx); errors.Is(err, errGospiderBodyClose) {
			return nil
		} else {
			return err
		}
	case <-obj.forceCtx.Done(): //force conn close
		return tools.WrapError(context.Cause(obj.forceCtx), "connecotr force close")
	}
}

func (obj *connecotr) taskMain(task *reqTask, waitBody bool) (retry bool) {
	defer func() {
		if retry {
			task.err = nil
			obj.closeWithError(errors.New("taskMain retry close"))
		} else {
			task.cnl()
			if task.err != nil {
				obj.closeWithError(task.err)
			} else if waitBody {
				if err := obj.waitBodyClose(); err != nil {
					obj.closeWithError(err)
				}
			}
		}
	}()
	select {
	case <-obj.safeCtx.Done():
		return true
	case <-obj.forceCtx.Done(): //force conn close
		return true
	default:
	}
	done := make(chan struct{})
	if obj.h3RawConn != nil {
		go obj.http3Req(task, done)
	} else if obj.h2RawConn != nil {
		go obj.http2Req(task, done)
	} else {
		go obj.http1Req(task, done)
	}
	select {
	case <-task.ctx.Done():
		task.err = tools.WrapError(context.Cause(task.ctx), "task.ctx error: ")
		return false
	case <-done:
		if task.err != nil {
			return task.suppertRetry()
		}
		if task.res == nil {
			task.err = context.Cause(task.ctx)
			if task.err == nil {
				task.err = errors.New("response is nil")
			}
			return task.suppertRetry()
		}
		if task.ctxData.logger != nil {
			task.ctxData.logger(Log{
				Id:   task.ctxData.requestId,
				Time: time.Now(),
				Type: LogType_ResponseHeader,
				Msg:  "response header",
			})
		}
		return false
	case <-obj.forceCtx.Done(): //force conn close
		err := context.Cause(obj.forceCtx)
		task.err = tools.WrapError(err, "taskMain delete ctx error: ")
		select {
		case <-obj.parentForceCtx.Done():
			return false
		default:
			if errors.Is(err, errConnectionForceClosed) {
				return false
			}
			return true
		}
	}
}

type connPool struct {
	forceCtx context.Context
	forceCnl context.CancelCauseFunc

	safeCtx context.Context
	safeCnl context.CancelCauseFunc

	connKey   string
	total     atomic.Int64
	tasks     chan *reqTask
	connPools *connPools
}

type connPools struct {
	connPools sync.Map
}

func newConnPools() *connPools {
	return new(connPools)
}

func (obj *connPools) get(key string) *connPool {
	val, ok := obj.connPools.Load(key)
	if !ok {
		return nil
	}
	return val.(*connPool)
}

func (obj *connPools) set(key string, pool *connPool) {
	obj.connPools.Store(key, pool)
}

func (obj *connPools) del(key string) {
	obj.connPools.Delete(key)
}

func (obj *connPools) iter(f func(key string, value *connPool) bool) {
	obj.connPools.Range(func(key, value any) bool {
		return f(key.(string), value.(*connPool))
	})
}

func (obj *connPool) notice(task *reqTask) {
	select {
	case obj.tasks <- task:
	case task.emptyPool <- struct{}{}:
	}
}

func (obj *connPool) rwMain(conn *connecotr) {
	conn.withCancel(obj.forceCtx, obj.safeCtx)
	defer func() {
		conn.closeWithError(errors.New("connPool rwMain close"))
		obj.total.Add(-1)
		if obj.total.Load() <= 0 {
			obj.safeClose()
		}
	}()
	if err := conn.waitBodyClose(); err != nil {
		return
	}
	for {
		select {
		case <-conn.safeCtx.Done(): //safe close conn
			return
		case <-conn.forceCtx.Done(): //force close conn
			return
		case task := <-obj.tasks: //recv task
			if task == nil {
				return
			}
			if conn.taskMain(task, true) {
				obj.notice(task)
				return
			}
			if task.err != nil {
				return
			}
		}
	}
}
func (obj *connPool) forceClose() {
	obj.safeClose()
	obj.forceCnl(errors.New("connPool forceClose"))
}

func (obj *connPool) safeClose() {
	obj.connPools.del(obj.connKey)
	obj.safeCnl(errors.New("connPool close"))
}
