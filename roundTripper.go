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

func (obj *Response) suppertRetry() bool {
	if obj.isNewConn {
		return false
	}
	if obj.request.Body == nil {
		return true
	} else if body, ok := obj.request.Body.(io.Seeker); ok {
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
	connPools map[string]chan http1.Conn
	dialer    *Dialer
	lock      sync.Mutex
}

var specClient = ja3.NewClient()

func newRoundTripper(preCtx context.Context) *roundTripper {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	return &roundTripper{
		ctx:       preCtx,
		connPools: make(map[string]chan http1.Conn),
		dialer:    new(Dialer),
	}
}

func (obj *roundTripper) getConnPool(key string) chan http1.Conn {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	val, ok := obj.connPools[key]
	if ok {
		return val
	}
	val = make(chan http1.Conn)
	obj.connPools[key] = val
	return val
}
func runConn(ctx context.Context, pool chan http1.Conn, conn http1.Conn) {
	select {
	case pool <- conn:
	case <-conn.Context().Done():
	case <-ctx.Done():
		conn.CloseWithError(context.Cause(ctx))
	}
}
func (obj *roundTripper) putConnPool(key string, con http1.Conn) {
	go runConn(obj.ctx, obj.getConnPool(key), con)
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
	if err != nil && udpConn != nil {
		udpConn.Close()
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
	conn = http3.NewConn(obj.ctx, netConn, udpConn)
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
	conn = http3.NewConn(obj.ctx, netConn, udpConn)
	return
}

func (obj *roundTripper) newConn(ctx *Response) (conn http1.Conn, err error) {
	defer func() {
		if err != nil && conn != nil {
			conn.CloseWithError(err)
		}
	}()
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
	var arch string
	if len(proxys) > 0 {
		arch = proxys[len(proxys)-1].Compression
		_, rawNetConn, err = obj.dialer.DialProxyContext(ctx, "tcp", ctx.option.TlsConfig.Clone(), append(proxys, remoteAddress)...)
	} else {
		_, rawNetConn, err = obj.dialer.DialProxyContext(ctx, "tcp", nil, remoteAddress)
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
		rawConn, h2, err = obj.dialAddTlsWithResponse(ctx, rawNetConn)
	} else {
		rawConn = rawNetConn
	}
	if arch != "" {
		rawConn, err = tools.NewCompressionConn(rawConn, arch)
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
	if h2 {
		var spec *http2.Spec
		if ctx.option.gospiderSpec != nil {
			spec = ctx.option.gospiderSpec.H2Spec
		}
		conn, err = http2.NewConn(obj.ctx, ctx.Context(), rawCon, spec)
	} else {
		conn = http1.NewConn(obj.ctx, rawCon)
	}
	return
}
func (obj *roundTripper) dialAddTlsWithResponse(ctx *Response, rawNetConn net.Conn) (tlsConn net.Conn, h2 bool, err error) {
	if ctx.option.TlsHandshakeTimeout > 0 {
		tlsCtx, tlsCnl := context.WithTimeout(ctx.Context(), ctx.option.TlsHandshakeTimeout)
		tlsConn, h2, err = obj.dialAddTls(tlsCtx, ctx.option, ctx.request, rawNetConn)
		tlsCnl()
	} else {
		tlsConn, h2, err = obj.dialAddTls(ctx.Context(), ctx.option, ctx.request, rawNetConn)
	}
	if ctx.option.Logger != nil {
		ctx.option.Logger(Log{
			Id:   ctx.requestId,
			Time: time.Now(),
			Type: LogType_TLSHandshake,
			Msg:  fmt.Sprintf("host:%s,  h2:%t", getHost(ctx.request), h2),
		})
	}
	if err != nil && tlsConn != nil {
		tlsConn.Close()
	}
	return tlsConn, h2, err
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
	ctx.connKey, err = getKey(ctx)
	if err != nil {
		return err
	}
	var conn http1.Conn
	for send := true; send; {
		select {
		case <-ctx.Context().Done():
			return context.Cause(ctx.Context())
		default:
		}
		ctx.response = nil
		select {
		case conn = <-obj.getConnPool(ctx.connKey):
			ctx.isNewConn = false
		default:
			ctx.isNewConn = true
			conn, err = obj.newConn(ctx)
		}
		if err == nil {
			err = ctx.doRequest(conn)
			if err == nil {
				break
			}
		}
		send = ctx.suppertRetry()
	}
	if err == nil && ctx.option.RequestCallBack != nil {
		if err2 := ctx.option.RequestCallBack(ctx); err2 != nil {
			err = err2
		}
	}
	return err
}
