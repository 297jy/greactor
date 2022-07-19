package core

import (
	"greactor/src/core/icodecs"
	"time"
)

type Options struct {

	Multicore bool

	LB LoadBalancing
	// 编码解码器
	Codec icodecs.ICodec

	TCPKeepAlive time.Duration
}
