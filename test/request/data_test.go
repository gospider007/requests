package main

import (
	"testing"

	"github.com/gospider007/gson"
	"github.com/gospider007/requests"
)

func TestSendDataWithMap(t *testing.T) {
	dataBody := map[string]any{
		"name": "test",
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Data: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/x-www-form-urlencoded" {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendDataWithString(t *testing.T) {
	dataBody := `{"name":"test"}`
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Data: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/x-www-form-urlencoded" {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendDataWithStruct(t *testing.T) {
	dataBody := struct{ Name string }{"test"}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Data: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/x-www-form-urlencoded" {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.Name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendDataWithGson(t *testing.T) {
	dataBody, err := gson.Decode(struct{ Name string }{"test"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Data: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/x-www-form-urlencoded" {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.Name").String() != "test" {
		t.Fatal("json data error")
	}
}
