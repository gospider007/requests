package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestDefaultClient(t *testing.T) {
	for i := 0; i < 2; i++ {
		resp, err := requests.Get(nil, "https://myip.top")
		if err != nil {
			t.Error(err)
		} else {
			if i == 0 {
				if !resp.IsNewConn() {
					t.Error("new conn")
				}
			} else {
				if resp.IsNewConn() {
					t.Error("not new conn")
				}
			}
		}
	}
}
