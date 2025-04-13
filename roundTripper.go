package requests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"net/http"

	"github.com/gospider007/http2"
	"github.com/gospider007/http3"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	"github.com/quic-go/quic-go"
	uquic "github.com/refraction-networking/uquic"
)

type reqTask struct {
	bodyCtx     context.Context
	ctx         context.Context
	cnl         context.CancelFunc
	reqCtx      *Response
	err         error
	enableRetry bool
	disRetry    bool
	isNotice    bool
	key         string
}

func (obj *reqTask) suppertRetry() bool {
	if obj.disRetry {
		return false
	}
	if obj.enableRetry {
		return true
	}
	if obj.reqCtx.request.Body == nil {
		return true
	} else if body, ok := obj.reqCtx.request.Body.(io.Seeker); ok {
		if i, err := body.Seek(0, io.SeekStart); i == 0 && err == nil {
			return true
		}
	}
	return false
}
func getKey(ctx *Response) (string, error) {
	adds := []string{}
	for _, p := range ctx.proxys {
		pd, err := GetAddressWithUrl(p)
		if err != nil {
			return "", err
		}
		adds = append(adds, pd.String())
	}
	pd, err := GetAddressWithUrl(ctx.Request().URL)
	if err != nil {
		return "", err
	}
	adds = append(adds, pd.String())
	return strings.Join(adds, "@"), nil
}

type roundTripper struct {
	ctx       context.Context
	cnl       context.CancelFunc
	connPools *connPools
	dialer    *Dialer
}

var specClient = ja3.NewClient()

func newRoundTripper(preCtx context.Context) *roundTripper {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	return &roundTripper{
		ctx:       ctx,
		cnl:       cnl,
		dialer:    new(Dialer),
		connPools: newConnPools(),
	}
}
func (obj *roundTripper) newConnPool(done chan struct{}, conn *connecotr, task *reqTask) *connPool {
	pool := new(connPool)
	pool.connKey = task.key
	pool.forceCtx, pool.forceCnl = context.WithCancelCause(obj.ctx)
	pool.tasks = make(chan *reqTask)

	pool.connPools = obj.connPools
	pool.total.Add(1)
	go pool.rwMain(done, conn)
	return pool
}
func (obj *roundTripper) putConnPool(task *reqTask, conn *connecotr) {
	pool := obj.connPools.get(task)
	done := make(chan struct{})
	if pool != nil {
		pool.total.Add(1)
		go pool.rwMain(done, conn)
	} else {
		obj.connPools.set(task, obj.newConnPool(done, conn, task))
	}
	<-done
}
func (obj *roundTripper) newConnecotr() *connecotr {
	conne := new(connecotr)
	conne.withCancel(obj.ctx)
	return conne
}

func (obj *roundTripper) http3Dial(ctx *Response, remtoeAddress Address, proxyAddress ...Address) (udpConn net.PacketConn, err error) {
	if len(proxyAddress) > 0 {
		if proxyAddress[len(proxyAddress)-1].Scheme != "socks5" {
			err = errors.New("http3 last proxy must socks5 proxy")
			return
		}
		udpConn, _, err = obj.dialer.DialProxyContext(ctx, "tcp", ctx.option.TlsConfig.Clone(), append(proxyAddress, remtoeAddress)...)
	} else {
		udpConn, err = net.ListenUDP("udp", nil)
	}
	return
}
func (obj *roundTripper) ghttp3Dial(ctx *Response, remoteAddress Address, proxyAddress ...Address) (conn *connecotr, err error) {
	udpConn, err := obj.http3Dial(ctx, remoteAddress, proxyAddress...)
	if err != nil {
		return nil, err
	}
	tlsConfig := ctx.option.TlsConfig.Clone()
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	tlsConfig.ServerName = remoteAddress.Host
	if remoteAddress.IP == nil && len(proxyAddress) == 0 {
		remoteAddress.IP, err = obj.dialer.loadHost(ctx.Context(), remoteAddress.Host, ctx.option.DialOption)
		if err != nil {
			return nil, err
		}
	}
	var quicConfig *quic.Config
	if ctx.option.UquicConfig != nil {
		quicConfig = ctx.option.QuicConfig.Clone()
	}
	var udpCtx context.Context
	if ct, ok := udpConn.(interface {
		Context() context.Context
	}); ok {
		udpCtx = ct.Context()
	}
	netConn, err := quic.DialEarly(ctx.Context(), udpConn, &net.UDPAddr{IP: remoteAddress.IP, Port: remoteAddress.Port}, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}
	conn = obj.newConnecotr()
	conn.Conn, err = http3.NewClient(udpCtx, netConn, func() {
		conn.forceCnl(errors.New("http3 client close"))
	})
	return
}

