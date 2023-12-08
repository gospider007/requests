package requests

import (
	"errors"
	"net/http"
)

const (
	chromeV        = "120"
	chromeE        = ".0.6099.28"
	UserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + chromeV + ".0.0.0 Safari/537.36 Edg/" + chromeV + chromeE
	SecChUa        = `"Chromium";v="` + chromeV + `", "Microsoft Edge";v="` + chromeV + `", "Not=A?Brand";v="99"`
	AcceptLanguage = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
)

func (obj *RequestOption) initHeaders() (http.Header, error) {
	if obj.Headers == nil {
		return nil, nil
	}
	switch headers := obj.Headers.(type) {
	case http.Header:
		return headers.Clone(), nil
	default:
		body, dataMap, _, err := obj.newBody(headers, mapType)
		if err != nil {
			return nil, err
		}
		if body != nil {
			return nil, errors.New("headers type error")
		}
		if dataMap == nil {
			return nil, nil
		}
		head, order := dataMap.parseHeaders()
		obj.OrderHeaders = order
		return head, err
	}
}
