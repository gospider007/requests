package main

import (
	"net"
	"testing"

	"github.com/gospider007/requests"
)

func TestDns(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Dns: &net.UDPAddr{ //set dns server
			IP:   net.ParseIP("223.5.5.5"),
			Port: 53,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatal("http status code is not 200")
	}
}
