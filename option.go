package requests

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/gospider007/ja3"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	"github.com/quic-go/quic-go"
	uquic "github.com/refraction-networking/uquic"
	utls "github.com/refraction-networking/utls"
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
	Spec                  any //support []bytes,hex string,bool
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
	USpec                 any           //support ja3.USpec,uquic.QUICID,bool
	Proxy                 string        //proxy,support https,http,socks5
	UserAgent             string        //headers User-Agent value
	Proxys                []string      //proxy list,support https,http,socks5
	HSpec                 ja3.HSpec     //h2 fingerprint
	MaxRetries            int           //try num
	MaxRedirect           int           //redirect num ,<0 no redirect,==0 no limit
	Timeout               time.Duration //request timeout
	ResponseHeaderTimeout time.Duration //ResponseHeaderTimeout ,default:300
	TlsHandshakeTimeout   time.Duration //tls timeout,default:15
	ForceHttp3            bool          //force  use http3 send requests
	ForceHttp1            bool          //force  use http1 send requests
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
	Stream       bool //disable auto read
	DisProxy     bool //force disable proxy
	once         bool
	orderHeaders *OrderData //order headers
}

// Upload files with form-data,
type File struct {
	Content     any
	FileName    string
	ContentType string
}

func randomBoundary() string {
	var buf [30]byte
	io.ReadFull(rand.Reader, buf[:])
	boundary := fmt.Sprintf("%x", buf[:])
	if strings.ContainsAny(boundary, `()<>@,;:\"/[]?= `) {
		boundary = `"` + boundary + `"`
	}
	return "multipart/form-data; boundary=" + boundary
}

func (obj *RequestOption) initBody(ctx context.Context) (io.Reader, error) {
	if obj.Body != nil {
		body, orderData, _, err := obj.newBody(obj.Body)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}
		con, err := orderData.MarshalJSON()
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(con), nil
	} else if obj.Form != nil {
		if obj.ContentType == "" {
			obj.ContentType = randomBoundary()
		}
		body, orderData, ok, err := obj.newBody(obj.Body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("not support type")
		}
		if body != nil {
			return body, nil
		}
		body, once, err := orderData.parseForm(ctx)
		if err != nil {
			return nil, err
		}
		obj.once = once
		return body, err
	} else if obj.Data != nil {
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
		body, orderData, ok, err := obj.newBody(obj.Body)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("not support type")
		}
		if body != nil {
			return body, nil
		}
		return orderData.parseData(), nil
	} else if obj.Json != nil {
		if obj.ContentType == "" {
			obj.ContentType = "application/json"
		}
		body, orderData, _, err := obj.newBody(obj.Json)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}
		return orderData.parseJson()
	} else if obj.Text != nil {
		if obj.ContentType == "" {
			obj.ContentType = "text/plain"
		}
		body, orderData, _, err := obj.newBody(obj.Text)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return body, nil
		}
		return orderData.parseText()
	} else {
		return nil, nil
	}
}
func (obj *RequestOption) initParams() (*url.URL, error) {
	baseUrl := cloneUrl(obj.Url)
	if obj.Params == nil {
		return baseUrl, nil
	}
	body, dataData, ok, err := obj.newBody(obj.Params)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("not support type")
	}
	var query string
	if body != nil {
		paramsBytes, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		query = tools.BytesToString(paramsBytes)
	} else {
		query = dataData.parseParams().String()
	}
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