func (obj *roundTripper) uhttp3Dial(ctx *Response, spec uquic.QUICSpec, remoteAddress Address, proxyAddress ...Address) (conn *connecotr, err error) {
	udpConn, err := obj.http3Dial(ctx, remoteAddress, proxyAddress...)
	if err != nil {
		return nil, err
	}
	tlsConfig := ctx.option.UtlsConfig.Clone()
	tlsConfig.NextProtos = []string{http3.NextProtoH3}
	tlsConfig.ServerName = remoteAddress.Host
	if remoteAddress.IP == nil && len(proxyAddress) == 0 {
		remoteAddress.IP, err = obj.dialer.loadHost(ctx.Context(), remoteAddress.Host, ctx.option.DialOption)
		if err != nil {
			return nil, err
		}
	}
	var quicConfig *uquic.Config
	if ctx.option.UquicConfig != nil {
		quicConfig = ctx.option.UquicConfig.Clone()
	}

	var udpCtx context.Context
	if ct, ok := udpConn.(interface {
		Context() context.Context
	}); ok {
		udpCtx = ct.Context()
	}
	netConn, err := (&uquic.UTransport{
		Transport: &uquic.Transport{
			Conn: udpConn,
		},
		QUICSpec: &spec,
	}).DialEarly(ctx.Context(), &net.UDPAddr{IP: remoteAddress.IP, Port: remoteAddress.Port}, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}
	conn = obj.newConnecotr()
	conn.Conn, err = http3.NewClient(udpCtx, netConn, func() {
		conn.forceCnl(errors.New("http3 client close"))
	})
	return
}

func (obj *roundTripper) dial(ctx *Response) (conn *connecotr, err error) {
	proxys, err := obj.initProxys(ctx)
	if err != nil {
		return nil, err
	}
	remoteAddress, err := GetAddressWithUrl(ctx.request.URL)
	if err != nil {
		return nil, err
	}
	if ctx.option.ForceHttp3 {
		if ctx.option.USpec != nil {
			spec, err := ja3.CreateUSpec(ctx.option.USpec)
			if err != nil {
				return nil, err
			}
			if spec.ClientHelloSpec != nil {
				return obj.uhttp3Dial(ctx, spec, remoteAddress, proxys...)
			}
		}
		return obj.ghttp3Dial(ctx, remoteAddress, proxys...)
	}
	var rawNetConn net.Conn
	if len(proxys) > 0 {
		_, rawNetConn, err = obj.dialer.DialProxyContext(ctx, "tcp", ctx.option.TlsConfig.Clone(), append(proxys, remoteAddress)...)
	} else {
		var remoteAddress Address
		remoteAddress, err = GetAddressWithUrl(ctx.request.URL)
		if err != nil {
			return nil, err
		}
		rawNetConn, err = obj.dialer.DialContext(ctx, "tcp", remoteAddress)
	}
	defer func() {
		if err != nil && rawNetConn != nil {
			rawNetConn.Close()
		}
	}()
	if err != nil {
		return nil, err
	}
	var h2 bool
	conne := obj.newConnecotr()
	conne.proxys = proxys
	if ctx.request.URL.Scheme == "https" {
		conne.c, h2, err = obj.dialAddTls(ctx.option, ctx.request, rawNetConn)
		if ctx.option.Logger != nil {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_TLSHandshake,
				Msg:  fmt.Sprintf("host:%s,  h2:%t", getHost(ctx.request), h2),
			})
		}
		if err != nil {
			return nil, err
		}
	} else {
		conne.c = rawNetConn
	}
	err = obj.dialConnecotr(ctx, conne, h2)
	if err != nil {
		return nil, err
	}
	return conne, err
}
func (obj *roundTripper) dialConnecotr(ctx *Response, conne *connecotr, h2 bool) (err error) {
	if h2 {
		var spec *http2.Spec
		if ctx.option.gospiderSpec != nil {
			spec = ctx.option.gospiderSpec.H2Spec
		}
		if conne.Conn, err = http2.NewClientConn(ctx.Context(), conne.c, spec, func(err error) {
			conne.forceCnl(tools.WrapError(err, "http2 client close"))
		}); err != nil {
			return err
		}
	} else {
		conne.Conn = newClientConn(conne.forceCtx, conne.c, func(err error) {
			conne.forceCnl(tools.WrapError(err, "http1 client close"))
		})
	}
	return err
}
func (obj *roundTripper) dialAddTls(option *RequestOption, req *http.Request, netConn net.Conn) (net.Conn, bool, error) {
	ctx, cnl := context.WithTimeout(req.Context(), option.TlsHandshakeTimeout)
	defer cnl()
	if option.gospiderSpec != nil && option.gospiderSpec.TLSSpec != nil {
		if tlsConn, err := obj.dialer.addJa3Tls(ctx, netConn, getHost(req), option.gospiderSpec.TLSSpec, option.UtlsConfig.Clone(), option.ForceHttp1); err != nil {
			return tlsConn, false, tools.WrapError(err, "add ja3 tls error")
		} else {
			return tlsConn, tlsConn.ConnectionState().NegotiatedProtocol == "h2", nil
		}
	}
	if tlsConn, err := obj.dialer.addTls(ctx, netConn, getHost(req), option.TlsConfig.Clone(), option.ForceHttp1); err != nil {
		return tlsConn, false, tools.WrapError(err, "add tls error")
	} else {
		return tlsConn, tlsConn.ConnectionState().NegotiatedProtocol == "h2", nil
	}
}
func (obj *roundTripper) initProxys(ctx *Response) ([]Address, error) {
	if ctx.option.DisProxy {
		return nil, nil
	}
	pps := ctx.proxys
	if len(pps) == 0 {
		if ctx.option.GetProxy == nil {
			return nil, nil
		}
		proxyA, err := ctx.option.GetProxy(ctx)
		if err != nil || proxyA == nil {
			return nil, err
		}
		pps, err = parseProxy(proxyA)
		if err != nil || len(pps) == 0 {
			return nil, err
		}
	}
	proxys := make([]Address, len(pps))
	for i, proxy := range pps {
		proxyAddress, err := GetAddressWithUrl(proxy)
		if err != nil {
			return nil, err
		}
		proxys[i] = proxyAddress
	}
	return proxys, nil
}

