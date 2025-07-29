package requests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/gospider007/http1"
	"github.com/gospider007/http2"
	"github.com/gospider007/http3"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	"github.com/quic-go/quic-go"
	uquic "github.com/refraction-networking/uquic"
)

type reqTask struct {
	ctx         context.Context
	cnl         context.CancelCauseFunc
	reqCtx      *Response
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
	connPools sync.Map
	dialer    *Dialer
	lock      sync.Mutex
}

var specClient = ja3.NewClient()

func newRoundTripper(preCtx context.Context) *roundTripper {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	return &roundTripper{
		ctx:    ctx,
		cnl:    cnl,
		dialer: new(Dialer),
	}
}

func (obj *roundTripper) getConnPool(task *reqTask) chan *reqTask {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	val, ok := obj.connPools.Load(task.key)
	if ok {
		return val.(chan *reqTask)
	}
	tasks := make(chan *reqTask)
	obj.connPools.Store(task.key, tasks)
	return tasks
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
func (obj *roundTripper) ghttp3Dial(ctx *Response, remoteAddress Address, proxyAddress ...Address) (conn http1.Conn, err error) {
	udpConn, err := obj.http3Dial(ctx, remoteAddress, proxyAddress...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			udpConn.Close()
		}
	}()
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
	netConn, err := quic.DialEarly(ctx.Context(), udpConn, &net.UDPAddr{IP: remoteAddress.IP, Port: remoteAddress.Port}, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}
	cctx, ccnl := context.WithCancelCause(obj.ctx)
	conn = http3.NewClient(cctx, netConn, udpConn, func() {
		ccnl(errors.New("http3 client close"))
	})
	if ct, ok := udpConn.(interface {
		SetTcpCloseFunc(f func(error))
	}); ok {
		ct.SetTcpCloseFunc(func(err error) {
			ccnl(errors.New("http3 client close with udp"))
		})
	}
	return
}

func (obj *roundTripper) uhttp3Dial(ctx *Response, remoteAddress Address, proxyAddress ...Address) (conn http1.Conn, err error) {
	spec, err := ja3.CreateUSpec(ctx.option.USpec)
	if err != nil {
		return nil, err
	}
	udpConn, err := obj.http3Dial(ctx, remoteAddress, proxyAddress...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			udpConn.Close()
		}
	}()
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
	netConn, err := (&uquic.UTransport{
		Transport: &uquic.Transport{
			Conn: udpConn,
		},
		QUICSpec: &spec,
	}).DialEarly(ctx.Context(), &net.UDPAddr{IP: remoteAddress.IP, Port: remoteAddress.Port}, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}
	cctx, ccnl := context.WithCancelCause(obj.ctx)
	conn = http3.NewClient(cctx, netConn, udpConn, func() {
		ccnl(errors.New("http3 client close"))
	})
	if ct, ok := udpConn.(interface {
		SetTcpCloseFunc(f func(error))
	}); ok {
		ct.SetTcpCloseFunc(func(err error) {
			ccnl(errors.New("uhttp3 client close with udp"))
		})
	}
	return
}

