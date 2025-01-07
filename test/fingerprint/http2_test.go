package main

import (
	"log"
	"testing"

	"github.com/gospider007/ja3"
	"github.com/gospider007/requests"
)

func TestH2(t *testing.T) {
	j := "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p"
	j2 := "1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p"
	h2Spec, err := ja3.CreateH2SpecWithStr(j) //create h2 spec with string
	if err != nil {
		t.Fatal(err)
	}
	// log.Print(h2Spec)
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		ClientOption: requests.ClientOption{
			H2Ja3Spec: h2Spec, //set h2 spec
		},
	})
	// log.Print(resp.Text())
	if err != nil {
		t.Fatal(err)
	}

	jsonData, err := resp.Json()
	ja3 := jsonData.Get("http2.fingerprint")
	if !ja3.Exists() {
		t.Fatal("not found http2")
	}
	if j2 != ja3.String() {
		log.Print(j)
		log.Print(ja3)
		t.Fatal("not equal")
	}
}
