package requests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"

	"net/http"

	"github.com/gospider007/bar"
	"github.com/gospider007/bs4"
	"github.com/gospider007/gson"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
)

type Response struct {
	rawConn   *readWriteCloser
	response  *http.Response
	webSocket *websocket.Conn
	sse       *Sse
	ctx       context.Context
	cnl       context.CancelFunc
	content   []byte
	encoding  string
	stream    bool
	disDecode bool
	disUnzip  bool
	filePath  string
	bar       bool
	isNewConn bool
	readBody  bool
}

type Sse struct {
	reader    *bufio.Reader
	closeFunc func()
}
type Event struct {
	Data    string //data
	Event   string //event
	Id      string //id
	Retry   int    //retry num
	Comment string //comment info
}

func newSse(rd io.Reader, closeFunc func()) *Sse {
	return &Sse{closeFunc: closeFunc, reader: bufio.NewReader(rd)}
}

// recv sse envent data
func (obj *Sse) Recv() (Event, error) {
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

// close sse
func (obj *Sse) Close() {
	obj.closeFunc()
}

// return websocket client
func (obj *Response) WebSocket() *websocket.Conn {
	return obj.webSocket
}

// return sse client
func (obj *Response) Sse() *Sse {
	return obj.sse
}

// return URL redirected address
func (obj *Response) Location() (*url.URL, error) {
	return obj.response.Location()
}

// return response Proto
func (obj *Response) Proto() string {
	return obj.response.Proto
}

// return response cookies
func (obj *Response) Cookies() Cookies {
	if obj.filePath != "" {
		return nil
	}
	return obj.response.Cookies()
}

// return response status code
func (obj *Response) StatusCode() int {
	if obj.filePath != "" {
		return 200
	}
	return obj.response.StatusCode
}

// return response status
func (obj *Response) Status() string {
	if obj.filePath != "" {
		return "200 OK"
	}
	return obj.response.Status
}

// return response url
func (obj *Response) Url() *url.URL {
	if obj.filePath != "" {
		return nil
	}
	return obj.response.Request.URL
}

// return response headers
func (obj *Response) Headers() http.Header {
	if obj.filePath != "" {
		return http.Header{
			"Content-Type": []string{obj.ContentType()},
		}
	}
	return obj.response.Header
}

// change decoding with content
func (obj *Response) Decode(encoding string) {
	if obj.encoding != encoding {
		obj.encoding = encoding
		obj.SetContent(tools.Decode(obj.Content(), encoding))
	}
}

// return content with map[string]any
func (obj *Response) Map() (data map[string]any, err error) {
	_, err = gson.Decode(obj.Content(), &data)
	return
}

// return content with json and you can parse struct
func (obj *Response) Json(vals ...any) (*gson.Client, error) {
	return gson.Decode(obj.Content(), vals...)
}

// return content with string
func (obj *Response) Text() string {
	return tools.BytesToString(obj.Content())
}

// set response content with []byte
func (obj *Response) SetContent(val []byte) {
	obj.content = val
}

// return content with []byte
func (obj *Response) Content() []byte {
	return obj.content
}

// return content with parse html
func (obj *Response) Html() *bs4.Client {
	return bs4.NewClient(obj.Text(), obj.Url().String())
}

// return content type
func (obj *Response) ContentType() string {
	if obj.filePath != "" {
		return http.DetectContentType(obj.content)
	}
	contentType := obj.response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(obj.content)
	}
	return contentType
}

// return content encoding
func (obj *Response) ContentEncoding() string {
	if obj.filePath != "" {
		return ""
	}
	return obj.response.Header.Get("Content-Encoding")
}

// return content length
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
	err := tools.CopyWitchContext(obj.response.Request.Context(), barData, obj.response.Body, true)
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

// return true if response is stream
func (obj *Response) IsStream() bool {
	return obj.webSocket != nil || obj.sse != nil || obj.stream
}

// read body
func (obj *Response) ReadBody() error {
	if obj.webSocket != nil || obj.sse != nil {
		return errors.New("ws or sse can not read")
	}
	if obj.readBody {
		return errors.New("already read body")
	}
	var bBody *bytes.Buffer
	var err error
	defer obj.response.Body.Close()
	if obj.bar && obj.ContentLength() > 0 {
		bBody, err = obj.barRead()
	} else {
		bBody = bytes.NewBuffer(nil)
		err = tools.CopyWitchContext(obj.response.Request.Context(), bBody, obj.response.Body, true)
	}
	obj.readBody = true
	if err != nil {
		obj.CloseConn()
		return errors.New("response read content error: " + err.Error())
	}
	if obj.IsStream() {
		obj.CloseBody()
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

// conn is new conn
func (obj *Response) IsNewConn() bool {
	return obj.isNewConn
}

// close body
func (obj *Response) CloseBody() error {
	if obj.webSocket != nil {
		obj.webSocket.Close()
	}
	if obj.sse != nil {
		obj.sse.Close()
	}
	if obj.IsStream() || !obj.readBody {
		obj.ForceCloseConn()
	} else if obj.rawConn != nil {
		obj.rawConn.Close()
	}
	obj.cnl()
	return nil
}

// conn proxy
func (obj *Response) Proxy() string {
	if obj.rawConn != nil {
		return obj.rawConn.Proxy()
	}
	return ""
}

// conn is in pool ?
func (obj *Response) InPool() bool {
	if obj.rawConn != nil {
		return obj.rawConn.InPool()
	}
	return false
}

// conn ja3
func (obj *Response) Ja3() string {
	if obj.rawConn != nil {
		return obj.rawConn.Ja3()
	}
	return ""
}

// conn h2ja3
func (obj *Response) H2Ja3() string {
	if obj.rawConn != nil {
		return obj.rawConn.H2Ja3()
	}
	return ""
}

// safe close conn
func (obj *Response) CloseConn() {
	if obj.rawConn != nil {
		obj.rawConn.CloseConn()
	}
}

// force close conn
func (obj *Response) ForceCloseConn() {
	if obj.rawConn != nil {
		obj.rawConn.ForceCloseConn()
	}
}
