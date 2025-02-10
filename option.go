package requests

import (
	"context"
	"crypto/tls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	"github.com/quic-go/quic-go"
	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
	"io"
	"net/url"
	"time"
)

type LogType string

const (
	LogType_DNSLookup          LogType = "DNSLookup"
	LogType_TCPConnect         LogType = "TCPConnect"
	LogType_TLSHandshake       LogType = "TLSHandshake"
	LogType_ProxyDNSLookup     LogType = "ProxyDNSLookup"
	LogType_ProxyTCPConnect    LogType = "ProxyTCPConnect"
	LogType_ProxyTLSHandshake  LogType = "ProxyTLSHandshake"
	LogType_ProxyConnectRemote LogType = "ProxyConnectRemote"
	LogType_ResponseHeader     LogType = "ResponseHeader"
	LogType_ResponseBody       LogType = "ResponseBody"
)

type Log struct {
	Time time.Time
	Msg  any     `json:"msg"`
	Id   string  `json:"id"`
	Type LogType `json:"type"`
}

// Connection Management Options
type ClientOption struct {
	Ja3Spec               ja3.Spec //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	DialOption            DialOption
	Headers               any                                   //default headers
	Jar                   Jar                                   //custom cookies
	Logger                func(Log)                             //debuggable
	OptionCallBack        func(ctx *Response) error             //option callback,if error is returnd, break request
	ResultCallBack        func(ctx *Response) error             //result callback,if error is returnd,next errCallback
	ErrCallBack           func(ctx *Response) error             //error callback,if error is returnd,break request
	RequestCallBack       func(ctx *Response) error             //request and response callback,if error is returnd,reponse is error
	GetProxy              func(ctx *Response) (string, error)   //proxy callback:support https,http,socks5 proxy
	GetProxys             func(ctx *Response) ([]string, error) //proxys callback:support https,http,socks5 proxy
	TlsConfig             *tls.Config
	UtlsConfig            *utls.Config
	QuicConfig            *quic.Config
	UquicConfig           *uquic.Config
	UJa3Spec              ja3.USpec     //h3 fingerprint
	Proxy                 string        //proxy,support https,http,socks5
	UserAgent             string        //headers User-Agent value
	OrderHeaders          []string      //order headers
	Proxys                []string      //proxy list,support https,http,socks5
	H2Ja3Spec             ja3.H2Spec    //h2 fingerprint
	MaxRetries            int           //try num
	MaxRedirect           int           //redirect num ,<0 no redirect,==0 no limit
	Timeout               time.Duration //request timeout
	ResponseHeaderTimeout time.Duration //ResponseHeaderTimeout ,default:300
	TlsHandshakeTimeout   time.Duration //tls timeout,default:15
	H3                    bool          //开启http3
	ForceHttp1            bool          //force  use http1 send requests
	Ja3                   bool          //enable ja3 fingerprint
	DisCookie             bool          //disable cookies
	DisDecode             bool          //disable auto decode
	Bar                   bool          ////enable bar display
}

// Options for sending requests
type RequestOption struct {
	WsOption websocket.Option //websocket option
	Cookies  any              // cookies,support :json,map,str，http.Header
	Params   any              //url params，join url query,json,map
	Json     any              //send application/json,support io.Reader,：string,[]bytes,json,map
	Data     any              //send application/x-www-form-urlencoded, support io.Reader, string,[]bytes,json,map
	Form     any              //send multipart/form-data,file upload,support io.Reader, json,map
	Text     any              //send text/xml,support: io.Reader, string,[]bytes,json,map
	Body     any              //not setting context-type,support io.Reader, string,[]bytes,json,map
	Url      *url.URL
	// other option
	Method      string //method
	Host        string
	Referer     string //set headers referer value
	ContentType string //headers Content-Type value
	ClientOption
	Stream   bool //disable auto read
	DisProxy bool //force disable proxy
	once     bool
}

// Upload files with form-data,
type File struct {
	Content     any
	FileName    string
	ContentType string
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
	err := tools.Merge(&option, obj.ClientOption)
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
		option.UserAgent = obj.ClientOption.UserAgent
	}
	if option.DisCookie {
		option.Jar = nil
	}
	if option.DisProxy {
		option.Proxy = ""
	}
	return option, err
}
