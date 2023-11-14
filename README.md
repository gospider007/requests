<p align="center">
  <a href="https://github.com/gospider007/requests"><img src="https://go.dev/images/favicon-gopher.png"></a>
</p>
<p align="center"><strong>Requests</strong> <em>- A next-generation HTTP client for Golang.</em></p>
<p align="center">
<a href="https://github.com/gospider007/requests">
    <img src="https://img.shields.io/github/last-commit/gospider007/requests">
</a>
<a href="https://github.com/gospider007/requests">
    <img src="https://img.shields.io/badge/build-passing-brightgreen">
</a>
<a href="https://github.com/gospider007/requests">
    <img src="https://img.shields.io/badge/language-golang-brightgreen">
</a>
</p>

Requests is a fully featured HTTP client library for Golang. Network requests can be completed with just a few lines of code
---
## Features
  * GET, POST, PUT, DELETE, HEAD, PATCH, OPTIONS, etc.
  * [Simple for settings and request](https://github.com/gospider007/requests#quickly-send-requests)
  * [Request](https://pkg.go.dev/github.com/gospider007/requests#RequestOption) Body can be `string`, `[]byte`, `struct`, `map`, `slice` and `io.Reader` too
    * Auto detects `Content-Type`
    * Buffer less processing for `io.Reader`
    * [Flow request](https://github.com/gospider007/requests/blob/master/test/stream_test.go)
  * Response object gives you more possibility
    * [Return whether to reuse connections](https://github.com/gospider007/requests/blob/master/test/isNewConn_test.go)
  * Automatic marshal and unmarshal for  content
  * Easy to upload one or more file(s) via `multipart/form-data`
    * Auto detects file content type
  * Request URL Path Params (aka URI Params)
  * Backoff Retry Mechanism with retry condition function
  * Optionally allows GET request with payload
  * Request design
    * Have client level settings & options and also override at Request level if you want to
    * Request and Response middleware
    * goroutine concurrent safe
    * Gzip - Go does it automatically also requests has fallback handling too
    * Works fine with `HTTP/2` and `HTTP/1.1`
  * [Session](https://github.com/gospider007/requests/blob/master/test/session_test.go)
  * [IPv4, IPv6 Address Control Parsing](https://github.com/gospider007/requests/blob/master/test/addType_test.go)
  * [DNS Settings](https://github.com/gospider007/requests/blob/master/test/dns_test.go)
  * [Fingerprint](https://github.com/gospider007/requests/blob/master/test/ja3_test.go)
    * JA3
    * HTTP2
    * JA4
    * OrderHeaders
    * Request header capitalization
  * [Proxy](https://github.com/gospider007/requests/blob/master/test/proxy_test.go)
    * HTTP
    * HTTPS
    * SOCKS5
  * Protocol
    * HTTP 
    * HTTPS 
    * [WebSocket](https://github.com/gospider007/requests/blob/master/test/websocket_test.go)
    * SSE  
  * Well tested client library
  
## Supported Go Versions
Recommended to use `go1.21.3` and above.
Initially Requests started supporting `go modules`

## Installation

```bash
go get github.com/gospider007/requests
```
## Usage
```go
import "github.com/gospider007/requests"
```
### Quickly send requests
```go
package main

import (
	"log"
	"time"

	"github.com/gospider007/requests"
)

func main() {
    resp, err := requests.Get(nil, "http://httpbin.org/anything")
    if err != nil {
      log.Panic(err)
    }
    log.Print(resp.Text())    // Get content and parse as string
    log.Print(resp.Content()) // Get content as bytes
    log.Print(resp.Json())    // Get content and parse as gjson JSON
    log.Print(resp.Html())    // Get content and parse as goquery DOM
    log.Print(resp.Cookies()) // Get cookies
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