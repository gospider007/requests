package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"sync"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/net/http2"
	"github.com/gospider007/tools"
	utls "github.com/refraction-networking/utls"
)

type reqTask struct {
	ctx                   context.Context
	cnl                   context.CancelFunc
	req                   *http.Request
	res                   *http.Response
	emptyPool             chan struct{}
	err                   error
	orderHeaders          []string
	responseHeaderTimeout time.Duration
}

func newReqTask(req *http.Request, ctxData *reqCtxData) *reqTask {
	if ctxData.responseHeaderTimeout == 0 {
		ctxData.responseHeaderTimeout = time.Second * 30
	}
	return &reqTask{
		req:                   req,
		emptyPool:             make(chan struct{}),
		orderHeaders:          ctxData.orderHeaders,
		responseHeaderTimeout: ctxData.responseHeaderTimeout,
	}
}
func (obj *reqTask) inPool() bool {
	return obj.err == nil && obj.res != nil && obj.res.StatusCode != 101 && obj.res.Header.Get("Content-Type") != "text/event-stream"
}

type connKey struct {
	proxy string
	addr  string
	ja3   string
	h2Ja3 string
}

func getKey(ctxData *reqCtxData, req *http.Request) connKey {
	var proxy string
	if ctxData.proxy != nil {
		proxy = ctxData.proxy.String()
	}
	return connKey{
		h2Ja3: ctxData.h2Ja3Spec.Fp(),
		ja3:   ctxData.ja3Spec.String(),
		proxy: proxy,
		addr:  getAddr(req.URL),
	}
}

type RoundTripper struct {
	ctx        context.Context
	cnl        context.CancelFunc
	connPools  map[connKey]*connPool
	connsLock  sync.Mutex
	dialer     *DialClient
	tlsConfig  *tls.Config
	utlsConfig *utls.Config
	proxy      func(ctx context.Context, url *url.URL) (string, error)
}

type roundTripperOption struct {
	DialTimeout time.Duration
	KeepAlive   time.Duration
	LocalAddr   *net.TCPAddr  //network card ip
	AddrType    gtls.AddrType //first ip type
	GetAddrType func(host string) gtls.AddrType
	Dns         *net.UDPAddr
	GetProxy    func(ctx context.Context, url *url.URL) (string, error)
}

func newRoundTripper(preCtx context.Context, option roundTripperOption) *RoundTripper {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	dialClient := NewDail(DialOption{
		DialTimeout: option.DialTimeout,
		Dns:         option.Dns,
		KeepAlive:   option.KeepAlive,
		LocalAddr:   option.LocalAddr,
		AddrType:    option.AddrType,
		GetAddrType: option.GetAddrType,
	})
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
		tlsConfig:  tlsConfig,
		utlsConfig: utlsConfig,
		ctx:        ctx,
		cnl:        cnl,
		connPools:  make(map[connKey]*connPool),
		dialer:     dialClient,
		proxy:      option.GetProxy,
	}
}
func (obj *RoundTripper) newConnPool(conn *connecotr) *connPool {
	pool := new(connPool)
	pool.deleteCtx, pool.deleteCnl = context.WithCancel(obj.ctx)
	pool.closeCtx, pool.closeCnl = context.WithCancel(pool.deleteCtx)
	pool.tasks = make(chan *reqTask)
	pool.rt = obj
	pool.total.Add(1)
	go pool.rwMain(conn)
	return pool
}
func (obj *RoundTripper) getConnPool(key connKey) *connPool {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	return obj.connPools[key]
}
func (obj *RoundTripper) putConnPool(key connKey, conn *connecotr) {
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
		obj.connPools[key] = obj.newConnPool(conn)
	}
}
func (obj *RoundTripper) tlsConfigClone() *tls.Config {
	return obj.tlsConfig.Clone()
}
func (obj *RoundTripper) utlsConfigClone() *utls.Config {
	return obj.utlsConfig.Clone()
}
func (obj *RoundTripper) dial(ctxData *reqCtxData, key *connKey, req *http.Request) (conn *connecotr, err error) {
	if !ctxData.disProxy && ctxData.proxy == nil {
		if ctxData.proxy, err = obj.getProxy(req.Context(), req.URL); err != nil {
			return conn, err
		}
	}
	if ctxData.proxy != nil {
		key.proxy = ctxData.proxy.String()
	}
	var netConn net.Conn
	host := getHost(req)
	if ctxData.proxy == nil {
		netConn, err = obj.dialer.DialContext(req.Context(), "tcp", key.addr)
	} else {
		netConn, err = obj.dialer.DialContextWithProxy(req.Context(), "tcp", req.URL.Scheme, key.addr, host, ctxData.proxy, obj.tlsConfigClone())
	}
	if err != nil {
		return conn, err
	}
	conne := new(connecotr)
	conne.rn = make(chan int)
	conne.rc = make(chan []byte)
	conne.withCancel(obj.ctx, obj.ctx)
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), ctxData.tlsHandshakeTimeout)
		defer cnl()
		if ctxData.ja3Spec.IsSet() {
			tlsConfig := obj.utlsConfigClone()
			if ctxData.forceHttp1 {
				tlsConfig.NextProtos = []string{"http/1.1"}
			}
			tlsConn, err := obj.dialer.addJa3Tls(ctx, netConn, host, ctxData.isWs || ctxData.forceHttp1, ctxData.ja3Spec, tlsConfig)
			if err != nil {
				return conne, tools.WrapError(err, "add ja3 tls error")
			}
			conne.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
			netConn = tlsConn
		} else {
			tlsConn, err := obj.dialer.addTls(ctx, netConn, host, ctxData.isWs || ctxData.forceHttp1, obj.tlsConfigClone())
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
		conne.w = bufio.NewWriter(conne)
	}
	return conne, err
}
func (obj *RoundTripper) setGetProxy(getProxy func(ctx context.Context, url *url.URL) (string, error)) {
	obj.proxy = getProxy
}
func (obj *RoundTripper) getProxy(ctx context.Context, proxyUrl *url.URL) (*url.URL, error) {
	if obj.proxy == nil {
		return nil, nil
	}
	proxy, err := obj.proxy(ctx, proxyUrl)
	if err != nil {
		return nil, err
	}
	return gtls.VerifyProxy(proxy)
}

func (obj *RoundTripper) poolRoundTrip(task *reqTask, key connKey) (bool, error) {
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

func (obj *RoundTripper) closeIdleConnections() {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	for key, pool := range obj.connPools {
		pool.Close()
		delete(obj.connPools, key)
	}
}

func (obj *RoundTripper) closeConnections() {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	for key, pool := range obj.connPools {
		pool.ForceClose()
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
	task := newReqTask(req, ctxData)
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
	ctxData.isNewConn = true
newConn:
	ckey := key
	conn, err := obj.dial(ctxData, &ckey, req)
	if err != nil {
		return nil, err
	}
	if _, _, notice := conn.taskMain(task, nil); notice {
		goto newConn
	}
	if task.err == nil && task.res == nil {
		task.err = obj.ctx.Err()
	}
	conn.key = ckey
	if task.inPool() && !ctxData.disAlive {
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
