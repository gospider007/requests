package requests

import (
	"context"
	"crypto/tls"

	utls "github.com/refraction-networking/utls"
)

// Connection Management
type Client struct {
	ctx          context.Context
	transport    *roundTripper
	cnl          context.CancelFunc
	ClientOption *ClientOption
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
	result.ClientOption = &option
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

// Close the client and cannot be used again after shutdown
func (obj *Client) Close() {
	obj.closed = true
	obj.cnl()
}
