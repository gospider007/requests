package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/gospider007/requests"
)

func TestRedirectCallBack(t *testing.T) {
	response, err := requests.Get(context.TODO(), "http://www.baidu.com", requests.RequestOption{
		ClientOption: requests.ClientOption{

			RequestCallBack: func(ctx context.Context, request *http.Request, response *http.Response) error {
				if response != nil {
					return requests.ErrUseLastResponse
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Error(err)
	}
	if response.StatusCode() != 302 {
		t.Error("redirect failed")
	}
}
