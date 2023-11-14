package main

import (
	"log"
	"testing"

	"github.com/gospider007/ja3"
	"github.com/gospider007/requests"
)

func TestH2(t *testing.T) {
	j := "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p"
	h2Spec, err := ja3.CreateH2SpecWithStr(j) //create h2 spec with string
	if err != nil {
		t.Fatal(err)
	}
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		H2Ja3Spec: h2Spec, //set h2 spec
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()
	ja3 := jsonData.Get("akamai.fingerprint")
	if ja3 == nil {
		t.Fatal("not found akamai")
	}
	if j != ja3.String() {
		log.Print(j)
		log.Print(ja3)
		t.Fatal("not equal")
	}
}
