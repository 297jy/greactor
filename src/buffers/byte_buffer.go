package buffers

import (
	"sync"
)

type ByteBuffer struct {
	B []byte
}

func (b *ByteBuffer) Bytes() (bs []byte) {
	return b.B
}

func (b *ByteBuffer) ShiftN(n int) {
	b.B = b.B[n:]
}

func (b *ByteBuffer) Append(bs []byte) {
	b.B = append(b.B, bs...)
}

func (b *ByteBuffer) IsEmpty() bool {
	return b.Len() == 0
}
func (b *ByteBuffer) IsNotEmpty() bool {
	return !b.IsEmpty()
}
func (b *ByteBuffer) Len() int {
	return len(b.B)
}

func (b *ByteBuffer) Reset() {
	b.B = b.B[:0]
}

var byteBufferPool = sync.Pool{New: func() interface{} { return new(ByteBuffer) }}

func GetByteBuffer() *ByteBuffer {
	return byteBufferPool.Get().(*ByteBuffer)
}

func PutByteBuffer(bb *ByteBuffer) {
	if bb == nil {
		return
	}
	bb.B = bb.B[:0]
	byteBufferPool.Put(bb)
}
