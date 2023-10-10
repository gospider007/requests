package requests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"net/http"

	"gitee.com/baixudong/bar"
	"gitee.com/baixudong/bs4"
	"gitee.com/baixudong/gson"
	"gitee.com/baixudong/tools"
	"gitee.com/baixudong/websocket"
)

type Response struct {
	response  *http.Response
	webSocket *websocket.Conn
	sseClient *SseClient

	ctx       context.Context
	cnl       context.CancelFunc
	content   []byte
	encoding  string
	disDecode bool
	disUnzip  bool
	filePath  string
	bar       bool
}

type SseClient struct {
	reader *bufio.Reader
}
type Event struct {
	Data    string
	Event   string
	Id      string
	Retry   int
	Comment string
}

func newSseClient(rd io.Reader) *SseClient {
	return &SseClient{reader: bufio.NewReader(rd)}
}
func (obj *SseClient) Recv() (Event, error) {
	var event Event
	for {
		readStr, err := obj.reader.ReadString('\n')
		if err != nil || readStr == "\n" {
			return event, err
		}
		if strings.HasPrefix(readStr, "data: ") {
			event.Data += readStr[6 : len(readStr)-1]
		} else if strings.HasPrefix(readStr, "event: ") {
			event.Event = readStr[7 : len(readStr)-1]
		} else if strings.HasPrefix(readStr, "id: ") {
			event.Id = readStr[4 : len(readStr)-1]
		} else if strings.HasPrefix(readStr, "retry: ") {
			if event.Retry, err = strconv.Atoi(readStr[7 : len(readStr)-1]); err != nil {
				return event, err
			}
		} else if strings.HasPrefix(readStr, ": ") {
			event.Comment = readStr[2 : len(readStr)-1]
		} else {
			return event, errors.New("内容解析错误：" + readStr)
		}
	}
}

type Cookies []*http.Cookie

// 返回cookies 的字符串形式
func (obj Cookies) String() string {
	cooks := []string{}
	for _, cook := range obj {
		cooks = append(cooks, fmt.Sprintf("%s=%s", cook.Name, cook.Value))
	}
	return strings.Join(cooks, "; ")
}

// 获取符合key 条件的所有cookies
func (obj Cookies) Gets(name string) Cookies {
	var result Cookies
	for _, cook := range obj {
		if cook.Name == name {
			result = append(result, cook)
		}
	}
	return result
}

// 获取符合key 条件的cookies
func (obj Cookies) Get(name string) *http.Cookie {
	vals := obj.Gets(name)
	if i := len(vals); i == 0 {
		return nil
	} else {
		return vals[i-1]
	}
}

// 获取符合key 条件的所有cookies的值
func (obj Cookies) GetVals(name string) []string {
	var result []string
	for _, cook := range obj {
		if cook.Name == name {
			result = append(result, cook.Value)
		}
	}
	return result
}

// 获取符合key 条件的cookies的值
func (obj Cookies) GetVal(name string) string {
	vals := obj.GetVals(name)
	if i := len(vals); i == 0 {
		return ""
	} else {
		return vals[i-1]
	}
}

// 返回原始http.Response
func (obj *Response) Response() *http.Response {
	return obj.response
}

// 返回websocket 对象,当发送websocket 请求时使用
func (obj *Response) WebSocket() *websocket.Conn {
	return obj.webSocket
}
func (obj *Response) SseClient() *SseClient {
	return obj.sseClient
}

// 返回当前的Location
func (obj *Response) Location() (*url.URL, error) {
	return obj.response.Location()
}

// 返回这个请求的setCookies
func (obj *Response) Cookies() Cookies {
	if obj.filePath != "" {
		return nil
	}
	return obj.response.Cookies()
}

// 返回这个请求的状态码
func (obj *Response) StatusCode() int {
	if obj.filePath != "" {
		return 200
	}
	return obj.response.StatusCode
}

// 返回这个请求的状态
func (obj *Response) Status() string {
	if obj.filePath != "" {
		return "200 OK"
	}
	return obj.response.Status
}

// 返回这个请求的url
func (obj *Response) Url() *url.URL {
	if obj.filePath != "" {
		return nil
	}
	return obj.response.Request.URL
}

// 返回response 的请求头
func (obj *Response) Headers() http.Header {
	if obj.filePath != "" {
		return http.Header{
			"Content-Type": []string{obj.ContentType()},
		}
	}
	return obj.response.Header
}

// 对内容进行解码
func (obj *Response) Decode(encoding string) {
	if obj.encoding != encoding {
		obj.encoding = encoding
		obj.SetContent(tools.Decode(obj.Content(), encoding))
	}
}

