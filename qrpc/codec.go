package qrpc

import (
	"bytes"
	"compress/gzip"
	"io"

	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
)

func compressPayload(bs mem.BufferSlice) (mem.Buffer, error) {
	rs := []byte{0, 0}
	pl := mem.DefaultBufferPool()
	wb := make(mem.BufferSlice, 0)
	wr := mem.NewWriter(&wb, pl)
	wr.Write(rs)
	gw := gzip.NewWriter(wr)
	if _, err := io.Copy(gw, bs.Reader()); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return wb.MaterializeToBuffer(pl), nil
}

func decompressPayload(rp codec.Payload) (mem.BufferSlice, error) {
	rs := []byte{0, 0}
	pl := mem.DefaultBufferPool()
	wb := make(mem.BufferSlice, 0)
	wr := mem.NewWriter(&wb, pl)
	pr := bytes.NewReader(rp.ReadOnlyData())
	pr.Read(rs)
	zr, err := gzip.NewReader(pr)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(wr, zr); err != nil {
		return nil, err
	}
	return wb, nil
}

func DecodePayload(v any, pl codec.Payload, z bool) (err error) {
	if pl == nil || pl.Len() <= 0 {
		return nil
	}

	var out mem.BufferSlice
	if z {
		out, err = decompressPayload(pl)
	} else {
		pd := pl.ReadOnlyData()
		out = mem.BufferSlice{mem.NewBuffer(&pd, nil)}
	}

	defer func() {
		out.Free()
	}()
	c := encoding.GetCodecV2("proto")
	return c.Unmarshal(out, v)
}

func EncodePayload(v any, z bool) (mem.Buffer, error) {
	if v == nil {
		return nil, nil
	}

	c := encoding.GetCodecV2("proto")
	out, err := c.Marshal(v)

	if err != nil {
		return nil, err
	}

	defer func() {
		out.Free()
	}()

	if z {
		return compressPayload(out)
	}

	bf := out.MaterializeToBuffer(mem.DefaultBufferPool())
	return bf, nil
}

type pooledPLMaker struct {
	p mem.BufferPool
}

type pooledSlicePayload struct {
	codec.SlicePayload
	p mem.BufferPool
}

func (p *pooledSlicePayload) Release() {
	if p.p != nil {
		b := []byte(p.SlicePayload)
		p.p.Put(&b)
	}
}

func (p pooledPLMaker) MakePayload(r io.Reader, l int) (codec.Payload, error) {
	if p.p == nil || mem.IsBelowBufferPoolingThreshold(l) {
		b := make(codec.SlicePayload, l)
		io.ReadFull(r, b)
		return &pooledSlicePayload{SlicePayload: b}, nil
	}
	b := p.p.Get(l)
	io.ReadFull(r, *b)
	pl := pooledSlicePayload{SlicePayload: *b, p: p.p}
	return &pl, nil
}

func FreePayload(c codec.PayloadContainer) {
	if c == nil || c.GetPayload() == nil {
		return
	}

	pl := c.GetPayload()
	if r, ok := pl.(interface {
		Release()
	}); ok {
		r.Release()
	}
	c.SetPayload(nil)
}
