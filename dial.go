package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/url"
	"strconv"
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
	host string
}

type DialOption struct {
	DialTimeout time.Duration
	KeepAlive   time.Duration
	LocalAddr   *net.TCPAddr  //network card ip
	AddrType    gtls.AddrType //first ip type
	Dns         *net.UDPAddr
	GetAddrType func(host string) gtls.AddrType
	// EnableNetPoll bool
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
func (obj *Dialer) dialContext(ctx context.Context, option *RequestOption, network string, addr string, isProxy bool) (net.Conn, error) {
	if option == nil {
		option = &RequestOption{}
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, tools.WrapError(err, "addrToIp error,SplitHostPort")
	}
	dialer := newDialer(option.DialOption)
	if _, ipInt := gtls.ParseHost(host); ipInt == 0 { //domain
		host, ok := obj.loadHost(host)
		if !ok { //dns parse
			var addrType gtls.AddrType
			if option.DialOption.AddrType != 0 {
				addrType = option.DialOption.AddrType
			} else if option.DialOption.GetAddrType != nil {
				addrType = option.DialOption.GetAddrType(host)
			}
			ips, err := dialer.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if host, err = obj.addrToIp(host, ips, addrType); err != nil {
				return nil, err
			}
			if option.Logger != nil {
				if isProxy {
					option.Logger(Log{
						Id:   option.requestId,
						Time: time.Now(),
						Type: LogType_ProxyDNSLookup,
						Msg:  host,
					})
				} else {
					option.Logger(Log{
						Id:   option.requestId,
						Time: time.Now(),
						Type: LogType_DNSLookup,
						Msg:  host,
					})
				}
			}
			addr = net.JoinHostPort(host, port)
		}
	}
	con, err := dialer.DialContext(ctx, network, addr)
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
func (obj *Dialer) DialContext(ctx context.Context, ctxData *RequestOption, network string, addr string) (net.Conn, error) {
	return obj.dialContext(ctx, ctxData, network, addr, false)
}
func (obj *Dialer) ProxyDialContext(ctx context.Context, ctxData *RequestOption, network string, addr string) (net.Conn, error) {
	return obj.dialContext(ctx, ctxData, network, addr, true)
}

func (obj *Dialer) DialProxyContext(ctx context.Context, ctxData *RequestOption, network string, proxyTlsConfig *tls.Config, proxyUrls ...*url.URL) (net.Conn, error) {
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
func getProxyAddr(proxyUrl *url.URL) (addr string, err error) {
	if proxyUrl.Port() == "" {
		switch proxyUrl.Scheme {
		case "http":
			addr = net.JoinHostPort(proxyUrl.Hostname(), "80")
		case "https":
			addr = net.JoinHostPort(proxyUrl.Hostname(), "443")
		case "socks5":
			addr = net.JoinHostPort(proxyUrl.Hostname(), "1080")
		default:
			return "", errors.New("not support scheme")
		}
	} else {
		addr = net.JoinHostPort(proxyUrl.Hostname(), proxyUrl.Port())
	}
	return
}
func (obj *Dialer) dialProxyContext(ctx context.Context, ctxData *RequestOption, network string, proxyUrl *url.URL) (net.Conn, error) {
	if proxyUrl == nil {
		return nil, errors.New("proxyUrl is nil")
	}
	if ctxData == nil {
		ctxData = &RequestOption{}
	}
	addr, err := getProxyAddr(proxyUrl)
	if err != nil {
		return nil, err
	}
	return obj.ProxyDialContext(ctx, ctxData, network, addr)
}

func (obj *Dialer) verifyProxyToRemote(ctx context.Context, option *RequestOption, conn net.Conn, proxyTlsConfig *tls.Config, proxyUrl *url.URL, remoteUrl *url.URL) (net.Conn, error) {
	var err error
	if proxyUrl.Scheme == "https" {
		if conn, err = obj.addTls(ctx, conn, proxyUrl.Host, true, proxyTlsConfig); err != nil {
			return conn, err
		}
		if option.Logger != nil {
			option.Logger(Log{
				Id:   option.requestId,
				Time: time.Now(),
				Type: LogType_ProxyTLSHandshake,
				Msg:  proxyUrl.String(),
			})
		}
	}
	done := make(chan struct{})
	go func() {
		switch proxyUrl.Scheme {
		case "http", "https":
			err = obj.clientVerifyHttps(ctx, conn, proxyUrl, remoteUrl)
			if option.Logger != nil {
				option.Logger(Log{
					Id:   option.requestId,
					Time: time.Now(),
					Type: LogType_ProxyConnectRemote,
					Msg:  remoteUrl.String(),
				})
			}
		case "socks5":
			err = obj.clientVerifySocks5(conn, proxyUrl, remoteUrl)
			if option.Logger != nil {
				option.Logger(Log{
					Id:   option.requestId,
					Time: time.Now(),
					Type: LogType_ProxyTCPConnect,
					Msg:  remoteUrl.String(),
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
func (obj *Dialer) loadHost(host string) (string, bool) {
	msgDataAny, ok := obj.dnsIpData.Load(host)
	if ok {
		msgdata := msgDataAny.(msgClient)
		if time.Since(msgdata.time) < time.Second*60*5 {
			return msgdata.host, true
		}
	}
	return host, false
}
func (obj *Dialer) addrToIp(host string, ips []net.IPAddr, addrType gtls.AddrType) (string, error) {
	ip, err := obj.lookupIPAddr(ips, addrType)
	if err != nil {
		return host, tools.WrapError(err, "addrToIp error,lookupIPAddr")
	}
	obj.dnsIpData.Store(host, msgClient{time: time.Now(), host: ip.String()})
	return ip.String(), nil
}
func (obj *Dialer) clientVerifySocks5(conn net.Conn, proxyUrl *url.URL, remoteUrl *url.URL) (err error) {
	if _, err = conn.Write([]byte{5, 2, 0, 2}); err != nil {
		return
	}
	readCon := make([]byte, 4)
	if _, err = io.ReadFull(conn, readCon[:2]); err != nil {
		return
	}
	switch readCon[1] {
	case 2:
		if proxyUrl.User == nil {
			err = errors.New("socks5 need auth")
			return
		}
		pwd, pwdOk := proxyUrl.User.Password()
		if !pwdOk {
			err = errors.New("socks5 auth error")
			return
		}
		usr := proxyUrl.User.Username()

		if usr == "" {
			err = errors.New("socks5 auth user format error")
			return
		}
		if _, err = conn.Write(append(
			append(
				[]byte{1, byte(len(usr))},
				tools.StringToBytes(usr)...,
			),
			append(
				[]byte{byte(len(pwd))},
				tools.StringToBytes(pwd)...,
			)...,
		)); err != nil {
			return
		}
		if _, err = io.ReadFull(conn, readCon[:2]); err != nil {
			return
		}
		switch readCon[1] {
		case 0:
		default:
			err = errors.New("socks5 auth error")
			return
		}
	case 0:
	default:
		err = errors.New("not support auth format")
		return
	}

	host := remoteUrl.Hostname()
	var port int
	if cport := remoteUrl.Port(); cport == "" {
		switch remoteUrl.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		case "socks5":
			port = 1080
		default:
			err = errors.New("not support scheme")
			return
		}
	} else {
		port, err = strconv.Atoi(cport)
		if err != nil {
			return
		}
	}
	writeCon := []byte{5, 1, 0}
	ip, ipInt := gtls.ParseHost(host)
	switch ipInt {
	case 4:
		writeCon = append(writeCon, 1)
		writeCon = append(writeCon, ip...)
	case 6:
		writeCon = append(writeCon, 4)
		writeCon = append(writeCon, ip...)
	case 0:
		if len(host) > 255 {
			err = errors.New("FQDN too long")
			return
		}
		writeCon = append(writeCon, 3)
		writeCon = append(writeCon, byte(len(host)))
		writeCon = append(writeCon, host...)
	}
	writeCon = append(writeCon, byte(port>>8), byte(port))
	if _, err = conn.Write(writeCon); err != nil {
		return
	}
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
	if readCon[3] != 1 {
		err = errors.New("socks conn type error")
		return
	}

	switch readCon[3] {
	case 1: //ipv4
		if _, err = io.ReadFull(conn, readCon); err != nil {
			return
		}
	case 3: //domain
		if _, err = io.ReadFull(conn, readCon[:1]); err != nil {
			return
		}
		if _, err = io.ReadFull(conn, make([]byte, readCon[0])); err != nil {
			return
		}
	case 4: //IPv6
		if _, err = io.ReadFull(conn, make([]byte, 16)); err != nil {
			return
		}
	default:
		err = errors.New("invalid atyp")
		return
	}
	_, err = io.ReadFull(conn, readCon[:2])
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

//	func (obj *Dialer) getDialer(option *RequestOption) *net.Dialer {
//		return newDialer(option.DialOption)
//	}
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
func (obj *Dialer) Socks5Proxy(ctx context.Context, ctxData *RequestOption, network string, proxyUrl *url.URL, remoteUrl *url.URL) (conn net.Conn, err error) {
	if conn, err = obj.DialContext(ctx, ctxData, network, net.JoinHostPort(proxyUrl.Hostname(), proxyUrl.Port())); err != nil {
		return
	}
	defer func() {
		if err != nil && conn != nil {
			conn.Close()
		}
	}()
	didVerify := make(chan struct{})
	go func() {
		err = obj.clientVerifySocks5(conn, proxyUrl, remoteUrl)
		close(didVerify)
	}()
	select {
	case <-ctx.Done():
		return conn, context.Cause(ctx)
	case <-didVerify:
		return
	}
}
func (obj *Dialer) clientVerifyHttps(ctx context.Context, conn net.Conn, proxyUrl *url.URL, remoteUrl *url.URL) (err error) {
	hdr := make(http.Header)
	hdr.Set("User-Agent", tools.UserAgent)
	if proxyUrl.User != nil {
		if password, ok := proxyUrl.User.Password(); ok {
			hdr.Set("Proxy-Authorization", "Basic "+tools.Base64Encode(proxyUrl.User.Username()+":"+password))
		}
	}
	addr, err := getProxyAddr(remoteUrl)
	if err != nil {
		return err
	}
	connectReq, err := NewRequestWithContext(ctx, http.MethodConnect, &url.URL{Opaque: addr}, nil)
	if err != nil {
		return err
	}
	connectReq.Header = hdr
	connectReq.Host = remoteUrl.Host
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
