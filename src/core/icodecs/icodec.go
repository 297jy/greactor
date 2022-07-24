package icodecs

type (
	// 编码解码器接口
	ICodec interface {
		// 对报文进行编码
		Encode(buf []byte) ([]byte, error)
		// 对报文进行解码
		Decode(buf []byte) ([]byte, error)
	}

	// 默认内置的编码解码器
	BuiltInFrameCodec struct{}
)

func (cc *BuiltInFrameCodec) Encode(buf []byte) ([]byte, error) {
	return buf, nil
}

func (cc *BuiltInFrameCodec) Decode(buf []byte) ([]byte, error) {
	return buf, nil
}
