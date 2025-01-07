package requests

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	utls "github.com/refraction-networking/utls"
)

type LogType string

const (
	LogType_DNSLookup LogType = "DNSLookup"

	LogType_TCPConnect LogType = "TCPConnect"

	LogType_TLSHandshake LogType = "TLSHandshake"

	LogType_ProxyDNSLookup LogType = "ProxyDNSLookup"

	LogType_ProxyTCPConnect LogType = "ProxyTCPConnect"

	LogType_ProxyTLSHandshake LogType = "ProxyTLSHandshake"

	LogType_ProxyConnectRemote LogType = "ProxyConnectRemote"

	LogType_ResponseHeader LogType = "ResponseHeader"

	LogType_ResponseBody LogType = "ResponseBody"
)

type Log struct {
	Id   string  `json:"id"`
	Type LogType `json:"type"`
	Time time.Time
	Msg  any `json:"msg"`
}

// Connection Management Options
type ClientOption struct {
	Logger                func(Log)                                                                             //debuggable
	H3                    bool                                                                                  //开启http3
	OrderHeaders          []string                                                                              //order headers
	Ja3Spec               ja3.Spec                                                                              //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec             ja3.H2Spec                                                                            //h2 fingerprint
	UJa3Spec              ja3.USpec                                                                             //h3 fingerprint
	Proxy                 string                                                                                //proxy,support https,http,socks5
	Proxys                []string                                                                              //proxy list,support https,http,socks5
	ForceHttp1            bool                                                                                  //force  use http1 send requests
	Ja3                   bool                                                                                  //enable ja3 fingerprint
	DisCookie             bool                                                                                  //disable cookies
	DisDecode             bool                                                                                  //disable auto decode
	DisUnZip              bool                                                                                  //disable auto zip decode
	Bar                   bool                                                                                  ////enable bar display
	OptionCallBack        func(ctx context.Context, option *RequestOption) error                                //option callback,if error is returnd, break request
	ResultCallBack        func(ctx context.Context, option *RequestOption, response *Response) error            //result callback,if error is returnd,next errCallback
	ErrCallBack           func(ctx context.Context, option *RequestOption, response *Response, err error) error //error callback,if error is returnd,break request
	RequestCallBack       func(ctx context.Context, request *http.Request, response *http.Response) error       //request and response callback,if error is returnd,reponse is error
	MaxRetries            int                                                                                   //try num
	MaxRedirect           int                                                                                   //redirect num ,<0 no redirect,==0 no limit
	Headers               any                                                                                   //default headers
	Timeout               time.Duration                                                                         //request timeout
	ResponseHeaderTimeout time.Duration                                                                         //ResponseHeaderTimeout ,default:300
	TlsHandshakeTimeout   time.Duration                                                                         //tls timeout,default:15
	UserAgent             string                                                                                //headers User-Agent value
	GetProxy              func(ctx context.Context, url *url.URL) (string, error)                               //proxy callback:support https,http,socks5 proxy
	GetProxys             func(ctx context.Context, url *url.URL) ([]string, error)                             //proxys callback:support https,http,socks5 proxy
	DialOption            DialOption
	Jar                   Jar //custom cookies
	TlsConfig             *tls.Config
	UtlsConfig            *utls.Config
}

// Options for sending requests
type RequestOption struct {
	ClientOption
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

	once      bool
	client    *Client
	requestId string
	proxy     *url.URL
	proxys    []*url.URL
	isNewConn bool
}

func (obj *RequestOption) Client() *Client {
	return obj.client
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
		var orderMap *OrderMap
		_, orderMap, _, err := obj.newBody(obj.Form, mapType)
		if err != nil {
			return nil, err
		}
		if orderMap == nil {
			return nil, nil
		}
		body, contentType, once, err := orderMap.parseForm(ctx)
		if err != nil {
			return nil, err
		}
		obj.once = once
		if obj.ContentType == "" {
			obj.ContentType = contentType
		}
		if body == nil {
			return nil, nil
		}
		return body, nil
	} else if obj.Data != nil {
		body, orderMap, _, err := obj.newBody(obj.Data, mapType)
		if err != nil {
			return body, err
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
		if body != nil {
			return body, nil
		}
		if orderMap == nil {
			return nil, nil
		}
		body2 := orderMap.parseData()
		if body2 == nil {
			return nil, nil
		}
		return body2, nil
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
			return nil, nil
		}
		return body, nil
	} else {
		return nil, nil
	}
}
func (obj *RequestOption) initParams() (*url.URL, error) {
	baseUrl := cloneUrl(obj.Url)
	if obj.Params == nil {
		return baseUrl, nil
	}
	_, dataMap, _, err := obj.newBody(obj.Params, mapType)
	if err != nil {
		return nil, err
	}
	query := dataMap.parseParams().String()
	if query == "" {
		return baseUrl, nil
	}
	pquery := baseUrl.Query().Encode()
	if pquery == "" {
		baseUrl.RawQuery = query
	} else {
		baseUrl.RawQuery = pquery + "&" + query
	}
	return baseUrl, nil
}
func (obj *Client) newRequestOption(option RequestOption) (RequestOption, error) {
	err := tools.Merge(&option, obj.option)
	//end
	if option.MaxRetries < 0 {
		option.MaxRetries = 0
	}
	if !option.Ja3Spec.IsSet() && option.Ja3 {
		option.Ja3Spec = ja3.DefaultSpec()
	}
	if !option.UJa3Spec.IsSet() && option.Ja3 {
		option.UJa3Spec = ja3.DefaultUSpec()
	}
	if option.UserAgent == "" {
		option.UserAgent = obj.option.UserAgent
	}
	if option.DisCookie {
		option.Jar = nil
	}
	if option.DisProxy {
		option.Proxy = ""
	}
	return option, err
}

