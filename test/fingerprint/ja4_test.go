package main

import (
	"log"
	"net/textproto"
	"slices"
	"testing"

	"github.com/gospider007/requests"
	"github.com/gospider007/tools"
)

func TestOrderHeaders(t *testing.T) {

	headers := requests.NewOrderData()
	headers.Add("Accept-Encoding", "gzip, deflate, br")
	headers.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	headers.Add("User-Agent", tools.UserAgent)
	headers.Add("Accept-Language", tools.AcceptLanguage)
	headers.Add("Sec-Ch-Ua", tools.SecChUa)
	headers.Add("Sec-Ch-Ua-Mobile", "?0")
	headers.Add("Sec-Ch-Ua-Platform", `"Windows"`)
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		ClientOption: requests.ClientOption{
			Headers: headers,
		},
		// ForceHttp1: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	header_order := jsonData.Find("ordered_headers_key")
	if !header_order.Exists() {
		t.Fatal("not found akamai")
	}
	i := -1
	log.Print(header_order)
	// log.Print(headers.Keys())
	kks := []string{}
	for _, kk := range headers.Keys() {
		kks = append(kks, textproto.CanonicalMIMEHeaderKey(kk))
	}
	for _, key := range header_order.Array() {
		kk := textproto.CanonicalMIMEHeaderKey(key.String())
		if slices.Contains(kks, kk) {
			i2 := slices.Index(kks, textproto.CanonicalMIMEHeaderKey(kk))
			if i2 < i {
				log.Print(header_order)
				t.Fatal("not equal")
			}
			i = i2
		}
	}
}
func TestOrderHeaders2(t *testing.T) {

	headers := map[string]any{
		"Accept-Encoding":    "gzip, deflate, br",
		"Accept":             "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"User-Agent":         tools.UserAgent,
		"Accept-Language":    tools.AcceptLanguage,
		"Sec-Ch-Ua":          tools.SecChUa,
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": `"Windows"`,
	}
	orderHeaders := []string{
		"Accept-Encoding",
		"Accept",
		"User-Agent",
		"Accept-Language",
		"Sec-Ch-Ua",
		"Sec-Ch-Ua-Mobile",
		"Sec-Ch-Ua-Platform",
	}
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		ClientOption: requests.ClientOption{

			Headers: headers,
		},
		// ForceHttp1: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	header_order := jsonData.Find("ordered_headers_key")
	if !header_order.Exists() {
		t.Fatal("not found akamai")
	}
	i := -1
	log.Print(header_order)
	// log.Print(headers.Keys())
	kks := []string{}
	for _, kk := range orderHeaders {
		kks = append(kks, textproto.CanonicalMIMEHeaderKey(kk))
	}
	for _, key := range header_order.Array() {
		kk := textproto.CanonicalMIMEHeaderKey(key.String())
		if slices.Contains(kks, kk) {
			i2 := slices.Index(kks, textproto.CanonicalMIMEHeaderKey(kk))
			if i2 < i {
				log.Print(header_order)
				t.Fatal("not equal")
			}
			i = i2
		}
	}
}
