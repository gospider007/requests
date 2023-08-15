package requests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"gitee.com/baixudong/ja3"
	"gitee.com/baixudong/websocket"
)

// 请求参数选项
type RequestOption struct {
	Ja3       bool          //开启ja3指纹
	Ja3Spec   ja3.Ja3Spec   //指定ja3Spec,使用ja3.CreateSpecWithStr 或者ja3.CreateSpecWithId 生成
	H2Ja3     bool          //开启h2指纹
	H2Ja3Spec ja3.H2Ja3Spec //h2指纹

	Method      string        //method
	Url         *url.URL      //请求的url
	Host        string        //网站的host
	Proxy       string        //代理,支持http,https,socks5协议代理,例如：http://127.0.0.1:7005
	Timeout     time.Duration //请求超时时间
	Headers     any           //请求头,支持：json,map，header
	Cookies     any           // cookies,支持json,map,str，http.Header
	Files       []File        //发送multipart/form-data,文件上传
	Params      any           //url 中的参数，用以拼接url,支持json,map
	Form        any           //发送multipart/form-data,适用于文件上传,支持json,map
	Data        any           //发送application/x-www-form-urlencoded,适用于key,val,支持string,[]bytes,json,map
	body        io.Reader
	Body        io.Reader
	Json        any    //发送application/json,支持：string,[]bytes,json,map
	Text        any    //发送text/xml,支持string,[]bytes,json,map
	ContentType string //headers 中Content-Type 的值
	Raw         any    //不设置context-type,支持string,[]bytes,json,map

	DisAlive  bool  //关闭连接复用
	DisCookie bool  //关闭cookies管理,这个请求不用cookies池
	DisDecode bool  //关闭自动解码
	Bar       bool  //是否开启bar
	DisProxy  bool  //是否关闭代理,强制关闭代理
	TryNum    int64 //重试次数

	OptionCallBack func(context.Context, *Client, *RequestOption) error //请求参数回调,用于对请求参数进行修改。返回error,中断重试请求,返回nil继续
	ResultCallBack func(context.Context, *Client, *Response) error      //结果回调,用于对结果进行校验。返回nil，直接返回,返回err的话，如果有errCallBack 走errCallBack，没有继续try
	ErrCallBack    func(context.Context, *Client, error) error          //错误回调,返回error,中断重试请求,返回nil继续

	RequestCallBack  func(context.Context, *http.Request) error
	ResponseCallBack func(context.Context, *http.Request, *http.Response) error

	Jar *Jar //自定义临时cookies 管理

	RedirectNum int              //重定向次数,小于零 关闭重定向
	DisRead     bool             //关闭默认读取请求体,不会主动读取body里面的内容，需用你自己读取
	DisUnZip    bool             //关闭自动解压
	WsOption    websocket.Option //websocket option,使用websocket 请求的option
	converUrl   string
}

func (obj *RequestOption) initBody() (err error) {
	if obj.Body != nil {
		obj.body = obj.Body
	} else if obj.Raw != nil {
		if obj.body, err = newBody(obj.Raw, rawType, nil); err != nil {
			return err
		}
	} else if obj.Form != nil {
		dataMap := map[string][]string{}
		if obj.body, err = newBody(obj.Form, formType, dataMap); err != nil {
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
		escapeQuotes := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
		for _, file := range obj.Files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes.Replace(file.Name), escapeQuotes.Replace(file.FileName)))
			if file.ContentType == "" {
				h.Set("Content-Type", "application/octet-stream")
			} else {
				h.Set("Content-Type", file.ContentType)
			}
			if wp, err := writer.CreatePart(h); err != nil {
				return err
			} else if _, err = wp.Write(file.Content); err != nil {
				return err
			}
		}
		if err = writer.Close(); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = writer.FormDataContentType()
		}
		obj.body = tempBody
	} else if obj.Files != nil {
		tempBody := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(tempBody)
		escapeQuotes := strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
		for _, file := range obj.Files {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes.Replace(file.Name), escapeQuotes.Replace(file.FileName)))
			if file.ContentType == "" {
				h.Set("Content-Type", "application/octet-stream")
			} else {
				h.Set("Content-Type", file.ContentType)
			}
			if wp, err := writer.CreatePart(h); err != nil {
				return err
			} else if _, err = wp.Write(file.Content); err != nil {
				return err
			}
		}
		if err = writer.Close(); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = writer.FormDataContentType()
		}
		obj.body = tempBody
	} else if obj.Data != nil {
		if obj.body, err = newBody(obj.Data, dataType, nil); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/x-www-form-urlencoded"
		}
	} else if obj.Json != nil {
		if obj.body, err = newBody(obj.Json, jsonType, nil); err != nil {
			return err
		}
		if obj.ContentType == "" {
			obj.ContentType = "application/json"
		}
	} else if obj.Text != nil {
		if obj.body, err = newBody(obj.Text, textType, nil); err != nil {
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
	//构造body
	if err = obj.initBody(); err != nil {
		return err
	}
	//构造params
	if obj.Params != nil {
		dataMap := map[string][]string{}
		if _, err = newBody(obj.Params, paramsType, dataMap); err != nil {
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
	//构造headers
	if err = obj.initHeaders(); err != nil {
		return err
	}
	//构造cookies
	return obj.initCookies()
}
func (obj *Client) newRequestOption(option RequestOption) RequestOption {
	if option.TryNum == 0 {
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
		if obj.headers == nil {
			option.Headers = DefaultHeaders()
		} else {
			option.Headers = obj.headers
		}
	}
	if !option.Bar {
		option.Bar = obj.bar
	}
	if option.RedirectNum == 0 {
		option.RedirectNum = obj.redirectNum
	}
	if option.Timeout == 0 {
		option.Timeout = obj.timeout
	}
	if !option.DisAlive {
		option.DisAlive = obj.disAlive
	}
	if !option.DisCookie {
		option.DisCookie = obj.disCookie
	}
	if !option.DisDecode {
		option.DisDecode = obj.disDecode
	}
	if !option.DisRead {
		option.DisRead = obj.disRead
	}
	if !option.DisUnZip {
		option.DisUnZip = obj.disUnZip
	}
	if option.Ja3Spec.IsSet() {
		option.Ja3 = true
	}
	if option.H2Ja3Spec.IsSet() {
		option.H2Ja3 = true
	}

	if option.RequestCallBack == nil {
		option.RequestCallBack = obj.requestCallBack
	}
	if option.ResponseCallBack == nil {
		option.ResponseCallBack = obj.responseCallBack
	}
	return option
}
