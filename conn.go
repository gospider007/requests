package requests

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gospider007/net/http2"
	"github.com/gospider007/tools"
)

type Connecotr struct {
	key       connKey
	err       error
	deleteCtx context.Context //force close
	deleteCnl context.CancelFunc

	closeCtx context.Context //safe close
	closeCnl context.CancelFunc

	bodyCtx context.Context //body close
	bodyCnl context.CancelFunc

	rawConn   net.Conn
	h2        bool
	r         *bufio.Reader
	w         *bufio.Writer
	h2RawConn *http2.ClientConn
	rc        chan []byte
	rn        chan int
	isRead    bool
	isPool    bool
}

func (obj *Connecotr) WithCancel(deleteCtx context.Context, closeCtx context.Context) {
	obj.deleteCtx, obj.deleteCnl = context.WithCancel(deleteCtx)
	obj.closeCtx, obj.closeCnl = context.WithCancel(closeCtx)
}
func (obj *Connecotr) Close() error {
	obj.deleteCnl()
	if obj.h2RawConn != nil {
		obj.h2RawConn.Close()
	}
	return obj.rawConn.Close()
}
func (obj *Connecotr) read() {
	if obj.isRead {
		return
	}
	obj.isRead = true
	defer obj.Close()
	con := make([]byte, 4096)
	var i int
	for {
		i, obj.err = obj.rawConn.Read(con)
		b := con[:i]
		for once := true; once || len(b) > 0; once = false {
			select {
			case obj.rc <- b:
				select {
				case nw := <-obj.rn:
					b = b[nw:]
				case <-obj.deleteCtx.Done():
					return
				}
			case <-obj.deleteCtx.Done():
				return
			}
		}
		if obj.err != nil {
			return
		}
	}
}
func (obj *Connecotr) Read(b []byte) (i int, err error) {
	if !obj.isRead {
		return obj.rawConn.Read(b)
	}
	select {
	case con := <-obj.rc:
		i, err = copy(b, con), obj.err
		select {
		case obj.rn <- i:
			if i < len(con) {
				err = nil
			}
		case <-obj.deleteCtx.Done():
			if err = obj.err; err == nil {
				err = tools.WrapError(obj.deleteCtx.Err(), "connecotr close")
			}
		}
	case <-obj.deleteCtx.Done():
		if err = obj.err; err == nil {
			err = tools.WrapError(obj.deleteCtx.Err(), "connecotr close")
		}
	}
	return
}
func (obj *Connecotr) Write(b []byte) (int, error) {
	return obj.rawConn.Write(b)
}
func (obj *Connecotr) LocalAddr() net.Addr {
	return obj.rawConn.LocalAddr()
}
func (obj *Connecotr) RemoteAddr() net.Addr {
	return obj.rawConn.RemoteAddr()
}
func (obj *Connecotr) SetDeadline(t time.Time) error {
	return obj.rawConn.SetDeadline(t)
}
func (obj *Connecotr) SetReadDeadline(t time.Time) error {
	return obj.rawConn.SetReadDeadline(t)
}
func (obj *Connecotr) SetWriteDeadline(t time.Time) error {
	return obj.rawConn.SetWriteDeadline(t)
}

func (obj *Connecotr) h2Closed() bool {
	state := obj.h2RawConn.State()
	return state.Closed || state.Closing
}
func (obj *Connecotr) wrapBody(task *reqTask) {
	body := new(ReadWriteCloser)
	obj.bodyCtx, obj.bodyCnl = context.WithCancel(obj.deleteCtx)
	body.body = task.res.Body
	body.conn = obj
	task.res.Body = body
}
func (obj *Connecotr) http1Req(task *reqTask) {
	defer task.cnl()
	if task.orderHeaders != nil && len(task.orderHeaders) > 0 {
		task.err = httpWrite(task.req, obj.w, task.orderHeaders)
	} else if task.err = task.req.Write(obj); task.err == nil {
		task.err = obj.w.Flush()
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
}

func (obj *Connecotr) http2Req(task *reqTask) {
	defer task.cnl()
	if task.res, task.err = obj.h2RawConn.RoundTrip(task.req); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	} else if task.err != nil {
		task.err = tools.WrapError(task.err, "http2 roundTrip error")
	}
}

func (obj *Connecotr) taskMain(task *reqTask, afterTime *time.Timer) (*http.Response, error, bool) {
	if obj.h2 && obj.h2Closed() {
		return nil, errors.New("conn is closed"), true
	}
	select {
	case <-obj.closeCtx.Done():
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
			<-obj.bodyCtx.Done() //wait body close
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
	deleteCnl context.CancelFunc
	closeCtx  context.Context
	closeCnl  context.CancelFunc
	key       connKey
	total     atomic.Int64
	tasks     chan *reqTask
	rt        *RoundTripper
}

func (obj *connPool) notice(task *reqTask) {
	select {
	case obj.tasks <- task:
	case task.emptyPool <- struct{}{}:
	}
}

func (obj *connPool) rwMain(conn *Connecotr) {
	conn.WithCancel(obj.deleteCtx, obj.closeCtx)
	var afterTime *time.Timer
	defer func() {
		if afterTime != nil {
			afterTime.Stop()
		}
		obj.rt.connsLock.Lock()
		defer obj.rt.connsLock.Unlock()
		conn.Close()
		obj.total.Add(-1)
		if obj.total.Load() <= 0 {
			obj.Close()
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
func (obj *connPool) ForceClose() {
	obj.deleteCnl()
	delete(obj.rt.connPools, obj.key)
}
func (obj *connPool) Close() {
	obj.closeCnl()
	delete(obj.rt.connPools, obj.key)
}
