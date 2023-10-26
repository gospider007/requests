package requests

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"net/url"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/net/http2"
	"github.com/gospider007/tools"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/exp/slices"
)

type roundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

type reqTask struct {
	ctx          context.Context
	cnl          context.CancelFunc
	req          *http.Request
	res          *http.Response
	emptyPool    chan struct{}
	err          error
	orderHeaders []string
}

func (obj *reqTask) isPool() bool {
	return obj.err == nil && obj.res != nil && obj.res.StatusCode != 101 && obj.res.Header.Get("Content-Type") != "text/event-stream"
}

type connPool struct {
	ctx   context.Context
	cnl   context.CancelFunc
	key   string
	total atomic.Int64
	tasks chan *reqTask
	rt    *RoundTripper
}

func (obj *connPool) Close() {
	obj.cnl()
}
func getAddr(uurl *url.URL) string {
	if uurl == nil {
		return ""
	}
	_, port, _ := net.SplitHostPort(uurl.Host)
	if port == "" {
		if uurl.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		return fmt.Sprintf("%s:%s", uurl.Host, port)
	}
	return uurl.Host
}
func getHost(req *http.Request) string {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	_, port, _ := net.SplitHostPort(host)
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		return fmt.Sprintf("%s:%s", host, port)
	}
	return host
}
func getKey(ctxData *reqCtxData, req *http.Request) string {
	return fmt.Sprintf("%s@%s@%s", ctxData.ja3Spec.String(), getAddr(ctxData.proxy), getAddr(req.URL))
}
func (obj *RoundTripper) newConnPool(key string, conn *Connecotr) *connPool {
	pool := new(connPool)
	pool.ctx, pool.cnl = context.WithCancel(obj.ctx)
	pool.tasks = make(chan *reqTask)
	pool.rt = obj
	pool.key = key
	pool.total.Add(1)
	go pool.rwMain(conn)
	return pool
}
func (obj *RoundTripper) getConnPool(key string) *connPool {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	return obj.connPools[key]
}
func (obj *RoundTripper) putConnPool(key string, conn *Connecotr) {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	conn.isPool = true
	if !conn.h2 {
		go conn.read()
	}
	pool, ok := obj.connPools[key]
	if ok {
		pool.total.Add(1)
		go pool.rwMain(conn)
	} else {
		obj.connPools[key] = obj.newConnPool(key, conn)
	}
}
func (obj *RoundTripper) TlsConfig() *tls.Config {
	return obj.tlsConfig.Clone()
}
func (obj *RoundTripper) UtlsConfig() *utls.Config {
	return obj.utlsConfig.Clone()
}
func (obj *RoundTripper) dial(ctxData *reqCtxData, addr string, key string, req *http.Request) (conn *Connecotr, err error) {
	if !ctxData.disProxy && ctxData.proxy == nil {
		if ctxData.proxy, err = obj.GetProxy(req.Context(), req.URL); err != nil {
			return conn, err
		}
	}
	var netConn net.Conn
	host := getHost(req)
	if ctxData.proxy == nil {
		netConn, err = obj.dialer.DialContext(req.Context(), "tcp", addr)
	} else {
		netConn, err = obj.dialer.DialContextWithProxy(req.Context(), "tcp", req.URL.Scheme, addr, host, ctxData.proxy, obj.TlsConfig())
	}
	if err != nil {
		return conn, err
	}
	conne := new(Connecotr)
	conne.rn = make(chan int)
	conne.rc = make(chan []byte)
	conne.WithCancel(obj.ctx)
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), obj.tlsHandshakeTimeout)
		defer cnl()
		if ctxData.ja3Spec.IsSet() {
			tlsConfig := obj.UtlsConfig()
			if ctxData.forceHttp1 {
				tlsConfig.NextProtos = []string{"http/1.1"}
			}
			tlsConn, err := obj.dialer.AddJa3Tls(ctx, netConn, host, ctxData.isWs || ctxData.forceHttp1, ctxData.ja3Spec, tlsConfig)
			if err != nil {
				return conne, tools.WrapError(err, "add tls error")
			}
			conne.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		} else {
			tlsConn, err := obj.dialer.AddTls(ctx, netConn, host, ctxData.isWs || ctxData.forceHttp1, obj.TlsConfig())
			if err != nil {
				return conne, tools.WrapError(err, "add tls error")
			}
			conne.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		}
	}
	conne.rawConn = netConn
	if conne.h2 {
		if conne.h2RawConn, err = http2.NewClientConn(func() {
			conne.closeCnl()
		}, netConn, ctxData.h2Ja3Spec); err != nil {
			return conne, err
		}
	} else {
		conne.r = bufio.NewReader(conne)
	}
	return conne, err
}

type Connecotr struct {
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
	h2RawConn *http2.ClientConn
	rc        chan []byte
	rn        chan int
	isRead    bool
	isPool    bool
}