func (obj *roundTripper) poolRoundTrip(task *reqTask) {
	connPool := obj.connPools.get(task)
	if connPool == nil {
		obj.newRoudTrip(task)
		return
	}
	task.ctx, task.cnl = context.WithTimeout(task.reqCtx.Context(), task.reqCtx.option.ResponseHeaderTimeout)
	select {
	case connPool.tasks <- task:
		<-task.ctx.Done()
		if task.err == nil && task.reqCtx.response == nil {
			task.err = context.Cause(task.ctx)
		}
	default:
		obj.newRoudTrip(task)
		return
	}
}

func (obj *roundTripper) newRoudTrip(task *reqTask) {
	task.reqCtx.isNewConn = true
	conn, err := obj.dial(task.reqCtx)
	if err != nil {
		task.err = tools.WrapError(err, "newRoudTrip dial error")
		if task.reqCtx.option.ErrCallBack != nil {
			task.reqCtx.err = err
			if err2 := task.reqCtx.option.ErrCallBack(task.reqCtx); err2 != nil {
				task.err = err2
			}
		}
		task.enableRetry = true
	}
	if task.err == nil {
		obj.putConnPool(task, conn)
		obj.poolRoundTrip(task)
	}
}

func (obj *roundTripper) closeConns() {
	for key, pool := range obj.connPools.Range() {
		pool.close(errors.New("close all conn"))
		obj.connPools.del(key)
	}
}

func (obj *roundTripper) newReqTask(ctx *Response) (*reqTask, error) {
	if ctx.option.ResponseHeaderTimeout == 0 {
		ctx.option.ResponseHeaderTimeout = time.Second * 300
	}
	task := new(reqTask)
	task.reqCtx = ctx
	task.reqCtx.response = nil
	key, err := getKey(ctx)
	if err != nil {
		return nil, err
	}
	task.key = key
	return task, nil
}
func (obj *roundTripper) RoundTrip(ctx *Response) (err error) {
	if ctx.option.RequestCallBack != nil {
		if err = ctx.option.RequestCallBack(ctx); err != nil {
			if err == http.ErrUseLastResponse {
				if ctx.response == nil {
					return errors.New("errUseLastResponse response is nil")
				} else {
					return nil
				}
			}
			return err
		}
	}
	currentRetry := 0
	var task *reqTask
	for ; currentRetry <= maxRetryCount; currentRetry++ {
		select {
		case <-ctx.Context().Done():
			return context.Cause(ctx.Context())
		default:
		}
		task, err = obj.newReqTask(ctx)
		if err != nil {
			return err
		}
		obj.poolRoundTrip(task)
		if task.err == nil || !task.suppertRetry() {
			break
		}
		if task.isNotice {
			currentRetry--
		}
	}
	if task.err == nil && ctx.option.RequestCallBack != nil {
		if err = ctx.option.RequestCallBack(ctx); err != nil {
			task.err = err
		}
	}
	return task.err
}
