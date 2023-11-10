package requests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"net/textproto"
	"net/url"
	"os"
	"strings"

	"net/http"

	"github.com/gospider007/gtls"
	"github.com/gospider007/ja3"
	"github.com/gospider007/re"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
	"golang.org/x/exp/slices"
)

type keyPrincipal string

const keyPrincipalID keyPrincipal = "gospiderContextData"

var errFatal = errors.New("Fatal error")

type reqCtxData struct {
	isWs                  bool
	forceHttp1            bool
	maxRedirectNum        int
	proxy                 *url.URL
	disProxy              bool
	disAlive              bool
	orderHeaders          []string
	responseHeaderTimeout time.Duration
	tlsHandshakeTimeout   time.Duration

	requestCallBack func(context.Context, *http.Request, *http.Response) error

	h2Ja3Spec ja3.H2Ja3Spec
	ja3Spec   ja3.Ja3Spec

	dialTimeout time.Duration
	keepAlive   time.Duration
	localAddr   *net.TCPAddr  //network card ip
	addrType    gtls.AddrType //first ip type
	dns         *net.UDPAddr

	isNewConn bool
}

// sends a GET request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Get(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodGet, href, options...)
}

// sends a Head request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Head(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodHead, href, options...)
}

// sends a Post request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Post(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodPost, href, options...)
}

// sends a Put request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Put(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodPut, href, options...)
}

// sends a Patch request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Patch(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodPatch, href, options...)
}

// sends a Delete request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Delete(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodDelete, href, options...)
}

// sends a Connect request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Connect(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodConnect, href, options...)
}

// sends a Options request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Options(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodOptions, href, options...)
}

// sends a Trace request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func Trace(ctx context.Context, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(ctx, http.MethodTrace, href, options...)
}

//Define a function named Request that takes in four parameters:
//- preCtx: a context.Context object for setting request-related context information
//- method: a string representing the request method (e.g., GET, POST)
//- href: a string representing the URL link for the request
//- options: a variadic parameter of type RequestOptions for setting specific options for the request

// Return a tuple of two values:
// - resp: a pointer to a Response object containing information about the response of the request
// - err: an error encountered during the request, which is nil if no error occurred
func Request(preCtx context.Context, method string, href string, options ...RequestOption) (resp *Response, err error) {
	return defaultClient.Request(preCtx, method, href, options...)
}

// sends a Get request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Get(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodGet, href, options...)
}

// sends a Head request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Head(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodHead, href, options...)
}

// sends a Post request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Post(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodPost, href, options...)
}

// sends a Put request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Put(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodPut, href, options...)
}

// sends a Patch request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Patch(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodPatch, href, options...)
}

// sends a Delete request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Delete(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodDelete, href, options...)
}

// sends a Connect request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Connect(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodConnect, href, options...)
}

// sends a Options request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Options(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodOptions, href, options...)
}

// sends a Trace request and returns the response.
// It takes the following parameters:
//   - preCtx: A context object used to control the timeout and cancellation of the request.
//   - href: The URL to be requested.
//   - options: A variadic parameter for optional request options.
//
// The function returns a tuple of the response object and an error object (if any).
func (obj *Client) Trace(preCtx context.Context, href string, options ...RequestOption) (*Response, error) {
	return obj.Request(preCtx, http.MethodTrace, href, options...)
}

//Define a function named Request that takes in four parameters:
//- preCtx: a context.Context object for setting request-related context information
//- method: a string representing the request method (e.g., GET, POST)
//- href: a string representing the URL link for the request
//- options: a variadic parameter of type RequestOptions for setting specific options for the request

