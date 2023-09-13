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

	"gitee.com/baixudong/ja3"
	"gitee.com/baixudong/re"
	"gitee.com/baixudong/tools"
	"gitee.com/baixudong/websocket"
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
	errFatal = errors.New("致命错误")
)

type reqCtxData struct {
	redirectNum int
	proxy       *url.URL
	disProxy    bool

	disAlive         bool
	ws               bool
	requestCallBack  func(context.Context, *http.Request) error
	responseCallBack func(context.Context, *http.Request, *http.Response) error

	h2Ja3Spec ja3.H2Ja3Spec
	ja3Spec   ja3.Ja3Spec
}

func Get(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodGet, href, options...)
}
func Head(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodHead, href, options...)
}
func Post(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodPost, href, options...)
}
func Put(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodPut, href, options...)
}
func Patch(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodPatch, href, options...)
}
func Delete(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodDelete, href, options...)
}
func Connect(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodConnect, href, options...)
}
func Options(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodOptions, href, options...)
}
func Trace(preCtx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, http.MethodTrace, href, options...)
}
func Request(preCtx context.Context, method string, href string, options ...RequestOption) (resp *Response, err error) {
	client, _ := NewClient(preCtx)
	defer func() {
		if err != nil || (resp.webSocket == nil && resp.sseClient == nil) {
			client.Close()
		}
	}()
	return client.Request(preCtx, method, href, options...)
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

// 发送请求
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
	//开始请求
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
					err = tools.WrapError(err, "url 解析错误")
					return
				}
			}
			resp, err = obj.request(preCtx, option)
			if err == nil || errors.Is(err, errFatal) { //致命错误,或没有错误,直接返回
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
		if err == nil && response.webSocket == nil && response.sseClient == nil && !option.DisRead { //判断是否读取body,和对body的处理
			err = response.ReadBody()
			defer response.Close()
		}
		if err == nil && option.ResultCallBack != nil { //result 回调处理
			err = option.ResultCallBack(preCtx, obj, response)
		}
		if err != nil { //err 回调处理
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
		err = tools.WrapError(err, "option 初始化错误")
		return
	}
	response.bar = option.Bar
	response.disUnzip = option.DisUnZip
	response.disDecode = option.DisDecode

	method := strings.ToUpper(option.Method)
	href := option.converUrl
	var reqs *http.Request
	//构造ctxData
	ctxData := new(reqCtxData)
	ctxData.disAlive = option.DisAlive
	ctxData.requestCallBack = option.RequestCallBack
	ctxData.responseCallBack = option.ResponseCallBack
	//构造代理
	ctxData.disProxy = option.DisProxy
	if !ctxData.disProxy {
		if option.Proxy != "" { //代理相关构造
			tempProxy, err := tools.VerifyProxy(option.Proxy)
			if err != nil {
				return response, tools.WrapError(errFatal, errors.New("tempRequest 构造代理失败"), err)
			}
			ctxData.proxy = tempProxy
		} else if obj.proxy != nil {
			ctxData.proxy = obj.proxy
		}
	}
	//指纹
	ctxData.ja3Spec = option.Ja3Spec
	ctxData.h2Ja3Spec = option.H2Ja3Spec

	//重定向
	if option.RedirectNum != 0 { //重定向次数
		ctxData.redirectNum = option.RedirectNum
	}
	//构造ctx,cnl
	if option.Timeout > 0 { //超时
		response.ctx, response.cnl = context.WithTimeout(context.WithValue(preCtx, keyPrincipalID, ctxData), option.Timeout)
	} else {
		response.ctx, response.cnl = context.WithCancel(context.WithValue(preCtx, keyPrincipalID, ctxData))
	}
	//创建request
	if option.Body != nil {
		reqs, err = http.NewRequestWithContext(response.ctx, method, href, option.Body)
	} else {
		reqs, err = http.NewRequestWithContext(response.ctx, method, href, nil)
	}
	if err != nil {
		return response, tools.WrapError(errFatal, errors.New("tempRequest 构造request失败"), err)
	}

	//判断file
	if reqs.URL.Scheme == "file" { //文件直接返回
		response.filePath = re.Sub(`^/+`, "", reqs.URL.Path)
		response.content, err = os.ReadFile(response.filePath)
		if err != nil {
			err = tools.WrapError(errFatal, errors.New("read filePath data error"), err)
		}
		return
	}

	//判断ws
	switch reqs.URL.Scheme {
	case "ws":
		ctxData.ws = true
		reqs.URL.Scheme = "http"
	case "wss":
		ctxData.ws = true
		reqs.URL.Scheme = "https"
	}

	//添加headers
	var headOk bool
	if reqs.Header, headOk = option.Headers.(http.Header); !headOk {
		return response, tools.WrapError(errFatal, "request headers 转换错误")
	}
	//添加Referer
	if option.Referer != "" && reqs.Header.Get("Referer") == "" {
		reqs.Header.Set("Referer", option.Referer)
	}

	//设置 ContentType 值
	if reqs.Header.Get("Content-Type") == "" && reqs.Header.Get("content-type") == "" && option.ContentType != "" {
		reqs.Header.Set("Content-Type", option.ContentType)
	}
	//host构造
	if option.Host != "" {
		reqs.Host = option.Host
	} else if reqs.Header.Get("Host") != "" {
		reqs.Host = reqs.Header.Get("Host")
	}
	//添加cookies
	if option.Cookies != nil {
		cooks, cookOk := option.Cookies.(Cookies)
		if !cookOk {
			return response, tools.WrapError(errFatal, "request cookies 转换错误")
		}
		for _, vv := range cooks {
			reqs.AddCookie(vv)
		}
	}
	//开始发送请求
	if ctxData.ws { //设置websocket headers
		websocket.SetClientHeaders(reqs.Header, option.WsOption)
	}
	if response.response, err = obj.getClient(option).Do(reqs); err != nil {
		return
	}
	if response.response == nil {
		err = errors.New("response is nil")
		return
	}
	if !response.disUnzip {
		response.disUnzip = response.response.Uncompressed
	}
	if ctxData.ws { //判断ws 的状态码是否正确
		if response.response.StatusCode == 101 {
			response.webSocket, err = websocket.NewClientConn(response.response)
		} else {
			err = errors.New("statusCode not 101, url为websocket链接，但是对方服务器没有将请求升级到websocket")
		}
	} else if response.response.Header.Get("Content-Type") == "text/event-stream" { //如果为sse协议就关闭读取
		response.sseClient = newSseClient(response)
	}
	return
}
