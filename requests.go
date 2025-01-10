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

var errFatal = errors.New("ErrFatal")

var ErrUseLastResponse = http.ErrUseLastResponse

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
	optionBak, err := obj.newRequestOption(rawOption)
	if err != nil {
		return nil, err
	}
	requestId := tools.NaoId()
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
	for ; optionBak.MaxRetries >= 0; optionBak.MaxRetries-- {
		option := optionBak
		option.Url = cloneUrl(uhref)
		response = NewResponse(ctx, option)
		response.client = obj
		response.requestId = requestId
		err = obj.request(response)
		if err == nil || errors.Is(err, errFatal) || option.once {
			return
		}
		optionBak.MaxRetries = option.MaxRedirect
	}
	return
}
func (obj *Client) request(ctx *Response) (err error) {

	defer func() {
		//read body
		if err == nil && !ctx.IsWebSocket() && !ctx.IsSSE() && !ctx.IsStream() {
			err = ctx.ReadBody()
		}
		//result callback

		if err == nil && ctx.option.ResultCallBack != nil {
			err = ctx.option.ResultCallBack(ctx)
		}
		if err != nil { //err callback, must close body
			ctx.CloseBody()
			if ctx.option.ErrCallBack != nil {
				ctx.err = err
				if err2 := ctx.option.ErrCallBack(ctx); err2 != nil {
					err = tools.WrapError(errFatal, err2)
				}
			}
		}
	}()
	if ctx.option.OptionCallBack != nil {
		if err = ctx.option.OptionCallBack(ctx); err != nil {
			return
		}
	}
	//init headers and orderheaders,befor init ctxData
	headers, err := ctx.option.initHeaders()
	if err != nil {
		return tools.WrapError(err, errors.New("tempRequest init headers error"), err)
	}
	if headers != nil && ctx.option.UserAgent != "" {
		headers.Set("User-Agent", ctx.option.UserAgent)
	}
	//设置 h2 请求头顺序
	if ctx.option.OrderHeaders != nil {
		if !ctx.option.H2Ja3Spec.IsSet() {
			ctx.option.H2Ja3Spec = ja3.DefaultH2Spec()
			ctx.option.H2Ja3Spec.OrderHeaders = ctx.option.OrderHeaders
		} else if ctx.option.H2Ja3Spec.OrderHeaders == nil {
			ctx.option.H2Ja3Spec.OrderHeaders = ctx.option.OrderHeaders
		}
	}
	//init tls timeout
	if ctx.option.TlsHandshakeTimeout == 0 {
		ctx.option.TlsHandshakeTimeout = time.Second * 15
	}
	//init proxy
	if ctx.option.Proxy != "" {
		tempProxy, err := gtls.VerifyProxy(ctx.option.Proxy)
		if err != nil {
			return tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
		}
		ctx.proxys = []*url.URL{tempProxy}
	}
	if l := len(ctx.option.Proxys); l > 0 {
		ctx.proxys = make([]*url.URL, l)
		for i, proxy := range ctx.option.Proxys {
			tempProxy, err := gtls.VerifyProxy(proxy)
			if err != nil {
				return tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
			}
			ctx.proxys[i] = tempProxy
		}
	}
	//init headers
	if headers == nil {
		headers = defaultHeaders()
	}
	//设置 h1 请求头顺序
	if ctx.option.OrderHeaders == nil {
		ctx.option.OrderHeaders = ja3.DefaultOrderHeaders()
	}
	//init ctx,cnl
	if ctx.option.Timeout > 0 { //超时
		ctx.ctx, ctx.cnl = context.WithTimeout(ctx.Context(), ctx.option.Timeout)
	} else {
		ctx.ctx, ctx.cnl = context.WithCancel(ctx.Context())
	}
	//init Scheme
	switch ctx.option.Url.Scheme {
	case "file":
		ctx.filePath = re.Sub(`^/+`, "", ctx.option.Url.Path)
		ctx.content, err = os.ReadFile(ctx.filePath)
		if err != nil {
			err = tools.WrapError(errFatal, errors.New("read filePath data error"), err)
		}
		return
	case "ws":
		ctx.option.ForceHttp1 = true
		ctx.option.Url.Scheme = "http"
		websocket.SetClientHeadersWithOption(headers, ctx.option.WsOption)
	case "wss":
		ctx.option.ForceHttp1 = true
		ctx.option.Url.Scheme = "https"
		websocket.SetClientHeadersWithOption(headers, ctx.option.WsOption)
	}
	//init url
	href, err := ctx.option.initParams()
	if err != nil {
		err = tools.WrapError(err, "url init error")
		return
	}
	if href.User != nil {
		headers.Set("Authorization", "Basic "+tools.Base64Encode(href.User.String()))
	}
	//init body
	body, err := ctx.option.initBody(ctx.ctx)
	if err != nil {
		return tools.WrapError(err, errors.New("tempRequest init body error"), err)
	}
	//create request
	reqs, err := NewRequestWithContext(ctx.Context(), ctx.option.Method, href, body)
	if err != nil {
		return tools.WrapError(errFatal, errors.New("tempRequest 构造request失败"), err)
	}
	reqs.Header = headers
	//add Referer

	if reqs.Header.Get("Referer") == "" && ctx.option.Referer != "" {
		reqs.Header.Set("Referer", ctx.option.Referer)
	}

	//set ContentType
	if ctx.option.ContentType != "" && reqs.Header.Get("Content-Type") == "" {
		reqs.Header.Set("Content-Type", ctx.option.ContentType)
	}

	//add host
	if ctx.option.Host != "" {
		reqs.Host = ctx.option.Host
	} else if reqs.Header.Get("Host") != "" {
		reqs.Host = reqs.Header.Get("Host")
	} else {
		reqs.Host = reqs.URL.Host
	}

	//init cookies
	cookies, err := ctx.option.initCookies()
	if err != nil {
		return tools.WrapError(err, errors.New("tempRequest init cookies error"), err)
	}
	if cookies != nil {
		addCookie(reqs, cookies)
	}
	ctx.request = reqs
	//send req
	err = obj.do(ctx)
	if err != nil && err != ErrUseLastResponse {
		err = tools.WrapError(err, "client do error")
		return
	}
	if ctx.response == nil {
		err = errors.New("response is nil")
		return
	}
	if ctx.Body() != nil {
		ctx.rawConn = ctx.Body().(*readWriteCloser)
	}
	if !ctx.option.DisUnZip {
		ctx.option.DisUnZip = ctx.response.Uncompressed
	}
	if ctx.response.StatusCode == 101 {
		ctx.webSocket = websocket.NewClientConn(ctx.rawConn.Conn(), websocket.GetResponseHeaderOption(ctx.response.Header))
	} else if strings.Contains(ctx.response.Header.Get("Content-Type"), "text/event-stream") {
		ctx.sse = newSSE(ctx)
	} else if !ctx.option.DisUnZip {
		var unCompressionBody io.ReadCloser
		unCompressionBody, err = tools.CompressionDecode(ctx.Body(), ctx.ContentEncoding())
		if err != nil {
			if err != io.ErrUnexpectedEOF && err != io.EOF {
				return
			}
		}
		if unCompressionBody != nil {
			ctx.response.Body = unCompressionBody
		}
	}
	return
}