// Return a tuple of two values:
// - resp: a pointer to a Response object containing information about the response of the request
// - err: an error encountered during the request, which is nil if no error occurred
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
			resp, err = obj.request(preCtx, &option)
			if err == nil || errors.Is(err, errFatal) || option.once {
				return
			}
		}
	}
	if err == nil {
		err = errors.New("max try num")
	}
	return resp, err
}
func (obj *Client) request(preCtx context.Context, option *RequestOption) (response *Response, err error) {
	response = new(Response)
	defer func() {
		if err == nil && !response.oneceAlive() {
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
		if err = option.OptionCallBack(preCtx, obj, option); err != nil {
			return
		}
	}
	response.bar = option.Bar
	response.disUnzip = option.DisUnZip
	response.disDecode = option.DisDecode
	response.stream = option.Stream

	method := strings.ToUpper(option.Method)

	var reqs *http.Request
	//init ctxData
	ctxData := new(reqCtxData)
	ctxData.ja3Spec = option.Ja3Spec
	ctxData.h2Ja3Spec = option.H2Ja3Spec
	ctxData.forceHttp1 = option.ForceHttp1
	ctxData.disAlive = option.DisAlive
	ctxData.maxRedirectNum = option.MaxRedirectNum
	ctxData.requestCallBack = option.RequestCallBack
	ctxData.responseHeaderTimeout = option.ResponseHeaderTimeout
	ctxData.addrType = option.AddrType

	ctxData.dialTimeout = option.DialTimeout
	ctxData.keepAlive = option.KeepAlive
	ctxData.localAddr = option.LocalAddr
	ctxData.dns = option.Dns

	//init tls timeout
	if option.TlsHandshakeTimeout == 0 {
		ctxData.tlsHandshakeTimeout = time.Second * 15
	} else {
		ctxData.tlsHandshakeTimeout = option.TlsHandshakeTimeout
	}
	//init orderHeaders
	if option.OrderHeaders == nil {
		if option.Ja3Spec.IsSet() {
			ctxData.orderHeaders = ja3.DefaultH1OrderHeaders()
		}
	} else {
		orderHeaders := []string{}
		for _, key := range option.OrderHeaders {
			key = textproto.CanonicalMIMEHeaderKey(key)
			if !slices.Contains(orderHeaders, key) {
				orderHeaders = append(orderHeaders, key)
			}
		}
		for _, key := range ja3.DefaultH1OrderHeaders() {
			if !slices.Contains(orderHeaders, key) {
				orderHeaders = append(orderHeaders, key)
			}
		}
		ctxData.orderHeaders = orderHeaders
	}
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
	//init ctx,cnl
	if option.Timeout > 0 { //超时
		response.ctx, response.cnl = context.WithTimeout(context.WithValue(preCtx, keyPrincipalID, ctxData), option.Timeout)
	} else {
		response.ctx, response.cnl = context.WithCancel(context.WithValue(preCtx, keyPrincipalID, ctxData))
	}
	//init url
	href, err := option.initUrl()
	if err != nil {
		err = tools.WrapError(err, "url init error")
		return
	}
	//init body
	body, err := option.initBody()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init body error"), err)
	}
	//create request
	if body != nil {
		reqs, err = http.NewRequestWithContext(response.ctx, method, href, body)
	} else {
		reqs, err = http.NewRequestWithContext(response.ctx, method, href, nil)
	}
	if err != nil {
		return response, tools.WrapError(errFatal, errors.New("tempRequest 构造request失败"), err)
	}

	//init headers
	headers, err := option.initHeaders()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init headers error"), err)
	}
	if headers != nil {
		reqs.Header = headers
	} else {
		reqs.Header = defaultHeaders()
	}
	//add Referer
	if reqs.Header.Get("Referer") == "" {
		if option.Referer != "" {
			reqs.Header.Set("Referer", option.Referer)
		} else {
			reqs.Header.Set("Referer", fmt.Sprintf("%s://%s", reqs.URL.Scheme, reqs.URL.Host))
		}
	}

	//set ContentType
	if reqs.Header.Get("Content-Type") == "" && reqs.Header.Get("content-type") == "" && option.ContentType != "" {
		reqs.Header.Set("Content-Type", option.ContentType)
	}

	//parse Scheme
	switch reqs.URL.Scheme {
	case "ws":
		ctxData.isWs = true
		reqs.URL.Scheme = "http"
		websocket.SetClientHeadersOption(reqs.Header, option.WsOption)
	case "wss":
		ctxData.isWs = true
		reqs.URL.Scheme = "https"
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
	} else {
		reqs.Host = reqs.URL.Host
	}

	//init cookies
	cookies, err := option.initCookies()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init cookies error"), err)
	}
	if cookies != nil {
		for _, vv := range cookies {
			reqs.AddCookie(vv)
		}
	}
	//send req
	response.response, err = obj.getClient(option).Do(reqs)
	response.isNewConn = ctxData.isNewConn
	if err != nil {
		err = tools.WrapError(err, "roundTripper error")
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
