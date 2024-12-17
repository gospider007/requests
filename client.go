package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/url"

	"net/http"

	"github.com/gospider007/gtls"
	utls "github.com/refraction-networking/utls"
)

// Connection Management
type Client struct {
	option    ClientOption
	transport *roundTripper
	ctx       context.Context
	cnl       context.CancelFunc
	closed    bool
}

var defaultClient, _ = NewClient(context.TODO())

// New Connection Management
func NewClient(preCtx context.Context, options ...ClientOption) (*Client, error) {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	var option ClientOption
	if len(options) > 0 {
		option = options[0]
	}
	result := new(Client)
	result.ctx, result.cnl = context.WithCancel(preCtx)
	result.transport = newRoundTripper(result.ctx, option)
	result.option = option
	if result.option.TlsConfig == nil {
		result.option.TlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(0),
		}
	}
	if result.option.UtlsConfig == nil {
		result.option.UtlsConfig = &utls.Config{
			InsecureSkipVerify:                 true,
			ClientSessionCache:                 utls.NewLRUClientSessionCache(0),
			InsecureSkipTimeVerify:             true,
			OmitEmptyPsk:                       true,
			PreferSkipResumptionOnNilExtension: true,
		}
	}
	//cookiesjar
	if !result.option.DisCookie {
		if result.option.Jar == nil {
			result.option.Jar = NewJar()
		}
	}
	var err error
	if result.option.Proxy != "" {
		_, err = gtls.VerifyProxy(result.option.Proxy)
	}
	return result, err
}

// Modifying the client's proxy
func (obj *Client) SetProxy(proxyUrl string) (err error) {
	_, err = gtls.VerifyProxy(proxyUrl)
	if err == nil {
		obj.option.Proxy = proxyUrl
	}
	return
}

// Modify the proxy method of the client
func (obj *Client) SetGetProxy(getProxy func(ctx context.Context, url *url.URL) (string, error)) {
	obj.option.GetProxy = getProxy
}

// Modifying the client's proxy
func (obj *Client) SetProxys(proxyUrls []string) (err error) {
	for _, proxy := range proxyUrls {
		_, err = gtls.VerifyProxy(proxy)
		if err != nil {
			return
		}
	}
	obj.option.Proxys = proxyUrls
	return
}

// Modify the proxy method of the client
func (obj *Client) SetGetProxys(getProxys func(ctx context.Context, url *url.URL) ([]string, error)) {
	obj.option.GetProxys = getProxys
}

// Close idle connections. If the connection is in use, wait until it ends before closing
func (obj *Client) CloseConns() {
	obj.transport.closeConns()
}

// Close the connection, even if it is in use, it will be closed
func (obj *Client) ForceCloseConns() {
	obj.transport.forceCloseConns()
}

// Close the client and cannot be used again after shutdown
func (obj *Client) Close() {
	obj.closed = true
	obj.ForceCloseConns()
	obj.cnl()
}

func (obj *Client) do(req *http.Request, option *RequestOption) (resp *http.Response, err error) {
	var redirectNum int
	for {
		redirectNum++
		resp, err = obj.send(req, option)
		if req.Body != nil {
			req.Body.Close()
		}
		if err != nil {
			return
		}
		if option.MaxRedirect < 0 { //dis redirect
			return
		}
		if option.MaxRedirect > 0 && redirectNum > option.MaxRedirect {
			return
		}
		loc := resp.Header.Get("Location")
		if loc == "" {
			return resp, nil
		}
		u, err := req.URL.Parse(loc)
		if err != nil {
			return resp, fmt.Errorf("failed to parse Location header %q: %v", loc, err)
		}
		ireq, err := NewRequestWithContext(req.Context(), http.MethodGet, u, nil)
		if err != nil {
			return resp, err
		}
		var shouldRedirect bool
		ireq.Method, shouldRedirect, _ = redirectBehavior(req.Method, resp, ireq)
		if !shouldRedirect {
			return resp, nil
		}
		ireq.Response = resp
		ireq.Header = defaultHeaders()
		ireq.Header.Set("Referer", req.URL.String())
		for key := range ireq.Header {
			if val := req.Header.Get(key); val != "" {
				ireq.Header.Set(key, val)
			}
		}
		if getDomain(u) == getDomain(req.URL) {
			if Authorization := req.Header.Get("Authorization"); Authorization != "" {
				ireq.Header.Set("Authorization", Authorization)
			}
			cookies := Cookies(req.Cookies()).String()
			if cookies != "" {
				ireq.Header.Set("Cookie", cookies)
			}
			addCookie(ireq, resp.Cookies())
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		req = ireq
	}
}
func (obj *Client) send(req *http.Request, option *RequestOption) (resp *http.Response, err error) {
	if option.Jar != nil {
		addCookie(req, option.Jar.GetCookies(req.URL))
	}
	resp, err = obj.transport.RoundTrip(req)
	if option.Jar != nil && resp != nil {
		if rc := resp.Cookies(); len(rc) > 0 {
			option.Jar.SetCookies(req.URL, rc)
		}
	}
	return resp, err
}
