package requests

import (
	"net/http"

	"github.com/gospider007/tools"
)

func defaultHeaders() http.Header {
	return http.Header{
		"User-Agent":         []string{tools.UserAgent},
		"Accept":             []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Encoding":    []string{"gzip, deflate, br, zstd"},
		"Accept-Language":    []string{tools.AcceptLanguage},
		"Sec-Ch-Ua":          []string{tools.SecChUa},
		"Sec-Ch-Ua-Mobile":   []string{"?0"},
		"Sec-Ch-Ua-Platform": []string{`"Windows"`},
	}
}

func (obj *RequestOption) initHeaders() (http.Header, error) {
	if obj.Headers == nil {
		return nil, nil
	}
	switch headers := obj.Headers.(type) {
	case http.Header:
		return headers.Clone(), nil
	case *OrderMap:
		head, order := headers.parseHeaders()
		obj.OrderHeaders = order
		return head, nil
	default:
		_, dataMap, _, err := obj.newBody(headers, mapType)
		if err != nil {
			return nil, err
		}
		if dataMap == nil {
			return nil, nil
		}
		head, _ := dataMap.parseHeaders()
		return head, err
	}
}
