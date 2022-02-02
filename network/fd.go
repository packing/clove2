package network

import (
	"github.com/packing/clove2/base"
)

type FdPacketParser struct {
}

type FdPacketPackager struct {
}

var FdPacketFormat = &PacketFormat{
	"fd",
	0,
	"unixmsg",
	false,
	&FdPacketParser{},
	&FdPacketPackager{},
	nil,
}

func (p *FdPacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	return nil, nil, 0
}

func (p *FdPacketParser) ParseFromBuffer(b base.Buffer) (error, []Packet) {
	return nil, nil
}

func (p *FdPacketParser) TestMatchScore(b base.Buffer) int {
	return -1
}

func (p *FdPacketPackager) Package(dst Packet) (error, []byte) {
	return nil, nil
}
