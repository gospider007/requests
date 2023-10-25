package requests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"

	"net/url"
	"os"
	"strings"
	_ "unsafe"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/re"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
)

//go:linkname ReadRequest net/http.readRequest
func ReadRequest(b *bufio.Reader) (*http.Request, error)

type RequestDebug struct {
	Proto  string
	Url    *url.URL
	Method string
	Header http.Header
	con    *bytes.Buffer
}
type ResponseDebug struct {
	Proto   string
	Url     *url.URL
	Method  string
	Header  http.Header
	con     *bytes.Buffer
	request *http.Request
	Status  string
}

func (obj *RequestDebug) Request() (*http.Request, error) {
	return ReadRequest(bufio.NewReader(bytes.NewBuffer(obj.con.Bytes())))
}
func (obj *RequestDebug) String() string {
	return obj.con.String()
}
func (obj *RequestDebug) HeadBuffer() *bytes.Buffer {
	con := bytes.NewBuffer(nil)
	con.WriteString(fmt.Sprintf("%s %s %s\r\n", obj.Method, obj.Url, obj.Proto))
	obj.Header.Write(con)
	con.WriteString("\r\n")
	return con
}

func CloneRequest(r *http.Request, bodys ...bool) (*RequestDebug, error) {
	request := new(RequestDebug)
	request.Proto = r.Proto
	request.Method = r.Method
	request.Url = r.URL
	request.Header = r.Header
	var err error
	var body bool
	if len(bodys) > 0 {
		body = bodys[0]
	}
	if body {
		request.con = bytes.NewBuffer(nil)
		if err = r.Write(request.con); err != nil {
			return request, err
		}
		req, err := request.Request()
		if err != nil {
			return nil, err
		}
		r.Body = req.Body
	} else {
		request.con = request.HeadBuffer()
	}
	return request, err
}

func (obj *ResponseDebug) Response() (*http.Response, error) {
	return http.ReadResponse(bufio.NewReader(bytes.NewBuffer(obj.con.Bytes())), obj.request)
}
func (obj *ResponseDebug) String() string {
	return obj.con.String()
}

func (obj *ResponseDebug) HeadBuffer() *bytes.Buffer {
	con := bytes.NewBuffer(nil)
	con.WriteString(fmt.Sprintf("%s %s\r\n", obj.Proto, obj.Status))
	obj.Header.Write(con)
	con.WriteString("\r\n")
	return con
}
func CloneResponse(r *http.Response, bodys ...bool) (*ResponseDebug, error) {
	response := new(ResponseDebug)
	response.con = bytes.NewBuffer(nil)
	response.Url = r.Request.URL
	response.Method = r.Request.Method
	response.Proto = r.Proto
	response.Status = r.Status
	response.Header = r.Header
	response.request = r.Request

	var err error
	var body bool
	if len(bodys) > 0 {
		body = bodys[0]
	}
	if body {
		response.con = bytes.NewBuffer(nil)
		if err = r.Write(response.con); err != nil {
			return response, err
		}
		rsp, err := response.Response()
		if err != nil {
			return nil, err
		}
		r.Body = rsp.Body
	} else {
		response.con = response.HeadBuffer()
	}
	return response, err
}

type keyPrincipal string

const keyPrincipalID keyPrincipal = "gospiderContextData"

var (
	errFatal = errors.New("Fatal error")
)

type reqCtxData struct {
	forceHttp1   bool
	redirectNum  int
	proxy        *url.URL
	disProxy     bool
	disAlive     bool
	orderHeaders []string

	requestCallBack func(context.Context, *http.Request, *http.Response) error

	h2Ja3Spec ja3.H2Ja3Spec
	ja3Spec   ja3.Ja3Spec
}

func Get(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodGet, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Head(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodHead, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Post(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodPost, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Put(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodPut, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Patch(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodPatch, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Delete(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodDelete, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Connect(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodConnect, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Options(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodOptions, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Trace(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, http.MethodTrace, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func Request(preCtx context.Context, method string, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx, ClientOption{DisAlive: true})
	resp, err = client.Request(preCtx, method, href, options...)
	if err != nil || resp == nil || !resp.oneceAlive() {
		client.Close()
	}
	return
}
func (obj *Client) Get(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodGet, href, options...)
}
func (obj *Client) Head(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodHead, href, options...)
}
func (obj *Client) Post(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodPost, href, options...)
}
func (obj *Client) Put(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodPut, href, options...)
}
func (obj *Client) Patch(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodPatch, href, options...)
}
func (obj *Client) Delete(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodDelete, href, options...)
}
func (obj *Client) Connect(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodConnect, href, options...)
}
func (obj *Client) Options(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodOptions, href, options...)
}
func (obj *Client) Trace(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodTrace, href, options...)
}

