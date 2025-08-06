package requests

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gospider007/gtls"
	"github.com/gospider007/http1"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/crypto/ssh"
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

func newDialer(dialOption *DialOption) dialer {
	var option DialOption
	if dialOption != nil {
		option = *dialOption
	}
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
		Control:       Control,
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
	if addr.Port == 0 {
		return nil, errors.New("port is nil")
	}
	if addr.IP == nil {
		addr.IP, err = obj.loadHost(ctx.Context(), addr.Host, ctx.option.DialOption)
	}
	if ctx.option != nil && ctx.option.Logger != nil {
		if isProxy {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_ProxyDNSLookup,
				Msg:  addr.Host,
			})
		} else {
			ctx.option.Logger(Log{
				Id:   ctx.requestId,
				Time: time.Now(),
				Type: LogType_DNSLookup,
				Msg:  addr.Host,
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
	conn, err := obj.dialContext(ctx, network, addr, false)
	if err != nil {
		err = tools.WrapError(err, "DialContext error")
	}
	return conn, err
}
func (obj *Dialer) ProxyDialContext(ctx *Response, network string, addr Address) (net.Conn, error) {
	conn, err := obj.dialContext(ctx, network, addr, true)
	if err != nil {
		err = tools.WrapError(err, "ProxyDialContext error")
	}
	return conn, err
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
			if conn, err = obj.dialProxyContext(ctx, network, oneProxy); err != nil {
				break
			}
		}
		packCon, conn, err = obj.verifyProxyToRemote(ctx, conn, proxyTlsConfig, oneProxy, remoteUrl, index == proxyLen-2, true)
		if err != nil {
			break
		}
	}
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		if packCon != nil {
			packCon.Close()
		}
	}
	return packCon, conn, err
}
func (obj *Dialer) dialProxyContext(ctx *Response, network string, proxyUrl Address) (net.Conn, error) {
	return obj.ProxyDialContext(ctx, network, proxyUrl)
}

