package codec

import (
	"errors"
	"io"
)

var (
	errBadMsgType        = errors.New("codec: message type is invalid")
	errBadLengthEncoding = errors.New("codec: remaining length field exceeded maximum of 4 bytes")
	errBadReturnCode     = errors.New("codec: is invalid")
	errDataExceedsPacket = errors.New("codec: data exceeds packet length")
	errMsgTooLong        = errors.New("codec: message is too long")
)

const (
	QosAtMostOnce = QosLevel(iota)
	QosAtLeastOnce

	qosFirstInvalid
)

type QosLevel uint8

type ReturnCode uint8

func (rc ReturnCode) IsValid() bool {
	return true
}

func (qos QosLevel) IsValid() bool {
	return qos < qosFirstInvalid
}

func (qos QosLevel) HasId() bool {
	return qos == QosAtLeastOnce
}

// DecodeOneMessage decodes one message from r. config provides specifics on
// how to decode messages, nil indicates that the DefaultDecoderConfig should
// be used.
func DecodeOneMessage(r io.Reader, builder PayloadBuilder) (msg Message, err error) {
	var hdr Header
	var msgType MessageType
	var packetRemaining int32
	msgType, packetRemaining, err = hdr.Decode(r)
	if err != nil {
		return
	}

	msg, err = newMessage(msgType)
	if err != nil {
		return
	}

	return msg, msg.Decode(r, hdr, packetRemaining, builder)
}

// NewMessage creates an instance of a Message value for the given message
// type. An error is returned if msgType is invalid.
func newMessage(msgType MessageType) (msg Message, err error) {
	switch msgType {
	case MsgConnect:
		msg = new(Connect)
	case MsgConnAck:
		msg = new(ConnAck)
	case MsgPublish:
		msg = new(Publish)
	case MsgPubAck:
		msg = new(PubAck)
	case MsgPingReq:
		msg = new(Ping)
	case MsgPingResp:
		msg = new(PingAck)
	case MsgDisconnect:
		msg = new(Disconnect)
	default:
		return nil, errBadMsgType
	}

	return
}

// panicErr wraps an error that caused a problem that needs to bail out of the
// API, such that errors can be recovered and returned as errors from the
// public API.
type panicErr struct {
	err error
}

func (p panicErr) Error() string {
	return p.err.Error()
}

func raiseError(err error) {
	panic(panicErr{err})
}

// recoverError recovers any panic in flight and, iff it's an error from
// raiseError, will return the error. Otherwise re-raises the panic value.
// If no panic is in flight, it returns existingErr.
//
// This must be used in combination with a defer in all public API entry
// points where raiseError could be called.
func recoverError(existingErr error, recovered interface{}) error {
	if recovered != nil {
		if pErr, ok := recovered.(panicErr); ok {
			return pErr.err
		} else {
			panic(recovered)
		}
	}
	return existingErr
}
