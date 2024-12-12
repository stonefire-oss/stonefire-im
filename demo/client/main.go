package main

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/demo/pb"
	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
	"google.golang.org/protobuf/proto"
)

const addr = "localhost:4242"

func main() {
	err := clientMain()
	if err != nil {
		fmt.Printf("%s", err)
		panic(err)
	}
}

func clientMain() error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, nil)
	if err != nil {
		return err
	}
	defer conn.CloseWithError(0, "")

	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	stu := pb.Student{
		Name:   "yemxinging",
		Male:   true,
		Scores: []int32{0, 1, 2},
	}
	c, err := proto.Marshal(&stu)
	if err != nil {
		return err
	}

	h := codec.Header{
		AckRequired: true,
	}
	psu := codec.Publish{
		Header:  h,
		Path:    "/pb.StudentService/CreateStudent",
		Payload: codec.SlicePayload(c),
	}
	psu.Encode(stream)
	msg, err := codec.DecodeOneMessage(stream, codec.SlicePayloadBuiler{})
	if ack, ok := msg.(*codec.PubAck); ok && ack.Payload != nil {
		sack := pb.Result{}

		err = unmarshal(ack.Payload.ReadOnlyData(), &sack)
		fmt.Printf("%V%n %V\n", sack, err)
	}
	return nil
}

func unmarshal(bs []byte, v any) error {
	c := encoding.GetCodecV2("proto")
	out := mem.BufferSlice{mem.NewBuffer(&bs, nil)}
	defer func() {
		out.Free()
	}()

	return c.Unmarshal(out, v)
}