func (obj *Dialer) verifySSH(ctx *Response, conn net.Conn, proxyAddress Address, remoteAddress Address) (net.Conn, error) {
	if proxyAddress.User == "" || proxyAddress.Password == "" {
		return conn, errors.New("ssh proxy user or password is nil")
	}
	config := &ssh.ClientConfig{
		User:            proxyAddress.User,
		Auth:            []ssh.AuthMethod{ssh.Password(proxyAddress.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, proxyAddress.String(), config)
	if err != nil {
		return conn, err
	}
	sshC, err := ssh.NewClient(c, chans, reqs).DialContext(ctx.Context(), "tcp", remoteAddress.String())
	if err != nil {
		return conn, err
	}
	return newSSHConn(sshC, conn), nil
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
		case "ssh":
			conn, err = obj.verifySSH(ctx, conn, proxyAddress, remoteAddress)
			// log.Print("verify ssh", remoteAddress.String(), err)
		case "socks5":
			if isLast && ctx.option.ForceHttp3 {
				packCon, err = obj.verifyUDPSocks5(ctx, conn, proxyAddress, remoteAddress)
			} else {
				err = obj.verifyTCPSocks5(ctx, conn, proxyAddress, remoteAddress)
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
		if err != nil {
			err = tools.WrapError(err, "verifyProxyToRemote error")
		}
		return packCon, conn, err
	}
}

func (obj *Dialer) loadHost(ctx context.Context, host string, option *DialOption) (net.IP, error) {
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
	if option != nil {
		if option.AddrType != 0 {
			addrType = option.AddrType
		} else if option.GetAddrType != nil {
			addrType = option.GetAddrType(host)
		}
	}
	ips, err := newDialer(option).LookupIPAddr(ctx, host)
	if err != nil {
		return net.IP{}, err
	}
	if ip, err = obj.addrToIp(host, ips, addrType); err != nil {
		return nil, err
	}
	return ip, nil
}
func readUdpAddr(r io.Reader) (Address, error) {
	var UdpAddress Address
	var addrType [1]byte
	var err error
	if _, err = r.Read(addrType[:]); err != nil {
		return UdpAddress, err
	}
	switch addrType[0] {
	case ipv4Address:
		addr := make(net.IP, net.IPv4len)
		if _, err := io.ReadFull(r, addr); err != nil {
			return UdpAddress, err
		}
		UdpAddress.IP = addr
	case ipv6Address:
		addr := make(net.IP, net.IPv6len)
		if _, err := io.ReadFull(r, addr); err != nil {
			return UdpAddress, err
		}
		UdpAddress.IP = addr
	case fqdnAddress:
		if _, err := r.Read(addrType[:]); err != nil {
			return UdpAddress, err
		}
		addrLen := int(addrType[0])
		fqdn := make([]byte, addrLen)
		if _, err := io.ReadFull(r, fqdn); err != nil {
			return UdpAddress, err
		}
		UdpAddress.Host = string(fqdn)
	default:
		return UdpAddress, errors.New("invalid atyp")
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return UdpAddress, err
	}
	UdpAddress.Port = int(binary.BigEndian.Uint16(port[:]))
	return UdpAddress, nil
}
func (obj *Dialer) ReadUdpAddr(ctx context.Context, r io.Reader, option *DialOption) (Address, error) {
	udpAddress, err := readUdpAddr(r)
	if err != nil {
		return udpAddress, err
	}
	if udpAddress.Host != "" {
		udpAddress.IP, err = obj.loadHost(ctx, udpAddress.Host, option)
	}
	return udpAddress, err
}
func (obj *Dialer) addrToIp(host string, ips []net.IPAddr, addrType gtls.AddrType) (net.IP, error) {
	ip, err := obj.lookupIPAddr(ips, addrType)
	if err != nil {
		return ip, tools.WrapError(err, "addrToIp error,lookupIPAddr")
	}
	obj.dnsIpData.Store(host, msgClient{time: time.Now(), ip: ip})
	return ip, nil
}
func (obj *Dialer) verifySocks5(ctx *Response, conn net.Conn, network string, proxyAddr Address, remoteAddr Address) (proxyAddress Address, err error) {
	err = obj.verifySocks5Auth(conn, proxyAddr)
	if err != nil {
		err = tools.WrapError(err, "verifySocks5Auth error")
		return
	}
	err = obj.writeCmd(conn, network)
	if err != nil {
		err = tools.WrapError(err, "write cmd error")
		return
	}
	remoteAddr.NetWork = network
	err = WriteUdpAddr(conn, remoteAddr)
	if err != nil {
		err = tools.WrapError(err, "write addr error")
		return
	}
	readCon := make([]byte, 3)
	if _, err = io.ReadFull(conn, readCon); err != nil {
		err = tools.WrapError(err, "read socks5 proxy error")
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
	proxyAddress, err = obj.ReadUdpAddr(ctx.Context(), conn, ctx.option.DialOption)
	return
}
func (obj *Dialer) verifyTCPSocks5(ctx *Response, conn net.Conn, proxyAddr Address, remoteAddr Address) (err error) {
	_, err = obj.verifySocks5(ctx, conn, "tcp", proxyAddr, remoteAddr)
	return
}
func (obj *Dialer) verifyUDPSocks5(ctx *Response, conn net.Conn, proxyAddr Address, remoteAddr Address) (wrapConn net.PacketConn, err error) {
	remoteAddr.NetWork = "udp"
	proxyAddress, err := obj.verifySocks5(ctx, conn, "udp", proxyAddr, remoteAddr)
	if err != nil {
		return
	}
	var listener net.ListenConfig
	wrapConn, err = listener.ListenPacket(ctx.Context(), "udp", ":0")
	if err != nil {
		return
	}
	wrapConn = NewUDPConn(conn, wrapConn, &net.UDPAddr{IP: proxyAddress.IP, Port: proxyAddress.Port}, remoteAddr)
	return
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
			return tools.WrapError(err, "socks5 user or password error")
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
		if ipType := gtls.ParseIp(ipAddr.IP); ipType == gtls.Ipv4 || ipType == gtls.Ipv6 {
			if addrType == gtls.AutoIp || addrType == ipType {
				return ipAddr.IP, nil
			}
		}
	}
	for _, ipAddr := range ips {
		if ipType := gtls.ParseIp(ipAddr.IP); ipType == gtls.Ipv4 || ipType == gtls.Ipv6 {
			return ipAddr.IP, nil
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
func (obj *Dialer) addJa3Tls(ctx context.Context, conn net.Conn, host string, spec *ja3.Spec, tlsConfig *utls.Config, forceHttp1 bool) (*utls.UConn, error) {
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
		err = obj.verifyTCPSocks5(ctx, conn, proxyAddr, remoteAddr)
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
		udpConn, err = obj.verifyUDPSocks5(ctx, conn, proxyAddress, remoteAddress)
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
	if proxyAddress.User != "" && proxyAddress.Password != "" {
		hdr.Set("Proxy-Authorization", "Basic "+tools.Base64Encode(proxyAddress.User+":"+proxyAddress.Password))
	}
	connectReq, err := NewRequestWithContext(ctx, http.MethodConnect, &url.URL{Opaque: remoteAddress.String()}, nil)
	if err != nil {
		return err
	}
	connectReq.Header = hdr
	connectReq.Host = remoteAddress.Host
	if err = http1.WriteRequest(connectReq, bufio.NewWriter(conn), connectReq.Header.Clone(), nil); err != nil {
		return err
	}
	resp, err := http1.ReadResponse(bufio.NewReader(conn))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return
}
