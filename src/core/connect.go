package core

type Conn interface {

	Read() (buf []byte)

	// ResetBuffer resets the buffers, which means all data in inbound ring-buffer and events-loop-buffer will be evicted.
	ResetBuffer()

}