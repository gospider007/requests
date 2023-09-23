package requests

import (
	"context"
	"net"
	"net/url"
	"time"

	"net/http"

	"gitee.com/baixudong/ja3"
	"gitee.com/baixudong/tools"
)

type ClientOption struct {
	GetProxy func(ctx context.Context, url *url.URL) (string, error) //根据url 返回代理，支持https,http,socks5 代理协议
	Proxy    string                                                  //设置代理,支持https,http,socks5 代理协议

	DisCookie bool         //关闭cookies管理
	LocalAddr *net.TCPAddr //本地网卡出口ip

	DialTimeout         time.Duration //dial tcp 超时时间,default:15
	TlsHandshakeTimeout time.Duration //tls 超时时间,default:15
	KeepAlive           time.Duration //keepalive保活检测定时,default:30

	AddrType    AddrType //优先使用的addr 类型
	GetAddrType func(string) AddrType
	Dns         net.IP //dns

	Ja3       bool          //开启ja3指纹
	Ja3Spec   ja3.Ja3Spec   //指定ja3Spec,使用ja3.CreateSpecWithStr 或者ja3.CreateSpecWithId 生成
	H2Ja3Spec ja3.H2Ja3Spec //h2指纹

	RedirectNum int //重定向次数,小于0为禁用,0:不限制

	DisAlive bool //关闭连接复用

	DisDecode      bool                                                 //关闭自动编码
	DisUnZip       bool                                                 //关闭自动解压
	TryNum         int                                                  //重试次数
	OptionCallBack func(context.Context, *Client, *RequestOption) error //请求参数回调,用于对请求参数进行修改。返回error,中断重试请求,返回nil继续
	ResultCallBack func(context.Context, *Client, *Response) error      //结果回调,用于对结果进行校验。返回nil，直接返回,返回err的话，如果有errCallBack 走errCallBack，没有继续try
	ErrCallBack    func(context.Context, *Client, error) error          //错误回调,返回error,中断重试请求,返回nil继续
	Timeout        time.Duration                                        //请求超时时间
	Headers        any                                                  //请求头
	Bar            bool                                                 //是否开启请求进度条

	RequestCallBack  func(context.Context, *http.Request) error
	ResponseCallBack func(context.Context, *http.Request, *http.Response) error
}
type Client struct {
	jar         *Jar
	redirectNum int  //重定向次数
	disDecode   bool //关闭自动编码
	disUnZip    bool //变比自动解压
	tryNum      int  //重试次数

	requestCallBack  func(context.Context, *http.Request) error
	responseCallBack func(context.Context, *http.Request, *http.Response) error

	optionCallBack func(context.Context, *Client, *RequestOption) error //请求参数回调,用于对请求参数进行修改。返回error,中断重试请求,返回nil继续
	resultCallBack func(context.Context, *Client, *Response) error      //结果回调,用于对结果进行校验。返回nil，直接返回,返回err的话，如果有errCallBack 走errCallBack，没有继续try
	errCallBack    func(context.Context, *Client, error) error          //错误回调,返回error,中断重试请求,返回nil继续

	timeout time.Duration //请求超时时间
	headers any           //请求头
	bar     bool          //是否开启bar

	disAlive    bool
	disCookie   bool
	client      *http.Client
	noJarClient *http.Client
	proxy       *url.URL

	ctx       context.Context
	cnl       context.CancelFunc
	transport *RoundTripper

	ja3Spec   ja3.Ja3Spec   //指定ja3Spec,使用ja3.CreateSpecWithStr 或者ja3.CreateSpecWithId 生成
	h2Ja3Spec ja3.H2Ja3Spec //h2指纹
}

// 新建一个请求客户端,发送请求必须创建哈
func NewClient(preCtx context.Context, options ...ClientOption) (*Client, error) {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	ctx, cnl := context.WithCancel(preCtx)
	var option ClientOption
	//初始化参数
	if len(options) > 0 {
		option = options[0]
	}
	if option.KeepAlive == 0 {
		option.KeepAlive = time.Second * 30
	}
	if option.DialTimeout == 0 {
		option.DialTimeout = time.Second * 15
	}
	if option.TlsHandshakeTimeout == 0 {
		option.TlsHandshakeTimeout = time.Second * 5
	}
	//创建cookiesjar
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
	transport := NewRoundTripper(ctx, RoundTripperOption{
		TlsHandshakeTimeout: option.TlsHandshakeTimeout,
		DialTimeout:         option.DialTimeout,
		KeepAlive:           option.KeepAlive,
		LocalAddr:           option.LocalAddr,
		AddrType:            option.AddrType,
		GetAddrType:         option.GetAddrType,
		Dns:                 option.Dns,
		GetProxy:            option.GetProxy,
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
		jar:              jar,
		ctx:              ctx,
		cnl:              cnl,
		client:           client,
		transport:        transport,
		noJarClient:      noJarClient,
		requestCallBack:  option.RequestCallBack,
		responseCallBack: option.ResponseCallBack,

		disCookie:      option.DisCookie,
		redirectNum:    option.RedirectNum,
		disAlive:       option.DisAlive,
		disDecode:      option.DisDecode,
		disUnZip:       option.DisUnZip,
		tryNum:         option.TryNum,
		optionCallBack: option.OptionCallBack,
		resultCallBack: option.ResultCallBack,
		errCallBack:    option.ErrCallBack,
		timeout:        option.Timeout,
		headers:        option.Headers,
		bar:            option.Bar,
	}
	var err error
	if option.Proxy != "" {
		result.proxy, err = tools.VerifyProxy(option.Proxy)
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
	obj.proxy, err = tools.VerifyProxy(proxyUrl)
	return
}
func (obj *Client) SetGetProxy(getProxy func(ctx context.Context, url *url.URL) (string, error)) {
	obj.transport.SetGetProxy(getProxy)
}

// 关闭客户端
func (obj *Client) CloseIdleConnections() {
	obj.transport.CloseIdleConnections()
}

func (obj *Client) Close() {
	obj.CloseIdleConnections()
	obj.cnl()
}

func (obj *Client) getClient(option RequestOption) *http.Client {
	if option.DisCookie && obj.noJarClient != nil {
		return obj.noJarClient
	}
	return obj.client
}
