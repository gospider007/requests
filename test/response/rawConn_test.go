package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestRawConn(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything")
	if err != nil {
		t.Error(err)
	}
	if resp.Conn() != nil {
		t.Error("conn is not nil")
	}
	resp, err = requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{Stream: true})
	if err != nil {
		t.Error(err)
	}
	if resp.Conn() == nil {
		t.Error("conn is nil")
	}
}
