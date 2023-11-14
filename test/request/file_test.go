package main

import (
	"strings"
	"testing"

	"github.com/gospider007/requests"
)

func TestSendFile(t *testing.T) {
	fileData := []requests.File{
		{
			Key:         "file",
			Val:         []byte("test"),
			FileName:    "test.txt",
			ContentType: "text/plain",
		},
	}
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Files: fileData,
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
	if jsonData.Get("files.file").String() != "test" {
		t.Fatal("json data error")
	}
}
