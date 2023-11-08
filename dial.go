package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	utls "github.com/refraction-networking/utls"
)

type DialClient struct {
	dns         *net.UDPAddr
	dialer      *net.Dialer
	dnsIpData   sync.Map
	addrType    AddrType
	getAddrType func(string) AddrType
	ctx         context.Context
}
type msgClient struct {
	time time.Time
	host string
}
type AddrType int

const (
	Auto AddrType = 0
	Ipv4 AddrType = 4
	Ipv6 AddrType = 6
)

type DialOption struct {
	DialTimeout time.Duration
	KeepAlive   time.Duration
	LocalAddr   *net.TCPAddr //network card ip
	AddrType    AddrType     //first ip type
	GetAddrType func(string) AddrType
	Dns         net.IP
}

func NewDail(ctx context.Context, option DialOption) *DialClient {
	if ctx == nil {
		ctx = context.TODO()
	}
	if option.KeepAlive == 0 {
		option.KeepAlive = time.Second * 30
	}
	if option.DialTimeout == 0 {
		option.DialTimeout = time.Second * 15
	}
	dialCli := &DialClient{
		ctx: ctx,
		dialer: &net.Dialer{
			Timeout:   option.DialTimeout,
			KeepAlive: option.KeepAlive,
			LocalAddr: option.LocalAddr,
		},
		addrType:    option.AddrType,
		getAddrType: option.GetAddrType,
	}
	if option.Dns != nil {
		dialCli.dns = &net.UDPAddr{IP: option.Dns, Port: 53}
		dialCli.dialer.Resolver = &net.Resolver{
			PreferGo: true,
			Dial:     dialCli.DnsDialContext,
		}
	}
	dialCli.dialer.SetMultipathTCP(true)
	return dialCli
}
func (obj *DialClient) loadHost(host string) (string, bool) {
	msgDataAny, ok := obj.dnsIpData.Load(host)
	if ok {
		msgdata := msgDataAny.(msgClient)
		if time.Since(msgdata.time) < time.Second*60*5 {
			return msgdata.host, true
		}
	}
	return host, false
}
func (obj *DialClient) AddrToIp(ctx context.Context, addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, tools.WrapError(err, "addrToIp error,SplitHostPort")
	}
	_, ipInt := gtls.ParseHost(host)
	if ipInt == 4 || ipInt == 6 {
		return addr, nil
	}
	host, ok := obj.loadHost(host)
	if !ok {
		ip, err := obj.lookupIPAddr(ctx, host)
		if err != nil {
			return addr, tools.WrapError(err, "addrToIp error,lookupIPAddr")
		}
		host = ip.String()
		obj.dnsIpData.Store(addr, msgClient{time: time.Now(), host: host})
	}
	return net.JoinHostPort(host, port), nil
}
func (obj *DialClient) clientVerifySocks5(ctx context.Context, proxyUrl *url.URL, addr string, conn net.Conn) (err error) {
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
	var host string
	var port int
	if host, port, err = gtls.SplitHostPort(addr); err != nil {
		return
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
func (obj *DialClient) lookupIPAddr(ctx context.Context, host string) (net.IP, error) {
	var addrType int
	if obj.addrType != 0 {
		addrType = int(obj.addrType)
	} else if obj.getAddrType != nil {
		addrType = int(obj.getAddrType(host))
	}
	ips, err := obj.dialer.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
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
func (obj *DialClient) DnsDialContext(ctx context.Context, netword string, addr string) (net.Conn, error) {
	if obj.dns != nil {
		return obj.dialer.DialContext(ctx, netword, obj.dns.String())
	}
	return obj.dialer.DialContext(ctx, netword, addr)
}
func (obj *DialClient) DialContext(ctx context.Context, netword string, addr string) (net.Conn, error) {
	revHost, err := obj.AddrToIp(ctx, addr)
	if err != nil {
		return nil, err
	}
	return obj.dialer.DialContext(ctx, netword, revHost)
}
func (obj *DialClient) Dialer() *net.Dialer {
	return obj.dialer
}
func (obj *DialClient) AddTls(ctx context.Context, conn net.Conn, host string, disHttp2 bool, tlsConfig *tls.Config) (*tls.Conn, error) {
	var tlsConn *tls.Conn
	tlsConfig.ServerName = gtls.GetServerName(host)
	if disHttp2 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	} else {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	tlsConn = tls.Client(conn, tlsConfig)
	return tlsConn, tlsConn.HandshakeContext(ctx)
}
func (obj *DialClient) AddJa3Tls(ctx context.Context, conn net.Conn, host string, disHttp2 bool, ja3Spec ja3.Ja3Spec, tlsConfig *utls.Config) (*utls.UConn, error) {
	tlsConfig.ServerName = gtls.GetServerName(host)
	if disHttp2 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	} else {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
	}
	return ja3.NewClient(ctx, conn, ja3Spec, disHttp2, tlsConfig)
}
func (obj *DialClient) Socks5Proxy(ctx context.Context, network string, addr string, proxyUrl *url.URL) (conn net.Conn, err error) {
	defer func() {
		if err != nil && conn != nil {
			conn.Close()
		}
	}()
	if conn, err = obj.DialContext(ctx, network, net.JoinHostPort(proxyUrl.Hostname(), proxyUrl.Port())); err != nil {
		return
	}
	didVerify := make(chan struct{})
	go func() {
		defer close(didVerify)
		err = obj.clientVerifySocks5(ctx, proxyUrl, addr, conn)
	}()
	select {
	case <-ctx.Done():
		return conn, ctx.Err()
	case <-didVerify:
		return
	}
}
func (obj *DialClient) clientVerifyHttps(ctx context.Context, scheme string, proxyUrl *url.URL, addr string, host string, conn net.Conn) (err error) {
	hdr := make(http.Header)
	hdr.Set("User-Agent", UserAgent)
	if proxyUrl.User != nil {
		if password, ok := proxyUrl.User.Password(); ok {
			hdr.Set("Proxy-Authorization", "Basic "+tools.Base64Encode(proxyUrl.User.Username()+":"+password))
		}
	}
	cHost := host
	_, hport, _ := net.SplitHostPort(host)
	if hport == "" {
		_, aport, _ := net.SplitHostPort(addr)
		if aport != "" {
			cHost = net.JoinHostPort(cHost, aport)
		} else if scheme == "http" {
			cHost = net.JoinHostPort(cHost, "80")
		} else if scheme == "https" {
			cHost = net.JoinHostPort(cHost, "443")
		} else {
			return errors.New("clientVerifyHttps not found port")
		}
	}

	var resp *http.Response
	didReadResponse := make(chan struct{}) // closed after CONNECT write+read is done or fails
	go func() {
		defer close(didReadResponse)
		connectReq := &http.Request{
			Method: http.MethodConnect,
			URL:    &url.URL{Opaque: addr},
			Host:   cHost,
			Header: hdr,
		}
		if err = connectReq.Write(conn); err != nil {
			return
		}
		resp, err = http.ReadResponse(bufio.NewReader(conn), connectReq)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-didReadResponse:
	}
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		_, text, ok := strings.Cut(resp.Status, " ")
		if !ok {
			return errors.New("unknown status code")
		}
		return errors.New(text)
	}
	return
}
func (obj *DialClient) DialContextWithProxy(ctx context.Context, netword string, scheme string, addr string, host string, proxyUrl *url.URL, tlsConfig *tls.Config) (net.Conn, error) {
	if proxyUrl == nil {
		return obj.DialContext(ctx, netword, addr)
	}
	if proxyUrl.Port() == "" {
		if proxyUrl.Scheme == "http" {
			proxyUrl.Host = net.JoinHostPort(proxyUrl.Hostname(), "80")
		} else if proxyUrl.Scheme == "https" {
			proxyUrl.Host = net.JoinHostPort(proxyUrl.Hostname(), "443")
		}
	}
	switch proxyUrl.Scheme {
	case "http", "https":
		conn, err := obj.DialContext(ctx, netword, net.JoinHostPort(proxyUrl.Hostname(), proxyUrl.Port()))
		if err != nil {
			return conn, err
		} else if proxyUrl.Scheme == "https" {
			if conn, err = obj.AddTls(ctx, conn, proxyUrl.Host, true, tlsConfig); err != nil {
				return conn, err
			}
		}
		return conn, obj.clientVerifyHttps(ctx, scheme, proxyUrl, addr, host, conn)
	case "socks5":
		return obj.Socks5Proxy(ctx, netword, addr, proxyUrl)
	default:
		return nil, errors.New("proxyUrl Scheme error")
	}
}
