package utils

import (
	"io"

	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"google.golang.org/grpc/mem"
)

type PooledPLMaker struct {
	p mem.BufferPool
}

type PooledSlicePayload struct {
	codec.SlicePayload
	p mem.BufferPool
}

func NewPooledPlMaker(p mem.BufferPool) *PooledPLMaker {
	return &PooledPLMaker{p}
}

func (p *PooledSlicePayload) Release() {
	if p.p != nil {
		b := []byte(p.SlicePayload)
		p.p.Put(&b)
	}
}

func (p PooledPLMaker) MakePayload(r io.Reader, l int) (codec.Payload, error) {
	var b *[]byte = nil
	if p.p == nil || !mem.IsBelowBufferPoolingThreshold(l) {
		*b = make(codec.SlicePayload, l)
	}
	b = p.p.Get(l)
	r.Read(*b)
	pl := PooledSlicePayload{SlicePayload: *b, p: p.p}
	return &pl, nil
}
