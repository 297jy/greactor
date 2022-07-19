package icodecs

import (
	"greactor/src/core"
	"greactor/src/errors"
)

type (
	// 编码解码器接口
	ICodec interface {
		// 对报文进行编码
		Encode(c core.Conn, buf []byte) ([]byte, error)
		// 对报文进行解码
		Decode(c core.Conn) ([]byte, error)
	}

	// 默认内置的编码解码器
	BuiltInFrameCodec struct{}
)


func (cc *BuiltInFrameCodec) Encode(c core.Conn, buf []byte) ([]byte, error) {
	return buf, nil
}


func (cc *BuiltInFrameCodec) Decode(c core.Conn) ([]byte, error) {
	buf := c.Read()
	if len(buf) == 0 {
		return nil, errors.ErrIncompletePacket
	}
	c.ResetBuffer()
	return buf, nil
}