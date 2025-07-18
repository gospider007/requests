package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestRedirectBreakWitRequest(t *testing.T) {
	href := "https://httpbin.co/absolute-redirect/2"
	resp, err := requests.Get(nil, href, requests.RequestOption{
		ClientOption: requests.ClientOption{
			RequestCallBack: func(ctx *requests.Response) error {
				if ctx.Response() != nil {
					return requests.ErrUseLastResponse
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 302 {
		t.Fatal("resp.StatusCode()!= 302")
	}
}
func TestRedirectWithRequest(t *testing.T) {
	href := "https://httpbin.co/absolute-redirect/2"
	n := 0
	resp, err := requests.Get(nil, href, requests.RequestOption{
		ClientOption: requests.ClientOption{
			RequestCallBack: func(ctx *requests.Response) error {
				if ctx.Response() == nil {
					n++
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("n != 3")
	}
	if resp.StatusCode() != 200 {
		t.Fatal("resp.StatusCode()!= 200")
	}
}
