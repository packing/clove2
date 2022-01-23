package tcp

import (
	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

type TextPacketParser struct {
}

type TextPacketPackager struct {
}

var TextPacketFormat = &PacketFormat{
	"text",
	0xFF,
	"text",
	true,
	&TextPacketParser{},
	&TextPacketPackager{},
	nil,
}

func (p *TextPacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	defer func() {
		base.LogPanic(recover())
	}()
	packet := new(TextPacket)
	packet.Text = string(in)

	return nil, packet, len(in)
}

func (p *TextPacketParser) ParseFromBuffer(b base.Buffer) (error, []Packet) {
	defer func() {
		base.LogPanic(recover())
	}()
	in, _ := b.Next(b.Len())
	e, pck, _ := p.ParseFromBytes(in)
	return e, []Packet{pck}
}

func (p *TextPacketParser) TestMatchScore(b base.Buffer) int {
	defer func() {
		base.LogPanic(recover())
	}()

	return -1
}

func (p *TextPacketPackager) Package(dst Packet) (error, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()
	packetTxt, ok := dst.(*TextPacket)
	if !ok {
		return errors.New(ErrorPacketUnsupported), nil
	}

	return nil, []byte(packetTxt.Text)
}
