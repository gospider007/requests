package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/url"
	"sync"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	utls "github.com/refraction-networking/utls"
)

type msgClient struct {
	time time.Time
	ip   net.IP
}

type DialOption struct {
	DialTimeout time.Duration
	KeepAlive   time.Duration
	LocalAddr   *net.TCPAddr  //network card ip
	AddrType    gtls.AddrType //first ip type
	Dns         *net.UDPAddr
	GetAddrType func(host string) gtls.AddrType
}

type dialer interface {
	DialContext(ctx context.Context, network string, address string) (net.Conn, error)
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// 自定义dialer
type Dialer struct {
	dnsIpData sync.Map
	// dialer    dialer
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
func (obj *Dialer) dialContext(ctx context.Context, option *RequestOption, network string, addr Address, isProxy bool) (net.Conn, error) {
	if option == nil {
		option = &RequestOption{}
	}
	var err error
	if addr.IP == nil {
		addr.IP, err = obj.loadHost(ctx, addr.Name, option)
	}
	if option.Logger != nil {
		if isProxy {
			option.Logger(Log{
				Id:   option.requestId,
				Time: time.Now(),
				Type: LogType_ProxyDNSLookup,
				Msg:  addr.Name,
			})
		} else {
			option.Logger(Log{
				Id:   option.requestId,
				Time: time.Now(),
				Type: LogType_DNSLookup,
				Msg:  addr.Name,
			})
		}
	}
	if err != nil {
		return nil, err
	}
	con, err := newDialer(option.DialOption).DialContext(ctx, network, addr.String())
	if option.Logger != nil {
		if isProxy {
			option.Logger(Log{
				Id:   option.requestId,
				Time: time.Now(),
				Type: LogType_ProxyTCPConnect,
				Msg:  addr,
			})
		} else {
			option.Logger(Log{
				Id:   option.requestId,
				Time: time.Now(),
				Type: LogType_TCPConnect,
				Msg:  addr,
			})
		}
	}
	return con, err
}
func (obj *Dialer) DialContext(ctx context.Context, ctxData *RequestOption, network string, addr Address) (net.Conn, error) {
	return obj.dialContext(ctx, ctxData, network, addr, false)
}
func (obj *Dialer) ProxyDialContext(ctx context.Context, ctxData *RequestOption, network string, addr Address) (net.Conn, error) {
	return obj.dialContext(ctx, ctxData, network, addr, true)
}

func (obj *Dialer) DialProxyContext(ctx context.Context, ctxData *RequestOption, network string, proxyTlsConfig *tls.Config, proxyUrls ...Address) (net.Conn, error) {
	proxyLen := len(proxyUrls)
	if proxyLen < 2 {
		return nil, errors.New("proxyUrls is nil")
	}
	var conn net.Conn
	var err error
	for index := range proxyLen - 1 {
		oneProxy := proxyUrls[index]
		remoteUrl := proxyUrls[index+1]
		if index == 0 {
			conn, err = obj.dialProxyContext(ctx, ctxData, network, oneProxy)
			if err != nil {
				return conn, err
			}
		}
		conn, err = obj.verifyProxyToRemote(ctx, ctxData, conn, proxyTlsConfig, oneProxy, remoteUrl)
	}
	return conn, err
}

//	func getProxyAddr(proxyUrl *url.URL) (addr string, err error) {
//		if proxyUrl.Port() == "" {
//			switch proxyUrl.Scheme {
//			case "http":
//				addr = net.JoinHostPort(proxyUrl.Hostname(), "80")
//			case "https":
//				addr = net.JoinHostPort(proxyUrl.Hostname(), "443")
//			case "socks5":
//				addr = net.JoinHostPort(proxyUrl.Hostname(), "1080")
//			default:
//				return "", errors.New("not support scheme")
//			}
//		} else {
//			addr = net.JoinHostPort(proxyUrl.Hostname(), proxyUrl.Port())
//		}
//		return
//	}
func (obj *Dialer) dialProxyContext(ctx context.Context, ctxData *RequestOption, network string, proxyUrl Address) (net.Conn, error) {
	if ctxData == nil {
		ctxData = &RequestOption{}
	}
	return obj.ProxyDialContext(ctx, ctxData, network, proxyUrl)
}

func (obj *Dialer) verifyProxyToRemote(ctx context.Context, option *RequestOption, conn net.Conn, proxyTlsConfig *tls.Config, proxyAddress Address, remoteAddress Address) (net.Conn, error) {
	var err error
	if proxyAddress.Scheme == "https" {
		if conn, err = obj.addTls(ctx, conn, proxyAddress.Host, true, proxyTlsConfig); err != nil {
			return conn, err
		}
		if option.Logger != nil {
			option.Logger(Log{
				Id:   option.requestId,
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
			err = obj.clientVerifyHttps(ctx, conn, proxyAddress, remoteAddress)
			if option.Logger != nil {
				option.Logger(Log{
					Id:   option.requestId,
					Time: time.Now(),
					Type: LogType_ProxyConnectRemote,
					Msg:  remoteAddress.String(),
				})
			}
		case "socks5":
			err = obj.verifyTCPSocks5(conn, proxyAddress, remoteAddress)
			if option.Logger != nil {
				option.Logger(Log{
					Id:   option.requestId,
					Time: time.Now(),
					Type: LogType_ProxyTCPConnect,
					Msg:  remoteAddress.String(),
				})
			}
		}
		close(done)
	}()
	select {
	case <-ctx.Done():
		return conn, context.Cause(ctx)
	case <-done:
		return conn, err
	}
}
func (obj *Dialer) loadHost(ctx context.Context, host string, option *RequestOption) (net.IP, error) {
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
	if option.DialOption.AddrType != 0 {
		addrType = option.DialOption.AddrType
	} else if option.DialOption.GetAddrType != nil {
		addrType = option.DialOption.GetAddrType(host)
	}
	ips, err := newDialer(option.DialOption).LookupIPAddr(ctx, host)
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
func (obj *Dialer) addTls(ctx context.Context, conn net.Conn, host string, forceHttp1 bool, tlsConfig *tls.Config) (*tls.Conn, error) {
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
func (obj *Dialer) addJa3Tls(ctx context.Context, conn net.Conn, host string, forceHttp1 bool, ja3Spec ja3.Ja3Spec, tlsConfig *utls.Config) (*utls.UConn, error) {
	tlsConfig.ServerName = gtls.GetServerName(host)
	if forceHttp1 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	} else {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	return ja3.NewClient(ctx, conn, ja3Spec, forceHttp1, tlsConfig)
}
func (obj *Dialer) Socks5TcpProxy(ctx context.Context, ctxData *RequestOption, proxyAddr Address, remoteAddr Address) (conn net.Conn, err error) {
	if conn, err = obj.DialContext(ctx, ctxData, "tcp", proxyAddr); err != nil {
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
	case <-ctx.Done():
		return conn, context.Cause(ctx)
	case <-didVerify:
		return
	}
}
func (obj *Dialer) Socks5UdpProxy(ctx context.Context, ctxData *RequestOption, proxyAddress Address, remoteAddress Address) (udpConn net.PacketConn, err error) {
	conn, err := obj.ProxyDialContext(ctx, ctxData, "tcp", proxyAddress)
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
		udpConn, err = obj.verifyUDPSocks5(ctx, conn, proxyAddress, remoteAddress)
		if ctxData.Logger != nil {
			ctxData.Logger(Log{
				Id:   ctxData.requestId,
				Time: time.Now(),
				Type: LogType_ProxyConnectRemote,
				Msg:  remoteAddress.String(),
			})
		}
	}()
	select {
	case <-ctx.Done():
		return udpConn, context.Cause(ctx)
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
