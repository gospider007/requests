package requests

import (
	"compress/flate"
	"io"
	"sync"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/minio/minlz"
)

type compreData struct {
	rpool      *sync.Pool
	wpool      *sync.Pool
	name       string
	openReader func(r io.Reader) (io.ReadCloser, error)
	openWriter func(w io.Writer) (io.WriteCloser, error)
}

var compressionData map[byte]compreData

func init() {
	compressionData = map[byte]compreData{
		40: {
			name:       "zstd",
			rpool:      &sync.Pool{New: func() any { return nil }},
			wpool:      &sync.Pool{New: func() any { return nil }},
			openReader: newZstdReader,
			openWriter: newZstdWriter,
		},
		255: {
			name:       "s2",
			rpool:      &sync.Pool{New: func() any { return nil }},
			wpool:      &sync.Pool{New: func() any { return nil }},
			openReader: newSnappyReader,
			openWriter: newSnappyWriter,
		},
		92: {
			name:       "flate",
			rpool:      &sync.Pool{New: func() any { return nil }},
			wpool:      &sync.Pool{New: func() any { return nil }},
			openReader: newFlateReader,
			openWriter: newFlateWriter,
		},
		93: {
			name:       "minlz",
			rpool:      &sync.Pool{New: func() any { return nil }},
			wpool:      &sync.Pool{New: func() any { return nil }},
			openReader: newMinlzReader,
			openWriter: newMinlzWriter,
		},
	}
}

func newZstdWriter(w io.Writer) (io.WriteCloser, error) {
	pool := compressionData[40].wpool
	cp := pool.Get()
	var z *zstd.Encoder
	var err error
	if cp == nil {
		z, err = zstd.NewWriter(w, zstd.WithWindowSize(32*1024))
	} else {
		z = cp.(*zstd.Encoder)
		z.Reset(w)
	}
	if err != nil {
		return nil, err
	}
	return newWriterCompression(z, func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}
func newZstdReader(w io.Reader) (io.ReadCloser, error) {
	pool := compressionData[40].rpool
	cp := pool.Get()
	var z *zstd.Decoder
	var err error
	if cp == nil {
		z, err = zstd.NewReader(w)
	} else {
		z = cp.(*zstd.Decoder)
		z.Reset(w)
	}
	if err != nil {
		return nil, err
	}
	return newReaderCompression(io.NopCloser(z), func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}

// snappy pool

func newSnappyWriter(w io.Writer) (io.WriteCloser, error) {
	pool := compressionData[255].wpool
	cp := pool.Get()
	var z *snappy.Writer
	if cp == nil {
		z = snappy.NewBufferedWriter(w)
	} else {
		z = cp.(*snappy.Writer)
		z.Reset(w)
	}
	return newWriterCompression(z, func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}
func newSnappyReader(w io.Reader) (io.ReadCloser, error) {
	pool := compressionData[255].rpool
	cp := pool.Get()
	var z *snappy.Reader
	if cp == nil {
		z = snappy.NewReader(w)
	} else {
		z = cp.(*snappy.Reader)
		z.Reset(w)
	}
	return newReaderCompression(io.NopCloser(z), func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}

// flate pool
func newFlateWriter(w io.Writer) (io.WriteCloser, error) {
	pool := compressionData[92].wpool
	cp := pool.Get()
	var z *flate.Writer
	var err error
	if cp == nil {
		z, err = flate.NewWriter(w, flate.DefaultCompression)
	} else {
		z = cp.(*flate.Writer)
		z.Reset(w)
	}
	if err != nil {
		return nil, err
	}
	return newWriterCompression(z, func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}

func newFlateReader(w io.Reader) (io.ReadCloser, error) {
	pool := compressionData[92].rpool
	cp := pool.Get()
	var z io.ReadCloser
	var f flate.Resetter
	if cp == nil {
		z = flate.NewReader(w)
		f = z.(flate.Resetter)
	} else {
		z = cp.(io.ReadCloser)
		f = z.(flate.Resetter)
		f.Reset(w, nil)
	}
	return newReaderCompression(z, func() {
		f.Reset(nil, nil)
		pool.Put(z)
	}), nil
}

// minlz pool

func newMinlzWriter(w io.Writer) (io.WriteCloser, error) {
	pool := compressionData[93].wpool
	cp := pool.Get()
	var z *minlz.Writer
	if cp == nil {
		z = minlz.NewWriter(w, minlz.WriterBlockSize(32*1024))
	} else {
		z = cp.(*minlz.Writer)
		z.Reset(w)
	}
	return newWriterCompression(z, func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}

func newMinlzReader(w io.Reader) (io.ReadCloser, error) {
	pool := compressionData[93].rpool
	cp := pool.Get()
	var z *minlz.Reader
	if cp == nil {
		z = minlz.NewReader(w, minlz.ReaderMaxBlockSize(32*1024))
	} else {
		z = cp.(*minlz.Reader)
		z.Reset(w)
	}
	return newReaderCompression(io.NopCloser(z), func() {
		z.Reset(nil)
		pool.Put(z)
	}), nil
}
