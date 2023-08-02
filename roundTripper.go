package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
	"sync"
	"sync/atomic"

	"net/http"

	h2ja3 "gitee.com/baixudong/http2"
	"golang.org/x/net/http2"

	utls "github.com/refraction-networking/utls"
)

type roundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

type connecotr struct {
	conn net.Conn
	h2   bool
	r    *bufio.Reader
	t2   *http2.Transport
	t22  *h2ja3.Upg

	c2 roundTripper
}

type reqTask struct {
	ctx context.Context //控制请求的生命周期
	cnl context.CancelFunc
	req *http.Request  //发送的请求
	res *http.Response //接收的请求
	err error
}

type connPool struct {
	ctx   context.Context
	cnl   context.CancelFunc
	key   string
	total atomic.Int64
	tasks chan *reqTask
	rt    *RoundTripper
}

func (obj *connPool) rwMain(conn *connecotr) {
	defer func() {
		if obj.total.Load() == 0 {
			obj.rt.delConnPool(obj.key)
		}
	}()
	defer obj.total.Add(-1)
	if conn.h2 {
		obj.h2Main(conn)
	} else {
		obj.h1Main(conn)
	}
}
func (obj *connPool) h1Main(conn *connecotr) {
	for {
		select {
		case task := <-obj.tasks:
			http1Req(conn, task)
		case <-obj.ctx.Done():
			return
		}
	}
}
func (obj *connPool) h2Main(conn *connecotr) {
	for {
		select {
		case task := <-obj.tasks:
			http2Req(conn, task)
		case <-obj.ctx.Done():
			return
		}
	}
}

type RoundTripper struct {
	ctx       context.Context
	cnl       context.CancelFunc
	connPools map[string]*connPool
	connsLock sync.Mutex
	dialer    *DialClient
	t2        *http2.Transport
	t22       *h2ja3.Upg
}

func NewRoundTripper(preCtx context.Context, dialClient *DialClient, t22 *h2ja3.Upg, option ClientOption) *RoundTripper {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	t2 := http2.Transport{
		DialTLSContext:   dialClient.requestHttp2DialTlsContext,
		TLSClientConfig:  dialClient.TlsConfig(),
		ReadIdleTimeout:  option.ResponseHeaderTimeout,
		PingTimeout:      option.TLSHandshakeTimeout,
		WriteByteTimeout: option.IdleConnTimeout,
	}
	return &RoundTripper{
		t2:        &t2,
		t22:       t22,
		ctx:       ctx,
		cnl:       cnl,
		connPools: make(map[string]*connPool),
		dialer:    dialClient,
	}
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
func getKey(req *http.Request) string {
	ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
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
func (obj *RoundTripper) delConnPool(key string) {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
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
		pool.rwMain(conn)
	} else {
		obj.connPools[key] = obj.newConnPool(key, conn)
	}
}
func (obj *RoundTripper) dial(key string, req *http.Request) (conn *connecotr, err error) {
	ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
	if !ctxData.disProxy && ctxData.proxy == nil { //确定代理
		if ctxData.proxy, err = obj.dialer.GetProxy(req.Context(), req.URL); err != nil {
			return nil, err
		}
	}
	var netConn net.Conn
	host := getHost(req)
	addr := getAddr(req.URL)
	if ctxData.proxy == nil {
		netConn, err = obj.dialer.DialContext(req.Context(), "tcp", addr)
	} else {
		netConn, err = obj.dialer.DialContextWithProxy(req.Context(), "tcp", req.URL.Scheme, addr, host, ctxData.proxy)
	}
	if err != nil {
		return nil, err
	}
	conne := new(connecotr)
	var h2 bool
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), obj.dialer.tlsHandshakeTimeout)
		defer cnl()
		netConn, err = obj.dialer.AddTls(ctx, netConn, host, ctxData.ws)
		if err != nil {
			return nil, err
		}
		if tlsConn, ok := netConn.(interface {
			ConnectionState() utls.ConnectionState
		}); ok {
			h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
		} else if tlsConn, ok := netConn.(interface {
			ConnectionState() tls.ConnectionState
		}); ok {
			h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
		}
	}
	conne.conn = netConn
	if h2 {
		conne.h2 = h2
		conne.t2 = obj.t2
		conne.t22 = obj.t22
		if conne.t22 != nil {
			if conne.c2, err = conne.t22.ClientConn(netConn); err != nil {
				return conne, err
			}
		} else {
			if conne.c2, err = conne.t2.NewClientConn(netConn); err != nil {
				return conne, err
			}
		}
	} else {
		conne.r = bufio.NewReader(netConn)
	}
	return conne, err
}
func http1Req(conn *connecotr, task *reqTask) {
	log.Print("===============222")
	log.Print(task, conn)
	defer task.cnl()
	err := task.req.Write(conn.conn)
	log.Print("===============555", err)
	if err != nil {
		task.err = err
	} else {
		log.Print("===============666", conn.r)
		task.res, task.err = http.ReadResponse(conn.r, task.req)
		log.Print(task, err)
	}
}
func http2Req(conn *connecotr, task *reqTask) {
	log.Print("===============444")
	log.Print(conn)
	log.Print(task, conn)
	log.Print("===============3")
	defer task.cnl()
	task.res, task.err = conn.c2.RoundTrip(task.req)
}
func (obj *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	key := getKey(req)
	pool := obj.getConnPool(key)
	ctx, cnl := context.WithCancel(req.Context())
	defer cnl()
	task := &reqTask{
		ctx: ctx,
		cnl: cnl,
		req: req,
	}
	if pool != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case pool.tasks <- task:
			return task.res, task.err
		default:
		}
	}
	conn, err := obj.dial(key, req)
	if err != nil {
		return nil, err
	}
	if !conn.h2 {
		http1Req(conn, task)
	} else {
		http2Req(conn, task)
	}
	if task.err == nil && task.res != nil {
		obj.putConnPool(key, conn)
	}
	return task.res, task.err
}
