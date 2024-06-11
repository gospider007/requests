package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestHttp2(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{DisAlive: true})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	if resp.Proto() != "HTTP/2.0" {
		t.Error("resp.Proto!= HTTP/2.0")
	}
}
