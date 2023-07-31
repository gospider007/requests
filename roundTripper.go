package requests

import (
	"bufio"
	"container/list"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

type Connector struct {
	element *list.Element
	h2      bool
	r       *bufio.Reader
	conn    net.Conn
}

type connecotr struct {
	conn net.Conn
	cnl  context.CancelFunc
	ctx  context.Context
}

func (obj *connecotr) run() {
	defer obj.Close()
	con := make([]byte, 0)
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		_, err := obj.conn.Read(con)
		if err != nil {
			return
		}
		t.Reset(time.Second * 5)
		select {
		case <-obj.ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (obj *connecotr) Close() error {
	obj.cnl()
	return obj.conn.Close()
}

func (obj *connecotr) Read(b []byte) (n int, err error) {
	obj.conn.SetDeadline(time.Now().Add(time.Second * 60 * 5))
	return obj.conn.Read(b)
}
func (obj *connecotr) Write(b []byte) (n int, err error) {
	obj.conn.SetDeadline(time.Now().Add(time.Second * 60 * 5))
	return obj.conn.Write(b)
}


type conns struct {
	conns     *list.List
	connsLock sync.Mutex
}
type RoundTripper struct {
	conns     map[string]*conns
	connsLock sync.Mutex
	dialer    *DialClient
}

func NewRoundTripper(dialClient *DialClient) *RoundTripper {
	return &RoundTripper{
		conns:  make(map[string]*conns),
		dialer: dialClient,
	}
}

func (obj *Connector) RoundTrip(req *http.Request) (*http.Response, error) {
	err := req.Write(obj.conn)
	if err != nil {
		return nil, err
	}
	n, err := obj.conn.Read(make([]byte, 0))
	log.Print("read end")
	log.Print(n, err)

	resp, err := http.ReadResponse(bufio.NewReader(obj.conn), req)
	return resp, err
}
func (obj *conns) getConn() *Connector {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	front := obj.conns.Front()
	if front == nil {
		return nil
	}
	conn := obj.conns.Remove(front).(*Connector)
	conn.element = nil
	return conn
}
func (obj *conns) putConn(conn *Connector) {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	element := obj.conns.PushBack(conn)
	conn.element = element
}
func (obj *conns) newConn(req *http.Request) *Connector {
	return nil
}

func (obj *RoundTripper) getAddr(uurl *url.URL) string {
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
func (obj *RoundTripper) getHost(req *http.Request) string {
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
func (obj *RoundTripper) getKey(req *http.Request) string {
	ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
	return fmt.Sprintf("%s@%s", obj.getAddr(ctxData.proxy), obj.getAddr(req.URL))
}
func (obj *RoundTripper) getConn(key string) *Connector {
	obj.connsLock.Lock()
	defer obj.connsLock.Unlock()
	conns, ok := obj.conns[key]
	if ok {
		return conns.getConn()
	}
	return nil
}
func (obj *RoundTripper) newConn(req *http.Request) (connector *Connector, err error) {
	ctxData := req.Context().Value(keyPrincipalID).(*reqCtxData)
	if !ctxData.disProxy && ctxData.proxy == nil { //确定代理
		if ctxData.proxy, err = obj.dialer.GetProxy(req.Context(), req.URL); err != nil {
			return nil, err
		}
	}
	var netConn net.Conn
	host := obj.getHost(req)
	addr := obj.getAddr(req.URL)
	if ctxData.proxy == nil {
		netConn, err = obj.dialer.DialContext(req.Context(), "tcp", addr)
	} else {
		netConn, err = obj.dialer.DialContextWithProxy(req.Context(), "tcp", req.URL.Scheme, addr, host, ctxData.proxy)
	}
	if err != nil {
		return nil, err
	}
	connector = new(Connector)
	if req.URL.Scheme == "https" {
		ctx, cnl := context.WithTimeout(req.Context(), obj.dialer.tlsHandshakeTimeout)
		defer cnl()
		netConn, err = obj.dialer.AddTls(ctx, netConn, host, ctxData.ws)
		if err != nil {
			return nil, err
		}
		if tlsConn, ok := netConn.(interface {
			ConnectionState() utls.ConnectionState
		}); ok {
			connector.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
		} else if tlsConn, ok := netConn.(interface {
			ConnectionState() tls.ConnectionState
		}); ok {
			connector.h2 = tlsConn.ConnectionState().NegotiatedProtocol == "h2"
		}
	}
	connector.conn = netConn
	return connector, nil
}
func (obj *RoundTripper) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	key := obj.getKey(req)
	conn := obj.getConn(key)
	if conn == nil {
		if conn, err = obj.newConn(req); err != nil {
			return nil, err
		}
	}
	return conn.RoundTrip(req)

	// // 2. 执行请求
	// // 3. 返回连接
	// obj.putConn(conn)
	// return resp, err
}
