package requests

import (
	"fmt"
	"net/http"

	"gopkg.in/errgo.v2/fmt/errors"
)

func (obj *RequestOption) initOrderHeaders() (http.Header, error) {
	if obj.Headers == nil {
		return make(http.Header), nil
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
