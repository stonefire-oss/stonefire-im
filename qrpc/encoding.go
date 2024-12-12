package qrpc

import (
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
)

var (
	C encoding.CodecV2 = encoding.GetCodecV2("proto")
)

func Unmarshal(pl []byte, v any) error {
	if pl == nil {
		return nil
	}
	out := mem.BufferSlice{mem.NewBuffer(&pl, nil)}
	defer func() {
		out.Free()
	}()
	return C.Unmarshal(out, v)
}
