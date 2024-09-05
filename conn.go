package requests

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gospider007/net/http2"
	"github.com/gospider007/tools"
)

type connecotr struct {
	deleteCtx context.Context //force close
	deleteCnl context.CancelCauseFunc
	closeCtx  context.Context //safe close
	closeCnl  context.CancelCauseFunc

	bodyCtx context.Context //body close
	bodyCnl context.CancelCauseFunc

	rawConn   net.Conn
	h2RawConn *http2.ClientConn
	proxy     string
	r         *bufio.Reader
	w         *bufio.Writer
	pr        *pipCon
	inPool    bool
}

func (obj *connecotr) withCancel(deleteCtx context.Context, closeCtx context.Context) {
	obj.deleteCtx, obj.deleteCnl = context.WithCancelCause(deleteCtx)
	obj.closeCtx, obj.closeCnl = context.WithCancelCause(closeCtx)
}
func (obj *connecotr) Close() error {
	return obj.CloseWithError(nil)
}
func (obj *connecotr) CloseWithError(err error) error {
	if err == nil {
		err = errors.New("connecotr close")
	} else {
		err = tools.WrapError(err, "connecotr close")
	}
	obj.deleteCnl(err)
	if obj.pr != nil {
		obj.pr.Close(err)
	}
	if obj.h2RawConn != nil {
		obj.h2RawConn.Close()
	}
	return obj.rawConn.Close()
}
func (obj *connecotr) read() (err error) {
	if obj.pr != nil {
		return nil
	}
	var pw *pipCon
	obj.pr, pw = pipe(obj.deleteCtx)
	if _, err = io.Copy(pw, obj.rawConn); err == nil {
		err = io.EOF
	}
	pw.Close(err)
	obj.CloseWithError(err)
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

func (obj *connecotr) h2Closed() bool {
	if obj.h2RawConn == nil {
		return false
	}
	state := obj.h2RawConn.State()
	return state.Closed || state.Closing
}
func (obj *connecotr) wrapBody(task *reqTask) {
	body := new(readWriteCloser)
	obj.bodyCtx, obj.bodyCnl = context.WithCancelCause(task.req.Context())
	body.body = task.res.Body
	body.conn = obj
	task.res.Body = body
}
func (obj *connecotr) http1Req(task *reqTask) {
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
	task.cnl()
}

func (obj *connecotr) http2Req(task *reqTask) {
	if task.res, task.err = obj.h2RawConn.RoundTripWithOrderHeaders(task.req, task.orderHeaders2); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	} else if task.err != nil {
		task.err = tools.WrapError(task.err, "http2 roundTrip error")
	}
	task.cnl()
}
func (obj *connecotr) waitBodyClose() error {
	select {
	case <-obj.bodyCtx.Done(): //wait body close
		if err := context.Cause(obj.bodyCtx); errors.Is(err, ErrgospiderBodyClose) {
			return nil
		} else {
			return err
		}
	case <-obj.deleteCtx.Done(): //force conn close
		return tools.WrapError(context.Cause(obj.deleteCtx), "delete ctx error: ")
	}
}

func (obj *connecotr) taskMain(task *reqTask, waitBody bool) (retry bool) {
	defer func() {
		if retry {
			task.err = nil
			obj.CloseWithError(task.err)
		} else if task.err != nil {
			if task.req.Body == nil {
				retry = true
			} else if body, ok := task.req.Body.(*requestBody); ok && !body.ok {
				retry = true
			}
			obj.CloseWithError(task.err)
		}
	}()
	if obj.h2Closed() {
		return true
	}
	select {
	case <-obj.closeCtx.Done():
		return true
	case <-obj.deleteCtx.Done(): //force conn close
		return true
	default:
	}
	if obj.h2RawConn != nil {
		go obj.http2Req(task)
	} else {
		go obj.http1Req(task)
	}
	select {
	case <-task.ctx.Done():
		if task.err != nil {
			return false
		}
		if task.res == nil {
			task.err = task.ctx.Err()
			if task.err == nil {
				task.err = errors.New("response is nil")
			}
			return false
		}
		if waitBody {
			task.err = obj.waitBodyClose()
		}
		return false
	case <-obj.deleteCtx.Done(): //force conn close
		task.err = tools.WrapError(obj.deleteCtx.Err(), "taskMain delete ctx error: ")
		task.cnl()
		return false
	}
}

type connPool struct {
	deleteCtx context.Context
	deleteCnl context.CancelCauseFunc
	closeCtx  context.Context
	closeCnl  context.CancelCauseFunc
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
	conn.withCancel(obj.deleteCtx, obj.closeCtx)
	defer func() {
		conn.CloseWithError(errors.New("connPool rwMain close"))
		obj.total.Add(-1)
		if obj.total.Load() <= 0 {
			obj.close()
		}
	}()
	if err := conn.waitBodyClose(); err != nil {
		return
	}
	for {
		select {
		case <-conn.closeCtx.Done(): //safe close conn
			return
		case <-conn.deleteCtx.Done(): //force close conn
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
	obj.close()
	obj.deleteCnl(errors.New("connPool forceClose"))
}
func (obj *connPool) close() {
	obj.connPools.del(obj.connKey)
	obj.closeCnl(errors.New("connPool close"))
}
