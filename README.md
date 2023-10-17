# introduce
Lightning fast HTTP client
# get started
## install
```
go get github.com/gospider007/requests
```
# Function Overview
- Only a few lines of code are needed to complete the request
- HTTP2 Fingerprint, JA3 Fingerprint
- SOCKS5 proxy, HTTP proxy, HTTPS proxy
- WebSocket protocol, SSE protocol ,HTTPS protocol,HTTP protocol
- Cookie switch
- Connection pool
- Automatic decompression and decoding
- DNS caching
- Automatic type conversion
- Retry attempts, request callbacks
- Force connection closure
- Powerful and convenient request callback, very convenient to obtain every request detail

# quick start
## Quickly send requests
```go
package main

import (
	"log"
	"time"

	"github.com/gospider007/requests"
)

func main() {
	href := "http://httpbin.org/anything"
	resp, err := requests.Post(nil, href, requests.RequestOption{
		Ja3:     true,                           //enable ja3 fingerprint
		Cookies: "a=1&b=2",                      //set cookies
		// Proxy:   "http://127.0.0.1:8888",        //set proxy
		Params:  map[string]any{"query": "cat"}, //set query params
		Headers: map[string]any{"token": 12345}, //set headers
		Json:    map[string]any{"age": 60},      //send json data
		Timeout: time.Second * 10,               //set timeout
	})
	if err != nil {
		log.Panic(err)
	}
	log.Print(resp.Text())    // Get content and parse as string
    log.Print(resp.Content()) // Get content as bytes
    log.Print(resp.Json())    // Get JSON and parse with gjson
    log.Print(resp.Html())    // Get content and parse as DOM
    log.Print(resp.Cookies()) // Get cookies
}
```
## use session
```go
package main

import (
	"log"

	"github.com/gospider007/requests"
)

func main() {
	href := "http://httpbin.org/anything"
	session, err := requests.NewClient(nil) //use session
	if err != nil {
		log.Panic(err)
	}
	resp, err := session.Get(nil, href)
	if err != nil {
		log.Panic(err)
	}
	log.Print(resp.StatusCode()) //return status code
}
```
## send websocket
```go
package main

import (
	"log"

	"github.com/gospider007/requests"
	"github.com/gospider007/websocket"
)

func main() {
	response, err := requests.Get(nil, "ws://82.157.123.54:9010/ajaxchattest", requests.RequestOption{Headers: map[string]string{
		"Origin": "http://coolaf.com",
	}}) // Send WebSocket request
	if err != nil {
		log.Panic(err)
	}
	defer response.Close()
	wsCli := response.WebSocket()
	if err = wsCli.Send(nil, websocket.MessageText, "test"); err != nil { // Send text message
		log.Panic(err)
	}
	msgType, con, err := wsCli.Recv(nil) // Receive message
	if err != nil {
		log.Panic(err)
	}
	log.Print(msgType)     // Message type
	log.Print(string(con)) // Message content
}
```
## IPv4, IPv6 Address Control Parsing
```go
package main

import (
	"log"

	"github.com/gospider007/requests"
)

func main() {
	session, _ := requests.NewClient(nil, requests.ClientOption{
		AddrType: requests.Ipv4, // Prioritize parsing IPv4 addresses
		// AddrType: requests.Ipv6, // Prioritize parsing IPv6 addresses
	})
	resp, err := session.Get(nil, "https://test.ipw.cn")
	if err != nil {
		log.Panic(err)
	}
	log.Print(resp.Text())
	log.Print(resp.StatusCode())
}
```
## Generate Fingerprint from String
```go
package main

import (
	"log"

	"github.com/gospider007/ja3"
	"github.com/gospider007/requests"
)

func main() {
	ja3Str := "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0"
	Ja3Spec, _ := ja3.CreateSpecWithStr(ja3Str) // Generate fingerprint from string
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{Ja3Spec: Ja3Spec})
	if err != nil {
		log.Panic(err)
	}
	jsonData, _ := resp.Json()
	log.Print(jsonData.Get("ja3").String())
	log.Print(jsonData.Get("ja3").String() == ja3Str)
}
```
## Generate Fingerprint from ID
```go
package main

import (
	"log"

	"github.com/gospider007/ja3"
	"github.com/gospider007/requests"
)

func main() {
	Ja3Spec, _ := ja3.CreateSpecWithId(ja3.HelloChrome_Auto) // Generate fingerprint from ID
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/ja3?extended=1", requests.RequestOption{Ja3Spec: Ja3Spec})
	if err != nil {
		log.Panic(err)
	}
	jsonData, _ := resp.Json()
	log.Print(jsonData.Get("ja3").String())
}
```
## Modify H2 Fingerprint
```go
package main

import (
	"log"

	"github.com/gospider007/ja3"
	"github.com/gospider007/requests"
)

func main() {
	h2ja3Spec := ja3.H2Ja3Spec{
		InitialSetting: []ja3.Setting{
			{Id: 1, Val: 65555},
			{Id: 2, Val: 1},
			{Id: 3, Val: 2000},
			{Id: 4, Val: 6291457},
			{Id: 6, Val: 262145},
		},
		ConnFlow: 15663106,
		OrderHeaders: []string{
			":method",
			":path",
			":scheme",
			":authority",
		},
	}
	resp, err := requests.Get(nil, "https://tools.scrapfly.io/api/fp/anything", requests.RequestOption{H2Ja3Spec: h2ja3Spec})
	if err != nil {
		log.Panic(err)
	}
	log.Print(resp.Text())
}
```

# Contributing
If you have a bug report or feature request, you can [open an issue](../../issues/new)
# Contact
If you have questions, feel free to reach out to us in the following ways:
* QQ Group (Chinese): 939111384 - <a href="http://qm.qq.com/cgi-bin/qm/qr?_wv=1027&k=yI72QqgPExDqX6u_uEbzAE_XfMW6h_d3&jump_from=webapi"><img src="https://pub.idqqimg.com/wpa/images/group.png"></a>
* WeChat (Chinese): gospider007

## Sponsors
If you like and it really helps you, feel free to reward me with a cup of coffee, and don't forget to mention your github id.
<table>
    <tr>
        <td align="center">
            <img src="https://github.com/gospider007/tools/blob/master/play/wx.jpg?raw=true" height="200px" width="200px"   alt=""/>
            <br />
            <sub><b>Wechat</b></sub>
        </td>
        <td align="center">
            <img src="https://github.com/gospider007/tools/blob/master/play/qq.jpg?raw=true" height="200px" width="200px"   alt=""/>
            <br />
            <sub><b>Alipay</b></sub>
        </td>
    </tr>
</table>