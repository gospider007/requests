package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"net/http"

	"gitee.com/baixudong/net/http2"
	"gitee.com/baixudong/tools"
	utls "github.com/refraction-networking/utls"
)

type roundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

type reqTask struct {
	ctx       context.Context //控制请求的生命周期
	cnl       context.CancelFunc
	req       *http.Request  //发送的请求
	res       *http.Response //接收的请求
	emptyPool chan struct{}
	err       error
}

func (obj *reqTask) isPool() bool {
	return obj.err == nil && obj.res != nil && obj.res.StatusCode != 101 && obj.res.Header.Get("Content-Type") != "text/event-stream"
}

type connPool struct {
	ctx   context.Context //控制请求的生命周期
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
func (obj *RoundTripper) newConnPool(key string, conn *Connecotr) *connPool { //新建连接池
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
	if !ctxData.disProxy && ctxData.proxy == nil { //确定代理
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
	isWebSocket := req.URL.Scheme == "wss"
	if req.URL.Scheme == "https" || isWebSocket {
		ctx, cnl := context.WithTimeout(req.Context(), obj.tlsHandshakeTimeout)
		defer cnl()
		if ctxData.ja3Spec.IsSet() {
			tlsConn, err := obj.dialer.AddJa3Tls(ctx, netConn, host, isWebSocket, ctxData.ja3Spec, obj.UtlsConfig())
			if err != nil {
				return conne, err
			}
			conne.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		} else {
			tlsConn, err := obj.dialer.AddTls(ctx, netConn, host, isWebSocket, obj.TlsConfig())
			if err != nil {
				return conne, err
			}
			conne.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		}
	}
	conne.rawConn = netConn
	if conne.h2 {
		if conne.h2RawConn, err = http2.NewClientConn(func() {
			conne.deleteCnl()
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
	deleteCtx context.Context //强制关闭，直接关闭所有底层conn
	deleteCnl context.CancelFunc

	closeCtx context.Context //通知关闭，安全的关闭，等待body 完成生命周期
	closeCnl context.CancelFunc

	bodyCtx context.Context    //body的生命周期
	bodyCnl context.CancelFunc //body的生命周期

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
			case <-obj.deleteCtx.Done():
				return
			case obj.rc <- b:
				select {
				case <-obj.deleteCtx.Done():
					return
				case nw := <-obj.rn:
					b = b[nw:]
				}
			}
		}
	}
}
func (obj *Connecotr) Read(b []byte) (i int, err error) {
	if !obj.isRead {
		return obj.rawConn.Read(b)
	}
	select {
	case <-obj.deleteCtx.Done():
		if err = obj.err; err == nil {
			err = tools.WrapError(obj.deleteCtx.Err(), "connecotr close")
		}
	case con := <-obj.rc:
		i, err = copy(b, con), obj.err
		select {
		case <-obj.deleteCtx.Done():
			if err = obj.err; err == nil {
				err = tools.WrapError(obj.deleteCtx.Err(), "connecotr close")
			}
		case obj.rn <- i:
			if i < len(con) {
				err = nil
			}
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

func (obj *ReadWriteCloser) Delete() { //通知关闭连接，不会影响正在传输中的数据
	obj.conn.closeCnl()
}

func (obj *ReadWriteCloser) ForceDelete() { //强制关闭连接，立刻马上
	obj.conn.Close()
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
	if task.err = task.req.Write(obj); task.err == nil {
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
	case obj.tasks <- task: //任务给池子里其它连接
	case task.emptyPool <- struct{}{}: //告诉提交任务方，池子没有可用连接
	}
}
func (obj *Connecotr) taskMain(task *reqTask, afterTime *time.Timer, responseHeaderTimeout time.Duration) (*http.Response, error, bool) {
	if obj.h2 && obj.h2Closed() { //连接不可用
		return nil, errors.New("连接不可用"), true
	}
	select {
	case <-obj.closeCtx.Done(): //连接池通知关闭，等待连接被释放掉
		return nil, obj.closeCtx.Err(), true
	default:
	}
	if obj.h2 {
		go obj.http2Req(task)
	} else {
		go obj.http1Req(task)
	}
	//等待任务完成
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
			<-obj.bodyCtx.Done() //等待body close
		}
	case <-obj.deleteCtx.Done(): //强制关闭
		task.err = obj.deleteCtx.Err()
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
	case <-conn.deleteCtx.Done(): //强制关闭所有连接
		return
	case <-conn.bodyCtx.Done(): //等待连接占用被释放
	}
	for {
		select {
		case <-conn.closeCtx.Done(): //连接池通知关闭，等待连接被释放掉
			return
		case task := <-obj.tasks: //接收到任务
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
	LocalAddr             *net.TCPAddr //使用本地网卡
	AddrType              AddrType     //优先使用的地址类型,ipv4,ipv6 ,或自动选项
	GetAddrType           func(string) AddrType
	Dns                   net.IP
	GetProxy              func(ctx context.Context, url *url.URL) (string, error)
}

func NewRoundTripper(preCtx context.Context, option RoundTripperOption) *RoundTripper {
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
	return VerifyProxy(proxy)
}

func (obj *RoundTripper) poolRoundTrip(task *reqTask, key string) (bool, error) {
	pool := obj.getConnPool(key)
	if pool == nil {
		return false, nil
	}
	select {
	case <-obj.ctx.Done():
		return false, obj.ctx.Err()
	case pool.tasks <- task:
		select {
		case <-task.emptyPool:
		case <-task.ctx.Done():
			if task.err == nil && task.res == nil {
				task.err = obj.ctx.Err()
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
		if err := ctxData.requestCallBack(req.Context(), req); err != nil {
			return nil, err
		}
	}
	key := getKey(ctxData, req) //pool key
	task := &reqTask{req: req, emptyPool: make(chan struct{})}
	task.ctx, task.cnl = context.WithCancel(obj.ctx)
	defer task.cnl()
	//get pool conn
	if ok, err := obj.poolRoundTrip(task, key); err != nil {
		return nil, err
	} else if ok { //is conn multi
		if ctxData.responseCallBack != nil {
			if err = ctxData.responseCallBack(task.req.Context(), req, task.res); err != nil {
				task.err = err
			}
		}
		return task.res, task.err
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
	if task.isPool() {
		obj.putConnPool(key, conn)
	}
	if ctxData.responseCallBack != nil {
		if err = ctxData.responseCallBack(task.req.Context(), req, task.res); err != nil {
			task.err = err
			conn.Close()
		}
	}
	return task.res, task.err
}
