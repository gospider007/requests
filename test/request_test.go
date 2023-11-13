package main

import (
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestDefaultClient(t *testing.T) {
	for i := 0; i < 2; i++ {
		resp, err := requests.Get(nil, "https://myip.top")
		if err != nil {
			t.Error(err)
		} else {
			log.Printf("send num: %d, new conn: %v", i, resp.IsNewConn())
		}
	}
}
