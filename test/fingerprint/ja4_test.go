package main

import (
	"log"
	"net/textproto"
	"slices"
	"testing"

	"github.com/gospider007/requests"
)

func TestOrderHeaders(t *testing.T) {
	orderHeaders := []string{
		"Accept-Encoding",
		"Accept",
		"Sec-Ch-Ua",
		"Sec-Ch-Ua-Platform",
		"Sec-Ch-Ua-Mobile",
		"Accept-Language",
		"User-Agent",
	}
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		OrderHeaders: orderHeaders, //set http1.1 order headers
		ForceHttp1:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	header_order := jsonData.Get("http.header_order")
	if header_order == nil {
		t.Fatal("not found akamai")
	}
	i := -1
	for _, key := range header_order.Array() {
		i2 := slices.Index(orderHeaders, textproto.CanonicalMIMEHeaderKey(key.String()))
		if i2 < i {
			log.Print(header_order)
			t.Fatal("not equal")
		}
		i = i2
	}
}
