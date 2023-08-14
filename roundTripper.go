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

	h2ja3 "gitee.com/baixudong/net/http2"
	"gitee.com/baixudong/tools"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
)

type roundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

type connecotr struct {
	ctx          context.Context
	ctx2         context.Context
	cnl          context.CancelFunc
	rawConn      net.Conn
	h2           bool
	r            *bufio.Reader
	h2RawConn    *http2.ClientConn
	h2Ja3RawConn *h2ja3.ClientConn
}

func (obj *connecotr) Close() error {
	obj.cnl()
	if obj.h2RawConn != nil {
		obj.h2RawConn.Close()
	}
	if obj.h2Ja3RawConn != nil {
		obj.h2Ja3RawConn.Close()
	}
	return obj.rawConn.Close()
}

type reqTask struct {
	ctx       context.Context //控制请求的生命周期
	cnl       context.CancelFunc
	req       *http.Request  //发送的请求
	res       *http.Response //接收的请求
	oneConn   bool
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
func (obj *RoundTripper) newConnPool(key string, conn *connecotr) *connPool {
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
func (obj *RoundTripper) putConnPool(key string, conn *connecotr) {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
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
func (obj *RoundTripper) dial(ctxData *reqCtxData, key string, req *http.Request) (conn *connecotr, err error) {
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
	conne := new(connecotr)
	conne.ctx, conne.cnl = context.WithCancel(obj.ctx)
	var h2 bool
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), obj.tlsHandshakeTimeout)
		defer cnl()
		if ctxData.ja3Spec.IsSet() {
			tlsConn, err := obj.dialer.AddJa3Tls(ctx, netConn, host, ctxData.ws, ctxData.ja3Spec, obj.UtlsConfig())
			if err != nil {
				return conne, err
			}
			h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		} else {
			tlsConn, err := obj.dialer.AddTls(ctx, netConn, host, ctxData.ws, obj.TlsConfig())
			if err != nil {
				return conne, err
			}
			h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		}
	}
	conne.rawConn = netConn
	if h2 {
		conne.h2 = h2
		if ctxData.h2Ja3Spec.IsSet() {
			conne.h2Ja3RawConn, err = h2ja3.NewClientConn(netConn, ctxData.h2Ja3Spec)
		} else {
			conne.h2RawConn, err = (&http2.Transport{}).NewClientConn(netConn)
		}
		if err != nil {
			return conne, err
		}
	} else {
		conne.r = bufio.NewReader(netConn)
	}
	return conne, err
}

type ClientConnState struct {
	Closed bool
	// Closing is whether the connection is in the process of
	// closing. It may be closing due to shutdown, being a
	// single-use connection, being marked as DoNotReuse, or
	// having received a GOAWAY frame.
	Closing bool

	// StreamsActive is how many streams are active.
	StreamsActive int

	// StreamsReserved is how many streams have been reserved via
	// ClientConn.ReserveNewRequest.
	StreamsReserved int

	// StreamsPending is how many requests have been sent in excess
	// of the peer's advertised MaxConcurrentStreams setting and
	// are waiting for other streams to complete.
	StreamsPending int

	// MaxConcurrentStreams is how many concurrent streams the
	// peer advertised as acceptable. Zero means no SETTINGS
	// frame has been received yet.
	MaxConcurrentStreams uint32

	// LastIdle, if non-zero, is when the connection last
	// transitioned to idle state.
	LastIdle time.Time
}

func (obj *connecotr) ping() error {
	if obj.h2RawConn != nil {
		state := obj.h2RawConn.State()
		if state.Closed || state.Closing {
			return errors.New("h2 is close")
		}
	} else if obj.h2Ja3RawConn != nil {
		state := obj.h2Ja3RawConn.State()
		if state.Closed || state.Closing {
			return errors.New("h2 is close")
		}
	}
	_, err := obj.rawConn.Write(make([]byte, 0))
	return err
}

type ReadWriteCloser struct {
	ctx  context.Context
	cnl  context.CancelFunc
	cnl2 context.CancelFunc
	body io.ReadCloser
	conn net.Conn
}

func (obj *ReadWriteCloser) Conn() net.Conn {
	return obj.conn
}
func (obj *ReadWriteCloser) Read(p []byte) (n int, err error) {
	return obj.body.Read(p)
}
func (obj *ReadWriteCloser) Close() error {
	defer obj.cnl()
	return obj.body.Close()
}
func (obj *ReadWriteCloser) Delete() {
	obj.cnl2()
}

func wrapBody(conn *connecotr, task *reqTask) {
	body := new(ReadWriteCloser)
	conn.ctx2, body.cnl = context.WithCancel(conn.ctx)
	body.cnl2 = conn.cnl
	body.body = task.res.Body
	if task.res.StatusCode == 101 {
		body.conn = conn.rawConn
		task.oneConn = true
	}
	task.res.Body = body
}
func http1Req(conn *connecotr, task *reqTask) {
	defer task.cnl()
	err := task.req.Write(conn.rawConn)
	if err != nil {
		task.err = err
	} else {
		task.res, task.err = http.ReadResponse(conn.r, task.req)
		if task.res != nil {
			wrapBody(conn, task)
		}
	}
}

func http2Req(conn *connecotr, task *reqTask) {
	defer task.cnl()
	if conn.h2RawConn != nil {
		task.res, task.err = conn.h2RawConn.RoundTrip(task.req)
	} else {
		task.res, task.err = conn.h2Ja3RawConn.RoundTrip(task.req)
	}
	if task.res != nil {
		wrapBody(conn, task)
	}
}

func (obj *connPool) rwMain(conn *connecotr) {
	conn.ctx, conn.cnl = context.WithCancel(obj.ctx)
	defer func() {
		if obj.total.Load() == 0 {
			obj.rt.delConnPool(obj.key, obj)
		}
	}()
	defer obj.total.Add(-1)
	defer conn.Close()
	wait := time.NewTimer(0)
	defer wait.Stop()
	defer conn.cnl()
	go func() {
		defer conn.cnl()
		for {
			wait.Reset(time.Second * 30)
			select {
			case <-conn.ctx.Done():
				return
			case <-wait.C:
				if conn.ping() != nil {
					return
				}
			}
		}
	}()
	for {
		select {
		case <-conn.ctx.Done():
			return
		case task := <-obj.tasks:
			if conn.ping() != nil {
				select {
				case obj.tasks <- task:
				case task.emptyPool <- struct{}{}:
				}
				return
			}
			if !conn.h2 {
				select {
				case <-conn.ctx.Done():
					return
				case <-conn.ctx2.Done():
				default:
					select {
					case obj.tasks <- task:
					case task.emptyPool <- struct{}{}:
					}
					return
				}
			}
			wait.Reset(time.Hour * 24 * 365)
			if conn.h2 {
				go http2Req(conn, task)
			} else {
				go http1Req(conn, task)
			}
			<-task.ctx.Done()
			if task.oneConn || task.req == nil {
				return
			}
			wait.Reset(time.Second * 30)
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
		ClientSessionCache:     utls.NewLRUClientSessionCache(0),
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
	if task.err == nil && task.res != nil && !task.oneConn {
		obj.putConnPool(key, conn)
	}
	if task.err == nil && task.res == nil {
		task.err = obj.ctx.Err()
	}
	if ctxData.responseCallBack != nil {
		if err = ctxData.responseCallBack(task.req.Context(), req, task.res); err != nil {
			task.err = err
		}
	}
	return task.res, task.err
}
