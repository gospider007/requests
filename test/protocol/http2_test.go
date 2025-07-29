package main

import (
	"context"
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestHttp2(t *testing.T) {
	session, _ := requests.NewClient(nil)
	resp, err := session.Get(context.TODO(), "https://httpbin.org/anything")
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode2!= 200")
	}
	if resp.Proto() != "HTTP/2.0" {
		t.Error("resp.Proto!= HTTP/2.0")
	}
	for range 3 {
		resp, err = session.Get(context.TODO(), "https://mp.weixin.qq.com")
		if err != nil {
			t.Error(err)
		}
		if resp.StatusCode() != 200 {
			t.Error("resp.StatusCode!= 200")
		}
		if resp.Proto() != "HTTP/2.0" {
			t.Error("resp.Proto!= HTTP/2.0")
		}
	}
	resp, err = session.Post(context.TODO(), "https://mp.weixin.qq.com", requests.RequestOption{
		Body: "fasfasfsdfdssdsfasdfasdfsadfsdf对方是大翻身大翻身大翻身对方的身份",
		ErrCallBack: func(ctx *requests.Response) error {
			log.Print(ctx.Err())
			return nil
		},
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	if resp.Proto() != "HTTP/2.0" {
		t.Error("resp.Proto!= HTTP/2.0")
	}
}
