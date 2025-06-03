package requests

import (
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/mholt/archives"
)

type CompressionConn struct {
	conn net.Conn
	w    io.WriteCloser
	r    io.ReadCloser
	f    interface{ Flush() error }
}

func NewCompressionConn(decode string, conn net.Conn) (net.Conn, error) {
	var r io.ReadCloser
	var w io.WriteCloser
	var err error
	switch strings.ToLower(decode) {
	case "zstd":
		r, w, err = newZstdConn(conn)
	default:
		return nil, errors.New("unsupported compression type")
	}
	if err != nil {
		return nil, err
	}
	ccon := &CompressionConn{conn: conn, r: r, w: w}
	if f, ok := w.(interface{ Flush() error }); ok {
		ccon.f = f
	}
	return ccon, nil
}

func newZstdConn(conn net.Conn) (io.ReadCloser, io.WriteCloser, error) {
	r, err := archives.Zstd{}.OpenReader(conn)
	if err != nil {
		return nil, nil, err
	}
	w, err := archives.Zstd{}.OpenWriter(conn)
	if err != nil {
		return nil, nil, err
	}
	return r, w, nil
}
func (obj *CompressionConn) Read(b []byte) (n int, err error) {
	return obj.r.Read(b)
}
func (obj *CompressionConn) Write(b []byte) (n int, err error) {
	n, err = obj.w.Write(b)
	if err != nil {
		return
	}
	err = obj.f.Flush()
	return
}
func (obj *CompressionConn) Close() error {
	return obj.conn.Close()
}
func (obj *CompressionConn) LocalAddr() net.Addr {
	return obj.conn.LocalAddr()
}
func (obj *CompressionConn) RemoteAddr() net.Addr {
	return obj.conn.RemoteAddr()
}
func (obj *CompressionConn) SetDeadline(t time.Time) error {
	return obj.conn.SetDeadline(t)
}

func (obj *CompressionConn) SetReadDeadline(t time.Time) error {
	return obj.conn.SetReadDeadline(t)
}
func (obj *CompressionConn) SetWriteDeadline(t time.Time) error {
	return obj.conn.SetWriteDeadline(t)
}
