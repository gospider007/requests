package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestUseProxy(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything")
	if err != nil {
		t.Error(err)
	}
	if resp.Proxy() != "" {
		t.Error("proxy error")
	}
}
