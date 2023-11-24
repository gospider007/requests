package requests

import (
	"context"
	"net/url"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
)

// Connection Management
type Client struct {
	forceHttp1   bool
	orderHeaders []string

	jar         *Jar
	maxRedirect int
	disDecode   bool
	disUnZip    bool
	disAlive    bool

	maxRetries int

	requestCallBack func(context.Context, *http.Request, *http.Response) error

	optionCallBack func(context.Context, *Client, *RequestOption) error
	resultCallBack func(ctx context.Context, client *Client, response *Response) error
	errCallBack    func(context.Context, *Client, *Response, error) error

	timeout               time.Duration
	responseHeaderTimeout time.Duration
	tlsHandshakeTimeout   time.Duration

	headers any
	bar     bool

	disCookie bool
	client    *http.Client
	proxy     *url.URL

	ctx       context.Context
	cnl       context.CancelFunc
	transport *RoundTripper

	ja3Spec   ja3.Ja3Spec
	h2Ja3Spec ja3.H2Ja3Spec

	addrType gtls.AddrType
}

var defaultClient, _ = NewClient(nil)

// New Connection Management
func NewClient(preCtx context.Context, options ...ClientOption) (*Client, error) {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	var option ClientOption

	if len(options) > 0 {
		option = options[0]
	}
	transport := newRoundTripper(ctx, roundTripperOption{
		DialTimeout: option.DialTimeout,
		KeepAlive:   option.KeepAlive,

		LocalAddr:   option.LocalAddr,
		AddrType:    option.AddrType,
		GetAddrType: option.GetAddrType,
		Dns:         option.Dns,
		GetProxy:    option.GetProxy,
	})
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			ctxData := GetReqCtxData(req.Context())
			if ctxData.maxRedirect == 0 || ctxData.maxRedirect >= len(via) {
				return nil
			}
			return http.ErrUseLastResponse
		},
	}
	result := &Client{
		ctx:                   ctx,
		cnl:                   cnl,
		client:                client,
		transport:             transport,
		forceHttp1:            option.ForceHttp1,
		requestCallBack:       option.RequestCallBack,
		orderHeaders:          option.OrderHeaders,
		disCookie:             option.DisCookie,
		maxRedirect:           option.MaxRedirect,
		disDecode:             option.DisDecode,
		disUnZip:              option.DisUnZip,
		disAlive:              option.DisAlive,
		maxRetries:            option.MaxRetries,
		optionCallBack:        option.OptionCallBack,
		resultCallBack:        option.ResultCallBack,
		errCallBack:           option.ErrCallBack,
		timeout:               option.Timeout,
		responseHeaderTimeout: option.ResponseHeaderTimeout,
		tlsHandshakeTimeout:   option.TlsHandshakeTimeout,
		headers:               option.Headers,
		bar:                   option.Bar,
		addrType:              option.AddrType,
	}
	//cookiesjar
	if !option.DisCookie {
		if option.Jar != nil {
			result.jar = option.Jar
		} else {
			result.jar = NewJar()
		}
		result.client.Jar = result.jar.jar
	}
	var err error
	if option.Proxy != "" {
		result.proxy, err = gtls.VerifyProxy(option.Proxy)
	}
	if option.Ja3Spec.IsSet() {
		result.ja3Spec = option.Ja3Spec
	} else if option.Ja3 {
		result.ja3Spec = ja3.DefaultJa3Spec()
	}
	result.h2Ja3Spec = option.H2Ja3Spec
	return result, err
}

// Modifying the client's proxy
func (obj *Client) SetProxy(proxyUrl string) (err error) {
	obj.proxy, err = gtls.VerifyProxy(proxyUrl)
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
