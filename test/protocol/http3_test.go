package main

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/gospider007/gtls"
	"github.com/gospider007/proxy"
	"github.com/gospider007/requests"
)

func TestHttp3(t *testing.T) {
	resp, err := requests.Get(context.TODO(), "https://cloudflare-quic.com/", requests.RequestOption{
		ClientOption: requests.ClientOption{

			ForceHttp3: true,
		},
	},
	)
	if err != nil {
		t.Error(err)
	}
	log.Print(resp.Proto())
	if resp.StatusCode() != 200 {
		t.Error("resp.StatusCode!= 200")
	}
	if resp.Proto() != "HTTP/3.0" {
		t.Error("resp.Proto!= HTTP/3.0")
	}
}
func TestHttp3Proxy(t *testing.T) {
	proxyAddress := "127.0.0.1:1080"
	// server, err := socks5.NewClassicServer("127.0.0.1:1080", "127.0.0.1", "", "", 0, 0)
	// if err != nil {
	// 	log.Println(err)
	// 	return
	// }
	// go server.ListenAndServe(nil)
	proxyC, err := proxy.NewClient(nil, proxy.ClientOption{Addr: proxyAddress, DisVerify: true})
	if err != nil {
		log.Println(err)
		return
	}
	defer proxyC.Close()
	go proxyC.Run()

	time.Sleep(time.Second)
	// href := "https://google.com"
	href := "https://cloudflare-quic.com/"

	resp, err := requests.Get(context.Background(), href,
		requests.RequestOption{
			ClientOption: requests.ClientOption{
				DialOption: requests.DialOption{
					// AddrType: gtls.Ipv4,
					GetAddrType: func(host string) gtls.AddrType {
						log.Print("我开始喽")
						return gtls.Ipv4
					},
				},
				Proxy:      "socks5://" + proxyAddress,
				ForceHttp3: true,
			},
		})
	if err != nil {
		log.Panic(err)
		return
	}
	log.Print(resp.StatusCode())
	log.Print(resp.Proto())
}
