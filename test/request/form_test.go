package main

import (
	"strings"
	"testing"

	"github.com/gospider007/gson"
	"github.com/gospider007/requests"
)

func TestSendFormWithMap(t *testing.T) {
	dataBody := map[string]any{
		"name": "test",
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Form: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(jsonData.Get("headers.Content-Type").String(), "multipart/form-data") {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendFormWithString(t *testing.T) {
	dataBody := `{"name":"test"}`
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Form: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(jsonData.Get("headers.Content-Type").String(), "multipart/form-data") {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendFormWithStruct(t *testing.T) {
	dataBody := struct{ Name string }{"test"}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Form: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(jsonData.Get("headers.Content-Type").String(), "multipart/form-data") {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.Name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendFormWithGson(t *testing.T) {
	dataBody, err := gson.Decode(struct{ Name string }{"test"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Form: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(jsonData.Get("headers.Content-Type").String(), "multipart/form-data") {
		t.Fatal("json data error")
	}
	if jsonData.Get("form.Name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendFormWithOrderMap(t *testing.T) {
	orderMap := requests.NewOrderMap()
	orderMap.Set("name", "test")
	orderMap.Set("age", 11)
	orderMap.Set("sex", "boy")

	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Form: orderMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(jsonData.Get("headers.Content-Type").String(), "multipart/form-data") {
		t.Fatal("json data error")
	}
	// log.Print(jsonData)
	if jsonData.Get("form.name").String() != "test" {
		t.Fatal("json data error")
	}
}
