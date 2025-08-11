package requests

import (
	"compress/flate"
	"io"
	"sync"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/minio/minlz"
)

// zstd pool
var zstdWriterPool = sync.Pool{
	New: func() any {
		c, _ := zstd.NewWriter(nil, zstd.WithWindowSize(32*1024))
		return c
	},
}

func newZstdWriter(w io.Writer) (io.WriteCloser, error) {
	z := zstdWriterPool.Get().(*zstd.Encoder)
	z.Reset(w)
	return newWriterCompression(z, func() {
		z.Reset(nil)
		zstdWriterPool.Put(z)
	}), nil
}

var zstdReaderPool = sync.Pool{
	New: func() any {
		w, _ := zstd.NewReader(nil)
		return w
	},
}

func newZstdReader(w io.Reader) (io.ReadCloser, error) {
	z := zstdReaderPool.Get().(*zstd.Decoder)
	z.Reset(w)
	return newReaderCompression(io.NopCloser(z), func() {
		z.Reset(nil)
		zstdReaderPool.Put(z)
	}), nil
}

// snappy pool
var snappyWriterPool = sync.Pool{
	New: func() any {
		return snappy.NewBufferedWriter(nil)
	},
}

func newSnappyWriter(w io.Writer) (io.WriteCloser, error) {
	s := snappyWriterPool.Get().(*snappy.Writer)
	s.Reset(w)
	return newWriterCompression(s, func() {
		s.Reset(nil)
		snappyWriterPool.Put(s)
	}), nil
}

var snappyReaderPool = sync.Pool{
	New: func() any {
		return snappy.NewReader(nil)
	},
}

func newSnappyReader(w io.Reader) (io.ReadCloser, error) {
	s := snappyReaderPool.Get().(*snappy.Reader)
	s.Reset(w)
	return newReaderCompression(io.NopCloser(s), func() {
		s.Reset(nil)
		snappyReaderPool.Put(s)
	}), nil
}

// flate pool
var flateWriterPool = sync.Pool{
	New: func() any {
		w, _ := flate.NewWriter(nil, flate.DefaultCompression)
		return w
	},
}

func newFlateWriter(w io.Writer) (io.WriteCloser, error) {
	f := flateWriterPool.Get().(*flate.Writer)
	f.Reset(w)
	return newWriterCompression(f, func() {
		f.Reset(nil)
		flateWriterPool.Put(f)
	}), nil
}

var flateReaderPool = sync.Pool{
	New: func() any {
		return flate.NewReader(nil)
	},
}

func newFlateReader(w io.Reader) (io.ReadCloser, error) {
	r := flateReaderPool.Get().(io.ReadCloser)
	f := r.(flate.Resetter)
	err := f.Reset(w, nil)
	return newReaderCompression(r, func() {
		f.Reset(nil, nil)
		flateReaderPool.Put(r)
	}), err
}

// minlz pool
var minlzWriterPool = sync.Pool{
	New: func() any {
		return minlz.NewWriter(nil, minlz.WriterBlockSize(32*1024))
	},
}

func newMinlzWriter(w io.Writer) (io.WriteCloser, error) {
	m := minlzWriterPool.Get().(*minlz.Writer)
	m.Reset(w)
	return newWriterCompression(m, func() {
		m.Reset(nil)
		minlzWriterPool.Put(m)
	}), nil
}

var minlzReaderPool = sync.Pool{
	New: func() any {
		return minlz.NewReader(nil, minlz.ReaderMaxBlockSize(32*1024))
	},
}

func newMinlzReader(w io.Reader) (io.ReadCloser, error) {
	m := minlzReaderPool.Get().(*minlz.Reader)
	m.Reset(w)
	return newReaderCompression(io.NopCloser(m), func() {
		m.Reset(nil)
		minlzReaderPool.Put(m)
	}), nil
}
