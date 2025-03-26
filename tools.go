package requests

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	_ "unsafe"

	"github.com/gospider007/gtls"
)

func getHost(req *http.Request) string {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	_, port, _ := net.SplitHostPort(host)
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		return fmt.Sprintf("%s:%s", host, port)
	}
	return host
}

func getAddr(uurl *url.URL) (addr string) {
	if uurl == nil {
		return ""
	}
	_, port, _ := net.SplitHostPort(uurl.Host)
	if port == "" {
		if uurl.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		return fmt.Sprintf("%s:%s", uurl.Host, port)
	}
	return uurl.Host
}
func GetAddressWithUrl(uurl *url.URL) (addr Address, err error) {
	if uurl == nil {
		return Address{}, errors.New("url is nil")
	}
	addr = Address{
		Name: uurl.Hostname(),
		Host: uurl.Host,
	}
	portStr := uurl.Port()
	addr.Scheme = uurl.Scheme
	if portStr == "" {
		switch uurl.Scheme {
		case "http":
			addr.Port = 80
		case "https":
			addr.Port = 443
		case "socks5":
			addr.Port = 1080
		default:
			return Address{}, errors.New("unknown scheme")
		}
	} else {
		addr.Port, err = strconv.Atoi(portStr)
		if err != nil {
			return Address{}, err
		}
	}
	addr.IP, _ = gtls.ParseHost(uurl.Hostname())
	if uurl.User != nil {
		addr.User = uurl.User.Username()
		addr.Password, _ = uurl.User.Password()
	}
	return
}
func GetAddressWithAddr(addrS string) (addr Address, err error) {
	host, port, err := net.SplitHostPort(addrS)
	if err != nil {
		return Address{}, err
	}
	ip, _ := gtls.ParseHost(host)
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return Address{}, err
	}
	addr = Address{
		Name: host,
		IP:   ip,
		Host: addrS,
		Port: portInt,
	}
	return
}
func cloneUrl(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	r := *u
	return &r
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func removeZone(host string) string {
	if !strings.HasPrefix(host, "[") {
		return host
	}
	i := strings.LastIndex(host, "]")
	if i < 0 {
		return host
	}
	j := strings.LastIndex(host[:i], "%")
	if j < 0 {
		return host
	}
	return host[:j] + host[i:]
}

type requestBody struct {
	r io.Reader
}

func (obj *requestBody) Close() error {
	b, ok := obj.r.(io.Closer)
	if ok {
		return b.Close()
	}
	return nil
}
func (obj *requestBody) Read(p []byte) (n int, err error) {
	return obj.r.Read(p)
}
func (obj *requestBody) Seek(offset int64, whence int) (int64, error) {
	return obj.r.(io.Seeker).Seek(0, whence)
}

func NewRequestWithContext(ctx context.Context, method string, u *url.URL, body io.Reader) (*http.Request, error) {
	req := (&http.Request{}).WithContext(ctx)
	if method == "" {
		req.Method = http.MethodGet
	} else {
		req.Method = strings.ToUpper(method)
	}
	req.URL = u
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	if body != nil {
		if v, ok := body.(interface{ Len() int }); ok {
			req.ContentLength = int64(v.Len())
		}
		if _, ok := body.(io.Seeker); ok {
			req.Body = &requestBody{body}
		} else if b, ok := body.(io.ReadCloser); ok {
			req.Body = b
		} else {
			req.Body = io.NopCloser(body)
		}
	}
	return req, nil
}

func addCookie(req *http.Request, cookies Cookies) {
	result, _ := http.ParseCookie(req.Header.Get("Cookie"))
	cooks := Cookies(result)
	for _, cook := range cookies {
		if val := cooks.Get(cook.Name); val == nil {
			cooks = cooks.append(cook)
		}
	}
	if result := cooks.String(); result != "" {
		req.Header.Set("Cookie", result)
	}
}

type fakeConn struct {
	body io.ReadWriteCloser
}

func (obj *fakeConn) Read(b []byte) (n int, err error) {
	return obj.body.Read(b)
}
func (obj *fakeConn) Write(b []byte) (n int, err error) {
	return obj.body.Write(b)
}
func (obj *fakeConn) Close() error {
	return obj.body.Close()
}
func (obj *fakeConn) LocalAddr() net.Addr {
	return &net.TCPAddr{}
}
func (obj *fakeConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}
func (obj *fakeConn) SetDeadline(t time.Time) error {
	return nil
}
func (obj *fakeConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (obj *fakeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func newFakeConn(body io.ReadWriteCloser) *fakeConn {
	return &fakeConn{body: body}
}
func parseProxy(value any) ([]*url.URL, error) {
	switch proxy := value.(type) {
	case string:
		if proxy == "" {
			return nil, nil
		}
		p, err := gtls.VerifyProxy(proxy)
		if err != nil {
			return nil, err
		}
		return []*url.URL{p}, nil
	case []string:
		if len(proxy) == 0 {
			return nil, nil
		}
		proxies := make([]*url.URL, len(proxy))
		for i, p := range proxy {
			p, err := gtls.VerifyProxy(p)
			if err != nil {
				return nil, err
			}
			proxies[i] = p
		}
		return proxies, nil
	default:
		return nil, errors.New("invalid proxy type")
	}
}
