package requests

import (
	"compress/flate"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

type CompressionConn struct {
	conn net.Conn
	w    *WriterCompression
	r    *ReaderCompression
}
type ReaderCompression struct {
	oneFunc func()
	r       io.Reader
}
type WriterCompression struct {
	oneFunc func()
	w       io.WriteCloser
	f       interface{ Flush() error }
}
type Compression interface {
	OpenReader(r io.Reader) (io.Reader, error)
	OpenWriter(w io.Writer) (io.WriteCloser, error)
}
type compression struct {
	openReader func(r io.Reader) (io.Reader, error)
	openWriter func(w io.Writer) (io.WriteCloser, error)
}

func (obj compression) OpenReader(r io.Reader) (io.Reader, error) {
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
func NewCompressionWithByte(b byte) (Compression, error) {
	switch b {
	case 40:
		return NewCompression("zstd")
	case 255:
		return NewCompression("s2")
	case 92:
		return NewCompression("flate")
	case 93:
		return NewCompression("minlz")
	}
	return nil, errors.New("unsupported compression type")
}
func NewCompression(decode string) (Compression, error) {
	b, err := GetCompressionByte(decode)
	if err != nil {
		return nil, err
	}
	br := func(r io.Reader) error {
		buf := make([]byte, 1)
		n, err := r.Read(buf)
		if err != nil {
			return err
		}
		if n != 1 || buf[0] != b {
			return errors.New("invalid response")
		}
		return nil
	}
	bw := func(w io.Writer) error {
		n, err := w.Write([]byte{b})
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("invalid response")
		}
		return nil
	}
	var arch Compression
	switch strings.ToLower(decode) {
	case "s2":
		arch = compression{
			openReader: func(r io.Reader) (io.Reader, error) {
				err := br(r)
				if err != nil {
					return nil, err
				}
				return getSnappyReader(r), nil
			},
			openWriter: func(w io.Writer) (io.WriteCloser, error) {
				err := bw(w)
				if err != nil {
					return nil, err
				}
				return getSnappyWriter(w), nil
			},
		}
	case "zstd":
		arch = compression{
			openReader: func(r io.Reader) (io.Reader, error) {
				err := br(r)
				if err != nil {
					return nil, err
				}
				decoder, err := zstd.NewReader(r)
				if err != nil {
					return nil, err
				}
				return decoder.IOReadCloser(), nil
			},
			openWriter: func(w io.Writer) (io.WriteCloser, error) {
				err := bw(w)
				if err != nil {
					return nil, err
				}
				encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
				if err != nil {
					return nil, err
				}
				return encoder, nil
			},
		}
	case "flate":
		arch = compression{
			openReader: func(r io.Reader) (io.Reader, error) {
				err := br(r)
				if err != nil {
					return nil, err
				}
				return flate.NewReader(r), nil
			},
			openWriter: func(w io.Writer) (io.WriteCloser, error) {
				err := bw(w)
				if err != nil {
					return nil, err
				}
				return flate.NewWriter(w, flate.DefaultCompression)
			},
		}
	case "minlz":
		arch = compression{
			openReader: func(r io.Reader) (io.Reader, error) {
				err := br(r)
				if err != nil {
					return nil, err
				}
				return getMinlzReader(r), nil
			},
			openWriter: func(w io.Writer) (io.WriteCloser, error) {
				err := bw(w)
				if err != nil {
					return nil, err
				}
				return getMinlzWriter(w), nil
			},
		}
	default:
		return nil, errors.New("unsupported compression type")
	}
	return arch, nil
}

func NewWriterCompression(conn io.Writer, arch Compression) (*WriterCompression, error) {
	w, err := arch.OpenWriter(conn)
	if err != nil {
		return nil, err
	}
	ccon := &WriterCompression{
		w: w,
		// oneFunc: sync.OnceFunc(func() {
		// 	switch snW := w.(type) {
		// 	case *snappy.Writer:
		// 		putSnappyWriter(snW)
		// 	case *minlz.Writer:
		// 		putMinlzWriter(snW)
		// 	}
		// }),
	}
	if f, ok := w.(interface{ Flush() error }); ok {
		ccon.f = f
	}
	return ccon, nil
}
func NewReaderCompression(conn io.Reader, arch Compression) (*ReaderCompression, error) {
	r, err := arch.OpenReader(conn)
	if err != nil {
		return nil, err
	}
	ccon := &ReaderCompression{
		r: r,
		// oneFunc: sync.OnceFunc(func() {
		// 	switch snR := r.(type) {
		// 	case *snappy.Reader:
		// 		putSnappyReader(snR)
		// 	case *minlz.Reader:
		// 		putMinlzReader(snR)
		// 	}
		// }),
	}
	return ccon, nil
}
func NewCompressionConn(conn net.Conn, arch Compression) (net.Conn, error) {
	w, err := NewWriterCompression(conn, arch)
	if err != nil {
		return nil, err
	}
	r, err := NewReaderCompression(conn, arch)
	if err != nil {
		return nil, err
	}
	ccon := &CompressionConn{
		conn: conn,
		r:    r,
		w:    w,
	}
	return ccon, nil
}
func (obj *WriterCompression) Write(b []byte) (n int, err error) {
	n, err = obj.w.Write(b)
	if err != nil {
		return
	}
	if obj.f != nil {
		err = obj.f.Flush()
	}
	return
}

func (obj *WriterCompression) Close() error {
	if obj.oneFunc != nil {
		obj.oneFunc()
	}
	return nil
}

func (obj *ReaderCompression) Read(b []byte) (n int, err error) {
	return obj.r.Read(b)
}
func (obj *ReaderCompression) Close() error {
	if obj.oneFunc != nil {
		obj.oneFunc()
	}
	return nil
}

func (obj *CompressionConn) Read(b []byte) (n int, err error) {
	return obj.r.Read(b)
}
func (obj *CompressionConn) Write(b []byte) (n int, err error) {
	return obj.w.Write(b)
}
func (obj *CompressionConn) Close() error {
	obj.w.Close()
	obj.r.Close()
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
