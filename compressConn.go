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
	w    io.WriteCloser
	r    io.ReadCloser
	f    interface{ Flush() error }
}
type Compression interface {
	OpenReader(r io.Reader) (io.ReadCloser, error)
	OpenWriter(w io.Writer) (io.WriteCloser, error)
}
type compression struct {
	openReader func(r io.Reader) (io.ReadCloser, error)
	openWriter func(w io.Writer) (io.WriteCloser, error)
}

func (obj compression) OpenReader(r io.Reader) (io.ReadCloser, error) {
	return obj.openReader(r)
}
func (obj compression) OpenWriter(w io.Writer) (io.WriteCloser, error) {
	return obj.openWriter(w)
}

type CompressionLevel int

const (
	CompressionLevelFast CompressionLevel = 1
	CompressionLevelBest CompressionLevel = 2
)

func NewCompression(decode string, leval CompressionLevel) (Compression, error) {
	var arch Compression
	switch strings.ToLower(decode) {
	case "zstd":
		options := []zstd.EOption{}
		options2 := []zstd.DOption{}
		switch leval {
		case CompressionLevelFast:
			options = append(options, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithZeroFrames(true), zstd.WithLowerEncoderMem(true))
			options2 = append(options2, zstd.WithDecoderLowmem(true))
		case CompressionLevelBest:
			options = append(options, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
		default:
			options = append(options, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithZeroFrames(true), zstd.WithLowerEncoderMem(true))
			options2 = append(options2, zstd.WithDecoderLowmem(true))
		}
		arch = compression{
			openReader: func(r io.Reader) (io.ReadCloser, error) {
				decoder, err := zstd.NewReader(r, options2...)
				if err != nil {
					return nil, err
				}
				return decoder.IOReadCloser(), nil
			},
			openWriter: func(w io.Writer) (io.WriteCloser, error) {
				encoder, err := zstd.NewWriter(w, options...)
				if err != nil {
					return nil, err
				}
				return encoder, nil
			},
		}
	case "flate":
		arch = compression{
			openReader: func(r io.Reader) (io.ReadCloser, error) {
				buf := make([]byte, 1)
				n, err := r.Read(buf)
				if err != nil {
					return nil, err
				}
				if n != 1 || buf[0] != 92 {
					return nil, errors.New("invalid response")
				}
				return flate.NewReader(r), nil
			},
			openWriter: func(w io.Writer) (io.WriteCloser, error) {
				n, err := w.Write([]byte{92})
				if err != nil {
					return nil, err
				}
				if n != 1 {
					return nil, errors.New("invalid response")
				}
				return flate.NewWriter(w, flate.BestCompression)
			},
		}
	default:
		return nil, errors.New("unsupported compression type")
	}
	return arch, nil
}

func NewCompressionConn(conn net.Conn, arch Compression) (net.Conn, error) {
	w, err := arch.OpenWriter(conn)
	if err != nil {
		return nil, err
	}
	r, err := arch.OpenReader(conn)
	if err != nil {
		return nil, err
	}
	ccon := &CompressionConn{conn: conn, r: r, w: w}
	if f, ok := w.(interface{ Flush() error }); ok {
		ccon.f = f
	}
	return ccon, nil
}

func (obj *CompressionConn) Read(b []byte) (n int, err error) {
	return obj.r.Read(b)
}
func (obj *CompressionConn) Write(b []byte) (n int, err error) {
	n, err = obj.w.Write(b)
	if err != nil {
		return
	}
	if obj.f != nil {
		err = obj.f.Flush()
	}
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
