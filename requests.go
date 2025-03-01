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
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		option := optionBak
		option.Url = cloneUrl(uhref)
		response = NewResponse(ctx, option)
		response.client = obj
		response.requestId = requestId
		err = obj.request(response)
		if err == nil || errors.Is(err, errFatal) || response.Option().once {
			return
		}
		optionBak.MaxRetries = response.Option().MaxRetries
	}
	return
}
func (obj *Client) request(ctx *Response) (err error) {
	defer func() {
		//read body
		if err == nil && !ctx.IsSSE() && !ctx.IsStream() {
			err = ctx.ReadBody()
		}
		//result callback

		if err == nil && ctx.option.ResultCallBack != nil {
			err = ctx.option.ResultCallBack(ctx)
		}
		if err != nil { //err callback, must close body
			ctx.CloseConn()
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
	//init tls timeout
	if ctx.option.TlsHandshakeTimeout == 0 {
		ctx.option.TlsHandshakeTimeout = time.Second * 15
	}
	//init proxy
	if ctx.option.Proxy != nil {
		ctx.proxys, err = parseProxy(ctx.option.Proxy)
		if err != nil {
			return tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
		}
	}
	//init ctx,cnl
	if ctx.option.Timeout > 0 { //超时
		ctx.ctx, ctx.cnl = context.WithTimeout(ctx.Context(), ctx.option.Timeout)
	} else {
		ctx.ctx, ctx.cnl = context.WithCancel(ctx.Context())
	}
	var isWebsocket bool
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
		isWebsocket = true
	case "wss":
		ctx.option.ForceHttp1 = true
		ctx.option.Url.Scheme = "https"
		isWebsocket = true
	}
	//init url
	href, err := ctx.option.initParams()
	if err != nil {
		err = tools.WrapError(err, "url init error")
		return
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
	//init headers
	if reqs.Header, err = ctx.option.initOrderHeaders(); err != nil {
		return tools.WrapError(err, errors.New("tempRequest init headers error"), err)
	}
	if isWebsocket && reqs.Header.Get("Sec-WebSocket-Key") == "" {
		websocket.SetClientHeadersWithOption(reqs.Header, ctx.option.WsOption)
	}
	if href.User != nil && reqs.Header.Get("Authorization") == "" {
		reqs.Header.Set("Authorization", "Basic "+tools.Base64Encode(href.User.String()))
	}
	if ctx.option.UserAgent != "" && reqs.Header.Get("User-Agent") == "" {
		reqs.Header.Set("User-Agent", ctx.option.UserAgent)
	}
	if ctx.option.Referer != "" && reqs.Header.Get("Referer") == "" {
		reqs.Header.Set("Referer", ctx.option.Referer)
	}
	if ctx.option.ContentType != "" && reqs.Header.Get("Content-Type") == "" {
		reqs.Header.Set("Content-Type", ctx.option.ContentType)
	}
	if ctx.option.Host != "" && reqs.Header.Get("Host") == "" {
		reqs.Header.Set("Host", ctx.option.Host)
	}
	//init headers ok
	//init cookies
	cookies, err := ctx.option.initCookies()
	if err != nil {
		return tools.WrapError(err, errors.New("tempRequest init cookies error"), err)
	}
	if cookies != nil {
		addCookie(reqs, cookies)
	}
	if err = ctx.option.initSpec(); err != nil {
		return err
	}
	ctx.request = reqs
	//init spec

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
	if strings.Contains(ctx.response.Header.Get("Content-Type"), "text/event-stream") {
		ctx.sse = newSSE(ctx)
	} else if encoding := ctx.ContentEncoding(); encoding != "" {
		var unCompressionBody io.ReadCloser
		unCompressionBody, err = tools.CompressionHeadersDecode(ctx.Context(), ctx.Body(), encoding)
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
