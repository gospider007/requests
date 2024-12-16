package main

import (
	"context"
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestHttp1(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		ForceHttp1: true,
		Logger: func(l requests.Log) {
			log.Print(l)
		},
		ErrCallBack: func(ctx context.Context, option *requests.RequestOption, response *requests.Response, err error) error {
			log.Print(err)
			return nil
		},
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	if resp.Proto() != "HTTP/1.1" {
		t.Error("resp.Proto!= HTTP/1.1")
	}
}
