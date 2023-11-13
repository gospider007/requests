package main

import (
	"context"
	"net/url"
	"testing"

	"github.com/gospider007/requests"
)

func TestProxy(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Proxy: "", //set proxy,ex:"http://127.0.0.1:8080","https://127.0.0.1:8080","socks5://127.0.0.1:8080"
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("status code is not 200")
	}
}
func TestGetProxy(t *testing.T) {
	session, _ := requests.NewClient(nil, requests.ClientOption{
		GetProxy: func(ctx context.Context, url *url.URL) (string, error) { //Penalty when creating a new connection
			proxy := "" //set proxy,ex:"http://127.0.0.1:8080","https://127.0.0.1:8080","socks5://127.0.0.1:8080"
			return proxy, nil
		},
	})
	resp, err := session.Get(nil, "https://httpbin.org/anything")
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("status code is not 200")
	}
}
