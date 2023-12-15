package main

import (
	"context"
	"errors"
	"testing"

	"github.com/gospider007/requests"
)

func TestMaxRetries(t *testing.T) {
	n := 0
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		MaxRetries: 3,
		ResultCallBack: func(ctx context.Context, option *requests.RequestOption, response *requests.Response) error {

			if n == 0 {
				n++
				return errors.New("try")
			}
			return nil
		},
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	if n != 1 {
		t.Error("n!=1")
	}
}