// func (obj *Client) newRequestOption(option RequestOption) RequestOption {
// 	// start
// 	if !option.Ja3Spec.IsSet() {
// 		option.Ja3Spec = obj.option.Ja3Spec
// 	}
// 	if !option.Ja3 {
// 		option.Ja3 = obj.option.Ja3
// 	}
// 	if !option.UJa3Spec.IsSet() {
// 		option.UJa3Spec = obj.option.UJa3Spec
// 	}

// 	if !option.H2Ja3Spec.IsSet() {
// 		option.H2Ja3Spec = obj.option.H2Ja3Spec
// 	}
// 	if option.Proxy == "" {
// 		option.Proxy = obj.option.Proxy
// 	}
// 	if len(option.Proxys) == 0 {
// 		option.Proxys = obj.option.Proxys
// 	}
// 	if !option.ForceHttp1 {
// 		option.ForceHttp1 = obj.option.ForceHttp1
// 	}
// 	if !option.DisCookie {
// 		option.DisCookie = obj.option.DisCookie
// 	}
// 	if !option.DisDecode {
// 		option.DisDecode = obj.option.DisDecode
// 	}
// 	if !option.DisUnZip {
// 		option.DisUnZip = obj.option.DisUnZip
// 	}
// 	if !option.Bar {
// 		option.Bar = obj.option.Bar
// 	}
// 	if !option.H3 {
// 		option.H3 = obj.option.H3
// 	}
// 	if option.Logger == nil {
// 		option.Logger = obj.option.Logger
// 	}
// 	if option.Headers == nil {
// 		option.Headers = obj.option.Headers
// 	}
// 	if option.Timeout == 0 {
// 		option.Timeout = obj.option.Timeout
// 	}
// 	if option.ResponseHeaderTimeout == 0 {
// 		option.ResponseHeaderTimeout = obj.option.ResponseHeaderTimeout
// 	}
// 	if option.TlsHandshakeTimeout == 0 {
// 		option.TlsHandshakeTimeout = obj.option.TlsHandshakeTimeout
// 	}
// 	if option.TlsConfig == nil {
// 		option.TlsConfig = obj.option.TlsConfig
// 	}
// 	if option.UtlsConfig == nil {
// 		option.UtlsConfig = obj.option.UtlsConfig
// 	}
// 	if option.OrderHeaders == nil {
// 		option.OrderHeaders = obj.option.OrderHeaders
// 	}
// 	if option.OptionCallBack == nil {
// 		option.OptionCallBack = obj.option.OptionCallBack
// 	}
// 	if option.ResultCallBack == nil {
// 		option.ResultCallBack = obj.option.ResultCallBack
// 	}
// 	if option.ErrCallBack == nil {
// 		option.ErrCallBack = obj.option.ErrCallBack
// 	}
// 	if option.RequestCallBack == nil {
// 		option.RequestCallBack = obj.option.RequestCallBack
// 	}
// 	if option.MaxRetries == 0 {
// 		option.MaxRetries = obj.option.MaxRetries
// 	}
// 	if option.MaxRedirect == 0 {
// 		option.MaxRedirect = obj.option.MaxRedirect
// 	}
// 	if option.Headers == nil {
// 		option.Headers = obj.option.Headers
// 	}
// 	if option.Timeout == 0 {
// 		option.Timeout = obj.option.Timeout
// 	}
// 	if option.ResponseHeaderTimeout == 0 {
// 		option.ResponseHeaderTimeout = obj.option.ResponseHeaderTimeout
// 	}
// 	if option.TlsHandshakeTimeout == 0 {
// 		option.TlsHandshakeTimeout = obj.option.TlsHandshakeTimeout
// 	}
// 	if option.DialOption.DialTimeout == 0 {
// 		option.DialOption.DialTimeout = obj.option.DialOption.DialTimeout
// 	}
// 	if option.DialOption.KeepAlive == 0 {
// 		option.DialOption.KeepAlive = obj.option.DialOption.KeepAlive
// 	}
// 	if option.DialOption.LocalAddr == nil {
// 		option.DialOption.LocalAddr = obj.option.DialOption.LocalAddr
// 	}
// 	if option.DialOption.AddrType == 0 {
// 		option.DialOption.AddrType = obj.option.DialOption.AddrType
// 	}
// 	if option.DialOption.Dns == nil {
// 		option.DialOption.Dns = obj.option.DialOption.Dns
// 	}
// 	if option.DialOption.GetAddrType == nil {
// 		option.DialOption.GetAddrType = obj.option.DialOption.GetAddrType
// 	}
// 	if option.Jar == nil {
// 		option.Jar = obj.option.Jar
// 	}
// 	//end
// 	if option.MaxRetries < 0 {
// 		option.MaxRetries = 0
// 	}
// 	if !option.Ja3Spec.IsSet() && option.Ja3 {
// 		option.Ja3Spec = ja3.DefaultJa3Spec()
// 	}
// 	if !option.UJa3Spec.IsSet() && option.Ja3 {
// 		option.UJa3Spec = ja3.DefaultUJa3Spec()
// 	}
// 	if option.UserAgent == "" {
// 		option.UserAgent = obj.option.UserAgent
// 	}
// 	if option.DisCookie {
// 		option.Jar = nil
// 	}
// 	if option.DisProxy {
// 		option.Proxy = ""
// 	}
// 	if option.GetProxy == nil {
// 		option.GetProxy = obj.option.GetProxy
// 	}
// 	if option.GetProxys == nil {
// 		option.GetProxys = obj.option.GetProxys
// 	}

// 	return option
// }
