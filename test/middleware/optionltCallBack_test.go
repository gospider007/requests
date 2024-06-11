package main

import (
	"context"
	"testing"

	"github.com/gospider007/requests"
)

func TestOptionCallBack(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		OptionCallBack: func(ctx context.Context, option *requests.RequestOption) error {
			option.Params = map[string]string{"name": "test"}
			return nil
		},
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Error(err)
	}
	if jsonData.Get("args.name").String() != "test" {
		t.Error("jsonData.Get(\"args.name\").String()!= test")
	}
}