func (obj *Connecotr) WithCancel(ctx context.Context) {
	obj.deleteCtx, obj.deleteCnl = context.WithCancel(ctx)
	obj.closeCtx, obj.closeCnl = context.WithCancel(obj.deleteCtx)
}
func (obj *Connecotr) Close() error {
	obj.deleteCnl()
	if obj.h2RawConn != nil {
		obj.h2RawConn.Close()
	}
	return obj.rawConn.Close()
}
func (obj *Connecotr) read() {
	obj.isRead = true
	defer obj.Close()
	con := make([]byte, 4096)
	var i int
	for {
		if i, obj.err = obj.rawConn.Read(con); obj.err != nil && i == 0 {
			return
		}
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

type ReadWriteCloser struct {
	body io.ReadCloser
	conn *Connecotr
}

func (obj *ReadWriteCloser) Conn() net.Conn {
	return obj.conn
}
func (obj *ReadWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *ReadWriteCloser) Close() (err error) {
	err = obj.body.Close()
	if !obj.conn.isPool {
		obj.ForceDelete()
	} else {
		obj.conn.bodyCnl()
	}
	return
}

// safe close conn
func (obj *ReadWriteCloser) Delete() {
	obj.conn.closeCnl()
}

// force close conn
func (obj *ReadWriteCloser) ForceDelete() {
	obj.conn.Close()
}

func (obj *Connecotr) wrapBody(task *reqTask) {
	body := new(ReadWriteCloser)
	obj.bodyCtx, obj.bodyCnl = context.WithCancel(obj.deleteCtx)
	body.body = task.res.Body
	body.conn = obj
	task.res.Body = body
}

type orderHeadersConn struct {
	w            io.Writer
	raw          []byte
	orderHeaders []string
}

func (obj *orderHeadersConn) Write(p []byte) (n int, err error) {
	if obj.raw == nil {
		return obj.w.Write(p)
	}
	obj.raw = append(obj.raw, p...)
	if lastIndex := bytes.Index(obj.raw, []byte{'\r', '\n', '\r', '\n'}); lastIndex != -1 {
		if firstIndex := bytes.Index(obj.raw, []byte{'\r', '\n'}) + 2; firstIndex < lastIndex {
			kvs := bytes.Split(obj.raw[firstIndex:lastIndex], []byte{'\r', '\n'})
			sort.Slice(kvs, func(i, j int) bool {
				iIndex := bytes.Index(kvs[i], []byte{':'})
				jIndex := bytes.Index(kvs[j], []byte{':'})
				if iIndex == -1 || jIndex == -1 {
					return false
				}
				return slices.Index(obj.orderHeaders, tools.BytesToString(kvs[i][:iIndex])) > slices.Index(obj.orderHeaders, tools.BytesToString(kvs[j][:jIndex]))
			})
			copy(obj.raw[firstIndex:lastIndex], bytes.Join(kvs, []byte{'\r', '\n'}))
		}
		_, err = obj.w.Write(obj.raw)
		obj.raw = nil
	}
	return len(p), err
}

func (obj *Connecotr) http1Req(task *reqTask) {
	defer task.cnl()
	if task.orderHeaders != nil && len(task.orderHeaders) > 0 {
		orderHeaders := make([]string, len(task.orderHeaders))
		total := len(task.orderHeaders) - 1
		for i, val := range task.orderHeaders {
			orderHeaders[total-i] = textproto.CanonicalMIMEHeaderKey(val)
		}
		task.err = task.req.Write(&orderHeadersConn{w: obj, raw: []byte{}, orderHeaders: orderHeaders})
	} else {
		task.err = task.req.Write(obj)
	}
	if task.err == nil {
		if task.res, task.err = http.ReadResponse(obj.r, task.req); task.res != nil && task.err == nil {
			obj.wrapBody(task)
		}
	}
}

func (obj *Connecotr) http2Req(task *reqTask) {
	defer task.cnl()
	if task.res, task.err = obj.h2RawConn.RoundTrip(task.req); task.res != nil && task.err == nil {
		obj.wrapBody(task)
	}
}
func (obj *connPool) notice(task *reqTask) {
	select {
	case obj.tasks <- task:
	case task.emptyPool <- struct{}{}:
	}
}
func (obj *Connecotr) taskMain(task *reqTask, afterTime *time.Timer, responseHeaderTimeout time.Duration) (*http.Response, error, bool) {
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
		afterTime = time.NewTimer(responseHeaderTimeout)
	} else {
		afterTime.Reset(responseHeaderTimeout)
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

func (obj *connPool) rwMain(conn *Connecotr) {
	conn.WithCancel(obj.ctx)
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
			delete(obj.rt.connPools, obj.key)
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
			res, err, notice := conn.taskMain(task, afterTime, obj.rt.responseHeaderTimeout)
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

type RoundTripper struct {
	ctx                   context.Context
	cnl                   context.CancelFunc
	connPools             map[string]*connPool
	connsLock             sync.Mutex
	dialer                *DialClient
	tlsConfig             *tls.Config
	utlsConfig            *utls.Config
	getProxy              func(ctx context.Context, url *url.URL) (string, error)
	tlsHandshakeTimeout   time.Duration
	responseHeaderTimeout time.Duration
}

type RoundTripperOption struct {
	TlsHandshakeTimeout   time.Duration
	DialTimeout           time.Duration
	KeepAlive             time.Duration
	ResponseHeaderTimeout time.Duration
	LocalAddr             *net.TCPAddr //network card ip
	AddrType              AddrType     //first ip type
	GetAddrType           func(string) AddrType
	Dns                   net.IP
	GetProxy              func(ctx context.Context, url *url.URL) (string, error)
}

func newRoundTripper(preCtx context.Context, option RoundTripperOption) *RoundTripper {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)

	dialClient := NewDail(ctx, DialOption{
		DialTimeout: option.DialTimeout,
		Dns:         option.Dns,
		KeepAlive:   option.KeepAlive,
		LocalAddr:   option.LocalAddr,
		AddrType:    option.AddrType,
		GetAddrType: option.GetAddrType,
	})
	if option.TlsHandshakeTimeout == 0 {
		option.TlsHandshakeTimeout = time.Second * 15
	}
	if option.ResponseHeaderTimeout == 0 {
		option.ResponseHeaderTimeout = time.Second * 30
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		SessionTicketKey:   [32]byte{},
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	utlsConfig := &utls.Config{
		InsecureSkipVerify:                 true,
		InsecureSkipTimeVerify:             true,
		SessionTicketKey:                   [32]byte{},
		ClientSessionCache:                 utls.NewLRUClientSessionCache(0),
		OmitEmptyPsk:                       true,
		PreferSkipResumptionOnNilExtension: true,
	}
	return &RoundTripper{
		tlsConfig:             tlsConfig,
		utlsConfig:            utlsConfig,
		ctx:                   ctx,
		cnl:                   cnl,
		connPools:             make(map[string]*connPool),
		dialer:                dialClient,
		getProxy:              option.GetProxy,
		tlsHandshakeTimeout:   option.TlsHandshakeTimeout,
		responseHeaderTimeout: option.ResponseHeaderTimeout,
	}
}
func (obj *RoundTripper) SetGetProxy(getProxy func(ctx context.Context, url *url.URL) (string, error)) {
	obj.getProxy = getProxy
}
func (obj *RoundTripper) GetProxy(ctx context.Context, proxyUrl *url.URL) (*url.URL, error) {
	if obj.getProxy == nil {
		return nil, nil
	}
	proxy, err := obj.getProxy(ctx, proxyUrl)
	if err != nil {
		return nil, err
	}
	return gtls.VerifyProxy(proxy)
}

func (obj *RoundTripper) poolRoundTrip(task *reqTask, key string) (bool, error) {
	pool := obj.getConnPool(key)
	if pool == nil {
		return false, nil
	}
	select {
	case <-obj.ctx.Done():
		return false, tools.WrapError(obj.ctx.Err(), "roundTripper close ctx error: ")
	case pool.tasks <- task:
		select {
		case <-task.emptyPool:
		case <-task.ctx.Done():
			if task.err == nil && task.res == nil {
				task.err = tools.WrapError(task.ctx.Err(), "task close ctx error: ")
			}
			return true, nil
		}
	default:
	}
	return false, nil
}

func (obj *RoundTripper) CloseIdleConnections() {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	for key, pool := range obj.connPools {
		pool.Close()
		delete(obj.connPools, key)
	}
}
func (obj *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
	if ctxData.requestCallBack != nil {
		if err := ctxData.requestCallBack(req.Context(), req, nil); err != nil {
			return nil, err
		}
	}
	key := getKey(ctxData, req) //pool key
	task := &reqTask{req: req, emptyPool: make(chan struct{}), orderHeaders: ctxData.orderHeaders}
	task.ctx, task.cnl = context.WithCancel(obj.ctx)
	defer task.cnl()
	//get pool conn
	if !ctxData.disAlive {
		if ok, err := obj.poolRoundTrip(task, key); err != nil {
			return nil, err
		} else if ok { //is conn multi
			if ctxData.requestCallBack != nil {
				if err = ctxData.requestCallBack(task.req.Context(), req, task.res); err != nil {
					task.err = err
				}
			}
			return task.res, task.err
		}
	}
newConn:
	addr := getAddr(req.URL)
	conn, err := obj.dial(ctxData, addr, key, req)
	if err != nil {
		return nil, err
	}
	if _, _, notice := conn.taskMain(task, nil, obj.responseHeaderTimeout); notice {
		goto newConn
	}
	if task.err == nil && task.res == nil {
		task.err = obj.ctx.Err()
	}
	if task.isPool() && !ctxData.disAlive {
		obj.putConnPool(key, conn)
	}
	if ctxData.requestCallBack != nil {
		if err = ctxData.requestCallBack(task.req.Context(), req, task.res); err != nil {
			task.err = err
			conn.Close()
		}
	}
	return task.res, task.err
}
