package requests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"net/http"

	"github.com/gospider007/bar"
	"github.com/gospider007/bs4"
	"github.com/gospider007/gson"
	"github.com/gospider007/gtls"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
)

type Response struct {
	response  *http.Response
	webSocket *websocket.Conn
	sseClient *SseClient

	ctx       context.Context
	cnl       context.CancelFunc
	content   []byte
	encoding  string
	stream    bool
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
			return event, errors.New("content parse error:" + readStr)
		}
	}
}

type Cookies []*http.Cookie

func (obj Cookies) String() string {
	cooks := []string{}
	for _, cook := range obj {
		cooks = append(cooks, fmt.Sprintf("%s=%s", cook.Name, cook.Value))
	}
	return strings.Join(cooks, "; ")
}

func (obj Cookies) Gets(name string) Cookies {
	var result Cookies
	for _, cook := range obj {
		if cook.Name == name {
			result = append(result, cook)
		}
	}
	return result
}

func (obj Cookies) Get(name string) *http.Cookie {
	vals := obj.Gets(name)
	if i := len(vals); i == 0 {
		return nil
	} else {
		return vals[i-1]
	}
}

func (obj Cookies) GetVals(name string) []string {
	var result []string
	for _, cook := range obj {
		if cook.Name == name {
			result = append(result, cook.Value)
		}
	}
	return result
}

func (obj Cookies) GetVal(name string) string {
	vals := obj.GetVals(name)
	if i := len(vals); i == 0 {
		return ""
	} else {
		return vals[i-1]
	}
}

func (obj *Response) WebSocket() *websocket.Conn {
	return obj.webSocket
}

func (obj *Response) SseClient() *SseClient {
	return obj.sseClient
}

func (obj *Response) Location() (*url.URL, error) {
	return obj.response.Location()
}

func (obj *Response) Cookies() Cookies {
	if obj.filePath != "" {
		return nil
	}
	return obj.response.Cookies()
}

func (obj *Response) StatusCode() int {
	if obj.filePath != "" {
		return 200
	}
	return obj.response.StatusCode
}

func (obj *Response) Status() string {
	if obj.filePath != "" {
		return "200 OK"
	}
	return obj.response.Status
}

func (obj *Response) Url() *url.URL {
	if obj.filePath != "" {
		return nil
	}
	return obj.response.Request.URL
}

func (obj *Response) Headers() http.Header {
	if obj.filePath != "" {
		return http.Header{
			"Content-Type": []string{obj.ContentType()},
		}
	}
	return obj.response.Header
}

func (obj *Response) Decode(encoding string) {
	if obj.encoding != encoding {
		obj.encoding = encoding
		obj.SetContent(tools.Decode(obj.Content(), encoding))
	}
}

func (obj *Response) Map() (data map[string]any, err error) {
	_, err = gson.Decode(obj.Content(), &data)
	return
}
func (obj *Response) Json(vals ...any) (*gson.Client, error) {
	return gson.Decode(obj.Content(), vals...)
}

func (obj *Response) Text() string {
	return tools.BytesToString(obj.Content())
}

func (obj *Response) SetContent(val []byte) {
	obj.content = val
}

func (obj *Response) Content() []byte {
	return obj.content
}

func (obj *Response) Html() *bs4.Client {
	return bs4.NewClient(obj.Text(), obj.Url().String())
}

func (obj *Response) ContentType() string {
	if obj.filePath != "" {
		return gtls.GetContentTypeWithBytes(obj.content)
	}
	contentType := obj.response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = gtls.GetContentTypeWithBytes(obj.content)
	}
	return contentType
}

func (obj *Response) ContentEncoding() string {
	if obj.filePath != "" {
		return ""
	}
	return obj.response.Header.Get("Content-Encoding")
}

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

func (obj *Response) Read(con []byte) (i int, err error) {
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

func (obj *Response) oneceAlive() bool {
	return obj.webSocket != nil || obj.sseClient != nil || obj.stream
}

// read body
func (obj *Response) ReadBody() error {
	if obj.webSocket != nil || obj.sseClient != nil {
		return errors.New("ws or sse can not read")
	}
	var bBody *bytes.Buffer
	var err error
	if obj.bar && obj.ContentLength() > 0 {
		bBody, err = obj.barRead()
	} else {
		bBody = bytes.NewBuffer(nil)
		err = tools.CopyWitchContext(obj.response.Request.Context(), bBody, obj.response.Body, false)
	}
	if err != nil {
		obj.Delete()
		return errors.New("response read content error: " + err.Error())
	}
	if !obj.disUnzip {
		if bBody, err = tools.CompressionDecode(obj.ctx, bBody, obj.ContentEncoding()); err != nil {
			return errors.New("response compressioin decode error: " + err.Error())
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

// safe close conn
func (obj *Response) Delete() {
	obj.response.Body.(interface{ Delete() }).Delete()
}

// force close conn
func (obj *Response) ForceDelete() {
	obj.response.Body.(interface{ ForceDelete() }).ForceDelete()
}

// conn proxy
func (obj *Response) Proxy() string {
	return obj.response.Body.(interface{ Proxy() string }).Proxy()
}

// conn is in pool ?
func (obj *Response) InPool() bool {
	return obj.response.Body.(interface{ InPool() bool }).InPool()
}

// conn ja3
func (obj *Response) Ja3() string {
	return obj.response.Body.(interface{ Ja3() string }).Ja3()
}

// conn h2ja3
func (obj *Response) H2Ja3() string {
	return obj.response.Body.(interface{ H2Ja3() string }).H2Ja3()
}

// close body
func (obj *Response) Close() error {
	if obj.cnl != nil {
		defer obj.cnl()
	}
	if obj.webSocket != nil {
		obj.webSocket.Close()
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
