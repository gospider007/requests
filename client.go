package requests

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"

	utls "github.com/refraction-networking/utls"
)

// Connection Management
type Client struct {
	ctx          context.Context
	transport    *roundTripper
	cnl          context.CancelFunc
	ClientOption ClientOption
	closed       bool
}

var defaultClient, _ = NewClient(context.TODO())

// New Connection Management
func NewClient(preCtx context.Context, options ...ClientOption) (*Client, error) {
	if preCtx == nil {
		preCtx = context.TODO()
	}
	var option ClientOption
	if len(options) > 0 {
		option = options[0]
	}
	result := new(Client)
	result.ctx, result.cnl = context.WithCancel(preCtx)
	result.transport = newRoundTripper(result.ctx)
	result.ClientOption = option
	if result.ClientOption.TlsConfig == nil {
		result.ClientOption.TlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(0),
		}
	}
	if result.ClientOption.UtlsConfig == nil {
		result.ClientOption.UtlsConfig = &utls.Config{
			InsecureSkipVerify:                 true,
			ClientSessionCache:                 utls.NewLRUClientSessionCache(0),
			InsecureSkipTimeVerify:             true,
			OmitEmptyPsk:                       true,
			PreferSkipResumptionOnNilExtension: true,
		}
	}
	//cookiesjar
	if !result.ClientOption.DisCookie {
		if result.ClientOption.Jar == nil {
			result.ClientOption.Jar = NewJar()
		}
	}
	var err error
	if result.ClientOption.Proxy != nil {
		_, err = parseProxy(result.ClientOption.Proxy)
	}
	return result, err
}

func (obj *Client) CloseConns() {
	obj.transport.closeConns()
}

// Close the client and cannot be used again after shutdown
func (obj *Client) Close() {
	obj.closed = true
	obj.CloseConns()
	obj.cnl()
}
func (obj *Client) do(ctx *Response) (err error) {
	var redirectNum int
	for {
		redirectNum++
		err = obj.send(ctx)
		if ctx.Request().Body != nil {
			ctx.Request().Body.Close()
		}
		if err != nil {
			return
		}
		if ctx.Option().MaxRedirect < 0 { //dis redirect
			return
		}
		if ctx.Option().MaxRedirect > 0 && redirectNum > ctx.Option().MaxRedirect {
			return
		}
		loc := ctx.response.Header.Get("Location")
		if loc == "" {
			return nil
		}
		u, err := ctx.Request().URL.Parse(loc)
		if err != nil {
			return fmt.Errorf("failed to parse Location header %q: %v", loc, err)
		}
		ctx.request, err = NewRequestWithContext(ctx.Context(), http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		var shouldRedirect bool
		ctx.request.Method, shouldRedirect, _ = redirectBehavior(ctx.Request().Method, ctx.response, ctx.request)
		if !shouldRedirect {
			return nil
		}
		ctx.request.Response = ctx.response
		ctx.request.Header = make(http.Header)
		ctx.request.Header.Set("Referer", ctx.Request().URL.String())
		for key := range ctx.request.Header {
			if val := ctx.Request().Header.Get(key); val != "" {
				ctx.request.Header.Set(key, val)
			}
		}
		if getDomain(u) == getDomain(ctx.Request().URL) {
			if Authorization := ctx.Request().Header.Get("Authorization"); Authorization != "" {
				ctx.request.Header.Set("Authorization", Authorization)
			}
			cookies := Cookies(ctx.Request().Cookies()).String()
			if cookies != "" {
				ctx.request.Header.Set("Cookie", cookies)
			}
			addCookie(ctx.request, ctx.response.Cookies())
		}
		io.Copy(io.Discard, ctx.response.Body)
		ctx.response.Body.Close()
	}
}
func (obj *Client) send(ctx *Response) (err error) {
	if ctx.Option().Jar != nil {
		addCookie(ctx.Request(), ctx.Option().Jar.GetCookies(ctx.Request().URL))
	}
	err = obj.transport.RoundTrip(ctx)
	if ctx.Option().Jar != nil && ctx.response != nil {
		if rc := ctx.response.Cookies(); len(rc) > 0 {
			ctx.Option().Jar.SetCookies(ctx.Request().URL, rc)
		}
	}
	return err
}
