package main

import (
	"context"
	"log"
	"net/http"
	"testing"

	"github.com/gospider007/requests"
)

func TestSetCookies(t *testing.T) {
	session, _ := requests.NewClient(context.TODO())

	_, err := session.Get(context.TODO(), "https://www.baidu.com")
	if err != nil {
		log.Panic(err)
	}
	_, err = session.Get(context.TODO(), "https://www.baidu.com", requests.RequestOption{
		RequestCallBack: func(ctx context.Context, request *http.Request, response *http.Response) error {
			if request.Cookies() == nil {
				log.Panic("cookie is nil")
			}
			return nil
		},
	})
	if err != nil {
		log.Panic(err)
	}
}
