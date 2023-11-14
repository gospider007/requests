package main

import (
	"testing"

	"github.com/gospider007/gson"
	"github.com/gospider007/requests"
)

func TestSendJsonWithMap(t *testing.T) {
	jsonBody := map[string]any{
		"name": "test",
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Json: jsonBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/json" {
		t.Fatal("json data error")
	}
	bodyJson, err := gson.Decode(jsonBody)
	if err != nil {
		t.Fatal(err)
	}
	if bodyJson.String() != jsonData.Get("data").String() {
		t.Fatal("json data error")
	}
}
func TestSendJsonWithString(t *testing.T) {
	jsonBody := `{"name":"test"}`
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Json: jsonBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/json" {
		t.Fatal("json data error")
	}
	if jsonBody != jsonData.Get("data").String() {
		t.Fatal("json data error")
	}
}
func TestSendJsonWithStruct(t *testing.T) {
	jsonBody := struct{ Name string }{"test"}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Json: jsonBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/json" {
		t.Fatal("json data error")
	}
	bodyJson, err := gson.Decode(jsonBody)
	if err != nil {
		t.Fatal(err)
	}
	if bodyJson.String() != jsonData.Get("data").String() {
		t.Fatal("json data error")
	}
}
func TestSendJsonWithGson(t *testing.T) {
	bodyJson, err := gson.Decode(struct{ Name string }{"test"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Json: bodyJson,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("headers.Content-Type").String() != "application/json" {
		t.Fatal("json data error")
	}
	if bodyJson.String() != jsonData.Get("data").String() {
		t.Fatal("json data error")
	}
}
