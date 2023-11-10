package main

import (
	"net"
	"testing"

	"github.com/gospider007/requests"
)

func TestMain(t *testing.T) {
	resp, err := requests.Get(nil, "https://myip.top", requests.RequestOption{
		Dns: &net.UDPAddr{
			// IP: net.ParseIP("223.6.6.6"),
			IP:   net.ParseIP("223.5.5.5"),
			Port: 53,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(resp.Text())
}
