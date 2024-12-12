package qrpc

import (
	"context"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
)

type serverStream struct {
	s      quic.Stream
	ctx    context.Context
	fr     *codec.Publish
	c      encoding.CodecV2
	method string
}

func newServerStream(ctx context.Context, req *codec.Publish, stream quic.Stream) grpc.ServerStream {
	return &serverStream{
		s:      stream,
		ctx:    ctx,
		method: req.Path,
		fr:     req,
	}
}

func (ss *serverStream) SetHeader(md metadata.MD) error {
	return nil
}

func (ss *serverStream) SendHeader(metadata.MD) error {
	return nil
}

func (ss *serverStream) SetTrailer(metadata.MD) {

}

func (ss *serverStream) Context() context.Context {
	return ss.ctx
}

func (ss *serverStream) SendMsg(m any) error {
	return nil
}

func (ss *serverStream) RecvMsg(m any) error {
	return nil
}
