package main

import (
	"context"
	"errors"
	"testing"

	"github.com/gospider007/requests"
)

func TestResultCallBack(t *testing.T) {
	var code int
	_, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		ResultCallBack: func(ctx context.Context, option *requests.RequestOption, response *requests.Response) error {
			if response.StatusCode() != 200 {
				return errors.New("resp.StatusCode!= 200")
			}
			code = response.StatusCode()
			return nil
		},
	})
	if err != nil {
		t.Error(err)
	}
	if code != 200 {
		t.Error("code!= 200")
	}
}
