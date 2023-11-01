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

type ClientOption struct {
	ForceHttp1      bool                                                       //force  use http1 send requests
	OrderHeaders    []string                                                   //order headers with http1
	Ja3             bool                                                       //enable ja3 fingerprint
	Ja3Spec         ja3.Ja3Spec                                                //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec       ja3.H2Ja3Spec                                              //h2 fingerprint
	Proxy           string                                                     //proxy,support https,http,socks5
	DisCookie       bool                                                       //disable cookies
	DisDecode       bool                                                       //disable auto decode
	DisUnZip        bool                                                       //disable auto zip decode
	DisAlive        bool                                                       //disable  keepalive
	Bar             bool                                                       ////enable bar display
	Timeout         time.Duration                                              //request timeout
	OptionCallBack  func(context.Context, *Client, *RequestOption) error       //option callback,if error is returnd, break request
	ResultCallBack  func(context.Context, *Client, *Response) error            //result callback,if error is returnd,next errCallback
	ErrCallBack     func(context.Context, *Client, error) error                //error callback,if error is returnd,break request
	RequestCallBack func(context.Context, *http.Request, *http.Response) error //request and response callback,if error is returnd,reponse is error
	TryNum          int                                                        //try num
	RedirectNum     int                                                        //redirect num ,<0 no redirect,==0 no limit
	Headers         any                                                        //default headers

	GetProxy              func(ctx context.Context, url *url.URL) (string, error) //proxy callback:support https,http,socks5 proxy
	LocalAddr             *net.TCPAddr                                            //network card ip
	DialTimeout           time.Duration                                           //dial tcp timeout,default:15
	TlsHandshakeTimeout   time.Duration                                           //tls timeout,default:15
	KeepAlive             time.Duration                                           //keepalive,default:30
	ResponseHeaderTimeout time.Duration                                           //ResponseHeaderTimeout ,default:30
	AddrType              AddrType                                                //dns parse addr type
	GetAddrType           func(string) AddrType
	Dns                   net.IP //dns
}
type Client struct {
	forceHttp1   bool
	orderHeaders []string

	jar         *Jar
	redirectNum int
	disDecode   bool
	disUnZip    bool
	disAlive    bool

	tryNum int

	requestCallBack func(context.Context, *http.Request, *http.Response) error

	optionCallBack func(context.Context, *Client, *RequestOption) error
	resultCallBack func(context.Context, *Client, *Response) error
	errCallBack    func(context.Context, *Client, error) error

	timeout time.Duration
	headers any
	bar     bool

	disCookie   bool
	client      *http.Client
	noJarClient *http.Client
	proxy       *url.URL

	ctx       context.Context
	cnl       context.CancelFunc
	transport *RoundTripper

	ja3Spec   ja3.Ja3Spec
	h2Ja3Spec ja3.H2Ja3Spec
}

func NewClient(preCtx context.Context, options ...ClientOption) (*Client, error) {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	var option ClientOption

	if len(options) > 0 {
		option = options[0]
	}
	if option.KeepAlive == 0 {
		option.KeepAlive = time.Second * 30
	}
	if option.ResponseHeaderTimeout == 0 {
		option.ResponseHeaderTimeout = time.Second * 30
	}
	if option.DialTimeout == 0 {
		option.DialTimeout = time.Second * 15
	}
	if option.TlsHandshakeTimeout == 0 {
		option.TlsHandshakeTimeout = time.Second * 15
	}
	//cookiesjar
	var jar *Jar
	if !option.DisCookie {
		jar = NewJar()
	}
	// var http3Transport *http3.RoundTripper
	// if option.Http3 {
	// 	http3Transport = &http3.RoundTripper{
	// 		TLSClientConfig: tlsConfig,
	// 		QuicConfig: &quic.Config{
	// 			EnableDatagrams:      true,
	// 			HandshakeIdleTimeout: option.TLSHandshakeTimeout,
	// 			MaxIdleTimeout:       option.IdleConnTimeout,
	// 			KeepAlivePeriod:      option.KeepAlive,
	// 		},
	// 		EnableDatagrams: true,
	// 	}
	// }
	transport := newRoundTripper(ctx, RoundTripperOption{
		TlsHandshakeTimeout:   option.TlsHandshakeTimeout,
		DialTimeout:           option.DialTimeout,
		KeepAlive:             option.KeepAlive,
		ResponseHeaderTimeout: option.ResponseHeaderTimeout,

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
			if ctxData.redirectNum == 0 || ctxData.redirectNum >= len(via) {
				return nil
			}
			return http.ErrUseLastResponse
		},
	}
	if jar != nil {
		client.Jar = jar.jar
	}
	var noJarClient *http.Client
	if client.Jar != nil {
		noJarClient = &http.Client{
			Transport:     transport,
			CheckRedirect: client.CheckRedirect,
		}
	}
	result := &Client{
		jar:             jar,
		ctx:             ctx,
		cnl:             cnl,
		client:          client,
		transport:       transport,
		noJarClient:     noJarClient,
		forceHttp1:      option.ForceHttp1,
		requestCallBack: option.RequestCallBack,
		orderHeaders:    option.OrderHeaders,
		disCookie:       option.DisCookie,
		redirectNum:     option.RedirectNum,
		disDecode:       option.DisDecode,
		disUnZip:        option.DisUnZip,
		disAlive:        option.DisAlive,
		tryNum:          option.TryNum,
		optionCallBack:  option.OptionCallBack,
		resultCallBack:  option.ResultCallBack,
		errCallBack:     option.ErrCallBack,
		timeout:         option.Timeout,
		headers:         option.Headers,
		bar:             option.Bar,
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
func (obj *Client) HttpClient() *http.Client {
	return obj.client
}
func (obj *Client) SetProxy(proxyUrl string) (err error) {
	obj.proxy, err = gtls.VerifyProxy(proxyUrl)
	return
}
func (obj *Client) SetGetProxy(getProxy func(ctx context.Context, url *url.URL) (string, error)) {
	obj.transport.SetGetProxy(getProxy)
}

func (obj *Client) CloseIdleConnections() {
	obj.transport.CloseIdleConnections()
}
func (obj *Client) CloseConnections() {
	obj.transport.CloseConnections()
}

func (obj *Client) Close() {
	obj.CloseConnections()
	obj.cnl()
}

func (obj *Client) getClient(option RequestOption) *http.Client {
	if option.DisCookie && obj.noJarClient != nil {
		return obj.noJarClient
	}
	return obj.client
}