// 尝试将内容解析成map
func (obj *Response) Map() (map[string]any, error) {
	var data map[string]any
	return data, json.Unmarshal(obj.Content(), &data)
}

// 尝试将请求解析成gjson, 如果传值将会解析到val中返回的gjson为空struct
func (obj *Response) Json(vals ...any) (*gson.Client, error) {
	return gson.Decode(obj.Content(), vals...)
}

// 返回内容的字符串形式
func (obj *Response) Text() string {
	return tools.BytesToString(obj.Content())
}

// 返回内容的二进制，也可设置内容
func (obj *Response) SetContent(val []byte) {
	obj.content = val
}

// body 数据的源头
func (obj *Response) Content() []byte {
	return obj.content
}

// 尝试解析成dom 对象
func (obj *Response) Html() *bs4.Client {
	return bs4.NewClient(obj.Text(), obj.Url().String())
}

// 获取headers 的Content-Type
func (obj *Response) ContentType() string {
	if obj.filePath != "" {
		return GetContentTypeWithBytes(obj.content)
	}
	contentType := obj.response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = GetContentTypeWithBytes(obj.content)
	}
	return contentType
}

// 获取headers 的Content-Encoding
func (obj *Response) ContentEncoding() string {
	if obj.filePath != "" {
		return ""
	}
	return obj.response.Header.Get("Content-Encoding")
}

// 获取response 的内容长度
func (obj *Response) ContentLength() int64 {
	if obj.filePath != "" {
		return int64(len(obj.content))
	}
	if obj.response.ContentLength >= 0 {
		return obj.response.ContentLength
	}
	return int64(len(obj.content))
}

type barBody struct {
	body *bytes.Buffer
	bar  *bar.Client
}

func (obj *barBody) Write(con []byte) (int, error) {
	l, err := obj.body.Write(con)
	obj.bar.Print(int64(l))
	return l, err
}
func (obj *Response) barRead() (*bytes.Buffer, error) {
	barData := &barBody{
		bar:  bar.NewClient(obj.response.ContentLength),
		body: bytes.NewBuffer(nil),
	}
	err := tools.CopyWitchContext(obj.response.Request.Context(), barData, obj.response.Body, false)
	if err != nil {
		return nil, err
	}
	return barData.body, nil
}
func (obj *Response) defaultDecode() bool {
	return strings.Contains(obj.ContentType(), "html")
}

func (obj *Response) Read(con []byte) (i int, err error) { //读取body
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if recErr := recover(); recErr != nil && err == nil {
				err, _ = recErr.(error)
			}
		}()
		i, err = obj.response.Body.Read(con)
	}()
	select {
	case <-obj.ctx.Done():
		obj.response.Body.Close()
		return 0, obj.ctx.Err()
	case <-done:
		return
	}
}

func (obj *Response) ReadBody() error { //读取body,对body 解压，解码操作
	var bBody *bytes.Buffer
	var err error
	if obj.bar && obj.ContentLength() > 0 { //是否打印进度条,读取内容
		bBody, err = obj.barRead()
	} else {
		bBody = bytes.NewBuffer(nil)
		err = tools.CopyWitchContext(obj.response.Request.Context(), bBody, obj.response.Body, false)
	}
	if err != nil {
		obj.Delete()
		return errors.New("response 读取内容 错误: " + err.Error())
	}
	if !obj.disUnzip {
		if bBody, err = tools.CompressionDecode(obj.ctx, bBody, obj.ContentEncoding()); err != nil {
			return errors.New("response 解压缩错误: " + err.Error())
		}
	}
	if !obj.disDecode && obj.defaultDecode() {
		if content, encoding, err := tools.Charset(bBody.Bytes(), obj.ContentType()); err == nil {
			obj.content, obj.encoding = content, encoding
		} else {
			obj.content = bBody.Bytes()
		}
	} else {
		obj.content = bBody.Bytes()
	}
	return nil
}

func (obj *Response) Delete() { //通知关闭连接，不会影响正在传输中的数据
	obj.response.Body.(interface{ Delete() }).Delete()
}
func (obj *Response) ForceDelete() { //强制关闭连接，立刻马上,正在传输中的数据立马中断
	obj.response.Body.(interface{ ForceDelete() }).ForceDelete()
}

// 关闭response ,当DisRead 为true,websocket,sse 协议 请一定要手动关闭
func (obj *Response) Close() error {
	if obj.cnl != nil {
		defer obj.cnl()
	}
	if obj.webSocket != nil {
		obj.webSocket.Close("close")
	}
	if obj.response != nil && obj.response.Body != nil {
		if err := tools.CopyWitchContext(obj.ctx, io.Discard, obj.response.Body, false); err != nil {
			obj.Delete()
		} else {
			return obj.response.Body.Close()
		}
	}
	return nil
}
