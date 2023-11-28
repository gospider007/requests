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
	key       connKey
	deleteCtx context.Context //force close
	deleteCnl context.CancelCauseFunc

	closeCtx context.Context //safe close
	closeCnl context.CancelCauseFunc

	bodyCtx context.Context //body close
	bodyCnl context.CancelCauseFunc

	rawConn   net.Conn
	h2        bool
	r         *bufio.Reader
	w         *bufio.Writer
	h2RawConn *http2.ClientConn
	pr        *pipCon
	isPool    bool
}

func (obj *connecotr) withCancel(deleteCtx context.Context, closeCtx context.Context) {
	obj.deleteCtx, obj.deleteCnl = context.WithCancelCause(deleteCtx)
	obj.closeCtx, obj.closeCnl = context.WithCancelCause(closeCtx)
}
func (obj *connecotr) Close() error {
	obj.deleteCnl(errors.New("connecotr close"))
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
	defer func() {
		pw.cnl(err)
		obj.pr.cnl(err)
		obj.Close()
	}()
	if _, err = io.Copy(pw, obj.rawConn); err == nil {
		err = io.EOF
	}
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
	defer task.cnl()
	if task.debug {
		debugPrint(task.requestId, "http1 req start")
	}
	if task.orderHeaders != nil && len(task.orderHeaders) > 0 {
		task.err = httpWrite(task.req, obj.w, task.orderHeaders)
	} else if task.err = task.req.Write(obj); task.err == nil {
		task.err = obj.w.Flush()
	}
	if task.debug {
		debugPrint(task.requestId, "http1 req write ok ,err: ", task.err)
	}
	if task.err == nil {
		if task.res, task.err = http.ReadResponse(obj.r, task.req); task.res != nil && task.err == nil {
			obj.wrapBody(task)
		} else if task.err != nil {
			task.err = tools.WrapError(task.err, "http1 read error")
		}
	} else {
		task.err = tools.WrapError(task.err, "http1 write error")
	}
	if task.debug {
		debugPrint(task.requestId, "http1 req ok ,err: ", task.err)
	}
}

func (obj *connecotr) http2Req(task *reqTask) {
	defer task.cnl()
	if task.debug {
		debugPrint(task.requestId, "http2 req start")
	}
	if task.res, task.err = obj.h2RawConn.RoundTrip(task.req); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	} else if task.err != nil {
		task.err = tools.WrapError(task.err, "http2 roundTrip error")
	}
	if task.debug {
		debugPrint(task.requestId, "http2 req ok,err: ", task.err)
	}
}

func (obj *connecotr) taskMain(task *reqTask, afterTime *time.Timer) (*http.Response, error, bool) {
	if obj.h2 && obj.h2Closed() {
		if task.debug {
			debugPrint(task.requestId, "h2 con is closed")
		}
		return nil, errors.New("conn is closed"), true
	}
	select {
	case <-obj.closeCtx.Done():
		if task.debug {
			debugPrint(task.requestId, "connecotr closeCnl")
		}
		return nil, tools.WrapError(obj.closeCtx.Err(), "close ctx error: "), true
	default:
	}
	if obj.h2 {
		go obj.http2Req(task)
	} else {
		go obj.http1Req(task)
	}
	if afterTime == nil {
		afterTime = time.NewTimer(task.responseHeaderTimeout)
	} else {
		afterTime.Reset(task.responseHeaderTimeout)
	}
	if !obj.isPool {
		defer afterTime.Stop()
	}
	select {
	case <-task.ctx.Done():
		if task.res != nil && task.err == nil && obj.isPool {
			select {
			case <-obj.bodyCtx.Done(): //wait body close
				task.err = tools.WrapError(obj.deleteCtx.Err(), "body ctx error: ")
			case <-obj.deleteCtx.Done(): //force conn close
				task.err = tools.WrapError(obj.deleteCtx.Err(), "delete ctx error: ")
			}
		}
	case <-obj.deleteCtx.Done(): //force conn close
		task.err = tools.WrapError(obj.deleteCtx.Err(), "delete ctx error: ")
		task.cnl()
	case <-afterTime.C:
		task.err = errors.New("response Header is Timeout")
		task.cnl()
	}
	return task.res, task.err, false
}

type connPool struct {
	deleteCtx context.Context
	deleteCnl context.CancelCauseFunc
	closeCtx  context.Context
	closeCnl  context.CancelCauseFunc
	key       connKey
	total     atomic.Int64
	tasks     chan *reqTask
	rt        *roundTripper
	lock      sync.Mutex
}

func (obj *connPool) notice(task *reqTask) {
	select {
	case obj.tasks <- task:
	case task.emptyPool <- struct{}{}:
	}
}

func (obj *connPool) rwMain(conn *connecotr) {
	conn.withCancel(obj.deleteCtx, obj.closeCtx)
	var afterTime *time.Timer
	defer func() {
		if afterTime != nil {
			afterTime.Stop()
		}
		conn.Close()
		obj.total.Add(-1)
		if obj.total.Load() <= 0 {
			obj.close()
		}
	}()
	select {
	case <-conn.deleteCtx.Done(): //force close all conn
		return
	case <-conn.bodyCtx.Done(): //wait body close
	}
	for {
		select {
		case <-conn.closeCtx.Done(): //safe close conn
			return
		case task := <-obj.tasks: //recv task
			if task.debug {
				debugPrint(task.requestId, "recv task")
			}
			if task == nil {
				if task.debug {
					debugPrint(task.requestId, "recv task is nil")
				}
				return
			}
			res, err, notice := conn.taskMain(task, afterTime)
			if notice {
				obj.notice(task)
				return
			}
			if res == nil || err != nil {
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
	obj.rt.delConnPool(obj.key)
}
