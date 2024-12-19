package codec

import (
	"bytes"
	"io"
)

const (
	// Maximum payload size in bytes (256MiB - 1B).
	MaxPayloadSize = (1 << (4 * 7)) - 1
)

// Header contains the common attributes of all messages. Some attributes are
// not applicable to some message types.
type Header struct {
	DupFlag                 bool
	Compressed, AckRequired bool
}

func (hdr *Header) Encode(w io.Writer, msgType MessageType, remainingLength int32) error {
	buf := new(bytes.Buffer)
	err := hdr.encodeInto(buf, msgType, remainingLength)
	if err != nil {
		return err
	}
	_, err = w.Write(buf.Bytes())
	return err
}

func (hdr *Header) encodeInto(buf *bytes.Buffer, msgType MessageType, remainingLength int32) error {
	if !msgType.IsValid() {
		return errBadMsgType
	}

	val := byte(msgType) << 4
	val |= (boolToByte(hdr.DupFlag) << 2)
	val |= (boolToByte(hdr.AckRequired) << 1)
	val |= boolToByte(hdr.Compressed)
	buf.WriteByte(val)
	encodeLength(remainingLength, buf)
	return nil
}

func (hdr *Header) Decode(r io.Reader) (msgType MessageType, remainingLength int32, err error) {
	defer func() {
		err = recoverError(err, recover())
	}()

	var buf [1]byte

	if _, err = io.ReadFull(r, buf[:]); err != nil {
		return
	}

	b := buf[0]
	msgType = MessageType(b & 0xF0 >> 4)

	*hdr = Header{
		DupFlag:     b&0x04 > 0,
		AckRequired: b&0x02 > 0,
		Compressed:  b&0x01 > 0,
	}

	remainingLength, _ = decodeLength(r)

	return
}

// MessageType constants.
const (
	MsgConnect = MessageType(iota + 1)
	MsgConnAck
	MsgPublish
	MsgPubAck
	MsgPingReq
	MsgPingResp
	MsgDisconnect

	msgTypeFirstInvalid
)

type Props map[string][]string

func (p Props) Encode(buf *bytes.Buffer) error {
	encodeLength(int32(len(p)), buf)
	if len(p) > 0 {
		for k, v := range p {
			setString(k, buf)
			lv := len(v)
			encodeLength(int32(lv), buf)
			for i := 0; i < lv; i++ {
				setString(v[i], buf)
			}
		}
	}
	return nil
}

func (p Props) Decode(r io.Reader, packetRemaining *int32) error {
	len, l := decodeLength(r)
	*packetRemaining = *packetRemaining - int32(l)

	for i := int32(0); i < len; i++ {
		k := getString(r, packetRemaining)
		vl, vll := decodeLength(r)
		*packetRemaining = *packetRemaining - int32(vll)
		va := make([]string, 0, vl)
		for i := 0; i < int(vl); i++ {
			v := getString(r, packetRemaining)
			va = append(va, v)
		}

		p[k] = va
	}
	return nil
}

type Status struct {
	Code    uint8
	Message string
}

func (p *Status) Encode(buf *bytes.Buffer) error {
	c := p.Code
	if len(p.Message) > 0 {
		c |= 0x80
		setUint8(c, buf)
		setString(p.Message, buf)
	} else {
		c &= 0x7F
		setUint8(c, buf)
	}
	return nil
}

func (p *Status) Decode(r io.Reader, packetRemaining *int32) error {
	c := getUint8(r, packetRemaining)

	if c&0x80 == 0x80 {
		p.Message = getString(r, packetRemaining)
	}
	p.Code = 0x7F & c
	return nil
}

// Message is the interface that all MQTT messages implement.
type Message interface {
	// Encode writes the message to w.
	Encode(w io.Writer) error

	// Decode reads the message extended headers and payload from
	// r. Typically the values for hdr and packetRemaining will
	// be returned from Header.Decode.
	Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) error
}

type PayloadContainer interface {
	GetPayload() Payload
	SetPayload(Payload)
}

type Payload interface {
	ReadOnlyData() []byte
	Len() int
}

type SlicePayload []byte

type SlicePayloadBuiler struct{}

func (b SlicePayloadBuiler) MakePayload(r io.Reader, l int) (Payload, error) {
	if l <= 0 {
		return nil, nil
	}
	bs := make(SlicePayload, l)
	_, err := io.ReadFull(r, bs)
	return bs, err
}

func (sp SlicePayload) Len() int {
	return len(sp)
}

func (sp SlicePayload) ReadOnlyData() []byte {
	return sp
}

type PayloadBuilder interface {
	MakePayload(io.Reader, int) (Payload, error)
}

type MessageType uint8

// IsValid returns true if the MessageType value is valid.
func (mt MessageType) IsValid() bool {
	return mt >= MsgConnect && mt < msgTypeFirstInvalid
}

func writeMessage(w io.Writer, msgType MessageType, hdr *Header, payloadBuf *bytes.Buffer, extraLength int32) error {
	totalPayloadLength := int64(len(payloadBuf.Bytes())) + int64(extraLength)
	if totalPayloadLength > MaxPayloadSize {
		return errMsgTooLong
	}

	buf := new(bytes.Buffer)
	err := hdr.encodeInto(buf, msgType, int32(totalPayloadLength))
	if err != nil {
		return err
	}

	buf.Write(payloadBuf.Bytes())
	_, err = w.Write(buf.Bytes())

	return err
}
