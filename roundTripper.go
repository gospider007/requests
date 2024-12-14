package requests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gospider007/http2"
	"github.com/gospider007/http3"
	"github.com/gospider007/tools"
)

type reqTask struct {
	option    *RequestOption
	ctx       context.Context
	cnl       context.CancelFunc
	req       *http.Request
	res       *http.Response
	emptyPool chan struct{}
	err       error
}

func (obj *reqTask) suppertRetry() bool {
	if obj.req.Body == nil {
		return true
	} else if body, ok := obj.req.Body.(io.Seeker); ok {
		if i, err := body.Seek(0, io.SeekStart); i == 0 && err == nil {
			return true
		}
	}
	return false
}
func getKey(option *RequestOption, req *http.Request) (key string) {
	if option.proxy != nil {
		var proxyUser string
		if option.proxy.User != nil {
			proxyUser = option.proxy.User.String()
		}

		return fmt.Sprintf("%s@%s@%s", proxyUser, option.proxy.Host, req.URL.Host)
	}

	return req.URL.Host
}

type roundTripper struct {
	ctx       context.Context
	cnl       context.CancelFunc
	connPools *connPools
	dialer    *DialClient
	getProxy  func(ctx context.Context, url *url.URL) (string, error)
	getProxys func(ctx context.Context, url *url.URL) ([]string, error)
}

func newRoundTripper(preCtx context.Context, option ClientOption) *roundTripper {
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
	return &roundTripper{
		ctx:       ctx,
		cnl:       cnl,
		dialer:    dialClient,
		getProxy:  option.GetProxy,
		getProxys: option.GetProxys,
		connPools: newConnPools(),
	}
}
func (obj *roundTripper) newConnPool(conn *connecotr, key string) *connPool {
	pool := new(connPool)
	pool.connKey = key
	pool.forceCtx, pool.forceCnl = context.WithCancelCause(obj.ctx)
	pool.safeCtx, pool.safeCnl = context.WithCancelCause(pool.forceCtx)
	pool.tasks = make(chan *reqTask)

	pool.connPools = obj.connPools
	pool.total.Add(1)
	go pool.rwMain(conn)
	return pool
}
func (obj *roundTripper) putConnPool(key string, conn *connecotr) {
	pool := obj.connPools.get(key)
	if pool != nil {
		pool.total.Add(1)
		go pool.rwMain(conn)
	} else {
		obj.connPools.set(key, obj.newConnPool(conn, key))
	}
}
func (obj *roundTripper) newConnecotr() *connecotr {
	conne := new(connecotr)
	conne.withCancel(obj.ctx, obj.ctx)
	return conne
}

func (obj *roundTripper) http3Dial(option *RequestOption, req *http.Request) (conn *connecotr, err error) {
	tlsConfig := option.TlsConfig.Clone()
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	tlsConfig.ServerName = req.Host
	netConn, err := http3.Dial(req.Context(), getAddr(req.URL), tlsConfig, nil)
	if err != nil {
		return
	}
	conn = obj.newConnecotr()
	conn.rawConn = http3.NewClient(netConn)
	return
}

func (obj *roundTripper) ghttp3Dial(option *RequestOption, req *http.Request) (conn *connecotr, err error) {
	tlsConfig := option.UtlsConfig.Clone()
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	tlsConfig.ServerName = req.Host
	netConn, err := http3.UDial(req.Context(), getAddr(req.URL), tlsConfig, nil)
	if err != nil {
		return
	}
	conn = obj.newConnecotr()
	conn.rawConn = http3.NewUClient(netConn)
	return
}

