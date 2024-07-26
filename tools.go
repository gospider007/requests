package requests

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
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
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

//go:linkname readCookies net/http.readCookies
func readCookies(h http.Header, filter string) []*http.Cookie

//go:linkname readSetCookies net/http.readSetCookies
func readSetCookies(h http.Header) []*http.Cookie

func removeZone(host string) string {
	if !strings.HasPrefix(host, "[") {
		return host
	}
	i := strings.LastIndex(host, "]")
	if i < 0 {
		return host
	}
	j := strings.LastIndex(host[:i], "%")
	if j < 0 {
		return host
	}
	return host[:j] + host[i:]
}

func chunked(te []string) bool    { return len(te) > 0 && te[0] == "chunked" }
func isIdentity(te []string) bool { return len(te) == 1 && te[0] == "identity" }
func shouldSendContentLength(t *http.Request) bool {
	if chunked(t.TransferEncoding) {
		return false
	}
	if t.ContentLength > 0 {
		return true
	}
	if t.ContentLength < 0 {
		return false
	}
	// Many servers expect a Content-Length for these methods
	if t.Method == "POST" || t.Method == "PUT" || t.Method == "PATCH" {
		return true
	}
	if t.ContentLength == 0 && isIdentity(t.TransferEncoding) {
		if t.Method == "GET" || t.Method == "HEAD" {
			return false
		}
		return true
	}

	return false
}

func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

// removeEmptyPort strips the empty port in ":port" to ""
// as mandated by RFC 3986 Section 6.2.3.
func removeEmptyPort(host string) string {
	if hasPort(host) {
		return strings.TrimSuffix(host, ":")
	}
	return host
}

func outgoingLength(r *http.Request) int64 {
	if r.Body == nil || r.Body == http.NoBody {
		return 0
	}
	if r.ContentLength != 0 {
		return r.ContentLength
	}
	return -1
}
func redirectBehavior(reqMethod string, resp *http.Response, ireq *http.Request) (redirectMethod string, shouldRedirect, includeBody bool) {
	switch resp.StatusCode {
	case 301, 302, 303:
		redirectMethod = reqMethod
		shouldRedirect = true
		includeBody = false

		// RFC 2616 allowed automatic redirection only with GET and
		// HEAD requests. RFC 7231 lifts this restriction, but we still
		// restrict other methods to GET to maintain compatibility.
		// See Issue 18570.
		if reqMethod != "GET" && reqMethod != "HEAD" {
			redirectMethod = "GET"
		}
	case 307, 308:
		redirectMethod = reqMethod
		shouldRedirect = true
		includeBody = true

		if ireq.GetBody == nil && outgoingLength(ireq) != 0 {
			// We had a request body, and 307/308 require
			// re-sending it, but GetBody is not defined. So just
			// return this response to the user instead of an
			// error, like we did in Go 1.7 and earlier.
			shouldRedirect = false
		}
	}
	return redirectMethod, shouldRedirect, includeBody
}

var filterHeaderKeys = ja3.DefaultOrderHeadersWithH2()

func httpWrite(r *http.Request, w *bufio.Writer, orderHeaders []string) (err error) {
	for i := range orderHeaders {
		orderHeaders[i] = textproto.CanonicalMIMEHeaderKey(orderHeaders[i])
	}
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
	if r.Header.Get("Connection") == "" {
		r.Header.Set("Connection", "keep-alive")
	}
	if r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", UserAgent)
	}
	if r.Header.Get("Content-Length") == "" && r.ContentLength != 0 && shouldSendContentLength(r) {
		r.Header.Set("Content-Length", fmt.Sprint(r.ContentLength))
	}
	if _, err = w.WriteString(fmt.Sprintf("%s %s %s\r\n", r.Method, ruri, r.Proto)); err != nil {
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
				if _, err = w.WriteString(fmt.Sprintf("%s: %s\r\n", k, v)); err != nil {
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
				if _, err = w.WriteString(fmt.Sprintf("%s: %s\r\n", k, v)); err != nil {
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
func NewRequestWithContext(ctx context.Context, method string, u *url.URL, body io.Reader) (*http.Request, error) {
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
