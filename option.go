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
	OrderHeaders          []string                                                                        //order headers with http1
	Ja3Spec               ja3.Ja3Spec                                                                     //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec             ja3.H2Ja3Spec                                                                   //h2 fingerprint
	Proxy                 string                                                                          //proxy,support https,http,socks5
	ForceHttp1            bool                                                                            //force  use http1 send requests
	Ja3                   bool                                                                            //enable ja3 fingerprint
	DisCookie             bool                                                                            //disable cookies
	DisDecode             bool                                                                            //disable auto decode
	DisUnZip              bool                                                                            //disable auto zip decode
	DisAlive              bool                                                                            //disable  keepalive
	Bar                   bool                                                                            ////enable bar display
	OptionCallBack        func(ctx context.Context, client *Client, option *RequestOption) error          //option callback,if error is returnd, break request
	ResultCallBack        func(ctx context.Context, client *Client, response *Response) error             //result callback,if error is returnd,next errCallback
	ErrCallBack           func(ctx context.Context, client *Client, response *Response, err error) error  //error callback,if error is returnd,break request
	RequestCallBack       func(ctx context.Context, request *http.Request, response *http.Response) error //request and response callback,if error is returnd,reponse is error
	MaxRetries            int                                                                             //try num
	MaxRedirect           int                                                                             //redirect num ,<0 no redirect,==0 no limit
	Headers               any                                                                             //default headers
	Timeout               time.Duration                                                                   //request timeout
	ResponseHeaderTimeout time.Duration                                                                   //ResponseHeaderTimeout ,default:30
	TlsHandshakeTimeout   time.Duration                                                                   //tls timeout,default:15

	//network card ip
	DialTimeout time.Duration //dial tcp timeout,default:15
	KeepAlive   time.Duration //keepalive,default:30
	LocalAddr   *net.TCPAddr
	Dns         *net.UDPAddr  //dns
	AddrType    gtls.AddrType //dns parse addr type
	Jar         *Jar          //custom cookies

	//other option
	GetProxy    func(ctx context.Context, url *url.URL) (string, error) //proxy callback:support https,http,socks5 proxy
	GetAddrType func(host string) gtls.AddrType
}

// Options for sending requests
type RequestOption struct {
	OrderHeaders    []string                                                                        //order headers with http1
	Ja3Spec         ja3.Ja3Spec                                                                     //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec       ja3.H2Ja3Spec                                                                   //custom h2 fingerprint
	Proxy           string                                                                          //proxy,support http,https,socks5,example：http://127.0.0.1:7005
	ForceHttp1      bool                                                                            //force  use http1 send requests
	Ja3             bool                                                                            //enable ja3 fingerprint
	DisCookie       bool                                                                            //disable cookies,not use cookies
	DisDecode       bool                                                                            //disable auto decode
	DisUnZip        bool                                                                            //disable auto zip decode
	DisAlive        bool                                                                            //disable  keepalive
	Bar             bool                                                                            //enable bar display
	OptionCallBack  func(ctx context.Context, client *Client, option *RequestOption) error          //option callback,if error is returnd, break request
	ResultCallBack  func(ctx context.Context, client *Client, response *Response) error             //result callback,if error is returnd,next errCallback
	ErrCallBack     func(ctx context.Context, client *Client, response *Response, err error) error  //error callback,if error is returnd,break request
	RequestCallBack func(ctx context.Context, request *http.Request, response *http.Response) error //request and response callback,if error is returnd,reponse is error

	MaxRetries            int           //try num
	MaxRedirect           int           //redirect num ,<0 no redirect,==0 no limit
	Headers               any           //request headers：json,map，header
	Timeout               time.Duration //request timeout
	ResponseHeaderTimeout time.Duration //ResponseHeaderTimeout ,default:30
	TlsHandshakeTimeout   time.Duration

	//network card ip
	DialTimeout time.Duration //dial tcp timeout,default:15
	KeepAlive   time.Duration //keepalive,default:30
	LocalAddr   *net.TCPAddr
	Dns         *net.UDPAddr  //dns
	AddrType    gtls.AddrType //dns parse addr type                                             //tls timeout,default:15
	Jar         *Jar          //custom cookies

	// other option
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
	once     bool
}

// Upload files with form-data,
type File struct {
	FileName    string
	ContentType string
	Content     any
}

