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
	connKey   connKey
	deleteCtx context.Context //force close
	deleteCnl context.CancelCauseFunc
	afterTime *time.Timer
	closeCtx  context.Context //safe close
	closeCnl  context.CancelCauseFunc

	bodyCtx context.Context //body close
	bodyCnl context.CancelCauseFunc

	rawConn   net.Conn
	h2RawConn *http2.ClientConn

	r      *bufio.Reader
	w      *bufio.Writer
	pr     *pipCon
	inPool bool
}

func newConnecotr(ctx context.Context, netConn net.Conn) *connecotr {
	conne := new(connecotr)
	conne.withCancel(ctx, ctx)
	conne.rawConn = netConn
	return conne
}
func (obj *connecotr) withCancel(deleteCtx context.Context, closeCtx context.Context) {
	obj.deleteCtx, obj.deleteCnl = context.WithCancelCause(deleteCtx)
	obj.closeCtx, obj.closeCnl = context.WithCancelCause(closeCtx)
}
func (obj *connecotr) Close() error {
	obj.deleteCnl(errors.New("connecotr close"))
	if obj.afterTime != nil {
		obj.afterTime.Stop()
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
	pw.cnl(err)
	obj.pr.cnl(err)
	obj.Close()
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

func (obj *connecotr) h2Closed() bool {
	if obj.h2RawConn == nil {
		return false
	}
	state := obj.h2RawConn.State()
	return state.Closed || state.Closing
}
func (obj *connecotr) wrapBody(task *reqTask) {
	body := new(readWriteCloser)
	obj.bodyCtx, obj.bodyCnl = context.WithCancelCause(obj.deleteCtx)
	body.body = task.res.Body
	body.conn = obj
	task.res.Body = body
}
func (obj *connecotr) http1Req(task *reqTask) {
	task.err = httpWrite(task.req, obj.w, task.orderHeaders)
	// if task.err = task.req.Write(obj); task.err == nil {
	// 	task.err = obj.w.Flush()
	// }
	if task.err == nil {
		if task.res, task.err = http.ReadResponse(obj.r, task.req); task.res != nil && task.err == nil {
			obj.wrapBody(task)
		} else if task.err != nil {
			task.err = tools.WrapError(task.err, "http1 read error")
		}
	} else {
		task.err = tools.WrapError(task.err, "http1 write error")
	}
	task.cnl()
}

func (obj *connecotr) http2Req(task *reqTask) {
	if task.res, task.err = obj.h2RawConn.RoundTrip(task.req); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	} else if task.err != nil {
		task.err = tools.WrapError(task.err, "http2 roundTrip error")
	}
	task.cnl()
}
func (obj *connecotr) waitBodyClose() error {
	select {
	case <-obj.bodyCtx.Done(): //wait body close
		return nil
	case <-obj.deleteCtx.Done(): //force conn close
		return tools.WrapError(context.Cause(obj.deleteCtx), "delete ctx error: ")
	}
}

func (obj *connecotr) taskMain(task *reqTask, waitBody bool) (retry bool) {
	if obj.h2Closed() {
		obj.Close()
		task.err = errors.New("conn is closed")
		return true
	}
	if !waitBody {
		select {
		case <-obj.closeCtx.Done():
			obj.Close()
			task.err = tools.WrapError(obj.closeCtx.Err(), "conn close ctx error: ")
			return true
		default:
		}
	}
	if obj.h2RawConn != nil {
		go obj.http2Req(task)
	} else {
		go obj.http1Req(task)
	}
	if obj.afterTime == nil {
		obj.afterTime = time.NewTimer(task.responseHeaderTimeout)
	} else {
		obj.afterTime.Reset(task.responseHeaderTimeout)
	}
	select {
	case <-task.ctx.Done():
		if task.err != nil {
			obj.Close()
			return false
		}
		if task.res == nil {
			obj.Close()
			task.err = errors.New("response is nil")
			return false
		}
		if waitBody {
			task.err = obj.waitBodyClose()
		}
		return false
	case <-obj.deleteCtx.Done(): //force conn close
		task.cnl()
		task.err = tools.WrapError(obj.deleteCtx.Err(), "delete ctx error: ")
		obj.Close()
		return false
	case <-obj.afterTime.C:
		task.cnl()
		task.err = errors.New("response Header is Timeout")
		obj.Close()
		return false
	}
}

type connPool struct {
	deleteCtx context.Context
	deleteCnl context.CancelCauseFunc
	closeCtx  context.Context
	closeCnl  context.CancelCauseFunc
	connKey   connKey
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
func (obj *connPools) get(key connKey) *connPool {
	val, ok := obj.connPools.Load(key)
	if !ok {
		return nil
	}
	pool := val.(*connPool)
	select {
	case <-pool.closeCtx.Done():
		return nil
	default:
		return pool
	}
}
func (obj *connPools) set(key connKey, pool *connPool) {
	obj.connPools.Store(key, pool)
}
func (obj *connPools) del(key connKey) {
	obj.connPools.Delete(key)
}
func (obj *connPools) iter(f func(key connKey, value *connPool) bool) {
	obj.connPools.Range(func(key, value any) bool {
		return f(key.(connKey), value.(*connPool))
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
		conn.Close()
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
	obj.deleteCnl(errors.New("connPool forceClose"))
	obj.close()
}
func (obj *connPool) close() {
	obj.closeCnl(errors.New("connPool close"))
	obj.connPools.del(obj.connKey)
}
