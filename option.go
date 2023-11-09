package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"time"

	"github.com/gospider007/ja3"
	"github.com/gospider007/websocket"
)

type RequestOption struct {
	ForceHttp1            bool                                                       //force  use http1 send requests
	OrderHeaders          []string                                                   //order headers with http1
	Ja3                   bool                                                       //enable ja3 fingerprint
	Ja3Spec               ja3.Ja3Spec                                                //custom ja3Spec,use ja3.CreateSpecWithStr or ja3.CreateSpecWithId create
	H2Ja3Spec             ja3.H2Ja3Spec                                              //custom h2 fingerprint
	Proxy                 string                                                     //proxy,support http,https,socks5,example：http://127.0.0.1:7005
	DisCookie             bool                                                       //disable cookies,not use cookies
	DisDecode             bool                                                       //disable auto decode
	DisUnZip              bool                                                       //disable auto zip decode
	DisAlive              bool                                                       //disable  keepalive
	Bar                   bool                                                       //enable bar display
	Timeout               time.Duration                                              //request timeout
	OptionCallBack        func(context.Context, *Client, *RequestOption) error       //option callback,if error is returnd, break request
	ResultCallBack        func(context.Context, *Client, *Response) error            //result callback,if error is returnd,next errCallback
	ErrCallBack           func(context.Context, *Client, error) error                //error callback,if error is returnd,break request
	RequestCallBack       func(context.Context, *http.Request, *http.Response) error //request and response callback,if error is returnd,reponse is error
	TryNum                int                                                        //try num
	MaxRedirectNum        int                                                        //redirect num ,<0 no redirect,==0 no limit
	Headers               any                                                        //request headers：json,map，header
	ResponseHeaderTimeout time.Duration                                              //ResponseHeaderTimeout ,default:30
	TlsHandshakeTimeout   time.Duration

	//network card ip
	DialTimeout time.Duration //dial tcp timeout,default:15
	KeepAlive   time.Duration //keepalive,default:30
	LocalAddr   *net.TCPAddr
	Dns         *net.UDPAddr //dns
	AddrType    AddrType     //dns parse addr type                                             //tls timeout,default:15

	Stream  bool   //disable auto read
	Referer string //set headers referer value
	Method  string //method
	Url     *url.URL
	Host    string
	Cookies any    // cookies,support :json,map,str，http.Header
	Files   []File //send multipart/form-data, file upload
	Params  any    //url params，join url query,json,map

	Json any //send application/json,support io.Reader,：string,[]bytes,json,map
	Data any //send application/x-www-form-urlencoded, support io.Reader, string,[]bytes,json,map
	Form any //send multipart/form-data,file upload,support io.Reader, json,map
	Text any //send text/xml,support: io.Reader, string,[]bytes,json,map
	Body any //not setting context-type,support io.Reader, string,[]bytes,json,map

	ContentType string //headers Content-Type value
	DisProxy    bool   //force disable proxy

	Jar *Jar //custom cookies

	WsOption  websocket.Option //websocket option
	converUrl string
	body      io.Reader
	once      bool
}
type File struct {
	Key string
	Val []byte

	FileName    string
	ContentType string
}

func (obj *RequestOption) fileWrite(writer *multipart.Writer) (err error) {
	for _, file := range obj.Files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(file.Key), escapeQuotes(file.FileName)))
		if file.ContentType == "" {
			h.Set("Content-Type", http.DetectContentType(file.Val))
		} else {
			h.Set("Content-Type", file.ContentType)
		}
		if wp, err := writer.CreatePart(h); err != nil {
			return err
		} else if _, err = wp.Write(file.Val); err != nil {
			return err
		}
	}
	if err = writer.Close(); err != nil {
		return err
	}
	if obj.ContentType == "" {
		obj.ContentType = writer.FormDataContentType()
	}
	return err
}
func (obj *RequestOption) initBody() (err error) {
	if obj.body != nil {
		return nil
	} else if obj.Body != nil {
		if err = obj.newBody(obj.Body, rawType, nil); err != nil {
			return err
		}
	} else if obj.Form != nil {
		dataMap := map[string][]string{}
		if err = obj.newBody(obj.Form, formType, dataMap); err != nil {
			return err
		}
		tempBody := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(tempBody)
		for key, vals := range dataMap {
			for _, val := range vals {
				if err = writer.WriteField(key, val); err != nil {
					return err
				}
			}
		}
		if err = obj.fileWrite(writer); err != nil {
			return err
		}
		obj.body = tempBody
	} else if obj.Files != nil {
		tempBody := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(tempBody)
		if err = obj.fileWrite(writer); err != nil {
			return err
		}
		obj.body = tempBody
	} else if obj.Data != nil {
		if err = obj.newBody(obj.Data, dataType, nil); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
	} else if obj.Json != nil {
		if err = obj.newBody(obj.Json, jsonType, nil); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/json"
		}
	} else if obj.Text != nil {
		if err = obj.newBody(obj.Text, textType, nil); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = "text/plain"
		}
	}
	return nil
}
func (obj *RequestOption) optionInit() error {
	obj.converUrl = obj.Url.String()
	var err error
	if err = obj.initBody(); err != nil {
		return err
	}
	if obj.Params != nil {
		dataMap := map[string][]string{}
		if err = obj.newBody(obj.Params, paramsType, dataMap); err != nil {
			return err
		}
		pu := cloneUrl(obj.Url)
		puValues := pu.Query()
		for kk, vvs := range dataMap {
			for _, vv := range vvs {
				puValues.Add(kk, vv)
			}
		}
		pu.RawQuery = puValues.Encode()
		obj.converUrl = pu.String()
	}
	if err = obj.initHeaders(); err != nil {
		return err
	}
	return obj.initCookies()
}
func (obj *Client) newRequestOption(option RequestOption) RequestOption {
	if option.TryNum < 0 {
		option.TryNum = 0
	} else if option.TryNum == 0 {
		option.TryNum = obj.tryNum
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
	if option.Headers == nil {
		option.Headers = obj.headers
	}
	if !option.Bar {
		option.Bar = obj.bar
	}
	if option.MaxRedirectNum == 0 {
		option.MaxRedirectNum = obj.maxRedirectNum
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
	if option.RequestCallBack == nil {
		option.RequestCallBack = obj.requestCallBack
	}
	return option
}
