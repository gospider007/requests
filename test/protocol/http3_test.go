package main

import (
	"context"
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestHttp3(t *testing.T) {
	resp, err := requests.Get(context.TODO(), "https://cloudflare-quic.com/", requests.RequestOption{
		ClientOption: requests.ClientOption{

			H3: true,
		},
	},
	)
	if err != nil {
		t.Error(err)
	}
	log.Print(resp.Proto())
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	if resp.Proto() != "HTTP/3.0" {
		t.Error("resp.Proto!= HTTP/3.0")
	}
}

// [0 87 164 1 116 253 33 138 106 82]
// [0 87 162 31 139 8 0 0 0 0]
