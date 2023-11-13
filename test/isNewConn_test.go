package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestDefaultClient(t *testing.T) {
	for i := 0; i < 2; i++ {
		resp, err := requests.Get(nil, "https://httpbin.org/anything")
		if err != nil {
			t.Error(err)
		}
		if i == 0 {
			if !resp.IsNewConn() { //return is NewConn
				t.Error("new conn error")
			}
		} else {
			if resp.IsNewConn() {
				t.Error("new conn error")
			}
		}
	}
}
