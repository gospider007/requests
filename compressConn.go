package requests

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type CompressionConn struct {
	conn net.Conn
	w    io.WriteCloser
	r    io.ReadCloser
}
type Compression interface {
	String() string
	OpenReader(r io.Reader) (io.ReadCloser, error)
	OpenWriter(w io.Writer) (io.WriteCloser, error)
}
type compression struct {
	name       string
	openReader func(r io.Reader) (io.ReadCloser, error)
	openWriter func(w io.Writer) (io.WriteCloser, error)
}

func (obj compression) String() string {
	return obj.name
}
func (obj compression) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return obj.openReader(r)
}
func (obj compression) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return obj.openWriter(w)
}

func GetCompressionByte(decode string) (byte, error) {
	switch strings.ToLower(decode) {
	case "zstd":
		return 40, nil
	case "s2":
		return 255, nil
	case "flate":
		return 92, nil
	case "minlz":
		return 93, nil
	default:
		return 0, errors.New("unsupported compression type")
	}
}

var compressionData = map[byte]compression{
	40: {
		name:       "zstd",
		openReader: newZstdReader,
		openWriter: newZstdWriter,
	},
	255: {
		name:       "s2",
		openReader: newSnappyReader,
		openWriter: newSnappyWriter,
	},
	92: {
		name:       "flate",
		openReader: newFlateReader,
		openWriter: newFlateWriter,
	},
	93: {
		name:       "minlz",
		openReader: newMinlzReader,
		openWriter: newMinlzWriter,
	},
}

func NewCompressionWithByte(b byte) (Compression, error) {
	c, ok := compressionData[b]
	if !ok {
		return nil, errors.New("unsupported compression type")
	}
	return compression{
		name: c.name,
		openReader: func(r io.Reader) (io.ReadCloser, error) {
			buf := make([]byte, 1)
			n, err := r.Read(buf)
			if err != nil {
				return nil, err
			}
			if n != 1 || buf[0] != b {
				return nil, errors.New("invalid response")
			}
			return c.openReader(r)
		},
		openWriter: func(w io.Writer) (io.WriteCloser, error) {
			n, err := w.Write([]byte{b})
			if err != nil {
				return nil, err
			}
			if n != 1 {
				return nil, errors.New("invalid response")
			}
			return c.openWriter(w)
		},
	}, nil
}
func NewCompression(decode string) (Compression, error) {
	decode = strings.ToLower(decode)
	for b, c := range compressionData {
		if c.String() == decode {
			return NewCompressionWithByte(b)
		}
	}
	return nil, errors.New("unsupported compression type")
}

func NewCompressionConn(conn net.Conn, decode string) (net.Conn, error) {
	arch, err := NewCompression(decode)
	if err != nil {
		return conn, err
	}
	w, err := arch.OpenWriter(conn)
	if err != nil {
		return conn, err
	}
	r, err := arch.OpenReader(conn)
	if err != nil {
		return conn, err
	}
	ccon := &CompressionConn{
		conn: conn,
		r:    r,
		w:    w,
	}
	return ccon, nil
}
func (obj *CompressionConn) Read(b []byte) (n int, err error) {
	return obj.r.Read(b)
}
func (obj *CompressionConn) Write(b []byte) (n int, err error) {
	return obj.w.Write(b)
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

type ReaderCompression struct {
	c         io.ReadCloser
	closed    bool
	lock      sync.Mutex
	closeFunc func()
}

func (obj *ReaderCompression) Read(p []byte) (int, error) {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	if obj.closed {
		return 0, errors.New("closed")
	}
	return obj.c.Read(p)
}
func (obj *ReaderCompression) Close() error {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	if obj.closed {
		return nil
	}
	obj.closed = true
	obj.c.Close()
	obj.closeFunc()
	return nil
}

type WriterCompression struct {
	c         io.WriteCloser
	closed    bool
	lock      sync.Mutex
	flush     interface{ Flush() error }
	closeFunc func()
}

func (obj *WriterCompression) Write(p []byte) (int, error) {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	if obj.closed {
		return 0, errors.New("closed")
	}
	n, err := obj.c.Write(p)
	if err != nil {
		return n, err
	}
	if obj.flush != nil {
		err = obj.flush.Flush()
	}
	return n, err
}

func (obj *WriterCompression) Close() error {
	obj.lock.Lock()
	defer obj.lock.Unlock()
	if obj.closed {
		return nil
	}
	obj.closed = true
	obj.c.Close()
	obj.closeFunc()
	return nil
}

func newWriterCompression(c io.WriteCloser, closeFunc func()) *WriterCompression {
	flush, _ := c.(interface{ Flush() error })
	return &WriterCompression{c: c, closeFunc: closeFunc, flush: flush}
}
func newReaderCompression(c io.ReadCloser, closeFunc func()) *ReaderCompression {
	return &ReaderCompression{c: c, closeFunc: closeFunc}
}
