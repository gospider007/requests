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

Requests is a fully featured HTTP client library for Golang. Network requests can be completed with just a few lines of code. Unified support for http1, http2, http3, websocket, sse protocols
---
## Innovative Features

| **gospider007/requests** | **Other Request Libraries** |
|---------------------------|----------------------------|
| Unlimited chained proxy   | Not supported             |
| HTTP/3 fingerprint spoofing protection | Not supported  |
| Arbitrary closure of underlying connections | Not supported |
| Genuine request-level proxy settings | Not supported   |
| Unique transport layer management mechanism, fully unifying HTTP/1, HTTP/2, HTTP/3, WebSocket, and SSE protocol handling | Not supported |

## Features
  * [Simple for settings and Request](https://github.com/gospider007/requests#quickly-send-requests)
  * [Request](https://github.com/gospider007/requests/tree/master/test/request) Support Automatic type conversion, Support orderly map
    * [Json Request](https://github.com/gospider007/requests/blob/master/test/request/json_test.go) with `application/json`
    * [Data Request](https://github.com/gospider007/requests/blob/master/test/request/data_test.go) with `application/x-www-form-urlencoded`
    * [Form Request](https://github.com/gospider007/requests/blob/master/test/request/form_test.go) with `multipart/form-data`
    * [Upload File Request](https://github.com/gospider007/requests/blob/master/test/request/file_test.go) with `multipart/form-data`
    * [Flow Request](https://github.com/gospider007/requests/blob/master/test/request/stream_test.go)
    * [Request URL Path Params](https://github.com/gospider007/requests/blob/master/test/request/params_test.go)
    * [Local network card](https://github.com/gospider007/requests/blob/master/test/request/localAddr_test.go)
  * [Response](https://github.com/gospider007/requests/tree/master/test/response)
    * [Return whether to reuse connections](https://github.com/gospider007/requests/blob/master/test/response/isNewConn_test.go)
    * [Return Raw Connection](https://github.com/gospider007/requests/blob/master/test/response/rawConn_test.go)
    * [Return Proxy](https://github.com/gospider007/requests/blob/master/test/response/useProxy_test.go)
  * [Middleware](https://github.com/gospider007/requests/tree/master/test/middleware)
    * [Option Callback Method](https://github.com/gospider007/requests/blob/master/test/middleware/optionltCallBack_test.go)
    * [Result Callback Method](https://github.com/gospider007/requests/blob/master/test/middleware/resultCallBack_test.go)
    * [Redirect Callback Method](https://github.com/gospider007/requests/blob/master/test/middleware/redirectCallBack_test.go)
    * [Error Callback Method](https://github.com/gospider007/requests/blob/master/test/middleware/errCallBack_test.go)
    * [Request Callback Method](https://github.com/gospider007/requests/blob/master/test/middleware/requestCallback_test.go)
    * [Retry](https://github.com/gospider007/requests/blob/master/test/middleware/try_test.go)
  * [Protocol](https://github.com/gospider007/requests/tree/master/test/protocol)
    * [HTTP1](https://github.com/gospider007/requests/blob/master/test/protocol/http1_test.go)
    * [HTTP2](https://github.com/gospider007/requests/blob/master/test/protocol/http2_test.go)
    * [HTTP3](https://github.com/gospider007/requests/blob/master/test/protocol/http3_test.go)
    * [WebSocket](https://github.com/gospider007/requests/blob/master/test/protocol/websocket_test.go)
    * [SSE](https://github.com/gospider007/requests/blob/master/test/protocol/sse_test.go)
  * [Fingerprint](https://github.com/gospider007/requests/tree/master/test/fingerprint)
    * [Ja3 Fingerprint](https://github.com/gospider007/requests/blob/master/test/fingerprint/ja3_test.go)
    * [Http2 Fingerprint](https://github.com/gospider007/requests/blob/master/test/fingerprint/http2_test.go)
    * [Quic Fingerprint](https://github.com/gospider007/requests/blob/master/test/fingerprint/quic_test.go)
    * [Ja4 Fingerprint](https://github.com/gospider007/requests/blob/master/test/fingerprint/ja4_test.go)
  * [Session](https://github.com/gospider007/requests/blob/master/test/session_test.go)
  * [IPv4, IPv6 Address Control Parsing](https://github.com/gospider007/requests/blob/master/test/addType_test.go)
  * [DNS Settings](https://github.com/gospider007/requests/blob/master/test/dns_test.go)
  * [Proxy](https://github.com/gospider007/requests/blob/master/test/proxy/proxy_test.go)
  * [Chain Proxy](https://github.com/gospider007/requests/blob/master/test/proxy/chain_proxy_test.go)
  * [Logger](https://github.com/gospider007/requests/blob/master/test/logger/logger_test.go)
  * [Well tested client library](https://github.com/gospider007/requests/tree/master/test)
## [Benchmark](https://github.com/gospider007/benchmark)

[gospider007/requests](https://github.com/gospider007/requests) > [imroc/req](github.com/imroc/req) > [go-resty](github.com/go-resty/resty) > [wangluozhe/requests](github.com/wangluozhe/requests) > [curl_cffi](https://github.com/yifeikong/curl_cffi) > [httpx](https://github.com/encode/httpx) > [psf/requests](https://github.com/psf/requests)
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
### Quickly Send Requests
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