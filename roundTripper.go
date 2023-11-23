package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"log"
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
	debug                 bool
	requestId             string
}

func newReqTask(req *http.Request, ctxData *reqCtxData) *reqTask {
	if ctxData.responseHeaderTimeout == 0 {
		ctxData.responseHeaderTimeout = time.Second * 30
	}
	return &reqTask{
		req:                   req,
		debug:                 ctxData.debug,
		requestId:             ctxData.requestId,
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
	connPools  sync.Map
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
		dialer:     dialClient,
		proxy:      option.GetProxy,
	}
}
func (obj *RoundTripper) newConnPool(conn *connecotr, key connKey) *connPool {
	pool := new(connPool)
	pool.key = key
	pool.deleteCtx, pool.deleteCnl = context.WithCancel(obj.ctx)
	pool.closeCtx, pool.closeCnl = context.WithCancel(pool.deleteCtx)
	pool.tasks = make(chan *reqTask)
	pool.rt = obj
	pool.total.Add(1)
	go pool.rwMain(conn)
	return pool
}
func (obj *RoundTripper) getConnPool(key connKey) *connPool {
	val, ok := obj.connPools.Load(key)
	if !ok {
		return nil
	}
	return val.(*connPool)
}
func (obj *RoundTripper) delConnPool(key connKey) {
	obj.connPools.Delete(key)
}
func (obj *RoundTripper) putConnPool(key connKey, conn *connecotr) {
	conn.isPool = true
	if !conn.h2 {
		go conn.read()
	}
	val, ok := obj.connPools.Load(key)
	if ok {
		pool := val.(*connPool)
		select {
		case <-pool.closeCtx.Done():
			obj.connPools.Store(key, obj.newConnPool(conn, key))
		default:
			pool.total.Add(1)
			go pool.rwMain(conn)
		}
	} else {
		obj.connPools.Store(key, obj.newConnPool(conn, key))
	}
}
func (obj *RoundTripper) tlsConfigClone() *tls.Config {
	return obj.tlsConfig.Clone()
}
func (obj *RoundTripper) utlsConfigClone() *utls.Config {
	return obj.utlsConfig.Clone()
}
func (obj *RoundTripper) dial(ctxData *reqCtxData, key *connKey, req *http.Request) (conn *connecotr, err error) {
	proxy := ctxData.proxy
	if !ctxData.disProxy && proxy == nil {
		if proxy, err = obj.getProxy(req.Context(), req.URL); err != nil {
			return conn, err
		}
	}
	if proxy != nil {
		key.proxy = proxy.String()
	}
	var netConn net.Conn
	host := getHost(req)
	if proxy == nil {
		netConn, err = obj.dialer.DialContext(req.Context(), "tcp", key.addr)
	} else {
		netConn, err = obj.dialer.DialContextWithProxy(req.Context(), "tcp", req.URL.Scheme, key.addr, host, proxy, obj.tlsConfigClone())
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
	if task.debug {
		log.Printf("requestId:%s: %s", task.requestId, "poolRoundTrip get conn pool")
	}
	pool := obj.getConnPool(key)
	if pool == nil {
		if task.debug {
			log.Printf("requestId:%s: %s", task.requestId, "poolRoundTrip not found conn pool")
		}
		return false, nil
	}
	select {
	case <-obj.ctx.Done():
		if task.debug {
			log.Printf("requestId:%s: %s", task.requestId, "RoundTripper already cloed")
		}
		return false, tools.WrapError(obj.ctx.Err(), "roundTripper close ctx error: ")
	case <-pool.closeCtx.Done():
		if task.debug {
			log.Printf("requestId:%s: %s", task.requestId, "pool already cloed")
		}
		return false, pool.closeCtx.Err()
	case pool.tasks <- task:
		if task.debug {
			log.Printf("requestId:%s: %s", task.requestId, "pool.tasks <- task")
		}
		select {
		case <-task.emptyPool:
			if task.debug {
				log.Printf("requestId:%s: %s", task.requestId, "poolRoundTrip emptyPool")
			}
		case <-task.ctx.Done():
			if task.err == nil && task.res == nil {
				task.err = tools.WrapError(task.ctx.Err(), "task close ctx error: ")
			}
			if task.debug {
				log.Printf("requestId:%s: %v", task.requestId, task.err)
			}
			return true, task.err
		}
	default:
		if task.debug {
			log.Printf("requestId:%s: %s", task.requestId, "conn pool not idle")
		}
	}
	return false, nil
}

func (obj *RoundTripper) closeConns() {
	obj.connPools.Range(func(key, value any) bool {
		pool := value.(*connPool)
		pool.close()
		obj.connPools.Delete(key)
		return true
	})
}

func (obj *RoundTripper) forceCloseConns() {
	obj.connPools.Range(func(key, value any) bool {
		pool := value.(*connPool)
		pool.forceClose()
		obj.connPools.Delete(key)
		return true
	})
}
func (obj *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctxData := GetReqCtxData(req.Context())
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
	if task.debug {
		log.Printf("requestId:%s: %s", ctxData.requestId, "new Conn")
	}
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
		if task.debug {
			log.Printf("requestId:%s: %s", ctxData.requestId, "conn put conn pool")
		}
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
