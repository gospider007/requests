package requests

import (
	"fmt"
	"net/http"

	"github.com/gospider007/tools"
	"gopkg.in/errgo.v2/fmt/errors"
)

func defaultHeaders() http.Header {
	return http.Header{
		"User-Agent":         []string{tools.UserAgent},
		"Connection":         []string{"keep-alive"},
		"Accept":             []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Encoding":    []string{"gzip, deflate, br, zstd"},
		"Accept-Language":    []string{tools.AcceptLanguage},
		"Sec-Ch-Ua":          []string{tools.SecChUa},
		"Sec-Ch-Ua-Mobile":   []string{"?0"},
		"Sec-Ch-Ua-Platform": []string{`"Windows"`},
	}
}

func (obj *RequestOption) initOrderHeaders() (http.Header, error) {
	if obj.Headers == nil {
		return defaultHeaders(), nil
	}
	switch headers := obj.Headers.(type) {
	case http.Header:
		return headers, nil
	case *OrderData:
		obj.orderHeaders = headers
		return make(http.Header), nil
	case map[string]any:
		results := make(http.Header)
		for key, val := range headers {
			results.Add(key, fmt.Sprintf("%v", val))
		}
		return results, nil
	case map[string]string:
		results := make(http.Header)
		for key, val := range headers {
			results.Add(key, val)
		}
		return results, nil
	default:
		return nil, errors.New("headers type error")
	}
}
