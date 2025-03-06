package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/gospider007/requests"
	"github.com/gospider007/websocket"
)

func TestWebSocket(t *testing.T) {
	response, err := requests.Get(nil, "ws://124.222.224.186:8800", requests.RequestOption{}) // Send WebSocket request
	if err != nil {
		log.Panic(err)
	}
	defer response.CloseConn()
	wsCli := response.WebSocket()
	defer wsCli.Close()
	log.Print(response.Headers())
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
			if n > 6 {
				break
			}
		}
		if err = wsCli.WriteMessage(websocket.TextMessage, "test1122332211"); err != nil { // Send text message
			log.Panic(err)
		}
		time.Sleep(time.Second * 2)
	}
}
