package requests

import (
	"bufio"
	"context"
	"crypto/tls"
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
	return fmt.Sprintf("%s@%s", getAddr(ctxData.proxy), getAddr(req.URL))
}
func (obj *RoundTripper) newConnPool(key string, conn *Connecotr) *connPool {
	pool := new(connPool)
	pool.ctx, pool.cnl = context.WithCancel(obj.ctx)
	pool.total.Add(1)
	pool.tasks = make(chan *reqTask)
	pool.rt = obj
	pool.key = key
	go pool.rwMain(conn)
	return pool
}
func (obj *RoundTripper) delConnPool(key string, pool *connPool) {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	pool.Close()
	delete(obj.connPools, key)
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
func (obj *RoundTripper) dial(ctxData *reqCtxData, key string, req *http.Request) (conn *Connecotr, err error) {
	if !ctxData.disProxy && ctxData.proxy == nil { //确定代理
		if ctxData.proxy, err = obj.GetProxy(req.Context(), req.URL); err != nil {
			return nil, err
		}
	}
	var netConn net.Conn
	host := getHost(req)
	addr := getAddr(req.URL)
	if ctxData.proxy == nil {
		netConn, err = obj.dialer.DialContext(req.Context(), "tcp", addr)
	} else {
		netConn, err = obj.dialer.DialContextWithProxy(req.Context(), "tcp", req.URL.Scheme, addr, host, ctxData.proxy, obj.TlsConfig())
	}
	if err != nil {
		return nil, err
	}
	conne := new(Connecotr)

	conne.rn = make(chan int)
	conne.rc = make(chan []byte)

	conne.ctx, conne.cnl = context.WithCancel(obj.ctx)
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), obj.tlsHandshakeTimeout)
		defer cnl()
		if ctxData.ja3Spec.IsSet() {
			tlsConn, err := obj.dialer.AddJa3Tls(ctx, netConn, host, ctxData.ws, ctxData.ja3Spec, obj.UtlsConfig())
			if err != nil {
				return conne, err
			}
			conne.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		} else {
			tlsConn, err := obj.dialer.AddTls(ctx, netConn, host, ctxData.ws, obj.TlsConfig())
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
			conne.cnl()
		}, netConn, ctxData.h2Ja3Spec); err != nil {
			return conne, err
		}
	} else {
		conne.r = bufio.NewReader(conne)
	}
	return conne, err
}

type Connecotr struct {
	ctx       context.Context
	ctx2      context.Context
	cnl       context.CancelFunc
	rawConn   net.Conn
	h2        bool
	r         *bufio.Reader
	h2RawConn *http2.ClientConn
	rc        chan []byte
	rn        chan int
	isRead    bool
	isPool    bool
}

func (obj *Connecotr) Close() error {
	obj.cnl()
	if obj.h2RawConn != nil {
		obj.h2RawConn.Close()
	}
	return obj.rawConn.Close()
}
func (obj *Connecotr) read() {
	defer obj.Close()
	obj.isRead = true
	con := make([]byte, 1024)
	for {
		i, err := obj.rawConn.Read(con)
		if err != nil {
			return
		}
		b := con[:i]
		for once := true; once || len(b) > 0; once = false {
			select {
			case obj.rc <- b:
				nw := <-obj.rn
				b = b[nw:]
			case <-obj.ctx.Done():
				return
			}
		}
	}
}
func (obj *Connecotr) Read(b []byte) (int, error) {
	if !obj.isRead {
		return obj.rawConn.Read(b)
	}
	select {
	case con := <-obj.rc:
		i := copy(b, con)
		select {
		case obj.rn <- i:
			return i, nil
		case <-obj.ctx.Done():
			return i, obj.ctx.Err()
		}
	case <-obj.ctx.Done():
		return 0, obj.ctx.Err()
	}
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
	ctx  context.Context
	cnl  context.CancelFunc
	cnl2 context.CancelFunc
	body io.ReadCloser
	conn *Connecotr
}

func (obj *ReadWriteCloser) Conn() net.Conn {
	return obj.conn
}
func (obj *ReadWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *ReadWriteCloser) Close() error {
	if !obj.conn.isPool {
		defer obj.Delete()
	}
	defer obj.cnl()
	return obj.body.Close()
}

func (obj *ReadWriteCloser) Delete() (err error) {
	err = obj.conn.Close()
	obj.cnl2()
	return
}

func wrapBody(conn *Connecotr, task *reqTask) {
	body := new(ReadWriteCloser)
	conn.ctx2, body.cnl = context.WithCancel(conn.ctx)
	body.cnl2 = conn.cnl
	body.body = task.res.Body
	body.conn = conn
	task.res.Body = body
}
func http1Req(conn *Connecotr, task *reqTask) {
	defer task.cnl()
	defer func() {
		if task.res == nil || task.err != nil {
			conn.Close()
		}
	}()
	err := task.req.Write(conn)
	if err != nil {
		task.err = err
	} else {
		task.res, task.err = http.ReadResponse(conn.r, task.req)
		if task.res != nil {
			wrapBody(conn, task)
		}
	}
}