func (obj *Client) Request(preCtx context.Context, method string, href string, options ...RequestOption) (resp *Response, err error) {
	if obj == nil {
		return nil, errors.New("client is nil")
	}
	if preCtx == nil {
		preCtx = obj.ctx
	}
	var rawOption RequestOption
	if len(options) > 0 {
		rawOption = options[0]
	}
	optionBak := obj.newRequestOption(rawOption)
	if rawOption.Body != nil {
		optionBak.TryNum = 0
	}
	for tryNum := 0; tryNum <= optionBak.TryNum; tryNum++ {
		select {
		case <-obj.ctx.Done():
			obj.Close()
			return nil, tools.WrapError(obj.ctx.Err(), "client ctx 错误")
		case <-preCtx.Done():
			return nil, tools.WrapError(preCtx.Err(), "request ctx 错误")
		default:
			option := optionBak
			if option.Method == "" {
				option.Method = method
			}
			if option.Url == nil {
				if option.Url, err = url.Parse(href); err != nil {
					err = tools.WrapError(err, "url parse error")
					return
				}
			}
			resp, err = obj.request(preCtx, option)
			if err == nil || errors.Is(err, errFatal) {
				return
			}
		}
	}
	if err == nil {
		err = errors.New("max try num")
	}
	return resp, err
}
func (obj *Client) request(preCtx context.Context, option RequestOption) (response *Response, err error) {
	response = new(Response)
	defer func() {
		if err == nil && !response.oneceAlive() && !option.DisRead {
			err = response.ReadBody()
			defer response.Close()
		}
		if err == nil && option.ResultCallBack != nil {
			err = option.ResultCallBack(preCtx, obj, response)
		}
		if err != nil {
			response.Close()
			if option.ErrCallBack != nil {
				if err2 := option.ErrCallBack(preCtx, obj, err); err2 != nil {
					err = tools.WrapError(errFatal, err2)
				}
			}
		}
	}()
	if option.OptionCallBack != nil {
		if err = option.OptionCallBack(preCtx, obj, &option); err != nil {
			return
		}
	}
	if err = option.optionInit(); err != nil {
		err = tools.WrapError(err, "option init error")
		return
	}
	response.bar = option.Bar
	response.disUnzip = option.DisUnZip
	response.disDecode = option.DisDecode

	method := strings.ToUpper(option.Method)
	href := option.converUrl
	var reqs *http.Request
	//init ctxData
	ctxData := new(reqCtxData)
	ctxData.forceHttp1 = option.ForceHttp1
	ctxData.disAlive = option.DisAlive
	ctxData.requestCallBack = option.RequestCallBack
	ctxData.orderHeaders = option.OrderHeaders
	//init proxy
	ctxData.disProxy = option.DisProxy
	if !ctxData.disProxy {
		if option.Proxy != "" {
			tempProxy, err := gtls.VerifyProxy(option.Proxy)
			if err != nil {
				return response, tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
			}
			ctxData.proxy = tempProxy
		} else if obj.proxy != nil {
			ctxData.proxy = obj.proxy
		}
	}
	//fingerprint
	ctxData.ja3Spec = option.Ja3Spec
	ctxData.h2Ja3Spec = option.H2Ja3Spec

	//redirect
	if option.RedirectNum != 0 {
		ctxData.redirectNum = option.RedirectNum
	}
	//init ctx,cnl
	if option.Timeout > 0 { //超时
		response.ctx, response.cnl = context.WithTimeout(context.WithValue(preCtx, keyPrincipalID, ctxData), option.Timeout)
	} else {
		response.ctx, response.cnl = context.WithCancel(context.WithValue(preCtx, keyPrincipalID, ctxData))
	}
	//create request
	if option.Body != nil {
		reqs, err = http.NewRequestWithContext(response.ctx, method, href, option.Body)
	} else {
		reqs, err = http.NewRequestWithContext(response.ctx, method, href, nil)
	}
	if err != nil {
		return response, tools.WrapError(errFatal, errors.New("tempRequest 构造request失败"), err)
	}
	//add headers
	var headOk bool
	if reqs.Header, headOk = option.Headers.(http.Header); !headOk {
		return response, tools.WrapError(errFatal, "request headers 转换错误")
	}
	//add Referer
	if option.Referer != "" && reqs.Header.Get("Referer") == "" {
		reqs.Header.Set("Referer", option.Referer)
	}

	//set ContentType
	if reqs.Header.Get("Content-Type") == "" && reqs.Header.Get("content-type") == "" && option.ContentType != "" {
		reqs.Header.Set("Content-Type", option.ContentType)
	}

	//parse Scheme
	switch reqs.URL.Scheme {
	case "ws", "wss":
		websocket.SetClientHeadersOption(reqs.Header, option.WsOption)
	case "file":
		response.filePath = re.Sub(`^/+`, "", reqs.URL.Path)
		response.content, err = os.ReadFile(response.filePath)
		if err != nil {
			err = tools.WrapError(errFatal, errors.New("read filePath data error"), err)
		}
		return
	case "http", "https":
	default:
		err = tools.WrapError(errFatal, fmt.Errorf("url scheme error: %s", reqs.URL.Scheme))
		return
	}

	//add host
	if option.Host != "" {
		reqs.Host = option.Host
	} else if reqs.Header.Get("Host") != "" {
		reqs.Host = reqs.Header.Get("Host")
	}
	//add cookies
	if option.Cookies != nil {
		cooks, cookOk := option.Cookies.(Cookies)
		if !cookOk {
			return response, tools.WrapError(errFatal, "request cookies transport error")
		}
		for _, vv := range cooks {
			reqs.AddCookie(vv)
		}
	}
	if response.response, err = obj.getClient(option).Do(reqs); err != nil {
		return
	} else if response.response == nil {
		err = errors.New("response is nil")
		return
	}
	if !response.disUnzip {
		response.disUnzip = response.response.Uncompressed
	}
	if response.response.StatusCode == 101 {
		response.webSocket, err = websocket.NewClientConn(response.response)
	} else if response.response.Header.Get("Content-Type") == "text/event-stream" {
		response.sseClient = newSseClient(response)
	}
	return
}
