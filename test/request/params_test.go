package main

import (
	"testing"

	"github.com/gospider007/gson"
	"github.com/gospider007/requests"
)

func TestSendParamsWithMap(t *testing.T) {
	dataBody := map[string]any{
		"name": "test",
	}
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Params: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	// log.Print(jsonData)
	if jsonData.Get("args.name").String() != "test" {
		t.Fatal("params args error")
	}
}

func TestSendParamsWithStruct(t *testing.T) {
	dataBody := struct{ Name string }{"test"}
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Params: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("args.Name").String() != "test" {
		t.Fatal("json data error")
	}
}
func TestSendParamsWithGson(t *testing.T) {
	dataBody, err := gson.Decode(struct{ Name string }{"test"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Params: dataBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	if jsonData.Get("args.Name").String() != "test" {
		t.Fatal("json data error")
	}
}

func TestSendParamsWithEmptiyMap(t *testing.T) {
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Params: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatal("status code error")
	}
}
