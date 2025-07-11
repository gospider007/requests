package main

import (
	"io"
	"testing"

	"github.com/gospider007/requests"
)

func TestStream(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatal("resp.StatusCode()!= 200")
	}
	body := resp.Body()
	defer body.Close()
	con, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(string(con)) == 0 {
		t.Fatal("con is empty")
	}
}
func TestStreamWithConn(t *testing.T) {
	for i := 0; i < 2; i++ {
		resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{Stream: true})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode() != 200 {
			t.Fatal("resp.StatusCode()!= 200")
		}
		body := resp.Body()
		defer body.Close()
		con, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		if len(string(con)) == 0 {
			t.Fatal("con is empty")
		}
		body.Close()
		if i == 1 && resp.IsNewConn() {
			t.Fatal("con is new")
		}
	}
}
