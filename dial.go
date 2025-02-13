package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gospider007/gtls"
	"github.com/gospider007/tools"
	utls "github.com/refraction-networking/utls"
)

type msgClient struct {
	time time.Time
	ip   net.IP
}
type DialOption struct {
	LocalAddr   *net.TCPAddr //network card ip
	Dns         *net.UDPAddr
	GetAddrType func(host string) gtls.AddrType
	DialTimeout time.Duration
	KeepAlive   time.Duration
	AddrType    gtls.AddrType //first ip type
}
type dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// 自定义dialer
type Dialer struct {
	dnsIpData sync.Map
}
type myDialer struct {
	dialer *net.Dialer
}

func (d *myDialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, address)
}
func (d *myDialer) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return d.dialer.Resolver.LookupIPAddr(ctx, host)
}
func newDialer(option DialOption) dialer {
	if option.KeepAlive == 0 {
		option.KeepAlive = time.Second * 5
	}
	if option.DialTimeout == 0 {
		option.DialTimeout = time.Second * 5
	}
	var dialer myDialer
	dialer.dialer = &net.Dialer{
		Timeout:       option.DialTimeout,
		KeepAlive:     option.KeepAlive,
		LocalAddr:     option.LocalAddr,
		FallbackDelay: time.Nanosecond,
		KeepAliveConfig: net.KeepAliveConfig{
			Enable:   true,
			Idle:     time.Second * 5,
			Interval: time.Second * 5,
			Count:    3,
		},
	}
	if option.LocalAddr != nil {
		dialer.dialer.LocalAddr = option.LocalAddr
	}
	if option.Dns != nil {
		dialer.dialer.Resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return (&net.Dialer{
					Timeout:   option.DialTimeout,
					KeepAlive: option.KeepAlive,
				}).DialContext(ctx, network, option.Dns.String())
			},
		}
	}
	dialer.dialer.SetMultipathTCP(true)
	return &dialer
}
func (obj *Dialer) dialContext(ctx *Response, network string, addr Address, isProxy bool) (net.Conn, error) {
	var err error
	if addr.IP == nil {
		addr.IP, err = obj.loadHost(ctx, addr.Name)
	}
	if ctx.option != nil && ctx.option.Logger != nil {
		if isProxy {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_ProxyDNSLookup,
				Msg:  addr.Name,
			})
		} else {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_DNSLookup,
				Msg:  addr.Name,
			})
		}
	}
	if err != nil {
		return nil, err
	}
	con, err := newDialer(ctx.option.DialOption).DialContext(ctx.Context(), network, addr.String())
	if ctx.option != nil && ctx.option.Logger != nil {
		if isProxy {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_ProxyTCPConnect,
				Msg:  addr,
			})
		} else {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_TCPConnect,
				Msg:  addr,
			})
		}
	}
	return con, err
}
func (obj *Dialer) DialContext(ctx *Response, network string, addr Address) (net.Conn, error) {
	return obj.dialContext(ctx, network, addr, false)
}
func (obj *Dialer) ProxyDialContext(ctx *Response, network string, addr Address) (net.Conn, error) {
	return obj.dialContext(ctx, network, addr, true)
}
func (obj *Dialer) DialProxyContext(ctx *Response, network string, proxyTlsConfig *tls.Config, proxyUrls ...Address) (net.PacketConn, net.Conn, error) {
	proxyLen := len(proxyUrls)
	if proxyLen < 2 {
		return nil, nil, errors.New("proxyUrls is nil")
	}
	var conn net.Conn
	var err error
	var packCon net.PacketConn
	for index := range proxyLen - 1 {
		oneProxy := proxyUrls[index]
		remoteUrl := proxyUrls[index+1]
		if index == 0 {
			conn, err = obj.dialProxyContext(ctx, network, oneProxy)
			if err != nil {
				return packCon, conn, err
			}
		}
		packCon, conn, err = obj.verifyProxyToRemote(ctx, conn, proxyTlsConfig, oneProxy, remoteUrl, index == proxyLen-2, true)
	}
	return packCon, conn, err
}
func (obj *Dialer) dialProxyContext(ctx *Response, network string, proxyUrl Address) (net.Conn, error) {
	return obj.ProxyDialContext(ctx, network, proxyUrl)
}
func (obj *Dialer) verifyProxyToRemote(ctx *Response, conn net.Conn, proxyTlsConfig *tls.Config, proxyAddress Address, remoteAddress Address, isLast bool, forceHttp1 bool) (net.PacketConn, net.Conn, error) {
	var err error
	var packCon net.PacketConn
	if proxyAddress.Scheme == "https" {
		if conn, err = obj.addTls(ctx.Context(), conn, proxyAddress.Host, proxyTlsConfig, forceHttp1); err != nil {
			return packCon, conn, err
		}
		if ctx.option.Logger != nil {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_ProxyTLSHandshake,
				Msg:  proxyAddress.String(),
			})
		}
	}
	done := make(chan struct{})
	go func() {
		switch proxyAddress.Scheme {
		case "http", "https":
			err = obj.clientVerifyHttps(ctx.Context(), conn, proxyAddress, remoteAddress)
			if ctx.option.Logger != nil {
				ctx.option.Logger(Log{
					Id:   ctx.requestId,
					Time: time.Now(),
					Type: LogType_ProxyConnectRemote,
					Msg:  remoteAddress.String(),
				})
			}
		case "socks5":
			if isLast && ctx.option.ForceHttp3 {
				packCon, err = obj.verifyUDPSocks5(ctx.Context(), conn, proxyAddress, remoteAddress)
			} else {
				err = obj.verifyTCPSocks5(conn, proxyAddress, remoteAddress)
			}
			if ctx.option.Logger != nil {
				ctx.option.Logger(Log{
					Id:   ctx.requestId,
					Time: time.Now(),
					Type: LogType_ProxyConnectRemote,
					Msg:  remoteAddress.String(),
				})
			}
		}
		close(done)
	}()
	select {
	case <-ctx.Context().Done():
		return packCon, conn, context.Cause(ctx.Context())
	case <-done:
		return packCon, conn, err
	}
}
func (obj *Dialer) loadHost(ctx *Response, host string) (net.IP, error) {
	msgDataAny, ok := obj.dnsIpData.Load(host)
	if ok {
		msgdata := msgDataAny.(msgClient)
		if time.Since(msgdata.time) < time.Second*60*5 {
			return msgdata.ip, nil
		}
	}
	ip, ipInt := gtls.ParseHost(host)
	if ipInt != 0 {
		return ip, nil
	}
	var addrType gtls.AddrType
	if ctx.option.DialOption.AddrType != 0 {
		addrType = ctx.option.DialOption.AddrType
	} else if ctx.option.DialOption.GetAddrType != nil {
		addrType = ctx.option.DialOption.GetAddrType(host)
	}
	ips, err := newDialer(ctx.option.DialOption).LookupIPAddr(ctx.Context(), host)
	if err != nil {
		return net.IP{}, err
	}
	if ip, err = obj.addrToIp(host, ips, addrType); err != nil {
		return nil, err
	}
	return ip, nil
}
func (obj *Dialer) addrToIp(host string, ips []net.IPAddr, addrType gtls.AddrType) (net.IP, error) {
	ip, err := obj.lookupIPAddr(ips, addrType)
	if err != nil {
		return ip, tools.WrapError(err, "addrToIp error,lookupIPAddr")
	}
	obj.dnsIpData.Store(host, msgClient{time: time.Now(), ip: ip})
	return ip, nil
}
func (obj *Dialer) verifySocks5(conn net.Conn, network string, proxyAddr Address, remoteAddr Address) (proxyAddress Address, err error) {
	err = obj.verifySocks5Auth(conn, proxyAddr)
	if err != nil {
		return
	}
	err = obj.writeCmd(conn, network)
	if err != nil {
		return
	}
	remoteAddr.NetWork = network
	err = WriteUdpAddr(conn, remoteAddr)
	if err != nil {
		return
	}
	readCon := make([]byte, 3)
	if _, err = io.ReadFull(conn, readCon); err != nil {
		return
	}
	if readCon[0] != 5 {
		err = errors.New("socks version error")
		return
	}
	if readCon[1] != 0 {
		err = errors.New("socks conn error")
		return
	}
	proxyAddress, err = ReadUdpAddr(conn)
	return
}
func (obj *Dialer) verifyTCPSocks5(conn net.Conn, proxyAddr Address, remoteAddr Address) (err error) {
	_, err = obj.verifySocks5(conn, "tcp", proxyAddr, remoteAddr)
	return
}
func (obj *Dialer) verifyUDPSocks5(ctx context.Context, conn net.Conn, proxyAddr Address, remoteAddr Address) (wrapConn net.PacketConn, err error) {
	remoteAddr.NetWork = "udp"
	proxyAddress, err := obj.verifySocks5(conn, "udp", proxyAddr, remoteAddr)
	if err != nil {
		return nil, err
	}
	var listener net.ListenConfig
	wrapConn, err = listener.ListenPacket(ctx, "udp", ":0")
	if err != nil {
		return nil, err
	}
	wrapConn, err = NewUDPConn(wrapConn, &net.UDPAddr{IP: proxyAddress.IP, Port: proxyAddress.Port})
	if err != nil {
		return wrapConn, err
	}
	go func() {
		var buf [1]byte
		for {
			_, err := conn.Read(buf[:])
			if err != nil {
				wrapConn.Close()
				break
			}
		}
	}()
	return wrapConn, nil
}
func (obj *Dialer) writeCmd(conn net.Conn, network string) (err error) {
	var cmd byte
	switch network {
	case "tcp":
		cmd = 1
	case "udp":
		cmd = 3
	default:
		return errors.New("not support network")
	}
	_, err = conn.Write([]byte{5, cmd, 0})
	return
}
func (obj *Dialer) verifySocks5Auth(conn net.Conn, proxyAddr Address) (err error) {
	if _, err = conn.Write([]byte{5, 2, 0, 2}); err != nil {
		return
	}
	readCon := make([]byte, 2)
	if _, err = io.ReadFull(conn, readCon); err != nil {
		return
	}
	switch readCon[1] {
	case 2:
		if proxyAddr.User == "" || proxyAddr.Password == "" {
			err = errors.New("socks5 need auth")
			return
		}
		if _, err = conn.Write(append(
			append(
				[]byte{1, byte(len(proxyAddr.User))},
				tools.StringToBytes(proxyAddr.User)...,
			),
			append(
				[]byte{byte(len(proxyAddr.Password))},
				tools.StringToBytes(proxyAddr.Password)...,
			)...,
		)); err != nil {
			return
		}
		if _, err = io.ReadFull(conn, readCon); err != nil {
			return
		}
		switch readCon[1] {
		case 0:
		default:
			err = errors.New("socks5 auth error")
		}
	case 0:
	default:
		err = errors.New("not support auth format")
	}
	return
}
func (obj *Dialer) lookupIPAddr(ips []net.IPAddr, addrType gtls.AddrType) (net.IP, error) {
	for _, ipAddr := range ips {
		ip := ipAddr.IP
		if ipType := gtls.ParseIp(ip); ipType == 4 || ipType == 6 {
			if addrType == 0 || addrType == ipType {
				return ip, nil
			}
		}
	}
	for _, ipAddr := range ips {
		ip := ipAddr.IP
		if ipType := gtls.ParseIp(ip); ipType == 4 || ipType == 6 {
			return ip, nil
		}
	}
	return nil, errors.New("dns parse host error")
}
func (obj *Dialer) addTls(ctx context.Context, conn net.Conn, host string, tlsConfig *tls.Config, forceHttp1 bool) (*tls.Conn, error) {
	var tlsConn *tls.Conn
	tlsConfig.ServerName = gtls.GetServerName(host)
	if forceHttp1 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	} else {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	tlsConn = tls.Client(conn, tlsConfig)
	return tlsConn, tlsConn.HandshakeContext(ctx)
}
func (obj *Dialer) addJa3Tls(ctx context.Context, conn net.Conn, host string, spec utls.ClientHelloSpec, tlsConfig *utls.Config, forceHttp1 bool) (*utls.UConn, error) {
	return specClient.Client(ctx, conn, spec, tlsConfig, gtls.GetServerName(host), forceHttp1)
}
func (obj *Dialer) Socks5TcpProxy(ctx *Response, proxyAddr Address, remoteAddr Address) (conn net.Conn, err error) {
	if conn, err = obj.DialContext(ctx, "tcp", proxyAddr); err != nil {
		return
	}
	defer func() {
		if err != nil && conn != nil {
			conn.Close()
		}
	}()
	didVerify := make(chan struct{})
	go func() {
		defer close(didVerify)
		err = obj.verifyTCPSocks5(conn, proxyAddr, remoteAddr)
	}()
	select {
	case <-ctx.Context().Done():
		return conn, context.Cause(ctx.Context())
	case <-didVerify:
		return
	}
}
func (obj *Dialer) Socks5UdpProxy(ctx *Response, proxyAddress Address, remoteAddress Address) (udpConn net.PacketConn, err error) {
	conn, err := obj.ProxyDialContext(ctx, "tcp", proxyAddress)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if conn != nil {
				conn.Close()
			}
			if udpConn != nil {
				udpConn.Close()
			}
		}
	}()
	didVerify := make(chan struct{})
	go func() {
		defer close(didVerify)
		udpConn, err = obj.verifyUDPSocks5(ctx.Context(), conn, proxyAddress, remoteAddress)
		if ctx.option.Logger != nil {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_ProxyConnectRemote,
				Msg:  remoteAddress.String(),
			})
		}
	}()
	select {
	case <-ctx.Context().Done():
		return udpConn, context.Cause(ctx.Context())
	case <-didVerify:
		return
	}
}
func (obj *Dialer) clientVerifyHttps(ctx context.Context, conn net.Conn, proxyAddress Address, remoteAddress Address) (err error) {
	hdr := make(http.Header)
	hdr.Set("User-Agent", tools.UserAgent)
	if proxyAddress.User != "" && proxyAddress.Password != "" {
		hdr.Set("Proxy-Authorization", "Basic "+tools.Base64Encode(proxyAddress.User+":"+proxyAddress.Password))
	}
	connectReq, err := NewRequestWithContext(ctx, http.MethodConnect, &url.URL{Opaque: remoteAddress.String()}, nil)
	if err != nil {
		return err
	}
	connectReq.Header = hdr
	connectReq.Host = remoteAddress.Host
	if err = connectReq.Write(conn); err != nil {
		return err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), connectReq)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return
}