func (obj *roundTripper) dial(option *RequestOption, req *http.Request) (conn *connecotr, err error) {
	if option.H3 {
		if option.Ja3Spec.IsSet() {
			return obj.ghttp3Dial(option, req)
		} else {
			return obj.http3Dial(option, req)
		}
	}

	var proxys []*url.URL
	if !option.DisProxy {
		if option.proxy != nil {
			proxys = []*url.URL{cloneUrl(option.proxy)}
		}
		if len(proxys) == 0 && len(option.proxys) > 0 {
			proxys = make([]*url.URL, len(option.proxys))
			for i, proxy := range option.proxys {
				proxys[i] = cloneUrl(proxy)
			}
		}
		if len(proxys) == 0 && obj.getProxy != nil {
			proxyStr, err := obj.getProxy(req.Context(), req.URL)
			if err != nil { // proxyStr might be empty
				return conn, err
			}

			proxy, err := verifyProxy(proxyStr)
			if err != nil {
				return conn, err
			}

			proxys = []*url.URL{proxy}
		}
		if len(proxys) == 0 && obj.getProxys != nil {
			proxyStrs, err := obj.getProxys(req.Context(), req.URL)
			if err != nil {
				return conn, err
			}

			if l := len(proxyStrs); l > 0 {
				proxys = make([]*url.URL, l)
				for i, proxyStr := range proxyStrs {
					proxy, err := verifyProxy(proxyStr)
					if err != nil {
						return conn, err
					}
					proxys[i] = proxy
				}
			}
		}
	}
	host := getHost(req)
	var netConn net.Conn
	if len(proxys) > 0 {
		netConn, err = obj.dialer.DialProxyContext(req.Context(), option, "tcp", option.TlsConfig.Clone(), append(proxys, cloneUrl(req.URL))...)
	} else {
		netConn, err = obj.dialer.DialContext(req.Context(), option, "tcp", getAddr(req.URL))
	}
	if err != nil {
		return conn, err
	}
	var h2 bool
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), option.TlsHandshakeTimeout)
		defer cnl()
		if option.Ja3Spec.IsSet() {
			if tlsConn, err := obj.dialer.addJa3Tls(ctx, netConn, host, option.ForceHttp1, option.Ja3Spec, option.UtlsConfig.Clone()); err != nil {
				return conn, tools.WrapError(err, "add ja3 tls error")
			} else {
				h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
				netConn = tlsConn
			}
		} else {
			if tlsConn, err := obj.dialer.addTls(ctx, netConn, host, option.ForceHttp1, option.TlsConfig.Clone()); err != nil {
				return conn, tools.WrapError(err, "add tls error")
			} else {
				h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
				netConn = tlsConn
			}
		}
		if option.Logger != nil {
			option.Logger(Log{
				Id:   option.requestId,
				Time: time.Now(),
				Type: LogType_TLSHandshake,
				Msg:  host,
			})
		}
	}
	conne := obj.newConnecotr()
	conne.proxys = proxys
	if h2 {
		if option.H2Ja3Spec.OrderHeaders != nil {
			option.OrderHeaders = option.H2Ja3Spec.OrderHeaders
		}
		if conne.rawConn, err = http2.NewClientConn(func() {
			conne.forceCnl(errors.New("http2 client close"))
		}, netConn, option.H2Ja3Spec); err != nil {
			return conne, err
		}
	} else {
		conne.rawConn = newConn(conne.forceCtx, netConn, func() {
			conne.forceCnl(errors.New("http1 client close"))
		})
	}
	return conne, err
}
func (obj *roundTripper) setGetProxy(getProxy func(ctx context.Context, url *url.URL) (string, error)) {
	obj.getProxy = getProxy
}
func (obj *roundTripper) setGetProxys(getProxys func(ctx context.Context, url *url.URL) ([]string, error)) {
	obj.getProxys = getProxys
}

func (obj *roundTripper) poolRoundTrip(option *RequestOption, pool *connPool, task *reqTask, key string) (isTry bool) {
	task.ctx, task.cnl = context.WithTimeout(task.req.Context(), option.ResponseHeaderTimeout)
	select {
	case pool.tasks <- task:
		select {
		case <-task.emptyPool:
			return true
		case <-task.ctx.Done():
			if task.err == nil && task.res == nil {
				task.err = context.Cause(task.ctx)
			}
			return false
		}
	default:
		obj.connRoundTripMain(option, task, key)
		return false
	}
}
func (obj *roundTripper) connRoundTripMain(option *RequestOption, task *reqTask, key string) {
	for range 10 {
		if !obj.connRoundTrip(option, task, key) {
			return
		}
	}
	task.err = errors.New("connRoundTripMain retry 5 times")
}

func (obj *roundTripper) connRoundTrip(option *RequestOption, task *reqTask, key string) (retry bool) {
	option.isNewConn = true
	conn, err := obj.dial(option, task.req)
	if err != nil {
		task.err = err
		return
	}
	task.ctx, task.cnl = context.WithTimeout(task.req.Context(), option.ResponseHeaderTimeout)
	retry = conn.taskMain(task, false)
	if retry || task.err != nil {
		return retry
	}
	obj.putConnPool(key, conn)
	return retry
}

func (obj *roundTripper) closeConns() {
	for key, pool := range obj.connPools.Range() {
		pool.safeClose()
		obj.connPools.del(key)
	}
}
func (obj *roundTripper) forceCloseConns() {
	for key, pool := range obj.connPools.Range() {
		pool.forceClose()
		obj.connPools.del(key)
	}
}
func (obj *roundTripper) newReqTask(req *http.Request, option *RequestOption) *reqTask {
	if option.ResponseHeaderTimeout == 0 {
		option.ResponseHeaderTimeout = time.Second * 300
	}
	task := new(reqTask)
	task.req = req
	task.option = option
	task.emptyPool = make(chan struct{})
	return task
}
func (obj *roundTripper) RoundTrip(req *http.Request) (response *http.Response, err error) {
	option := GetRequestOption(req.Context())
	if option.RequestCallBack != nil {
		if err = option.RequestCallBack(req.Context(), req, nil); err != nil {
			if err == http.ErrUseLastResponse {
				if req.Response == nil {
					return nil, errors.New("errUseLastResponse response is nil")
				} else {
					return req.Response, nil
				}
			}
			return nil, err
		}
	}
	key := getKey(option, req) //pool key
	task := obj.newReqTask(req, option)
	for {
		pool := obj.connPools.get(key)
		if pool == nil {
			obj.connRoundTripMain(option, task, key)
			break
		}
		if !obj.poolRoundTrip(option, pool, task, key) {
			break
		}
	}
	if task.err == nil && option.RequestCallBack != nil {
		if err = option.RequestCallBack(task.req.Context(), task.req, task.res); err != nil {
			task.err = err
		}
	}
	return task.res, task.err
}
