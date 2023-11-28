package requests

import (
	"context"
	"net/url"

	"net/http"

	"github.com/gospider007/gtls"
)

// Connection Management
type Client struct {
	option    ClientOption
	client    *http.Client
	ctx       context.Context
	cnl       context.CancelFunc
	transport *roundTripper
}

var defaultClient, _ = NewClient(nil)

func checkRedirect(req *http.Request, via []*http.Request) error {
	ctxData := GetReqCtxData(req.Context())
	if ctxData.maxRedirect == 0 || ctxData.maxRedirect >= len(via) {
		return nil
	}
	return http.ErrUseLastResponse
}

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
	result.client = &http.Client{Transport: result.transport, CheckRedirect: checkRedirect}
	result.option = option
	//cookiesjar
	if !result.option.DisCookie {
		if result.option.Jar == nil {
			result.option.Jar = NewJar()
		}
		result.client.Jar = result.option.Jar.jar
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
	obj.transport.setGetProxy(getProxy)
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
	obj.ForceCloseConns()
	obj.cnl()
}

func (obj *Client) getClient(option *RequestOption) *http.Client {
	if option.DisCookie {
		return &http.Client{
			Transport:     obj.client.Transport,
			CheckRedirect: obj.client.CheckRedirect,
			Timeout:       obj.client.Timeout,
		}
	}
	if option.Jar != nil {
		return &http.Client{
			Transport:     obj.client.Transport,
			CheckRedirect: obj.client.CheckRedirect,
			Timeout:       obj.client.Timeout,
			Jar:           option.Jar.jar,
		}
	}
	return obj.client
}
