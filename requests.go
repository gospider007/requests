package requests

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"net/textproto"
	"net/url"
	"os"

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
	maxRedirect           int
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
	isNewConn   bool
}

func NewReqCtxData(ctx context.Context, option *RequestOption) (*reqCtxData, error) {
	//init ctxData
	ctxData := new(reqCtxData)
	ctxData.ja3Spec = option.Ja3Spec
	ctxData.h2Ja3Spec = option.H2Ja3Spec
	ctxData.forceHttp1 = option.ForceHttp1
	ctxData.disAlive = option.DisAlive
	ctxData.maxRedirect = option.MaxRedirect
	ctxData.requestCallBack = option.RequestCallBack
	ctxData.responseHeaderTimeout = option.ResponseHeaderTimeout
	ctxData.addrType = option.AddrType
	ctxData.dialTimeout = option.DialTimeout
	ctxData.keepAlive = option.KeepAlive
	ctxData.localAddr = option.LocalAddr
	ctxData.dns = option.Dns
	ctxData.disProxy = option.DisProxy
	ctxData.tlsHandshakeTimeout = option.TlsHandshakeTimeout
	ctxData.orderHeaders = option.OrderHeaders

	//init scheme
	if option.Url != nil {
		if option.Url.Scheme == "ws" {
			ctxData.isWs = true
			option.Url.Scheme = "http"
		} else if option.Url.Scheme == "wss" {
			ctxData.isWs = true
			option.Url.Scheme = "https"
		}
	}
	//init tls timeout
	if option.TlsHandshakeTimeout == 0 {
		ctxData.tlsHandshakeTimeout = time.Second * 15
	}

	//init orderHeaders,this must after init headers
	if ctxData.orderHeaders == nil {
		ctxData.orderHeaders = ja3.DefaultH1OrderHeaders()
	} else {
		orderHeaders := []string{}
		for _, key := range ctxData.orderHeaders {
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
		if err == nil && !response.IsStream() {
			err = response.ReadBody()
		}
		//result callback
		if err == nil && option.ResultCallBack != nil {
			err = option.ResultCallBack(ctx, obj, response)
		}

		if err != nil { //err callback, must close body
			response.CloseBody()
			if option.ErrCallBack != nil {
				if err2 := option.ErrCallBack(ctx, obj, response, err); err2 != nil {
					err = tools.WrapError(errFatal, err2)
				}
			}
		} else if !response.IsStream() { //is not is stream must close body
			response.CloseBody()
		}
	}()
	if option.OptionCallBack != nil {
		if err = option.OptionCallBack(ctx, obj, option); err != nil {
			return
		}
	}
	response.bar = option.Bar
	response.disUnzip = option.DisUnZip
	response.disDecode = option.DisDecode
	response.stream = option.Stream

	//init headers and orderheaders,befor init ctxData
	headers, err := option.initHeaders()
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init headers error"), err)
	}
	if headers == nil {
		headers = http.Header{
			"User-Agent":         []string{UserAgent},
			"Accept":             []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
			"Accept-Encoding":    []string{"gzip, deflate, br"},
			"Accept-Language":    []string{AcceptLanguage},
			"Sec-Ch-Ua":          []string{SecChUa},
			"Sec-Ch-Ua-Mobile":   []string{"?0"},
			"Sec-Ch-Ua-Platform": []string{`"Windows"`},
		}
	}
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
	body, err := option.initBody(response.ctx)
	if err != nil {
		return response, tools.WrapError(err, errors.New("tempRequest init body error"), err)
	}
	//create request
	reqs, err := newRequestWithContext(response.ctx, option.Method, href, body)
	if err != nil {
		return response, tools.WrapError(errFatal, errors.New("tempRequest 构造request失败"), err)
	}
	reqs.Header = headers
	//add Referer
	if reqs.Header.Get("Referer") == "" {
		if option.Referer != "" {
			reqs.Header.Set("Referer", option.Referer)
		} else if reqs.URL.Scheme != "" && reqs.URL.Host != "" {
			referBuild := builderPool.Get().(strings.Builder)
			referBuild.WriteString(reqs.URL.Scheme)
			referBuild.WriteString("://")
			referBuild.WriteString(reqs.URL.Host)
			reqs.Header.Set("Referer", referBuild.String())
			referBuild.Reset()
			builderPool.Put(referBuild)
		}
	}

	//set ContentType
	if option.ContentType != "" && reqs.Header.Get("Content-Type") == "" {
		reqs.Header.Set("Content-Type", option.ContentType)
	}

	//init ws
	if ctxData.isWs {
		websocket.SetClientHeadersOption(reqs.Header, option.WsOption)
	}

	if reqs.URL.Scheme == "file" {
		response.filePath = re.Sub(`^/+`, "", reqs.URL.Path)
		response.content, err = os.ReadFile(response.filePath)
		if err != nil {
			err = tools.WrapError(errFatal, errors.New("read filePath data error"), err)
		}
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
	response.response, err = obj.send(option, reqs)
	response.isNewConn = ctxData.isNewConn
	if err != nil {
		err = tools.WrapError(err, "roundTripper error")
		return
	}
	if response.response == nil {
		err = errors.New("response is nil")
		return
	}
	if response.response.Body != nil {
		response.rawConn = response.response.Body.(*readWriteCloser)
	}
	if !response.disUnzip {
		response.disUnzip = response.response.Uncompressed
	}
	if response.response.StatusCode == 101 {
		response.webSocket, err = websocket.NewClientConn(response.rawConn.Conn(), response.response.Header, response.ForceCloseConn)
	} else if response.response.Header.Get("Content-Type") == "text/event-stream" {
		response.sse = newSse(response.response.Body, response.ForceCloseConn)
	} else if !response.disUnzip {
		var unCompressionBody io.ReadCloser
		unCompressionBody, err = tools.CompressionDecode(response.response.Body, response.ContentEncoding())
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
