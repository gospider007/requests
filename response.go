package requests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gospider007/bar"
	"github.com/gospider007/bs4"
	"github.com/gospider007/gson"
	"github.com/gospider007/re"
	"github.com/gospider007/tools"
	"github.com/gospider007/websocket"
)

func NewResponse(ctx context.Context, option RequestOption) *Response {
	return &Response{
		ctx:    ctx,
		option: &option,
	}
}
func (obj *Response) Err() error {
	if obj.err != nil {
		return obj.err
	}
	if obj.request != nil {
		return obj.request.Context().Err()
	}
	return obj.ctx.Err()
}
func (obj *Response) Request() *http.Request {
	return obj.request
}
func (obj *Response) Response() *http.Response {
	return obj.response
}
func (obj *Response) Context() context.Context {
	if obj.request != nil {
		return obj.request.Context()
	}
	return obj.ctx
}
func (obj *Response) Option() *RequestOption {
	return obj.option
}
func (obj *Response) Client() *Client {
	return obj.client
}

type Response struct {
	err          error
	ctx          context.Context
	request      *http.Request
	body         *wrapBody
	response     *http.Response
	webSocket    *websocket.Conn
	sse          *SSE
	cnl          context.CancelFunc
	option       *RequestOption
	client       *Client
	encoding     string
	filePath     string
	requestId    string
	content      []byte
	proxys       []*url.URL
	readBody     bool
	readBodyLock sync.Mutex
	isNewConn    bool
}
type SSE struct {
	reader   *bufio.Reader
	response *Response
}
type Event struct {
	Data    string //data
	Event   string //event
	Id      string //id
	Comment string //comment info
	Retry   int    //retry num
}

func newSSE(response *Response) *SSE {
	return &SSE{response: response, reader: bufio.NewReader(response.Body())}
}

// recv SSE envent data
func (obj *SSE) Recv() (Event, error) {
	var event Event
	for {
		readStr, err := obj.reader.ReadString('\n')
		if err != nil || readStr == "\n" {
			return event, err
		}
		reResult := re.Search(`data:\s?(.*)`, readStr)
		if reResult != nil {
			event.Data += reResult.Group(1)
			continue
		}
		reResult = re.Search(`event:\s?(.*)`, readStr)
		if reResult != nil {
			event.Event = reResult.Group(1)
			continue
		}
		reResult = re.Search(`id:\s?(.*)`, readStr)
		if reResult != nil {
			event.Id = reResult.Group(1)
			continue
		}
		reResult = re.Search(`retry:\s?(.*)`, readStr)
		if reResult != nil {
			if event.Retry, err = strconv.Atoi(reResult.Group(1)); err != nil {
				return event, err
			}
			continue
		}
		reResult = re.Search(`:\s?(.*)`, readStr)
		if reResult != nil {
			event.Comment = reResult.Group(1)
			continue
		}
		return event, errors.New("content parse error:" + readStr)
	}
}
func (obj *SSE) Range() iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		defer obj.Close()
		for {
			event, err := obj.Recv()
			if err == io.EOF {
				return
			}
			if !yield(event, err) || err != nil {
				return
			}
		}
	}
}

// close SSE
func (obj *SSE) Close() {
	obj.response.CloseConn()
}

// return websocket client
func (obj *Response) WebSocket() *websocket.Conn {
	return obj.webSocket
}

// return SSE client
func (obj *Response) SSE() *SSE {
	return obj.sse
}

// return URL redirected address
func (obj *Response) Location() (*url.URL, error) {
	u, err := obj.response.Location()
	if err == http.ErrNoLocation {
		err = nil
	}
	return u, err
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
	if obj.webSocket == nil && obj.sse == nil {
		obj.ReadBody()
	}
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
	obj.bar.Add(int64(l))
	return l, err
}
func (obj *Response) defaultDecode() bool {
	return strings.Contains(obj.ContentType(), "html")
}
func (obj *Response) Body() io.ReadCloser {
	return obj.response.Body
}

// read body
func (obj *Response) ReadBody() (err error) {
	obj.readBodyLock.Lock()
	defer obj.readBodyLock.Unlock()
	if obj.readBody {
		return nil
	}
	defer func() {
		if err != nil {
			obj.CloseConn()
		} else {
			obj.Close()
			if obj.response.StatusCode == 101 && obj.webSocket == nil {
				obj.webSocket = websocket.NewConn(newFakeConn(obj.body.connStream()), true, obj.Headers().Get("Sec-WebSocket-Extensions"))
			}
		}
	}()
	obj.readBody = true
	bBody := bytes.NewBuffer(nil)
	done := make(chan struct{})
	var readErr error
	go func() {
		defer close(done)
		if obj.option.Bar && obj.ContentLength() > 0 {
			_, readErr = tools.Copy(&barBody{
				bar:  bar.NewClient(obj.response.ContentLength),
				body: bBody,
			}, obj.Body())
		} else {
			_, readErr = tools.Copy(bBody, obj.Body())
		}
		if readErr == io.ErrUnexpectedEOF {
			readErr = nil
		}
	}()
	select {
	case <-obj.ctx.Done():
		return tools.WrapError(obj.ctx.Err(), "response read ctx error")
	case <-done:
		if readErr != nil {
			return tools.WrapError(readErr, "response read content error")
		}
	}
	if !obj.option.DisDecode && obj.defaultDecode() {
		obj.content, obj.encoding, _ = tools.Charset(bBody.Bytes(), obj.ContentType())
	} else {
		obj.content = bBody.Bytes()
	}
	return
}

// conn is new conn
func (obj *Response) IsNewConn() bool {
	return obj.isNewConn
}

// conn proxy
func (obj *Response) Proxys() []Address {
	if obj.body != nil {
		return obj.body.Proxys()
	}
	return nil
}

// close
func (obj *Response) CloseConn() {
	if obj.body != nil {
		obj.body.CloseConn()
	}
}

// close
func (obj *Response) Close() {
	if obj.body != nil {
		obj.body.Close()
	}
}
