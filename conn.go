package requests

import (
	"context"
	"errors"
	"io"
	"iter"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gospider007/tools"
)

var maxRetryCount = 10

type Conn interface {
	CloseWithError(err error) error
	DoRequest(*http.Request, []interface {
		Key() string
		Val() any
	}) (*http.Response, context.Context, error)
	CloseCtx() context.Context
	Stream() io.ReadWriteCloser
}
type connecotr struct {
	forceCtx context.Context //force close
	forceCnl context.CancelCauseFunc
	Conn     Conn
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
	rawBody := task.reqCtx.response.Body
	body.rawBody = rawBody
	body.conn = obj
	task.reqCtx.response.Body = body
}

func (obj *connecotr) httpReq(task *reqTask, done chan struct{}) {
	defer close(done)
	task.reqCtx.response, task.bodyCtx, task.err = obj.Conn.DoRequest(task.reqCtx.request, task.reqCtx.option.orderHeaders.Data())
	if task.reqCtx.response != nil {
		obj.wrapBody(task)
	}
	if task.err != nil {
		task.err = tools.WrapError(task.err, "roundTrip error")
	}
}

func (obj *connecotr) taskMain(task *reqTask) {
	defer func() {
		if task.err != nil && task.reqCtx.option.ErrCallBack != nil {
			task.reqCtx.err = task.err
			if err2 := task.reqCtx.option.ErrCallBack(task.reqCtx); err2 != nil {
				task.isNotice = false
				task.disRetry = true
				task.err = err2
			}
		}
		if task.err != nil {
			if errors.Is(task.err, errLastTaskRuning) {
				task.isNotice = true
			}
			obj.CloseWithError(tools.WrapError(task.err, errors.New("taskMain close with error")))
		}
		task.cnl()
		if task.err == nil && task.reqCtx.response != nil && task.reqCtx.response.Body != nil {
			select {
			case <-obj.forceCtx.Done():
				task.err = context.Cause(obj.forceCtx)
			case <-task.bodyCtx.Done():
				if context.Cause(task.bodyCtx) != context.Canceled {
					task.err = context.Cause(task.bodyCtx)
				}
			}
		}
	}()
	select {
	case <-obj.forceCtx.Done(): //force conn close
		task.err = context.Cause(obj.forceCtx)
		task.enableRetry = true
		task.isNotice = true
		return
	case <-obj.Conn.CloseCtx().Done():
		task.err = context.Cause(obj.Conn.CloseCtx())
		task.enableRetry = true
		task.isNotice = true
		return
	default:
	}
	done := make(chan struct{})
	go obj.httpReq(task, done)
	select {
	case <-task.ctx.Done():
		task.err = tools.WrapError(context.Cause(task.ctx), "task.ctx error: ")
	case <-done:
		if task.reqCtx.response == nil {
			task.err = context.Cause(task.ctx)
			if task.err == nil {
				task.err = errors.New("response is nil")
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
	case <-obj.forceCtx.Done(): //force conn close
		task.err = tools.WrapError(context.Cause(obj.forceCtx), "taskMain delete ctx error: ")
	}
}

type connPool struct {
	forceCtx  context.Context
	forceCnl  context.CancelCauseFunc
	tasks     chan *reqTask
	connPools *connPools
	connKey   string
	total     atomic.Int64
}
type connPools struct {
	connPools sync.Map
}

func newConnPools() *connPools {
	return new(connPools)
}
func (obj *connPools) get(task *reqTask) *connPool {
	val, ok := obj.connPools.Load(task.key)
	if !ok {
		return nil
	}
	return val.(*connPool)
}
func (obj *connPools) set(task *reqTask, pool *connPool) {
	obj.connPools.Store(task.key, pool)
}
func (obj *connPools) del(key string) {
	obj.connPools.Delete(key)
}
func (obj *connPools) Range() iter.Seq2[string, *connPool] {
	return func(yield func(string, *connPool) bool) {
		obj.connPools.Range(func(key, value any) bool {
			return yield(key.(string), value.(*connPool))
		})
	}
}

func (obj *connPool) rwMain(done chan struct{}, conn *connecotr) {
	conn.withCancel(obj.forceCtx)
	defer func() {
		conn.CloseWithError(errors.New("connPool rwMain close"))
		obj.total.Add(-1)
		if obj.total.Load() <= 0 {
			obj.close(errors.New("conn pool close"))
		}
	}()
	close(done)
	for {
		select {
		case <-conn.forceCtx.Done(): //force close conn
			return
		case <-conn.Conn.CloseCtx().Done():
			return
		case task := <-obj.tasks: //recv task
			if task == nil {
				return
			}
			conn.taskMain(task)
			if task.err != nil {
				return
			}
		}
	}
}
func (obj *connPool) close(err error) {
	obj.connPools.del(obj.connKey)
	obj.forceCnl(tools.WrapError(err, errors.New("connPool close")))
}