func (obj *roundTripper) dial(ctx *Response) (conn http1.Conn, err error) {
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
			return obj.uhttp3Dial(ctx, remoteAddress, proxys...)
		} else {
			return obj.ghttp3Dial(ctx, remoteAddress, proxys...)
		}
	}
	var rawNetConn net.Conn
	var arch Compression
	if len(proxys) > 0 {
		comp := proxys[len(proxys)-1]
		if comp.Compression != "" {
			arch, err = NewCompression(comp.Compression)
			if err != nil {
				return nil, err
			}
		}
		_, rawNetConn, err = obj.dialer.DialProxyContext(ctx, "tcp", ctx.option.TlsConfig.Clone(), append(proxys, remoteAddress)...)
	} else {
		var remoteAddress Address
		remoteAddress, err = GetAddressWithUrl(ctx.request.URL)
		if err != nil {
			return nil, err
		}
		rawNetConn, err = obj.dialer.DialContext(ctx, "tcp", remoteAddress)
	}
	if err != nil {
		if rawNetConn != nil {
			rawNetConn.Close()
		}
		return nil, err
	}
	var h2 bool
	var rawConn net.Conn
	if ctx.request.URL.Scheme == "https" {
		if ctx.option.TlsHandshakeTimeout > 0 {
			tlsCtx, tlsCnl := context.WithTimeout(ctx.Context(), ctx.option.TlsHandshakeTimeout)
			rawConn, h2, err = obj.dialAddTls(tlsCtx, ctx.option, ctx.request, rawNetConn)
			tlsCnl()
		} else {
			rawConn, h2, err = obj.dialAddTls(ctx.Context(), ctx.option, ctx.request, rawNetConn)
		}
		if ctx.option.Logger != nil {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_TLSHandshake,
				Msg:  fmt.Sprintf("host:%s,  h2:%t", getHost(ctx.request), h2),
			})
		}
	} else {
		rawConn = rawNetConn
	}
	if err != nil {
		if rawConn != nil {
			rawConn.Close()
		}
		return nil, err
	}
	if arch != nil {
		rawConn, err = NewCompressionConn(rawConn, arch)
	}
	if err != nil {
		if rawConn != nil {
			rawConn.Close()
		}
		return nil, err
	}
	return obj.dialConnecotr(ctx, rawConn, h2)
}
func (obj *roundTripper) dialConnecotr(ctx *Response, rawCon net.Conn, h2 bool) (conn http1.Conn, err error) {
	cctx, ccnl := context.WithCancelCause(obj.ctx)
	if h2 {
		var spec *http2.Spec
		if ctx.option.gospiderSpec != nil {
			spec = ctx.option.gospiderSpec.H2Spec
		}
		conn, err = http2.NewClientConn(cctx, ctx.Context(), rawCon, spec, func(err error) {
			ccnl(tools.WrapError(err, "http2 client close"))
		})
	} else {
		conn = http1.NewClientConn(cctx, rawCon, func(err error) {
			ccnl(tools.WrapError(err, "http1 client close"))
		})
	}
	return
}
func (obj *roundTripper) dialAddTls(ctx context.Context, option *RequestOption, req *http.Request, netConn net.Conn) (net.Conn, bool, error) {
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

func (obj *roundTripper) waitTask(task *reqTask) error {
	<-task.ctx.Done()
	err := context.Cause(task.ctx)
	if errors.Is(err, tools.ErrNoErr) {
		err = nil
	}
	return err
}
func (obj *roundTripper) poolRoundTrip(task *reqTask) error {
	task.ctx, task.cnl = context.WithCancelCause(task.reqCtx.Context())
	select {
	case obj.getConnPool(task) <- task:
		return obj.waitTask(task)
	default:
		return obj.newRoundTrip(task)
	}
}

func (obj *roundTripper) newRoundTrip(task *reqTask) error {
	task.reqCtx.isNewConn = true
	conn, err := obj.dial(task.reqCtx)
	if err != nil {
		if conn != nil {
			conn.CloseWithError(err)
		}
		err = tools.WrapError(err, "newRoudTrip dial error")
		if task.reqCtx.option.ErrCallBack != nil {
			task.reqCtx.err = err
			if err2 := task.reqCtx.option.ErrCallBack(task.reqCtx); err2 != nil {
				err = err2
			}
		}
		task.enableRetry = true
	}
	if err == nil {
		go rwMain(conn, task, obj.getConnPool(task))
		return obj.waitTask(task)
	}
	return err
}

func (obj *roundTripper) newReqTask(ctx *Response) (*reqTask, error) {
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
		err = obj.poolRoundTrip(task)
		if err == nil || !task.suppertRetry() {
			break
		}
		if task.isNotice {
			currentRetry--
		}
	}
	if err == nil && ctx.option.RequestCallBack != nil {
		if err2 := ctx.option.RequestCallBack(ctx); err2 != nil {
			err = err2
		}
	}
	return err
}
