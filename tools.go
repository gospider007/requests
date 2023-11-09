package requests

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
func getAddr(uurl *url.URL) string {
	if uurl == nil {
		return ""
	}
	_, port, _ := net.SplitHostPort(uurl.Host)
	if port == "" {
		if uurl.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
		return fmt.Sprintf("%s:%s", uurl.Host, port)
	}
	return uurl.Host
}
func cloneUrl(u *url.URL) *url.URL {
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

//go:linkname stringContainsCTLByte net/http.stringContainsCTLByte
func stringContainsCTLByte(s string) bool

func httpWrite(r *http.Request, w *bufio.Writer, orderHeaders []string) (err error) {
	host := r.Host
	if host == "" {
		host = r.URL.Host
	}
	host, err = httpguts.PunycodeHostPort(host)
	if err != nil {
		return err
	}
	if !httpguts.ValidHostHeader(host) {
		return errors.New("http: invalid Host header")
	}
	host = removeZone(host)
	ruri := r.URL.RequestURI()
	if r.Method == "CONNECT" && r.URL.Path == "" {
		// CONNECT requests normally give just the host and port, not a full URL.
		ruri = host
		if r.URL.Opaque != "" {
			ruri = r.URL.Opaque
		}
	}
	if stringContainsCTLByte(ruri) {
		return errors.New("net/http: can't write control character in Request.URL")
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
	if _, err = fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", r.Method, ruri); err != nil {
		return err
	}
	for _, k := range orderHeaders {
		if k2, ok := replaceMap[k]; ok {
			k = k2
		}
		for _, v := range r.Header.Values(k) {
			if _, err = fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	for k, vs := range r.Header {
		if !slices.Contains(orderHeaders, k) {
			if k2, ok := replaceMap[k]; ok {
				k = k2
			}
			for _, v := range vs {
				if _, err = fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
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