func http2Req(conn *Connecotr, task *reqTask) {
	defer task.cnl()
	defer func() {
		if task.res == nil || task.err != nil {
			conn.Close()
		}
	}()
	task.res, task.err = conn.h2RawConn.RoundTrip(task.req)
	if task.res != nil {
		wrapBody(conn, task)
	}
}

func (obj *connPool) rwMain(conn *Connecotr) {
	conn.ctx, conn.cnl = context.WithCancel(obj.ctx)
	defer func() {
		if obj.total.Load() == 0 {
			obj.rt.delConnPool(obj.key, obj)
		}
	}()
	defer obj.total.Add(-1)
	defer conn.Close()
	defer conn.cnl()
	for {
		select {
		case <-conn.ctx.Done(): //连接池通知关闭，等待连接被释放掉
			<-conn.ctx2.Done()
			return
		case task := <-obj.tasks: //接收到任务
			if conn.h2 {
				if conn.h2Closed() { //判断连接是否异常
					select {
					case obj.tasks <- task: //任务给池子里其它连接
					case task.emptyPool <- struct{}{}: //告诉提交任务方，池子没有可用连接
					}
					return //由于连接异常直接结束
				}
				go http2Req(conn, task)
			} else {
				select {
				case <-conn.ctx2.Done(): //http1.1 连接被占用
				default:
					select {
					case obj.tasks <- task: //任务给池子里其它连接
					case task.emptyPool <- struct{}{}: //告诉提交任务方，池子没有可用连接
					}
					continue //由于连接被占用，开始下一个循环
				}
				go http1Req(conn, task)
			}
			//等待任务完成
			<-task.ctx.Done()
			//如果没有response返回，就认定这个连接异常，直接返回
			if task.req == nil || task.err != nil {
				return
			}
		}
	}
}

type RoundTripper struct {
	ctx                 context.Context
	cnl                 context.CancelFunc
	connPools           map[string]*connPool
	connsLock           sync.Mutex
	dialer              *DialClient
	tlsConfig           *tls.Config
	utlsConfig          *utls.Config
	getProxy            func(ctx context.Context, url *url.URL) (string, error)
	tlsHandshakeTimeout time.Duration
}
type RoundTripperOption struct {
	TlsHandshakeTimeout time.Duration
	DialTimeout         time.Duration
	KeepAlive           time.Duration
	LocalAddr           *net.TCPAddr //使用本地网卡
	AddrType            AddrType     //优先使用的地址类型,ipv4,ipv6 ,或自动选项
	GetAddrType         func(string) AddrType
	Dns                 net.IP
	GetProxy            func(ctx context.Context, url *url.URL) (string, error)
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
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		SessionTicketKey:   [32]byte{},
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	utlsConfig := &utls.Config{
		InsecureSkipVerify:     true,
		InsecureSkipTimeVerify: true,
		SessionTicketKey:       [32]byte{},
		// ClientSessionCache:     utls.NewLRUClientSessionCache(0),
	}
	return &RoundTripper{
		tlsConfig:           tlsConfig,
		utlsConfig:          utlsConfig,
		ctx:                 ctx,
		cnl:                 cnl,
		connPools:           make(map[string]*connPool),
		dialer:              dialClient,
		getProxy:            option.GetProxy,
		tlsHandshakeTimeout: option.TlsHandshakeTimeout,
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
	return tools.VerifyProxy(proxy)
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
	for key, pool := range obj.connPools {
		obj.delConnPool(key, pool)
	}
}
func (obj *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
	if ctxData.requestCallBack != nil {
		if err := ctxData.requestCallBack(req.Context(), req); err != nil {
			return nil, err
		}
	}
	key := getKey(ctxData, req)
	task := &reqTask{req: req, emptyPool: make(chan struct{})}
	task.ctx, task.cnl = context.WithCancel(obj.ctx)
	defer task.cnl()
	if !ctxData.disAlive {
		ok, err := obj.poolRoundTrip(task, key)
		if err != nil {
			return nil, err
		}
		if ok {
			if ctxData.responseCallBack != nil {
				if err = ctxData.responseCallBack(task.req.Context(), req, task.res); err != nil {
					task.err = err
				}
			}
			return task.res, task.err
		}
	}
	conn, err := obj.dial(ctxData, key, req)
	if err != nil {
		return nil, err
	}
	if !conn.h2 {
		go http1Req(conn, task)
	} else {
		go http2Req(conn, task)
	}
	<-task.ctx.Done()
	if task.err == nil && task.res == nil {
		task.err = obj.ctx.Err()
	}
	if task.err == nil && task.res != nil && task.res.StatusCode != 101 && !ctxData.disAlive {
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
