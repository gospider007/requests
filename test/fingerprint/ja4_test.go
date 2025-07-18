package main

import (
	"net/textproto"
	"slices"
	"testing"

	"github.com/gospider007/requests"
)

func TestOrderHeaders(t *testing.T) {
	orderKeys := []string{
		"Accept-Encoding",
		"Accept",
		"Sec-Ch-Ua-Mobile",
		"Sec-Ch-Ua-Platform",
	}

	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		ClientOption: requests.ClientOption{
			OrderHeaders: orderKeys,
			// Headers: headers,
		},
		// ForceHttp1: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	if err != nil {
		t.Fatal(err)
	}
	header_order := jsonData.Find("ordered_headers_key")
	if !header_order.Exists() {
		t.Fatal("not found akamai")
	}
	i := -1
	for _, key := range header_order.Array() {
		// log.Print(key)
		kk := textproto.CanonicalMIMEHeaderKey(key.String())
		if slices.Contains(orderKeys, kk) {
			i2 := slices.Index(orderKeys, textproto.CanonicalMIMEHeaderKey(kk))
			if i2 < i {
				// log.Print(header_order)
				t.Fatal("not equal")
			}
			i = i2
		}
	}
}
