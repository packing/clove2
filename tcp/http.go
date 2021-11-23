package tcp

import (
	"bytes"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

type HTTPPacketParser struct {
}

type HTTPPacketPackager struct {
}

var HTTPPacketFormat = PacketFormat{
	"clove",
	0,
	"binary",
	&HTTPPacketParser{},
	&HTTPPacketPackager{},
}

func (p *HTTPPacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	defer func() {
		base.LogPanic(recover())
	}()

	return nil, nil, 0
}

func (p *HTTPPacketParser) ParseFromBuffer(b base.Buffer) (error, []Packet) {
	defer func() {
		base.LogPanic(recover())
	}()

	return nil, nil

}

func (p *HTTPPacketParser) TestMatchScore(b base.Buffer) int {
	defer func() {
		base.LogPanic(recover())
	}()

	score := -1

	return score
}

func (p *HTTPPacketPackager) Package(dst Packet, in []byte) (error, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()

	if dst.GetType() != "http" {
		return errors.New(ErrorPacketUnsupported), in
	}

	header := make([]byte, 0)

	return nil, bytes.Join([][]byte{header, in}, []byte(""))
}
