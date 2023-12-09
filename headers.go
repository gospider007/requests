package requests

import (
	"net/http"
)

const (
	chromeV        = "120"
	chromeE        = ".0.6099.28"
	UserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeV + ".0.0.0 Safari/537.36 Edg/" + chromeV + chromeE
	SecChUa        = `"Chromium";v="` + chromeV + `", "Microsoft Edge";v="` + chromeV + `", "Not=A?Brand";v="99"`
	AcceptLanguage = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
)

func defaultHeaders() http.Header {
	return http.Header{
		"User-Agent":         []string{UserAgent},
		"Accept":             []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Encoding":    []string{"gzip, deflate, br"},
		"Accept-Language":    []string{AcceptLanguage},
		"Sec-Ch-Ua":          []string{SecChUa},
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
	default:
		_, dataMap, _, err := obj.newBody(headers, mapType)
		if err != nil {
			return nil, err
		}
		if dataMap == nil {
			return nil, nil
		}
		head, order := dataMap.parseHeaders()
		obj.OrderHeaders = order
		return head, err
	}
}
