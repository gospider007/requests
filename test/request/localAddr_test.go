package main

import (
	"net"
	"testing"

	"github.com/gospider007/requests"
)

func TestLocalAddr(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{

		DialOption: &requests.DialOption{
			LocalAddr: &net.TCPAddr{ //set dns server
				// IP: net.ParseIP("192.168.1.239"),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatal("http status code is not 200")
	}
}
