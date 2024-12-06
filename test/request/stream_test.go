package main

import (
	"log"
	"testing"
	"time"

	"github.com/gospider007/requests"
)

func TestStream(t *testing.T) {
	resp, err := requests.Get(nil, "https://httpbin.org/anything", requests.RequestOption{
		Stream: true,
		Logger: func(l requests.Log) {
			log.Print(l)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsStream() {
		// con, err := io.ReadAll(resp.Body())
		// if err != nil {
		// 	t.Fatal(err)
		// }
		// resp.ReadBody()
		// bBody := bytes.NewBuffer(nil)
		// io.Copy(bBody, resp.Body())

		// t.Log(string(con))
		// t.Log(resp.Text())
		time.Sleep(2 * time.Second)
		resp.CloseBody()
		time.Sleep(2 * time.Second)
		if resp.StatusCode() != 200 {
			t.Fatal("resp.StatusCode()!= 200")
		}
	} else {
		t.Fatal("resp.IsStream() is false")
	}
}
