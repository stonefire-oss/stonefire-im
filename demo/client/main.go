package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/stonefire-oss/stonefire-im/demo/pb"
	"github.com/stonefire-oss/stonefire-im/pkg/codec"
	"github.com/stonefire-oss/stonefire-im/qrpc"
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
	uany(stream)

	if stream2, err := conn.OpenStreamSync(context.Background()); err != nil {
		return err
	} else {
		testStream(stream2)
	}
	return err
}

func testStream(stream quic.Stream) {
	defer stream.Close()

	var (
		wg sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		write(stream, &wg)
	}()
	wg.Add(1)
	go func() {
		read(stream, &wg)
	}()

	wg.Wait()
}

func read(stream quic.Stream, wg *sync.WaitGroup) error {
	defer func() {
		wg.Done()
	}()
	for {
		msg, err := codec.DecodeOneMessage(stream, codec.SlicePayloadBuiler{})
		if err != nil {
			fmt.Printf("%v\n", err)
			return err
		}

		if ack, ok := msg.(*codec.Publish); ok && ack.Payload != nil {
			sack := pb.Echo{}
			qrpc.DecodePayload(&sack, ack.Payload, ack.Compressed)
			qrpc.FreePayload(ack)
			fmt.Printf("%s%n %V\n", sack.Name, err)
		}
	}
}

func write(stream quic.Stream, wg *sync.WaitGroup) error {
	defer func() {
		wg.Done()
	}()

	stu := pb.Student{
		Male:   true,
		Scores: []int32{0, 1, 2},
	}

	for i := 0; i < 10; i++ {
		stu.Name = fmt.Sprintf("%s %d", "helloword", i)
		c, err := qrpc.EncodePayload(&stu, true)
		if err != nil {
			return err
		}
		h := codec.Header{
			AckRequired: true,
			Compressed:  true,
		}
		psu := codec.Publish{
			Header:  h,
			Path:    "/pb.StudentService/Hello",
			Payload: c,
		}
		psu.Encode(stream)
		c.Free()
	}
	return nil
}

func uany(stream quic.Stream) error {
	stu := pb.Student{
		Name:   "hello",
		Male:   true,
		Scores: []int32{0, 1, 2},
	}
	defer stream.Close()
	c, err := proto.Marshal(&stu)
	if err != nil {
		return err
	}

	h := codec.Header{
		AckRequired: true,
	}
	psu := codec.Publish{
		Header:  h,
		Path:    "/pb.StudentService/CreateStudents",
		Payload: codec.SlicePayload(c),
	}
	psu.Encode(stream)
	msg, err := codec.DecodeOneMessage(stream, codec.SlicePayloadBuiler{})
	if ack, ok := msg.(*codec.PubAck); ok {
		if ack.Payload != nil {
			sack := pb.Result{}
			qrpc.DecodePayload(&sack, ack.Payload, ack.Compressed)
			qrpc.FreePayload(ack)
			fmt.Printf("%s%n %V\n", sack.Message, err)
		} else {
			if ack.Status.Code != 0 {
				fmt.Printf("%d\n", ack.Status.Code)
			}
		}

	}
	if err != nil {
		fmt.Printf("%V\n", err)
	}
	return nil
}
