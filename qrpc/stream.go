package qrpc

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"github.com/stonefire-oss/stonefire-im/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type serverStream struct {
	mu     sync.Mutex
	s      quic.Stream
	ctx    context.Context
	fr     atomic.Pointer[codec.Publish]
	md     metadata.MD
	plmk   codec.PayloadBuilder
	quit   *utils.Event
	sd     *grpc.StreamDesc
	z      bool
	method string
}

func newServerStream(ctx context.Context, req *codec.Publish, stream quic.Stream, con *qrpcConn, sd *grpc.StreamDesc) grpc.ServerStream {
	ss := &serverStream{
		s:      stream,
		ctx:    ctx,
		method: req.Path,
		quit:   con.quit,
		md:     make(metadata.MD),
		z:      req.Compressed,
		plmk:   con.plmk,
		sd:     sd,
	}

	ss.fr.Store(req)
	return ss
}

func (ss *serverStream) SetHeader(md metadata.MD) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.md = metadata.Join(md)
	return nil
}

func (ss *serverStream) SendHeader(metadata.MD) error {
	return ss.SendMsg(nil)
}

func (ss *serverStream) SetTrailer(metadata.MD) {

}

func (ss *serverStream) Context() context.Context {
	return ss.ctx
}

func (ss *serverStream) SendMsg(m any) error {
	bf, err := EncodePayload(m, ss.z)
	if err != nil {
		return err
	}
	defer func() {
		if bf != nil {
			bf.Free()
		}
	}()

	ss.mu.Lock()
	pub := &codec.Publish{
		Header:  codec.Header{AckRequired: false, Compressed: ss.z},
		Payload: bf,
		Props:   codec.Props(ss.md),
	}
	if ss.md.Len() > 0 {
		ss.md = make(metadata.MD)
	}
	ss.mu.Unlock()

	return pub.Encode(ss.s)
}

func (ss *serverStream) RecvMsg(m any) error {
	fr := ss.fr.Swap(nil)
	if fr != nil {
		defer func() {
			if r, ok := fr.Payload.(interface {
				Release()
			}); ok {
				r.Release()
			}
			fr.Payload = nil
		}()
		return DecodePayload(m, fr.Payload, ss.z)
	}

	msg, err := codec.DecodeOneMessage(ss.s, ss.plmk)
	if err != nil {
		return err
	}

	switch vv := msg.(type) {
	case *codec.Publish:
		defer FreePayload(vv)
		return DecodePayload(m, vv.Payload, vv.Compressed)
	case codec.PayloadContainer:
		FreePayload(vv)
	default:

	}
	return nil
}
