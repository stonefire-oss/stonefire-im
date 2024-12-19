package codec

import (
	"bytes"
	"io"
)

type Publish struct {
	Header
	MessageId uint16
	Path      string
	Payload   Payload
	Props     Props
}

func (msg *Publish) Encode(w io.Writer) (err error) {
	buf := new(bytes.Buffer)

	setString(msg.Path, buf)
	if msg.Header.AckRequired {
		setUint16(msg.MessageId, buf)
	}
	msg.Props.Encode(buf)

	var pl int32 = 0
	if msg.Payload != nil {
		pl = int32(msg.Payload.Len())
	}
	if err = writeMessage(w, MsgPublish, &msg.Header, buf, pl); err != nil {
		return
	}

	if msg.Payload != nil {
		_, err = w.Write(msg.Payload.ReadOnlyData())
	}
	return
}

func (msg *Publish) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) (err error) {
	defer func() {
		err = recoverError(err, recover())
	}()

	msg.Header = hdr

	msg.Path = getString(r, &packetRemaining)
	if msg.Header.AckRequired {
		msg.MessageId = getUint16(r, &packetRemaining)
	}
	msg.Props = make(Props)
	msg.Props.Decode(r, &packetRemaining)

	if packetRemaining > 0 {
		payloadReader := &io.LimitedReader{R: r, N: int64(packetRemaining)}
		msg.Payload, err = builder.MakePayload(payloadReader, int(packetRemaining))
	}

	return
}

func (msg *Publish) String() string {
	return "Publish"
}

func (msg *Publish) GetPayload() Payload {
	return msg.Payload
}

func (msg *Publish) SetPayload(p Payload) {
	msg.Payload = p
}

type PubAck struct {
	Header
	MessageId uint16
	Status    Status
	Payload   Payload
}

func (msg *PubAck) Encode(w io.Writer) (err error) {
	buf := new(bytes.Buffer)

	setUint16(msg.MessageId, buf)

	msg.Status.Encode(buf)
	pl := int32(0)
	if msg.Payload != nil {
		pl = pl + int32(msg.Payload.Len())
	}

	if err = writeMessage(w, MsgPubAck, &msg.Header, buf, pl); err != nil {
		return
	}

	if msg.Payload != nil {
		_, err = w.Write(msg.Payload.ReadOnlyData())
	}
	return
}

func (msg *PubAck) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) (err error) {
	defer func() {
		err = recoverError(err, recover())
	}()

	msg.Header = hdr

	msg.MessageId = getUint16(r, &packetRemaining)
	msg.Status.Decode(r, &packetRemaining)

	if packetRemaining > 0 {
		payloadReader := &io.LimitedReader{R: r, N: int64(packetRemaining)}
		msg.Payload, err = builder.MakePayload(payloadReader, int(packetRemaining))
	}

	return
}

func (msg *PubAck) String() string {
	return "PubAck"
}

func (msg *PubAck) GetPayload() Payload {
	return msg.Payload
}

func (msg *PubAck) SetPayload(p Payload) {
	msg.Payload = p
}

type Connect struct {
	Header
	ProtocolName                    string
	ProtocolVersion                 uint8
	CleanSession                    bool
	KeepAliveTimer                  uint16
	ClientId                        string
	Authorization                   string
	ClientVersion                   string
	OSType                          string
	Props                           Props
	AuthFlag, ClientVerFlag, OSFlag bool
}

func (msg *Connect) Encode(w io.Writer) (err error) {

	buf := new(bytes.Buffer)

	flags := boolToByte(msg.OSFlag) << 4
	flags |= boolToByte(msg.ClientVerFlag) << 3
	flags |= boolToByte(msg.AuthFlag) << 2
	flags |= boolToByte(msg.CleanSession) << 1

	setString(msg.ProtocolName, buf)
	setUint8(msg.ProtocolVersion, buf)
	buf.WriteByte(flags)
	setUint16(msg.KeepAliveTimer, buf)
	setString(msg.ClientId, buf)
	if msg.AuthFlag {
		setString(msg.Authorization, buf)
	}
	if msg.ClientVerFlag {
		setString(msg.ClientVersion, buf)
	}
	if msg.OSFlag {
		setString(msg.OSType, buf)
	}
	msg.Props.Encode(buf)
	return writeMessage(w, MsgConnect, &msg.Header, buf, 0)
}

