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
		Header:    Header{AckRequired: true},
		Path:      "/path/b",
		MessageId: 1,
		Payload:   SlicePayload([]byte("abcd")),
		Props:     Props{"a": {"a"}, "b": {"b"}},
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
		Status:  Status{Code: 127, Message: "OK"},
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

func makePingAck() *testCase {
	ack := PingAck{
		Header: Header{AckRequired: false},
	}

	buf := new(bytes.Buffer)
	ack.Encode(buf)
	return &testCase{
		name:    "pingack",
		reader:  buf,
		wantMsg: &ack,
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
		Props:           Props{"a": {"a"}, "b": {"b"}},
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

func makeConnAck() *testCase {
	ack := ConnAck{
		Header:         Header{AckRequired: false},
		SessionPresent: true,
		ReturnCode:     ReturnCode(20),
		KeepAliveTimer: 100,
		Domain:         "china.com",
		AuthSchema:     "NTLM",
		OptDomains:     "opt1,opt2,opt3",
		DomainFlag:     true,
		AuthSchemaFlag: true,
		OptDomainFlag:  true,
	}
	buf := new(bytes.Buffer)
	ack.Encode(buf)
	return &testCase{
		name:    "ConnAck",
		reader:  buf,
		wantMsg: &ack,
		wantErr: false,
	}
}

func makeDiscon() *testCase {
	dis := Disconnect{
		Header:     Header{AckRequired: false},
		ReasonCode: 0,
	}
	buf := new(bytes.Buffer)
	dis.Encode(buf)
	return &testCase{
		name:    "DisConnect",
		reader:  buf,
		wantMsg: &dis,
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
		makePing(),
		makePingAck(),
		makeConnAck(),
		makeDiscon(),
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
