package main

import (
	"testing"

	"github.com/gospider007/gson"
	"github.com/gospider007/requests"
)

func TestSse(t *testing.T) {
	response, err := requests.Get(nil, "https://sse.dev/test") // Send WebSocket request
	if err != nil {
		t.Error(err)
	}
	defer response.CloseBody()
	sseCli := response.Sse()
	if sseCli == nil {
		t.Error("not is sseCli")
	}
	for maxNum := 0; maxNum < 3; maxNum++ {
		data, err := sseCli.Recv()
		if err != nil {
			t.Error(err)
		}
		jsonData, err := gson.Decode(data.Data)
		if err != nil {
			t.Error(err)
		}
		if !jsonData.Get("testing").Bool() {
			t.Error("testing")
		}
	}
}