func (msg *Connect) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) (err error) {
	defer func() {
		err = recoverError(err, recover())
	}()

	protocolName := getString(r, &packetRemaining)
	protocolVersion := getUint8(r, &packetRemaining)
	flags := getUint8(r, &packetRemaining)
	keepAliveTimer := getUint16(r, &packetRemaining)
	clientId := getString(r, &packetRemaining)

	*msg = Connect{
		ProtocolName:    protocolName,
		ProtocolVersion: protocolVersion,
		OSFlag:          flags&0x10 > 0,
		ClientVerFlag:   flags&0x08 > 0,
		AuthFlag:        flags&0x04 > 0,
		CleanSession:    flags&0x02 > 0,
		KeepAliveTimer:  keepAliveTimer,
		ClientId:        clientId,
	}

	msg.Header = hdr
	msg.Props = make(Props)

	if msg.AuthFlag {
		msg.Authorization = getString(r, &packetRemaining)
	}
	if msg.ClientVerFlag {
		msg.ClientVersion = getString(r, &packetRemaining)
	}
	if msg.OSFlag {
		msg.OSType = getString(r, &packetRemaining)
	}
	return msg.Props.Decode(r, &packetRemaining)
}

func (msg *Connect) String() string {
	return "Connect"
}

type ConnAck struct {
	Header
	SessionPresent                            bool
	ReturnCode                                ReturnCode
	KeepAliveTimer                            uint16
	Domain                                    string
	AuthSchema                                string
	OptDomains                                string
	AuthSchemaFlag, DomainFlag, OptDomainFlag bool
}

func (msg *ConnAck) Encode(w io.Writer) (err error) {

	buf := new(bytes.Buffer)

	flags := boolToByte(msg.OptDomainFlag) << 3
	flags |= boolToByte(msg.DomainFlag) << 2
	flags |= boolToByte(msg.AuthSchemaFlag) << 1
	flags |= boolToByte(msg.SessionPresent)

	setUint8(flags, buf)
	setUint8(uint8(msg.ReturnCode), buf)
	setUint16(msg.KeepAliveTimer, buf)
	if msg.AuthSchemaFlag {
		setString(msg.AuthSchema, buf)
	}
	if msg.DomainFlag {
		setString(msg.Domain, buf)
	}
	if msg.OptDomainFlag {
		setString(msg.OptDomains, buf)
	}

	return writeMessage(w, MsgConnAck, &msg.Header, buf, 0)
}

func (msg *ConnAck) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) (err error) {
	defer func() {
		err = recoverError(err, recover())
	}()

	msg.Header = hdr

	flags := getUint8(r, &packetRemaining)
	returnCode := ReturnCode(getUint8(r, &packetRemaining))
	keepAliveTimer := getUint16(r, &packetRemaining)

	if !returnCode.IsValid() {
		return errBadReturnCode
	}

	*msg = ConnAck{
		Header:         hdr,
		ReturnCode:     returnCode,
		KeepAliveTimer: keepAliveTimer,
		SessionPresent: flags&0x01 > 0,
		AuthSchemaFlag: flags&0x02 > 0,
		DomainFlag:     flags&0x04 > 0,
		OptDomainFlag:  flags&0x08 > 0,
	}

	if msg.AuthSchemaFlag {
		msg.AuthSchema = getString(r, &packetRemaining)
	}
	if msg.DomainFlag {
		msg.Domain = getString(r, &packetRemaining)
	}
	if msg.OptDomainFlag {
		msg.OptDomains = getString(r, &packetRemaining)
	}

	return nil
}

func (msg *ConnAck) String() string {
	return "ConnAck"
}

type Ping struct {
	Header
}

func (msg *Ping) Encode(w io.Writer) error {
	return msg.Header.Encode(w, MsgPingReq, 0)
}

func (msg *Ping) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) error {
	if packetRemaining != 0 {
		return errMsgTooLong
	}
	msg.Header = hdr
	return nil
}

func (msg *Ping) String() string {
	return "Ping"
}

type PingAck struct {
	Header
}

func (msg *PingAck) Encode(w io.Writer) error {
	return msg.Header.Encode(w, MsgPingResp, 0)
}

func (msg *PingAck) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) error {
	if packetRemaining != 0 {
		return errMsgTooLong
	}
	msg.Header = hdr
	return nil
}

func (msg *PingAck) String() string {
	return "PingAck"
}

type Disconnect struct {
	Header
	ReasonCode uint8
}

func (msg *Disconnect) Encode(w io.Writer) error {
	buf := new(bytes.Buffer)
	setUint8(msg.ReasonCode, buf)
	return writeMessage(w, MsgDisconnect, &msg.Header, buf, 0)
}

func (msg *Disconnect) Decode(r io.Reader, hdr Header, packetRemaining int32, builder PayloadBuilder) error {
	msg.Header = hdr
	msg.ReasonCode = getUint8(r, &packetRemaining)
	if packetRemaining != 0 {
		return errMsgTooLong
	}
	return nil
}

func (msg *Disconnect) String() string {
	return "Disconnect"
}
