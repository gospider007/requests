package main

import (
	"log"
	"net/textproto"
	"slices"
	"testing"

	"github.com/gospider007/ja3"
	"github.com/gospider007/requests"
)

func TestJa3(t *testing.T) {
	j := "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,5-27-13-35-16-18-43-17513-65281-51-45-11-0-10-23,12092-29-23-24,0"
	ja3Spec, err := ja3.CreateSpecWithStr(j) //create ja3 spec with string
	if err != nil {
		t.Fatal(err)
	}
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
		Ja3Spec: ja3Spec,
	})
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := resp.Json()   //parse json
	ja3 := jsonData.Get("ja3.ja3") //get ja3 value
	if ja3 == nil {
		t.Fatal("not found ja3")
	}
	if j != ja3.String() {
		log.Print(j)
		log.Print(ja3)
		t.Fatal("not equal")
	}
}
func TestJa3Psk(t *testing.T) {
	j := "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,5-27-13-35-16-18-43-17513-65281-51-45-11-0-10-23-41,12092-29-23-24,0"
	j2 := "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,5-27-13-35-16-18-43-17513-65281-51-45-11-0-10-23,12092-29-23-24,0"
	ja3Spec, err := ja3.CreateSpecWithStr(j) //create ja3 spec with string
	if err != nil {
		t.Fatal(err)
	}
	session, _ := requests.NewClient(nil)
	for i := 0; i < 2; i++ {
		resp, err := session.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{
			Ja3Spec: ja3Spec, //set ja3 spec
		})
		if err != nil {
			t.Fatal(err)
		}
		jsonData, err := resp.Json()
		ja3 := jsonData.Get("ja3.ja3")
		if ja3 == nil {
			t.Fatal("not found ja3")
		}
		var eqJ string
		if i == 0 {
			eqJ = j2
		} else {
			eqJ = j
		}
		if eqJ != ja3.String() {
			log.Print(j)
			log.Print(ja3)
			t.Fatal("not equal")
		}
	}
}

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
