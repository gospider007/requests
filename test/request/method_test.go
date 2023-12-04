package main

import (
	"net/http"
	"testing"

	"github.com/gospider007/requests"
)

func TestMethodGet(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything")
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("method").String() != http.MethodGet {
		t.Fatal("method error")
	}
}
func TestMethodPost(t *testing.T) {
	resp, err := requests.Post(nil, "https://httpbin.org/anything")
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("method").String() != http.MethodPost {
		t.Fatal("method error")
	}
}
func TestMethodPost2(t *testing.T) {
	resp, err := requests.Request(nil, "post", "https://httpbin.org/anything")
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("method").String() != http.MethodPost {
		t.Fatal("method error")
	}
}
