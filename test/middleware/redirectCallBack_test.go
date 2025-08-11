package main

import (
	"context"
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestRedirectCallBack(t *testing.T) {
	response, err := requests.Get(context.TODO(), "http://www.baidu.com", requests.RequestOption{
		RequestCallBack: func(ctx *requests.Response) error {
			if ctx.Response() != nil {
				return requests.ErrUseLastResponse
			}
			return nil
		},
	})
	if err != nil {
		t.Error(err)
	}
	if response.StatusCode() != 302 {
		log.Print(response.StatusCode())
		t.Error("redirect failed")
	}
}
