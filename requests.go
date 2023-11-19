package requests

import (
	"context"
	"errors"
	"fmt"
	"log"
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

type contextKey string

const gospiderContextKey contextKey = "GospiderContextKey"

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
	debug     bool
	requestId string
}

func NewReqCtxData(ctx context.Context, option *RequestOption) (*reqCtxData, error) {
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
	ctxData.debug = option.Debug
	if option.Debug {
		ctxData.requestId = tools.NaoId()
	}
	//init scheme
	if option.Url != nil {
		switch option.Url.Scheme {
		case "ws":
			ctxData.isWs = true
			option.Url.Scheme = "http"
		case "wss":
			ctxData.isWs = true
			option.Url.Scheme = "https"
		}
	}

	//init tls timeout
	if option.TlsHandshakeTimeout == 0 {
		ctxData.tlsHandshakeTimeout = time.Second * 15
	} else {
		ctxData.tlsHandshakeTimeout = option.TlsHandshakeTimeout
	}
	//init orderHeaders,this must after init headers
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

	if option.Proxy != "" {
		tempProxy, err := gtls.VerifyProxy(option.Proxy)
		if err != nil {
			return nil, tools.WrapError(errFatal, errors.New("tempRequest init proxy error"), err)
		}
		ctxData.proxy = tempProxy
	}
	return ctxData, nil
}
func CreateReqCtx(ctx context.Context, ctxData *reqCtxData) context.Context {
	return context.WithValue(ctx, gospiderContextKey, ctxData)
}
func GetReqCtxData(ctx context.Context) *reqCtxData {
	return ctx.Value(gospiderContextKey).(*reqCtxData)
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
func (obj *Client) Request(ctx context.Context, method string, href string, options ...RequestOption) (resp *Response, err error) {
	if obj == nil {
		return nil, errors.New("client is nil")
	}
	if ctx == nil {
		ctx = obj.ctx
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
		case <-ctx.Done():
			return nil, tools.WrapError(ctx.Err(), "request ctx 错误")
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
			resp, err = obj.request(ctx, &option)
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
func (obj *Client) request(ctx context.Context, option *RequestOption) (response *Response, err error) {
	response = new(Response)
	defer func() {
		if err == nil && !response.IsStream() {
			err = response.ReadBody()
			defer response.CloseBody()
		}
		if err == nil && option.ResultCallBack != nil {
			err = option.ResultCallBack(ctx, obj, response)
		}
		if err != nil {
			response.CloseBody()
			if option.ErrCallBack != nil {
				if err2 := option.ErrCallBack(ctx, obj, response, err); err2 != nil {
					err = tools.WrapError(errFatal, err2)
				}
			}
		}
	}()
	if option.OptionCallBack != nil {
		if err = option.OptionCallBack(ctx, obj, option); err != nil {
			return
		}
	}
	//init headers and orderheaders,befor init ctxData
	headers, err := option.initHeaders()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init headers error"), err)
	}
	if headers == nil {
		headers = defaultHeaders()
	}
	response.bar = option.Bar
	response.disUnzip = option.DisUnZip
	response.disDecode = option.DisDecode
	response.stream = option.Stream

	method := strings.ToUpper(option.Method)

	var reqs *http.Request
	//init ctxData
	ctxData, err := NewReqCtxData(ctx, option)
	if err != nil {
		return response, tools.WrapError(err, " reqCtxData init error")
	}
	//init ctx,cnl
	if option.Timeout > 0 { //超时
		response.ctx, response.cnl = context.WithTimeout(CreateReqCtx(ctx, ctxData), option.Timeout)
	} else {
		response.ctx, response.cnl = context.WithCancel(CreateReqCtx(ctx, ctxData))
	}
	//init url
	href, err := option.initParams()
	if err != nil {
		err = tools.WrapError(err, "url init error")
		return
	}
	//init body
	body, err := option.initBody()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init body error"), err)
	}
	if ctxData.debug {
		log.Printf("requestId:%s: %s", ctxData.requestId, "create request")
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
	reqs.Header = headers
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

	//init ws
	if ctxData.isWs {
		if ctxData.debug {
			log.Printf("requestId:%s: %s", ctxData.requestId, "init websocket headers")
		}
		websocket.SetClientHeadersOption(reqs.Header, option.WsOption)
	}
	switch reqs.URL.Scheme {
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
	if ctxData.debug {
		log.Printf("requestId:%s: %s", ctxData.requestId, "send request start")
	}
	//send req
	response.response, err = obj.getClient(option).Do(reqs)
	if ctxData.debug {
		log.Printf("requestId:%s: %s", ctxData.requestId, "send request end")
	}
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
		response.webSocket, err = websocket.NewClientConn(response.response, response.CloseConn)
	} else if response.response.Header.Get("Content-Type") == "text/event-stream" {
		response.sse = newSse(response.response.Body, response.CloseConn)
	}
	return
}
