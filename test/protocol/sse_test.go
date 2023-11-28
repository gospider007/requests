package main

import (
	"io"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gospider007/requests"
)

func TestSse(t *testing.T) {
	// Start the server
	go func() {
		err := http.ListenAndServe(":3333", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			// SSE event format
			event := "message"
			data := "testing"
			// Start SSE loop
			for i := 0; i < 3; i++ {
				// Send SSE event
				_, err := w.Write([]byte("event: " + event + "\n"))
				if err != nil {
					log.Println("Error writing SSE event:", err)
					return
				}
				_, err = w.Write([]byte("data: " + data + "\n\n"))
				if err != nil {
					log.Println("Error writing SSE data:", err)
					return
				}
				// Flush the response writer
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				// Delay before sending the next event
				time.Sleep(1 * time.Second)
			}
		}))
		if err != nil {
			t.Error(err)
		}
	}()
	response, err := requests.Get(nil, "http://127.0.0.1:3333/events") // Send WebSocket request
	if err != nil {
		t.Error(err)
	}
	defer response.CloseBody()
	sseCli := response.Sse()
	if sseCli == nil {
		t.Error("not is sseCli")
	}
	for {
		data, err := sseCli.Recv()
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}
		if data.Data != "testing" {
			t.Error("testing")
		}
	}
}
