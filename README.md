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
## send request,support :"GET","HEAD","POST","PUT","PATCH"
```go
resp,error:=request.Get(nil,"http://httpbin.org/get")
```
## use proxy
```go
resp, error := request.Get(nil, "http://httpbin.org/get", requests.RequestOption{Proxy: "http://127.0.0.1:8888"})
```
## use ja3 fingerprint
```go
resp, _ := request.Get(nil, "http://httpbin.org/get", requests.RequestOption{Ja3: true})
```
## use session
```go
session, _ := requests.NewClient(nil)
resp, _ := session.Get(nil, "http://httpbin.org/get")
```
## close body
```go
resp,_:=request.Get(nil,"http://httpbin.org/get")
resp.Close()
```
## safe delete net.conn
```go
resp,_:=request.Get(nil,"http://httpbin.org/get")
resp.Delete()
```
## force delete net.conn
```go
resp,_:=request.Get(nil,"http://httpbin.org/get")
resp.ForceDelete()
```
## websocket
```go
response, _ := request.Get(nil, "ws://82.157.123.54:9010/ajaxchattest", requests.RequestOption{Headers: map[string]string{"Origin": "http://coolaf.com"}})
defer response.Close()
wsCli := response.WebSocket()
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