func (obj *RequestOption) initBody(ctx context.Context) (io.Reader, error) {
	if obj.Body != nil {
		body, _, _, err := obj.newBody(obj.Body, readType)
		if err != nil || body == nil {
			return nil, err
		}
		return body, err
	} else if obj.Form != nil {
		var orderMap *orderMap
		_, orderMap, _, err := obj.newBody(obj.Form, mapType)
		if err != nil {
			return nil, err
		}
		body, contentType, once, err := orderMap.parseForm(ctx)
		obj.once = once
		if obj.ContentType == "" {
			obj.ContentType = contentType
		}
		if body == nil {
			return body, nil
		}
		return body, nil
	} else if obj.Data != nil {
		_, orderMap, _, err := obj.newBody(obj.Data, mapType)
		if err != nil {
			return nil, err
		}
		body := orderMap.parseData()
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
		if body == nil {
			return body, nil
		}
		return body, nil
	} else if obj.Json != nil {
		body, _, _, err := obj.newBody(obj.Json, readType)
		if err != nil {
			return nil, err
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/json"
		}
		if body == nil {
			return nil, nil
		}
		return body, nil
	} else if obj.Text != nil {
		body, _, _, err := obj.newBody(obj.Text, readType)
		if err != nil {
			return nil, err
		}
		if obj.ContentType == "" {
			obj.ContentType = "text/plain"
		}
		if body == nil {
			return nil, err
		}
		return body, nil
	} else {
		return nil, nil
	}
}
func (obj *RequestOption) initParams() (*url.URL, error) {
	if obj.Params == nil {
		return obj.Url, nil
	}
	_, dataMap, _, err := obj.newBody(obj.Params, mapType)
	if err != nil {
		return obj.Url, err
	}
	query := dataMap.parseParams().String()
	if query == "" {
		return obj.Url, nil
	}
	pquery := obj.Url.Query().Encode()
	if pquery == "" {
		obj.Url.RawQuery = query
	} else {
		obj.Url.RawQuery = pquery + "&" + query
	}
	return obj.Url, nil
}
func (obj *Client) newRequestOption(option RequestOption) RequestOption {
	// start
	if option.OrderHeaders == nil {
		option.OrderHeaders = obj.option.OrderHeaders
	}
	if !option.Ja3Spec.IsSet() {
		option.Ja3Spec = obj.option.Ja3Spec
	}
	if !option.H2Ja3Spec.IsSet() {
		option.H2Ja3Spec = obj.option.H2Ja3Spec
	}
	if option.Proxy == "" {
		option.Proxy = obj.option.Proxy
	}
	if !option.ForceHttp1 {
		option.ForceHttp1 = obj.option.ForceHttp1
	}
	if !option.Ja3 {
		option.Ja3 = obj.option.Ja3
	}
	if !option.DisCookie {
		option.DisCookie = obj.option.DisCookie
	}
	if !option.DisDecode {
		option.DisDecode = obj.option.DisDecode
	}
	if !option.DisUnZip {
		option.DisUnZip = obj.option.DisUnZip
	}
	if !option.DisAlive {
		option.DisAlive = obj.option.DisAlive
	}
	if !option.Bar {
		option.Bar = obj.option.Bar
	}
	if option.OptionCallBack == nil {
		option.OptionCallBack = obj.option.OptionCallBack
	}
	if option.ResultCallBack == nil {
		option.ResultCallBack = obj.option.ResultCallBack
	}
	if option.ErrCallBack == nil {
		option.ErrCallBack = obj.option.ErrCallBack
	}
	if option.RequestCallBack == nil {
		option.RequestCallBack = obj.option.RequestCallBack
	}
	if option.MaxRetries == 0 {
		option.MaxRetries = obj.option.MaxRetries
	}
	if option.MaxRedirect == 0 {
		option.MaxRedirect = obj.option.MaxRedirect
	}
	if option.Headers == nil {
		option.Headers = obj.option.Headers
	}
	if option.Timeout == 0 {
		option.Timeout = obj.option.Timeout
	}
	if option.ResponseHeaderTimeout == 0 {
		option.ResponseHeaderTimeout = obj.option.ResponseHeaderTimeout
	}
	if option.TlsHandshakeTimeout == 0 {
		option.TlsHandshakeTimeout = obj.option.TlsHandshakeTimeout
	}
	if option.DialTimeout == 0 {
		option.DialTimeout = obj.option.DialTimeout
	}
	if option.KeepAlive == 0 {
		option.KeepAlive = obj.option.KeepAlive
	}
	if option.LocalAddr == nil {
		option.LocalAddr = obj.option.LocalAddr
	}
	if option.Dns == nil {
		option.Dns = obj.option.Dns
	}
	if option.AddrType == 0 {
		option.AddrType = obj.option.AddrType
	}
	if option.Jar == nil {
		option.Jar = obj.option.Jar
	}
	//end
	if option.MaxRetries < 0 {
		option.MaxRetries = 0
	}
	if !option.Ja3Spec.IsSet() && option.Ja3 {
		option.Ja3Spec = ja3.DefaultJa3Spec()
	}
	return option
}
