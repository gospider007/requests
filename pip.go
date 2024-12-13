package requests

import (
	"context"
	"sync"
)

type pipCon struct {
	reader  <-chan []byte
	writer  chan<- []byte
	readerI <-chan int
	writerI chan<- int
	lock    sync.Mutex
	ctx     context.Context
	cnl     context.CancelCauseFunc
}

func (obj *pipCon) Read(b []byte) (n int, err error) {
	select {
	case con := <-obj.reader:
		n = copy(b, con)
		select {
		case obj.writerI <- n:
			return
		case <-obj.ctx.Done():
			return n, context.Cause(obj.ctx)
		}
	case <-obj.ctx.Done():
		return n, context.Cause(obj.ctx)
	}
}
func (obj *pipCon) Write(b []byte) (n int, err error) {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	for once := true; once || len(b) > 0; once = false {
		select {
		case obj.writer <- b:
			select {
			case i := <-obj.readerI:
				b = b[i:]
				n += i
			case <-obj.ctx.Done():
				return n, context.Cause(obj.ctx)
			}
		case <-obj.ctx.Done():
			return n, context.Cause(obj.ctx)
		}
	}
	return
}
func (obj *pipCon) CloseWitError(err error) error {
	obj.cnl(err)
	return nil
}
func (obj *pipCon) Close() error {
	return obj.CloseWitError(nil)
}

func pipe(preCtx context.Context) (*pipCon, *pipCon) {
	ctx, cnl := context.WithCancelCause(preCtx)
	readerCha := make(chan []byte)
	writerCha := make(chan []byte)

	readerI := make(chan int)
	writerI := make(chan int)
	localpipCon := &pipCon{
		reader:  readerCha,
		readerI: readerI,
		writer:  writerCha,
		writerI: writerI,
		ctx:     ctx,
		cnl:     cnl,
	}
	remotepipCon := &pipCon{
		reader:  writerCha,
		readerI: writerI,
		writer:  readerCha,
		writerI: readerI,
		ctx:     ctx,
		cnl:     cnl,
	}
	return localpipCon, remotepipCon
}
