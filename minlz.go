package requests

import (
	"io"

	"github.com/minio/minlz"
)

// // 定义 minlz.Writer 池，包装 io.Writer
// var minlzWriterPool = sync.Pool{
// 	New: func() interface{} {
// 		return minlz.NewWriter(nil, minlz.WriterBlockSize(64*1024), minlz.WriterConcurrency(1), minlz.WriterLevel(minlz.LevelSmallest))
// 	},
// }

// // 定义 minlz.Reader 池，包装 io.Reader
// var minlzReaderPool = sync.Pool{
// 	New: func() interface{} {
// 		// 先给一个空 reader，后面可以Reset替换输入来源
// 		return minlz.NewReader(nil, minlz.ReaderMaxBlockSize(64*1024))
// 	},
// }

// 获取并初始化 minlz.Writer
func getMinlzWriter(w io.Writer) *minlz.Writer {
	return minlz.NewWriter(w, minlz.WriterBlockSize(64*1024), minlz.WriterConcurrency(1))
	// sw := minlzWriterPool.Get().(*minlz.Writer)
	// sw.Reset(w)
	// return sw
}

// 获取并初始化 minlz.Reader
func getMinlzReader(r io.Reader) *minlz.Reader {
	// sr := minlzReaderPool.Get().(*minlz.Reader)
	// sr.Reset(r)
	// return sr
	return minlz.NewReader(r, minlz.ReaderMaxBlockSize(64*1024))
}
