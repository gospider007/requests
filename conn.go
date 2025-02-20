package requests

import (
	"bufio"
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
	DoRequest(*http.Request, []string) (*http.Response, error)
	CloseCtx() context.Context
}
type conn struct {
	err      error
	r        *bufio.Reader
	w        *bufio.Writer
	conn     net.Conn
	bodyLock sync.Mutex

	bodyRun   atomic.Bool
	closeFunc func(error)
	closeCtx  context.Context
	closeCnl  context.CancelCauseFunc
}

var errIoCopyClosedOk = errors.New("io copy is closed ok")

func newConn(ctx context.Context, con net.Conn, closeFunc func(error)) *conn {
	c := &conn{
		conn:      con,
		closeFunc: closeFunc,
	}
	c.closeCtx, c.closeCnl = context.WithCancelCause(ctx)
	pr, pw := io.Pipe()
	// c.r = bufio.NewReader(pr)
	// c.w = bufio.NewWriter(con)

	c.r = bufio.NewReader(pr)
	c.w = bufio.NewWriter(c)
	go func() {
		stop := context.AfterFunc(ctx, func() {
			pr.CloseWithError(ctx.Err())
			pw.CloseWithError(ctx.Err())
		})
		defer stop()
		_, err := io.Copy(pw, c.conn)
		c.closeCnl(err)
		if c.err == nil {
			if err == nil {
				c.CloseWithError(errIoCopyClosedOk)
			} else {
				c.CloseWithError(err)
			}
		}
		pr.CloseWithError(c.err)
		pw.CloseWithError(c.err)
	}()
	return c
}
func (obj *conn) CloseCtx() context.Context {
	return obj.closeCtx
}
func (obj *conn) Close() error {
	return obj.CloseWithError(nil)
}
func (obj *conn) CloseWithError(err error) error {
	if err == nil {
		obj.err = errors.New("connecotr closeWithError close")
	} else {
		obj.err = tools.WrapError(err, "connecotr closeWithError close")
	}
	if obj.closeFunc != nil {
		obj.closeFunc(obj.err)
	}
	return obj.conn.Close()
}
func (obj *conn) DoRequest(req *http.Request, orderHeaders []string) (*http.Response, error) {
	go func() {
		obj.httpWrite(req, orderHeaders)
	}()
	res, err := http.ReadResponse(obj.r, req)
	if err != nil {
		err = tools.WrapError(err, "http1 read error")
		return nil, err
	}
	if res == nil {
		err = errors.New("response is nil")
	}
	return res, err
}

func (obj *conn) Read(b []byte) (i int, err error) {
	return obj.r.Read(b)
}

func (obj *conn) Write(b []byte) (int, error) {
	return obj.conn.Write(b)
}
func (obj *conn) LocalAddr() net.Addr {
	return obj.conn.LocalAddr()
}
func (obj *conn) RemoteAddr() net.Addr {
	return obj.conn.RemoteAddr()
}
func (obj *conn) SetDeadline(t time.Time) error {
	return obj.conn.SetDeadline(t)
}
func (obj *conn) SetReadDeadline(t time.Time) error {
	return obj.conn.SetReadDeadline(t)
}
func (obj *conn) SetWriteDeadline(t time.Time) error {
	return obj.conn.SetWriteDeadline(t)
}

type connecotr struct {
	parentForceCtx context.Context //parent force close
	forceCtx       context.Context //force close
	forceCnl       context.CancelCauseFunc
	safeCtx        context.Context //safe close
	safeCnl        context.CancelCauseFunc
	bodyCtx        context.Context //body close
	bodyCnl        context.CancelCauseFunc
	Conn           Conn

	c      net.Conn
	proxys []Address
}

