package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestDisProxy(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Proxy:    "http://192.368.7.256:9887",
		DisProxy: true,
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("status code is not 200")
	}
}
