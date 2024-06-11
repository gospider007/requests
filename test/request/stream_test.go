package main

import (
	"testing"

	"github.com/gospider007/requests"
)

func TestStream(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsStream() {
		if resp.StatusCode() != 200 {
			t.Fatal("resp.StatusCode()!= 200")
		}
		resp.CloseBody()
		resp.CloseBody()
	} else {
		t.Fatal("resp.IsStream() is false")
	}
}
