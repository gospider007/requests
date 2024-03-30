package requests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	_ "unsafe"

	"github.com/gospider007/ja3"
	"golang.org/x/exp/slices"
	"golang.org/x/net/http/httpguts"
)

func getHost(req *http.Request) string {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	_, port, _ := net.SplitHostPort(host)
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		return fmt.Sprintf("%s:%s", host, port)
	}
	return host
}
func getAddr(uurl *url.URL) (addr string) {
	if uurl == nil {
		return ""
	}
	_, port, _ := net.SplitHostPort(uurl.Host)
	if port == "" {
		bs := builderPool.Get().(*strings.Builder)
		bs.WriteString(uurl.Host)
		bs.WriteString(":")
		if uurl.Scheme == "https" {
			bs.WriteString("443")
		} else {
			bs.WriteString("80")
		}
		addr = bs.String()
		bs.Reset()
		builderPool.Put(bs)
		return
	}
	return uurl.Host
}
func cloneUrl(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	r := *u
	return &r
}

var replaceMap = map[string]string{
	"Sec-Ch-Ua":          "sec-ch-ua",
	"Sec-Ch-Ua-Mobile":   "sec-ch-ua-mobile",
	"Sec-Ch-Ua-Platform": "sec-ch-ua-platform",
}

//go:linkname escapeQuotes mime/multipart.escapeQuotes
func escapeQuotes(string) string

//go:linkname readCookies net/http.readCookies
func readCookies(h http.Header, filter string) []*http.Cookie

//go:linkname readSetCookies net/http.readSetCookies
func readSetCookies(h http.Header) []*http.Cookie

//go:linkname ReadRequest net/http.readRequest
func ReadRequest(b *bufio.Reader) (*http.Request, error)

//go:linkname removeZone net/http.removeZone
func removeZone(host string) string

//go:linkname shouldSendContentLength net/http.(*transferWriter).shouldSendContentLength
func shouldSendContentLength(t *http.Request) bool

//go:linkname removeEmptyPort net/http.removeEmptyPort
func removeEmptyPort(host string) string

//go:linkname redirectBehavior net/http.redirectBehavior
func redirectBehavior(reqMethod string, resp *http.Response, ireq *http.Request) (redirectMethod string, shouldRedirect, includeBody bool)

//go:linkname readTransfer net/http.readTransfer
func readTransfer(msg any, r *bufio.Reader) (err error)

var filterHeaderKeys = ja3.DefaultOrderHeadersWithH2()

func httpWrite(r *http.Request, w *bufio.Writer, orderHeaders []string) (err error) {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	host, err = httpguts.PunycodeHostPort(host)
	if err != nil {
		return err
	}
	host = removeZone(host)
	ruri := r.URL.RequestURI()
	if r.Method == "CONNECT" && r.URL.Path == "" {
		if r.URL.Opaque != "" {
			ruri = r.URL.Opaque
		} else {
			ruri = host
		}
	}
	if r.Header.Get("Host") == "" {
		r.Header.Set("Host", host)
	}
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", UserAgent)
	}
	if r.Header.Get("Content-Length") == "" && shouldSendContentLength(r) {
		r.Header.Set("Content-Length", fmt.Sprint(r.ContentLength))
	}
	bs := builderPool.Get().(*strings.Builder)
	defer func() {
		bs.Reset()
		builderPool.Put(bs)
	}()
	bs.WriteString(r.Method)
	bs.WriteString(" ")
	bs.WriteString(ruri)
	bs.WriteString(" ")
	bs.WriteString(r.Proto)
	bs.WriteString("\r\n")
	if _, err = w.WriteString(bs.String()); err != nil {
		return err
	}
	for _, k := range orderHeaders {
		if vs, ok := r.Header[k]; ok {
			if k2, ok := replaceMap[k]; ok {
				k = k2
			}
			if slices.Contains(filterHeaderKeys, k) {
				continue
			}
			for _, v := range vs {
				bs.Reset()
				bs.WriteString(k)
				bs.WriteString(": ")
				bs.WriteString(v)
				bs.WriteString("\r\n")
				if _, err = w.WriteString(bs.String()); err != nil {
					return err
				}
			}
		}
	}
	for k, vs := range r.Header {
		if !slices.Contains(orderHeaders, k) {
			if k2, ok := replaceMap[k]; ok {
				k = k2
			}
			if slices.Contains(filterHeaderKeys, k) {
				continue
			}
			for _, v := range vs {

				bs.Reset()
				bs.WriteString(k)
				bs.WriteString(": ")
				bs.WriteString(v)
				bs.WriteString("\r\n")
				if _, err = w.WriteString(bs.String()); err != nil {
					return err
				}
			}
		}
	}
	if _, err = w.WriteString("\r\n"); err != nil {
		return err
	}
	if r.Body != nil {
		if _, err = io.Copy(w, r.Body); err != nil {
			return err
		}
	}
	return w.Flush()
}

var bufferPool sync.Pool
var builderPool sync.Pool

func init() {
	bufferPool.New = func() interface{} {
		return bytes.NewBuffer(nil)
	}
	builderPool.New = func() interface{} {
		return &strings.Builder{}
	}
}

func newRequestWithContext(ctx context.Context, method string, u *url.URL, body io.Reader) (*http.Request, error) {
	req := (&http.Request{}).WithContext(ctx)
	if method == "" {
		req.Method = http.MethodGet
	} else {
		req.Method = strings.ToUpper(method)
	}
	req.URL = u
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1
	req.Host = u.Host
	u.Host = removeEmptyPort(u.Host)
	if body != nil {
		if v, ok := body.(interface{ Len() int }); ok {
			req.ContentLength = int64(v.Len())
		}
		rc, ok := body.(io.ReadCloser)
		if !ok {
			rc = io.NopCloser(body)
		}
		req.Body = rc
	}
	return req, nil
}

func readResponse(tp *textproto.Reader, req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Request: req,
	}
	// Parse the first line of the response.
	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	proto, status, ok := strings.Cut(line, " ")
	if !ok {
		return nil, errors.New("malformed HTTP response")
	}
	resp.Proto = proto
	resp.Status = strings.TrimLeft(status, " ")
	statusCode, _, _ := strings.Cut(resp.Status, " ")
	if resp.StatusCode, err = strconv.Atoi(statusCode); err != nil {
		return nil, errors.New("malformed HTTP status code")
	}
	if resp.ProtoMajor, resp.ProtoMinor, ok = http.ParseHTTPVersion(resp.Proto); !ok {
		return nil, errors.New("malformed HTTP version")
	}
	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	resp.Header = http.Header(mimeHeader)
	return resp, readTransfer(resp, tp.R)
}

func addCookie(req *http.Request, cookies Cookies) {
	cooks := Cookies(readCookies(req.Header, ""))
	for _, cook := range cookies {
		if val := cooks.Get(cook.Name); val == nil {
			cooks = cooks.append(cook)
		}
	}
	if result := cooks.String(); result != "" {
		req.Header.Set("Cookie", result)
	}
}
