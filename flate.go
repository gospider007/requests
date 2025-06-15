package requests

import (
	"compress/flate"
	"io"
	"sync"
)

var flateWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := flate.NewWriter(io.Discard, flate.DefaultCompression)
		return w
	},
}

func getFlateWriter(dst io.Writer) *flate.Writer {
	w := flateWriterPool.Get().(*flate.Writer)
	w.Reset(dst)
	return w
}

func putFlateWriter(w *flate.Writer) {
	// w.Close() // flush buffer
	w.Reset(io.Discard)
	flateWriterPool.Put(w)
}
