package requests

import (
	"context"
	"net"
	"net/url"
	"time"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
)

// Connection Management Options
type ClientOption struct {
	ForceHttp1            bool                                                                            //force  use http1 send requests
	OrderHeaders          []string                                                                        //order headers with http1
	Ja3                   bool                                                                            //enable ja3 fingerprint
	Ja3Spec               ja3.Ja3Spec                                                                     //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec             ja3.H2Ja3Spec                                                                   //h2 fingerprint
	Proxy                 string                                                                          //proxy,support https,http,socks5
	DisCookie             bool                                                                            //disable cookies
	DisDecode             bool                                                                            //disable auto decode
	DisUnZip              bool                                                                            //disable auto zip decode
	DisAlive              bool                                                                            //disable  keepalive
	Bar                   bool                                                                            ////enable bar display
	Timeout               time.Duration                                                                   //request timeout
	OptionCallBack        func(ctx context.Context, client *Client, option *RequestOption) error          //option callback,if error is returnd, break request
	ResultCallBack        func(ctx context.Context, client *Client, response *Response) error             //result callback,if error is returnd,next errCallback
	ErrCallBack           func(ctx context.Context, client *Client, response *Response, err error) error  //error callback,if error is returnd,break request
	RequestCallBack       func(ctx context.Context, request *http.Request, response *http.Response) error //request and response callback,if error is returnd,reponse is error
	TryNum                int                                                                             //try num
	MaxRedirectNum        int                                                                             //redirect num ,<0 no redirect,==0 no limit
	Headers               any                                                                             //default headers
	ResponseHeaderTimeout time.Duration                                                                   //ResponseHeaderTimeout ,default:30
	TlsHandshakeTimeout   time.Duration                                                                   //tls timeout,default:15

	//network card ip
	DialTimeout time.Duration //dial tcp timeout,default:15
	KeepAlive   time.Duration //keepalive,default:30
	LocalAddr   *net.TCPAddr
	Dns         *net.UDPAddr  //dns
	AddrType    gtls.AddrType //dns parse addr type
	Jar         *Jar          //custom cookies

	GetProxy    func(ctx context.Context, url *url.URL) (string, error) //proxy callback:support https,http,socks5 proxy
	GetAddrType func(host string) gtls.AddrType
}

// Connection Management
type Client struct {
	forceHttp1   bool
	orderHeaders []string

	jar            *Jar
	maxRedirectNum int
	disDecode      bool
	disUnZip       bool
	disAlive       bool

	tryNum int

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
			ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
			if ctxData.maxRedirectNum == 0 || ctxData.maxRedirectNum >= len(via) {
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
		maxRedirectNum:        option.MaxRedirectNum,
		disDecode:             option.DisDecode,
		disUnZip:              option.DisUnZip,
		disAlive:              option.DisAlive,
		tryNum:                option.TryNum,
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