func (obj *connecotr) withCancel(forceCtx context.Context, safeCtx context.Context) {
	obj.parentForceCtx = forceCtx
	obj.forceCtx, obj.forceCnl = context.WithCancelCause(forceCtx)
	obj.safeCtx, obj.safeCnl = context.WithCancelCause(safeCtx)
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
	body := new(readWriteCloser)
	obj.bodyCtx, obj.bodyCnl = context.WithCancelCause(task.reqCtx.Context())
	body.body = task.reqCtx.response.Body
	body.conn = obj
	task.reqCtx.response.Body = body
}
func (obj *connecotr) httpReq(task *reqTask, done chan struct{}) {
	defer close(done)
	task.reqCtx.response, task.err = obj.Conn.DoRequest(task.reqCtx.request, task.reqCtx.option.OrderHeaders)
	if task.reqCtx.response != nil {
		obj.wrapBody(task)
	}
	if task.err != nil {
		task.err = tools.WrapError(task.err, "roundTrip error")
	}
}

func (obj *connecotr) taskMain(task *reqTask) (isNotice bool) {
	task.head = make(chan struct{})
	defer func() {
		if task.err != nil && task.reqCtx.option.ErrCallBack != nil {
			task.reqCtx.err = task.err
			if err2 := task.reqCtx.option.ErrCallBack(task.reqCtx); err2 != nil {
				isNotice = false
				task.disRetry = true
				task.err = err2
			}
		}
		if task.err != nil {
			obj.CloseWithError(errors.New("taskMain retry close"))
			if task.reqCtx.response != nil && task.reqCtx.response.Body != nil {
				task.reqCtx.response.Body.Close()
			}
		} else {
			if task.reqCtx.response != nil && task.reqCtx.response.Body != nil {
				task.cnl()
				select {
				case <-obj.bodyCtx.Done(): //wait body close
					if task.err = context.Cause(obj.bodyCtx); !errors.Is(task.err, errGospiderBodyClose) {
						task.err = tools.WrapError(task.err, "bodyCtx  close")
					}
				case <-task.reqCtx.Context().Done(): //wait request close
					task.err = tools.WrapError(context.Cause(task.reqCtx.Context()), "requestCtx close")
				case <-obj.forceCtx.Done(): //force conn close
					task.err = tools.WrapError(context.Cause(obj.forceCtx), "connecotr force close")
				}
				if task.reqCtx.option.Logger != nil {
					task.reqCtx.option.Logger(Log{
						Id:   task.reqCtx.requestId,
						Time: time.Now(),
						Type: LogType_ResponseBody,
						Msg:  "response body",
					})
				}
			}
			if task.err != nil {
				obj.CloseWithError(task.err)
				if task.reqCtx.response != nil && task.reqCtx.response.Body != nil {
					task.reqCtx.response.Body.Close()
				}
			}
		}
	}()
	select {
	case <-obj.safeCtx.Done():
		task.err = obj.safeCtx.Err()
		task.enableRetry = true
		isNotice = true
		return
	case <-obj.forceCtx.Done(): //force conn close
		task.err = obj.forceCtx.Err()
		task.enableRetry = true
		isNotice = true
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
	return false
}

type connPool struct {
	forceCtx  context.Context
	safeCtx   context.Context
	forceCnl  context.CancelCauseFunc
	safeCnl   context.CancelCauseFunc
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

func (obj *connPool) notices(task *reqTask) bool {
	select {
	case obj.tasks <- task:
		return true
	default:
		task.isNotice = true
		return false
	}
}
func (obj *connPool) rwMain(done chan struct{}, conn *connecotr) {
	conn.withCancel(obj.forceCtx, obj.safeCtx)
	defer func() {
		conn.CloseWithError(errors.New("connPool rwMain close"))
		obj.total.Add(-1)
		if obj.total.Load() <= 0 {
			obj.safeClose()
		}
	}()
	close(done)
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
			task.isNotice = false
			task.disRetry = false
			task.enableRetry = false
			task.err = nil
			if !conn.taskMain(task) || !obj.notices(task) {
				task.cnl()
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
