package requests

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"net/url"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/re"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
)

type contextKey string

const gospiderContextKey contextKey = "GospiderContextKey"

var errFatal = errors.New("ErrFatal")

var ErrUseLastResponse = http.ErrUseLastResponse

func CreateReqCtx(ctx context.Context, option *RequestOption) context.Context {
	return context.WithValue(ctx, gospiderContextKey, option)
}
func GetRequestOption(ctx context.Context) *RequestOption {
	option, ok := ctx.Value(gospiderContextKey).(*RequestOption)
	if ok {
		return option
	}
	return nil
}

// sends a GET request and returns the response.
func Get(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodGet, href, options...)
}

// sends a Head request and returns the response.
func Head(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodHead, href, options...)
}

// sends a Post request and returns the response.
func Post(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodPost, href, options...)
}

// sends a Put request and returns the response.
func Put(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodPut, href, options...)
}

// sends a Patch request and returns the response.
func Patch(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodPatch, href, options...)
}

// sends a Delete request and returns the response.
func Delete(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodDelete, href, options...)
}

// sends a Connect request and returns the response.
func Connect(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodConnect, href, options...)
}

// sends a Options request and returns the response.
func Options(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodOptions, href, options...)
}

// sends a Trace request and returns the response.
func Trace(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodTrace, href, options...)
}

// Define a function named Request that takes in four parameters:
func Request(ctx context.Context, method string, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, method, href, options...)
}

// sends a Get request and returns the response.
func (obj *Client) Get(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodGet, href, options...)
}

// sends a Head request and returns the response.
func (obj *Client) Head(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodHead, href, options...)
}

// sends a Post request and returns the response.
func (obj *Client) Post(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodPost, href, options...)
}

// sends a Put request and returns the response.
func (obj *Client) Put(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodPut, href, options...)
}

// sends a Patch request and returns the response.
func (obj *Client) Patch(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodPatch, href, options...)
}

// sends a Delete request and returns the response.
func (obj *Client) Delete(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodDelete, href, options...)
}

// sends a Connect request and returns the response.
func (obj *Client) Connect(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodConnect, href, options...)
}

// sends a Options request and returns the response.
func (obj *Client) Options(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodOptions, href, options...)
}

// sends a Trace request and returns the response.
func (obj *Client) Trace(ctx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(ctx, http.MethodTrace, href, options...)
}

