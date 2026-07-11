package logger

import "sync"

// bufferPool defines a concurrent safe free list of byte slices used to provide
// temporary buffers for formatting log messages prior to outputting them.
var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 120)
		return &b // pointer to slice to avoid boxing alloc
	},
}

// buffer returns a byte slice from the free list.  A new buffer is allocated if
// there are not any available on the free list.  The returned byte slice should
// be returned to the fee list by using the recycleBuffer function when the
// caller is done with it.
func buffer() *[]byte {
	return bufferPool.Get().(*[]byte)
}

// recycleBuffer puts the provided byte slice, which should have been obtain via
// the buffer function, back on the free list.
func recycleBuffer(b *[]byte) {
	*b = (*b)[:0]
	bufferPool.Put(b)
}
