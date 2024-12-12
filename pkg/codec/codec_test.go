package codec

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

type testCase struct {
	name    string
	reader  io.Reader
	wantMsg Message
	wantErr bool
}

func makePublish() *testCase {
	pub := Publish{
		Header:  Header{AckRequired: true},
		Path:    "/path/b",
		Payload: SlicePayload([]byte("abcd")),
		Props:   Props{"a": "a", "b": "b"},
	}

	buf := new(bytes.Buffer)
	pub.Encode(buf)
	return &testCase{
		name:    "publish",
		reader:  buf,
		wantMsg: &pub,
		wantErr: false,
	}
}

func makePubAck() *testCase {
	pub := PubAck{
		Header:  Header{AckRequired: true},
		Payload: SlicePayload([]byte("abcd")),
	}

	buf := new(bytes.Buffer)
	pub.Encode(buf)
	return &testCase{
		name:    "PubAck",
		reader:  buf,
		wantMsg: &pub,
		wantErr: false,
	}
}

func makePing() *testCase {
	ping := Ping{
		Header: Header{AckRequired: true},
	}

	buf := new(bytes.Buffer)
	ping.Encode(buf)
	return &testCase{
		name:    "ping",
		reader:  buf,
		wantMsg: &ping,
		wantErr: false,
	}
}

func makeConn() *testCase {
	con := Connect{
		Header:          Header{AckRequired: true},
		ProtocolName:    "Proto",
		ProtocolVersion: 2,
		CleanSession:    true,
		KeepAliveTimer:  100,
		ClientId:        "ClientId",
		Authorization:   "Authorization",
		ClientVersion:   "ClientVersion",
		OSType:          "OSType",
		Props:           Props{"a": "a", "b": "b"},
		AuthFlag:        true,
		ClientVerFlag:   true,
		OSFlag:          true,
	}

	buf := new(bytes.Buffer)
	con.Encode(buf)
	return &testCase{
		name:    "Connect",
		reader:  buf,
		wantMsg: &con,
		wantErr: false,
	}
}

func TestDecodeOneMessage(t *testing.T) {
	type args struct {
		r io.Reader
	}

	tests := []*testCase{
		makePublish(),
		makeConn(),
		makePubAck(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := DecodeOneMessage(tt.reader, SlicePayloadBuiler{})
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeOneMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("DecodeOneMessage() = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}
