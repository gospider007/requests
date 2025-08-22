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
	"github.com/gospider007/http1"
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
	lastResponse *http.Response
	ctx          context.Context
	request      *http.Request
	rawBody      *http1.Body
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
	readBodyLock sync.Mutex
	connLock     sync.Mutex
	isNewConn    bool
	bodyErr      error
	connKey      string
	putOk        bool
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
	if obj.webSocket != nil {
		return obj.webSocket
	}
	if obj.StatusCode() != 101 {
		return nil
	}
	obj.webSocket = websocket.NewConn(newFakeConn(obj.rawBody.Stream()), true, obj.Headers().Get("Sec-WebSocket-Extensions"))
	return obj.webSocket
}

// return SSE client
func (obj *Response) SSE() *SSE {
	return obj.sse
}

// return URL redirected address
func (obj *Response) Location() (*url.URL, error) {
	if obj.filePath != "" {
		return nil, nil
	}
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
	if obj.webSocket == nil && obj.sse == nil && obj.filePath == "" {
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

// conn is new conn
func (obj *Response) IsNewConn() bool {
	return obj.isNewConn
}

// close
func (obj *Response) CloseConn() {
	if obj.rawBody != nil {
		obj.rawBody.CloseWithError(errors.New("force close conn"))
	}
	obj.cnl()
}

// close
func (obj *Response) CloseBody(err error) {
	obj.closeBody(true, err)
}
func (obj *Response) closeBody(i bool, err error) { // 如果关闭body，这一步几乎是必须的，body 所有情况都要考虑调用这个函数
	if obj.bodyErr != io.EOF { //body 读取有错误
		obj.CloseConn()
		return
	} else { //没有错误
		if obj.StatusCode() == 101 { //如果为websocket
			if i && obj.webSocket == nil { //用户没有启用websocket，则关闭连接
				obj.CloseConn()
				return
			}
		} else {
			if obj.response.Close {
				obj.CloseConn()
				return
			}
		}
	}
	if err == nil {
		err = tools.ErrNoErr
	}
	if err == tools.ErrNoErr { //没有错误
		obj.rawBody.CloseWithError(err)
		if obj.StatusCode() != 101 {
			obj.PutConn()
		}
	} else { //有错误
		obj.CloseConn()
	}
	obj.cnl()
}

// read body
func (obj *Response) PutConn() {
	obj.connLock.Lock()
	defer obj.connLock.Unlock()
	if obj.putOk {
		return
	}
	obj.putOk = true
	obj.client.transport.putConnPool(obj.connKey, obj.rawBody.Conn())
}
func (obj *Response) ReadBody() (err error) {
	obj.readBodyLock.Lock()
	defer obj.readBodyLock.Unlock()
	if obj.bodyErr != nil {
		return nil
	}
	body := obj.Body()
	if body == nil {
		return errors.New("not found body")
	}
	defer body.close(false)
	bBody := bytes.NewBuffer(nil)
	done := make(chan struct{})
	var readErr error
	go func() {
		defer close(done)
		if obj.option.Bar && obj.ContentLength() > 0 {
			_, readErr = tools.Copy(&barBody{
				bar:  bar.NewClient(obj.response.ContentLength),
				body: bBody,
			}, body)
		} else {
			_, readErr = tools.Copy(bBody, body)
		}
		if readErr == io.ErrUnexpectedEOF {
			readErr = nil
		}
	}()
	select {
	case <-obj.ctx.Done():
		if readErr == nil && obj.bodyErr == io.EOF && body.err == nil {
			err = nil
		} else {
			err = tools.WrapError(obj.ctx.Err(), "response read ctx error")
		}
	case <-done:
		if readErr != nil {
			err = tools.WrapError(readErr, "response read content error")
		}
	}
	if err != nil {
		return
	}
	if !obj.option.DisDecode && obj.defaultDecode() {
		obj.content, obj.encoding, _ = tools.Charset(bBody.Bytes(), obj.ContentType())
	} else {
		obj.content = bBody.Bytes()
	}
	return
}

type body struct {
	ctx *Response
	err error
}

func (obj *body) Read(p []byte) (n int, err error) {
	n, err = obj.ctx.response.Body.Read(p)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		obj.ctx.bodyErr = err
		if err != io.EOF {
			obj.closeWithError(false, err)
		} else {
			obj.closeWithError(false, nil)
		}
	}
	return
}

func (obj *body) Close() (err error) {
	return obj.close(true)
}
func (obj *body) close(i bool) (err error) {
	return obj.closeWithError(i, obj.err)
}
func (obj *body) closeWithError(i bool, err error) error {
	if obj.err == nil {
		obj.err = err
	}
	obj.ctx.closeBody(i, err)
	return obj.err
}

func (obj *Response) Body() *body {
	if obj.response == nil || obj.response.Body == nil {
		return nil
	}
	return &body{ctx: obj}
}

func (obj *Client) newResponse(ctx context.Context, option RequestOption, uhref *url.URL, requestId string) *Response {
	option.Url = cloneUrl(uhref)
	response := NewResponse(ctx, option)
	response.client = obj
	response.requestId = requestId
	return response
}
