package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gospider007/requests"
)

func TestRequestCallBack(t *testing.T) {
	_, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		RequestCallBack: func(ctx context.Context, request *http.Request, response *http.Response) error {
			if response != nil {
				if response.ContentLength > 100 {
					return errors.New("max length")
				}
			}
			return nil
		},
	})
	if !strings.Contains(err.Error(), "max length") {
		t.Error("err is not max length")
	}
}
