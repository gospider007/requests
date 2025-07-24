package main

import (
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gospider007/requests"
)

var wsOk bool

func websocketServer() {
	if wsOk {
		return
	}
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许跨域
		},
	}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(messageType, []byte("服务端回复："+string(message)))
		}
	})
	log.Println("WebSocket 服务器启动于 ws://localhost:8080/ws")
	wsOk = true
	log.Fatal(http.ListenAndServe(":8800", nil))
}
func TestWebSocket(t *testing.T) {
	go websocketServer()
	time.Sleep(time.Second * 1)                                                                          // Send WebSocket request
	response, err := requests.Get(nil, "ws://localhost:8800/ws", requests.RequestOption{DisProxy: true}) // Send WebSocket request
	if err != nil {
		log.Panic(err)
	}
	defer response.CloseConn()
	wsCli := response.WebSocket()
	defer wsCli.Close()
	log.Print(wsCli)
	log.Print(response.Headers())
	log.Print(response.StatusCode())
	if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err != nil { // Send text message
		log.Panic(err)
	}
	n := 0
	for {
		msgType, con, err := wsCli.ReadMessage() // Receive message
		if err != nil {
			log.Panic(err)
		}
		if msgType != websocket.TextMessage {
			log.Panic("Message type is not text")
		}
		log.Print(string(con))
		if strings.Contains(string(con), "test1122332211") {
			n++
			if n > 2 {
				break
			}
		}
		if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err != nil { // Send text message
			log.Panic(err)
		}
	}
}
func TestWebSocketClose(t *testing.T) {
	go websocketServer()
	time.Sleep(time.Second * 1)                                                                                        // Send WebSocket request
	response, err := requests.Get(nil, "ws://localhost:8800/ws", requests.RequestOption{DisProxy: true, Stream: true}) // Send WebSocket request
	if err != nil {
		log.Panic(err)
	}
	defer response.CloseConn()
	response.CloseBody(nil)
	wsCli := response.WebSocket()
	if wsCli == nil {
		t.Fatal("WebSocket client is nil")
	}
	defer wsCli.Close()
	log.Print(wsCli)
	log.Print(response.Headers())
	log.Print(response.StatusCode())
	if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err == nil { // Send text message
		t.Fatal("这里必须报错")
	}
}
func TestWebSocketClose2(t *testing.T) {
	go websocketServer()
	time.Sleep(time.Second * 1)                                                                                        // Send WebSocket request
	response, err := requests.Get(nil, "ws://localhost:8800/ws", requests.RequestOption{DisProxy: true, Stream: true}) // Send WebSocket request
	if err != nil {
		log.Panic(err)
	}
	defer response.CloseConn()
	body := response.Body()
	io.ReadAll(body)
	response.CloseBody(nil)
	wsCli := response.WebSocket()
	if wsCli == nil {
		t.Fatal("WebSocket client is nil")
	}
	defer wsCli.Close()
	log.Print(wsCli)
	log.Print(response.Headers())
	log.Print(response.StatusCode())
	if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err == nil { // Send text message
		t.Fatal("这里必须报错")
	}
}
func TestWebSocketClose3(t *testing.T) {
	go websocketServer()
	time.Sleep(time.Second * 1)                                                                                        // Send WebSocket request
	response, err := requests.Get(nil, "ws://localhost:8800/ws", requests.RequestOption{DisProxy: true, Stream: true}) // Send WebSocket request
	if err != nil {
		log.Panic(err)
	}
	defer response.CloseConn()
	body := response.Body()
	io.ReadAll(body)
	// body.Close()
	wsCli := response.WebSocket()
	response.CloseBody(nil)
	if wsCli == nil {
		t.Fatal("WebSocket client is nil")
	}
	defer wsCli.Close()
	log.Print(wsCli)
	log.Print(response.Headers())
	log.Print(response.StatusCode())
	if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err != nil { // Send text message
		log.Panic(err)
	}
	n := 0
	for {
		msgType, con, err := wsCli.ReadMessage() // Receive message
		if err != nil {
			log.Panic(err)
		}
		if msgType != websocket.TextMessage {
			log.Panic("Message type is not text")
		}
		log.Print(string(con))
		if strings.Contains(string(con), "test1122332211") {
			n++
			if n > 2 {
				break
			}
		}
		if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err != nil { // Send text message
			log.Panic(err)
		}
	}
}
