package main

import (
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestSession(t *testing.T) {
	session, _ := requests.NewClient(nil)
	for i := 0; i < 2; i++ {
		resp, err := session.Get(nil, "https://www.baidu.com")
		if err != nil {
			t.Error(err)
		}
		log.Print(resp.Proto(), resp.IsNewConn())
		if i == 0 {
			if !resp.IsNewConn() { //return is NewConn
				t.Error("new conn error: ", i)
			}
		} else {
			if resp.IsNewConn() {
				t.Error("new conn error: ", i)
			}
		}
	}
}

func TestSession2(t *testing.T) {
	session, _ := requests.NewClient(nil)
	for i := 0; i < 2; i++ {
		resp, err := session.Get(nil, "https://httpbin.org/anything")
		if err != nil {
			t.Error(err)
		}
		log.Print(resp.Proto(), resp.IsNewConn())
		if i == 0 {
			if !resp.IsNewConn() { //return is NewConn
				t.Error("new conn error: ", i)
			}
		} else {
			if resp.IsNewConn() {
				t.Error("new conn error: ", i)
			}
		}
	}
}
func TestSession3(t *testing.T) {
	session, _ := requests.NewClient(nil)
	for i := 0; i < 2; i++ {
		resp, err := session.Get(nil, "https://cloudflare-quic.com/", requests.RequestOption{ClientOption: requests.ClientOption{H3: true}})
		if err != nil {
			t.Error(err)
		}
		log.Print(resp.Proto(), resp.IsNewConn())
		if i == 0 {
			if !resp.IsNewConn() { //return is NewConn
				t.Error("new conn error: ", i)
			}
		} else {
			if resp.IsNewConn() {
				t.Error("new conn error: ", i)
			}
		}
	}
}
