package main

import (
	"context"
	"errors"
	"testing"

	"github.com/gospider007/requests"
)

func TestErrCallBack(t *testing.T) {
	n := 0
	_, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		MaxRetries: 3,
		ResultCallBack: func(ctx context.Context, option *requests.RequestOption, response *requests.Response) error {
			return errors.New("try")
		},
		ErrCallBack: func(ctx context.Context, option *requests.RequestOption, response *requests.Response, err error) error {
			if n == 0 {
				n++
				return nil
			}
			return errors.New("test")
		},
	})
	if err == nil {
		t.Error("callback error")
	}
	if n != 1 {
		t.Error("n!=1")
	}
}
