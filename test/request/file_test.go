package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gospider007/requests"
)

func TestSendFileWithReader(t *testing.T) {
	resp, err := requests.Post(nil, "https://httpbin.org/anything", requests.RequestOption{
		Form: map[string]any{
			"file": requests.File{
				Content:     bytes.NewBuffer([]byte("test")), //support: io.Reader, string, []byte
				FileName:    "test.txt",
				ContentType: "text/plain",
			},
		},
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
