package requests

import (
	"io"

	"github.com/golang/snappy"
)

// // 定义 snappy.Writer 池，包装 io.Writer
// var snappyWriterPool = sync.Pool{
// 	New: func() interface{} {
// 		// 先给一个空 buffer，后面可以Reset替换输出目标
// 		return snappy.NewBufferedWriter(nil)
// 	},
// }

// // 定义 snappy.Reader 池，包装 io.Reader
// var snappyReaderPool = sync.Pool{
// 	New: func() interface{} {
// 		// 先给一个空 reader，后面可以Reset替换输入来源
// 		return snappy.NewReader(nil)
// 	},
// }

// 获取并初始化 snappy.Writer
func getSnappyWriter(w io.Writer) *snappy.Writer {
	return snappy.NewBufferedWriter(w)
	// sw := snappyWriterPool.Get().(*snappy.Writer)
	// sw.Reset(w)
	// return sw
}

// 获取并初始化 snappy.Reader
func getSnappyReader(r io.Reader) *snappy.Reader {
	return snappy.NewReader(r)
	// sr := snappyReaderPool.Get().(*snappy.Reader)
	// sr.Reset(r)
	// return sr
}

// // 释放 snappy.Writer
// func putSnappyWriter(sw *snappy.Writer) {
// 	sw.Close()
// 	sw.Reset(nil)
// 	snappyWriterPool.Put(sw)
// }

// // 释放 snappy.Reader
// func putSnappyReader(sr *snappy.Reader) {
// 	sr.Reset(nil)
// 	snappyReaderPool.Put(sr)
// }
