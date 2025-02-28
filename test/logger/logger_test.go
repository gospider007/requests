package main

import (
	"log"
	"testing"

	"github.com/gospider007/requests"
)

func TestLogger(t *testing.T) {
	for i := 0; i < 2; i++ {
		response, err := requests.Get(nil, "https://www.httpbin.org", requests.RequestOption{
			ClientOption: requests.ClientOption{
				Logger: func(l requests.Log) {
					log.Print(l)
				},
			},
		})
		if err != nil {
			log.Panic(err)
		}
		log.Print(len(response.Text()))
		log.Print(response.Status())
		log.Print(response.Proto())
	}
}
