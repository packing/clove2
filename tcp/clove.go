package tcp

import (
	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

const (
	PacketCloveHeaderLength  = 5
	MaskClove                = 0x80
	MaskCloveCompressSupport = 0x1
	MaskCloveEncrypt         = 0x1 << 1
	MaskCloveCompressed      = 0x1 << 2
	MaskCloveReserved        = 0x1 << 3
	MaskCloveFeature         = MaskCloveReserved | MaskClove
)

type ClovePacketParser struct {
}

type ClovePacketPackager struct {
}

func (p *ClovePacketParser) ParseFromBytes(b []byte) (error, Packet, int) {
	return errors.New("The data is not ready."), nil, 0
}

func (p *ClovePacketParser) ParseFromBuffer(b base.Buffer) (error, Packet, int) {
	return errors.New("The data is not ready."), nil, 0
}
