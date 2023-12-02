package requests

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	_ "unsafe"

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
		bs := builderPool.Get().(strings.Builder)
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
	bs := builderPool.Get().(strings.Builder)
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
		return strings.Builder{}
	}
}
