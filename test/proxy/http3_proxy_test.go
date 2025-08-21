package main

import (
	"context"
	"crypto/tls"
	"log"
	"testing"

	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gospider007/gtls"
	"github.com/gospider007/requests"
	"github.com/quic-go/quic-go/http3"
	"github.com/txthinking/socks5"
)

var (
	proxyHost  = "127.0.0.1:1080"
	proxyHost2 = "127.0.0.1:10801"
	remoteHost = "127.0.0.1:8080"
)

func client() {
	for range 5 {
		resp, err := requests.Post(nil, "https://"+remoteHost, requests.RequestOption{
			ForceHttp3: true,
			Logger: func(l requests.Log) {
				log.Print(l)
			},
			Proxy: "socks5://" + proxyHost,
			Body:  []byte("hello, server!"),
		})
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println(resp.StatusCode())
		fmt.Println(resp.Text())
		time.Sleep(time.Second)
	}
}
func client2() {
	for range 5 {
		resp, err := requests.Post(nil, "https://"+remoteHost, requests.RequestOption{

			ForceHttp3: true,
			Logger: func(l requests.Log) {
				log.Print(l)
			},
			Proxy: []string{
				"http://" + proxyHost,
				"socks5://" + proxyHost,
			},
			Body: []byte("hello, server!"),
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(resp.StatusCode())
		fmt.Println(resp.Text())
		time.Sleep(time.Second)
	}
}

func server() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, "hello, world!")
		case http.MethodPost:
			result, err := io.ReadAll(r.Body)
			if err != nil {
				fmt.Fprint(w, "error:", err.Error())
				return
			}
			fmt.Fprintf(w, "echo:'%s'", string(result))
		default:
			fmt.Fprint(w, "method is not supported")
		}
	})
	server := http3.Server{
		Addr:    "0.0.0.0:8080",
		Handler: mux,
		TLSConfig: &tls.Config{
			GetCertificate: gtls.GetCertificate,
			NextProtos:     []string{"http3-echo-example"},
		},
	}
	fmt.Println("Server is listening...")
	fmt.Println(server.ListenAndServe())
}
func proxyServer(addr string) {
	server, err := socks5.NewClassicServer(addr, "127.0.0.1", "", "", 0, 0)
	if err != nil {
		log.Println(err)
		return
	}
	go server.ListenAndServe(nil)
}

func TestHttp3Proxy(t *testing.T) {
	go server()
	go proxyServer(proxyHost)
	go proxyServer(proxyHost2)
	time.Sleep(time.Second * 3)
	// client()
	client2()
}
func TestHttp3Proxy2(t *testing.T) {
	go proxyServer(proxyHost)
	for range 5 {
		resp, err := requests.Get(context.TODO(), "https://quic.nginx.org/", requests.RequestOption{
			USpec:      true,
			ForceHttp3: true,
			// Logger: func(l requests.Log) {
			// 	log.Print(l)
			// },
			Proxy: []string{
				// "http://" + proxyHost,
				"socks5://" + proxyHost,
			},
			Body: []byte("hello, server!"),
		})
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(resp.StatusCode())
		fmt.Println(resp.Proto())
		resp.CloseConn()
		time.Sleep(time.Second)
	}
}
