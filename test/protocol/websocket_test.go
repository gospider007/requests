package main

import (
	"testing"

	"github.com/gospider007/requests"
	"github.com/gospider007/websocket"
)

func TestWebSocket(t *testing.T) {
	response, err := requests.Get(nil, "ws://82.157.123.54:9010/ajaxchattest", requests.RequestOption{Headers: map[string]string{
		"Origin": "http://coolaf.com",
	}}) // Send WebSocket request
	if err != nil {
		t.Error(err)
	}
	defer response.CloseBody()
	wsCli := response.WebSocket()
	defer wsCli.Close()
	if err = wsCli.Send(nil, websocket.MessageText, "test"); err != nil { // Send text message
		t.Error(err)
	}
	msgType, con, err := wsCli.Recv(nil) // Receive message
	if err != nil {
		t.Error(err)
	}
	if msgType != websocket.MessageText {
		t.Error("Message type is not text")
	}
	if string(con) != "test" {
		t.Error("Message content is not test")
	}
}
