package main

import (
	"context"
	"testing"

	"github.com/gospider007/requests"
)

func TestChainProxy(t *testing.T) {
	resp, err := requests.Get(context.TODO(), "https://httpbin.org/anything", requests.RequestOption{
		ClientOption: requests.ClientOption{

			Proxy: []string{}, //set proxy,ex:"http://127.0.0.1:8080","https://127.0.0.1:8080","socks5://127.0.0.1:8080"
		},
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("status code is not 200")
	}
}