// Define a function named Request that takes in four parameters:
func (obj *Client) Request(ctx context.Context, method string, href string, options ...RequestOption) (response *Response, err error) {
	if obj.closed {
		return nil, errors.New("client is closed")
	}
	if ctx == nil {
		ctx = obj.ctx
	}
	var rawOption RequestOption
	if len(options) > 0 {
		rawOption = options[0]
	}
	optionBak := obj.newRequestOption(rawOption)
	optionBak.requestId = tools.NaoId()
	if optionBak.Method == "" {
		optionBak.Method = method
	}
	uhref := optionBak.Url
	if uhref == nil {
		if uhref, err = url.Parse(href); err != nil {
			err = tools.WrapError(err, "url parse error")
			return
		}
	}
	for maxRetries := 0; maxRetries <= optionBak.MaxRetries; maxRetries++ {
		option := optionBak
		option.Url = cloneUrl(uhref)
		option.client = obj
		response, err = obj.request(ctx, &option)
		if err == nil || errors.Is(err, errFatal) || option.once {
			return
		}
	}
	return
}
func (obj *Client) request(ctx context.Context, option *RequestOption) (response *Response, err error) {
	response = new(Response)
	defer func() {
		//read body
		if err == nil && !response.IsWebSocket() && !response.IsSSE() && !response.IsStream() {
			err = response.ReadBody()
		}
		//result callback
		if err == nil && option.ResultCallBack != nil {
			err = option.ResultCallBack(ctx, option, response)
		}
		if err != nil { //err callback, must close body
			response.CloseBody()
			if option.ErrCallBack != nil {
				if err2 := option.ErrCallBack(ctx, option, response, err); err2 != nil {
					err = tools.WrapError(errFatal, err2)
				}
			}
		}
	}()
	if option.OptionCallBack != nil {
		if err = option.OptionCallBack(ctx, option); err != nil {
			return
		}
	}
	response.requestOption = option
	//init headers and orderheaders,befor init ctxData
	headers, err := option.initHeaders()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init headers error"), err)
	}
	if headers != nil && option.UserAgent != "" {
		headers.Set("User-Agent", option.UserAgent)
	}
	//设置 h2 请求头顺序
	if option.OrderHeaders != nil {
		if !option.H2Ja3Spec.IsSet() {
			option.H2Ja3Spec = ja3.DefaultH2Ja3Spec()
			option.H2Ja3Spec.OrderHeaders = option.OrderHeaders
		} else if option.H2Ja3Spec.OrderHeaders == nil {
			option.H2Ja3Spec.OrderHeaders = option.OrderHeaders
		}
	}
	//init tls timeout
	if option.TlsHandshakeTimeout == 0 {
		option.TlsHandshakeTimeout = time.Second * 15
	}
	//init proxy
	if option.Proxy != "" {
		tempProxy, err := gtls.VerifyProxy(option.Proxy)
		if err != nil {
			return nil, tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
		}
		option.proxy = tempProxy
	}
	if l := len(option.Proxys); l > 0 {
		option.proxys = make([]*url.URL, l)
		for i, proxy := range option.Proxys {
			tempProxy, err := gtls.VerifyProxy(proxy)
			if err != nil {
				return response, tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
			}
			option.proxys[i] = tempProxy
		}
	}
	//init headers
	if headers == nil {
		headers = defaultHeaders()
	}
	//设置 h1 请求头顺序
	if option.OrderHeaders == nil {
		option.OrderHeaders = ja3.DefaultOrderHeaders()
	}
	//init ctx,cnl
	if option.Timeout > 0 { //超时
		response.ctx, response.cnl = context.WithTimeout(CreateReqCtx(ctx, option), option.Timeout)
	} else {
		response.ctx, response.cnl = context.WithCancel(CreateReqCtx(ctx, option))
	}
	//init Scheme
	switch option.Url.Scheme {
	case "file":
		response.filePath = re.Sub(`^/+`, "", option.Url.Path)
		response.content, err = os.ReadFile(response.filePath)
		if err != nil {
			err = tools.WrapError(errFatal, errors.New("read filePath data error"), err)
		}
		return
	case "ws":
		option.ForceHttp1 = true
		option.Url.Scheme = "http"
		websocket.SetClientHeadersWithOption(headers, option.WsOption)
	case "wss":
		option.ForceHttp1 = true
		option.Url.Scheme = "https"
		websocket.SetClientHeadersWithOption(headers, option.WsOption)
	}
	//init url
	href, err := option.initParams()
	if err != nil {
		err = tools.WrapError(err, "url init error")
		return
	}
	if href.User != nil {
		headers.Set("Authorization", "Basic "+tools.Base64Encode(href.User.String()))
	}
	//init body
	body, err := option.initBody(response.ctx)
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init body error"), err)
	}
	//create request
	reqs, err := NewRequestWithContext(response.ctx, option.Method, href, body)
	if err != nil {
		return response, tools.WrapError(errFatal, errors.New("tempRequest 构造request失败"), err)
	}
	reqs.Header = headers
	//add Referer

	if reqs.Header.Get("Referer") == "" && option.Referer != "" {
		reqs.Header.Set("Referer", option.Referer)
	}

	//set ContentType
	if option.ContentType != "" && reqs.Header.Get("Content-Type") == "" {
		reqs.Header.Set("Content-Type", option.ContentType)
	}

	//add host
	if option.Host != "" {
		reqs.Host = option.Host
	} else if reqs.Header.Get("Host") != "" {
		reqs.Host = reqs.Header.Get("Host")
	} else {
		reqs.Host = reqs.URL.Host
	}

	//init cookies
	cookies, err := option.initCookies()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init cookies error"), err)
	}
	if cookies != nil {
		addCookie(reqs, cookies)
	}
	//send req
	response.response, err = obj.do(reqs, option)
	if err != nil && err != ErrUseLastResponse {
		err = tools.WrapError(err, "client do error")
		return
	}
	if response.response == nil {
		err = errors.New("response is nil")
		return
	}
	if response.Body() != nil {
		response.rawConn = response.Body().(*readWriteCloser)
	}
	if !response.requestOption.DisUnZip {
		response.requestOption.DisUnZip = response.response.Uncompressed
	}
	if response.response.StatusCode == 101 {
		response.webSocket = websocket.NewClientConn(response.rawConn.Conn(), websocket.GetResponseHeaderOption(response.response.Header))
	} else if strings.Contains(response.response.Header.Get("Content-Type"), "text/event-stream") {
		response.sse = newSSE(response)
	} else if !response.requestOption.DisUnZip {
		var unCompressionBody io.ReadCloser
		unCompressionBody, err = tools.CompressionDecode(response.Body(), response.ContentEncoding())
		if err != nil {
			if err != io.ErrUnexpectedEOF && err != io.EOF {
				return
			}
		}
		if unCompressionBody != nil {
			response.response.Body = unCompressionBody
		}
	}
	return
}
