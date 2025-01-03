package main

import (
	"context"
	"log"
	"testing"

	"github.com/gospider007/proxy"
	"github.com/gospider007/requests"
)

func TestProxy2(t *testing.T) {
	proCliPre, err := proxy.NewClient(nil, proxy.ClientOption{
		DisVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCliPre.Close()
	go proCliPre.Run()
	proIp := "http://" + proCliPre.Addr()

	proCli, err := proxy.NewClient(nil, proxy.ClientOption{
		DisVerify: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer proCli.Close()
	go proCli.Run()
	proIp2 := "http://" + proCli.Addr()
	reqCli, err := requests.NewClient(nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reqCli.Request(nil, "get", "https://httpbin.org/ip", requests.RequestOption{Proxys: []string{
		proIp,
		proIp2,
	}})
	if err != nil {
		t.Fatal(err)
	}
	log.Print(resp.Text())
}

func TestChainProxy(t *testing.T) {
	resp, err := requests.Get(context.TODO(), "https://httpbin.org/anything", requests.RequestOption{
		Proxys: []string{}, //set proxy,ex:"http://127.0.0.1:8080","https://127.0.0.1:8080","socks5://127.0.0.1:8080"
	})
	if err != nil {
		t.Error(err)
	}
	if resp.StatusCode() != 200 {
		t.Error("status code is not 200")
	}
}
