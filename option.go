package requests

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/websocket"
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
	MaxRetries            int                                                                             //try num
	MaxRedirect           int                                                                             //redirect num ,<0 no redirect,==0 no limit
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

// Options for sending requests
type RequestOption struct {
	ForceHttp1      bool                                                                            //force  use http1 send requests
	OrderHeaders    []string                                                                        //order headers with http1
	Ja3             bool                                                                            //enable ja3 fingerprint
	Ja3Spec         ja3.Ja3Spec                                                                     //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec       ja3.H2Ja3Spec                                                                   //custom h2 fingerprint
	Proxy           string                                                                          //proxy,support http,https,socks5,example：http://127.0.0.1:7005
	DisCookie       bool                                                                            //disable cookies,not use cookies
	DisDecode       bool                                                                            //disable auto decode
	DisUnZip        bool                                                                            //disable auto zip decode
	DisAlive        bool                                                                            //disable  keepalive
	Bar             bool                                                                            //enable bar display
	Timeout         time.Duration                                                                   //request timeout
	OptionCallBack  func(ctx context.Context, client *Client, option *RequestOption) error          //option callback,if error is returnd, break request
	ResultCallBack  func(ctx context.Context, client *Client, response *Response) error             //result callback,if error is returnd,next errCallback
	ErrCallBack     func(ctx context.Context, client *Client, response *Response, err error) error  //error callback,if error is returnd,break request
	RequestCallBack func(ctx context.Context, request *http.Request, response *http.Response) error //request and response callback,if error is returnd,reponse is error

	MaxRetries            int           //try num
	MaxRedirect           int           //redirect num ,<0 no redirect,==0 no limit
	Headers               any           //request headers：json,map，header
	ResponseHeaderTimeout time.Duration //ResponseHeaderTimeout ,default:30
	TlsHandshakeTimeout   time.Duration

	//network card ip
	DialTimeout time.Duration //dial tcp timeout,default:15
	KeepAlive   time.Duration //keepalive,default:30
	LocalAddr   *net.TCPAddr
	Dns         *net.UDPAddr  //dns
	AddrType    gtls.AddrType //dns parse addr type                                             //tls timeout,default:15
	Jar         *Jar          //custom cookies

	Method      string //method
	Url         *url.URL
	Host        string
	Referer     string //set headers referer value
	ContentType string //headers Content-Type value
	Cookies     any    // cookies,support :json,map,str，http.Header

	Params any //url params，join url query,json,map
	Json   any //send application/json,support io.Reader,：string,[]bytes,json,map
	Data   any //send application/x-www-form-urlencoded, support io.Reader, string,[]bytes,json,map
	Form   any //send multipart/form-data,file upload,support io.Reader, json,map
	Text   any //send text/xml,support: io.Reader, string,[]bytes,json,map
	Body   any //not setting context-type,support io.Reader, string,[]bytes,json,map

	Stream   bool             //disable auto read
	WsOption websocket.Option //websocket option
	DisProxy bool             //force disable proxy
	Debug    bool             //enable debugger
	once     bool
}

// Upload files with form-data,
type File struct {
	FileName    string
	ContentType string
	Content     any
}

func (obj *RequestOption) initBody(ctx context.Context) (body io.Reader, err error) {
	if obj.Body != nil {
		body, _, _, err = obj.newBody(obj.Body, readType)
	} else if obj.Form != nil {
		var orderMap *orderMap
		if body, orderMap, _, err = obj.newBody(obj.Form, mapType); err != nil {
			return
		}
		tempBody, contentType, once, err := orderMap.parseForm(ctx)
		obj.once = once
		if obj.ContentType == "" {
			obj.ContentType = contentType
		}
		return tempBody, err
	} else if obj.Data != nil {
		var orderMap *orderMap
		if _, orderMap, _, err = obj.newBody(obj.Data, mapType); err != nil {
			return
		}
		body = orderMap.parseData()
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
	} else if obj.Json != nil {
		if body, _, _, err = obj.newBody(obj.Json, readType); err != nil {
			return
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/json"
		}
	} else if obj.Text != nil {
		if body, _, _, err = obj.newBody(obj.Text, readType); err != nil {
			return
		}
		if obj.ContentType == "" {
			obj.ContentType = "text/plain"
		}
	}
	return
}
func (obj *RequestOption) initParams() (string, error) {
	if obj.Params == nil {
		return obj.Url.String(), nil
	}
	_, dataMap, _, err := obj.newBody(obj.Params, mapType)
	if err != nil {
		return obj.Url.String(), err
	}
	query := dataMap.parseParams()
	if query == "" {
		return obj.Url.String(), nil
	}
	pu := cloneUrl(obj.Url)
	puValues := pu.Query()
	pu.RawQuery = puValues.Encode() + "&" + query
	return pu.String(), nil
}
func (obj *Client) newRequestOption(option RequestOption) RequestOption {
	if !option.DisProxy {
		if option.Proxy == "" && obj.proxy != nil {
			option.Proxy = obj.proxy.String()
		}
	} else {
		option.Proxy = ""
	}

	if option.MaxRetries < 0 {
		option.MaxRetries = 0
	} else if option.MaxRetries == 0 {
		option.MaxRetries = obj.maxRetries
	}
	if option.Headers == nil {
		option.Headers = obj.headers
	}
	if !option.Bar {
		option.Bar = obj.bar
	}
	if option.MaxRedirect == 0 {
		option.MaxRedirect = obj.maxRedirect
	}
	if option.Timeout == 0 {
		option.Timeout = obj.timeout
	}
	if option.ResponseHeaderTimeout == 0 {
		option.ResponseHeaderTimeout = obj.responseHeaderTimeout
	}
	if option.AddrType == 0 {
		option.AddrType = obj.addrType
	}
	if option.TlsHandshakeTimeout == 0 {
		option.TlsHandshakeTimeout = obj.tlsHandshakeTimeout
	}
	if !option.DisCookie {
		option.DisCookie = obj.disCookie
	}
	if !option.DisDecode {
		option.DisDecode = obj.disDecode
	}
	if !option.DisUnZip {
		option.DisUnZip = obj.disUnZip
	}

	if !option.ForceHttp1 {
		option.ForceHttp1 = obj.forceHttp1
	}

	if !option.DisAlive {
		option.DisAlive = obj.disAlive
	}
	if option.OrderHeaders == nil {
		option.OrderHeaders = obj.orderHeaders
	}

	if !option.Ja3Spec.IsSet() {
		if obj.ja3Spec.IsSet() {
			option.Ja3Spec = obj.ja3Spec
		} else if option.Ja3 {
			option.Ja3Spec = ja3.DefaultJa3Spec()
		}
	}
	if !option.H2Ja3Spec.IsSet() {
		option.H2Ja3Spec = obj.h2Ja3Spec
	}
	if option.OptionCallBack == nil {
		option.OptionCallBack = obj.optionCallBack
	}
	if option.ResultCallBack == nil {
		option.ResultCallBack = obj.resultCallBack
	}
	if option.ErrCallBack == nil {
		option.ErrCallBack = obj.errCallBack
	}
	if option.RequestCallBack == nil {
		option.RequestCallBack = obj.requestCallBack
	}
	return option
}
