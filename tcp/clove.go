package tcp

import (
	"encoding/binary"

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

var ClovePacketFormat = PacketFormat{
	"clove",
	0,
	"binary",
	&ClovePacketParser{},
	&ClovePacketPackager{},
}

func (p *ClovePacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	defer func() {
		base.LogPanic(recover())
	}()

	if len(in) < PacketCloveHeaderLength {
		return errors.New(ErrorDataNotReady), nil, 0
	}

	peekData := in
	opFlag := peekData[0]
	mask := opFlag & MaskCloveFeature
	packetLen := binary.BigEndian.Uint32(peekData[1:PacketCloveHeaderLength])
	ptop := byte((packetLen & 0xF0000000) >> 28)
	ptov := byte((packetLen & 0xF000000) >> 24)
	packetLen = packetLen & PacketMaxLength

	if ptop == 0 || ptov == 0 {
		return errors.New(ErrorDataNotMatch), nil, 0
	}

	if packetLen > PacketMaxLength || packetLen < PacketCloveHeaderLength {
		return errors.New(ErrorDataIsDamage), nil, 0
	}

	if mask != MaskCloveFeature {
		return errors.New(ErrorDataNotMatch), nil, 0
	}

	if uint(packetLen) > uint(len(in)) {
		return errors.New(ErrorDataNotReady), nil, 0
	}

	packet := new(BinaryPacket)
	packet.Compressed = (opFlag & MaskCloveCompressed) == MaskCloveCompressed
	packet.Encrypted = (opFlag & MaskCloveEncrypt) == MaskCloveEncrypt
	packet.CompressSupport = (opFlag & MaskCloveCompressSupport) == MaskCloveCompressSupport
	packet.ProtocolType = ptop
	packet.ProtocolVer = ptov
	packet.Raw = peekData[PacketCloveHeaderLength:packetLen]

	return nil, packet, int(packetLen)
}

func (p *ClovePacketParser) ParseFromBuffer(b base.Buffer) (error, []Packet) {
	defer func() {
		base.LogPanic(recover())
	}()

	var pcks = make([]Packet, 0)

	for {
		l := b.Len()
		peekData, n := b.Peek(PacketCloveHeaderLength)
		if n < PacketCloveHeaderLength {
			return nil, pcks
		}

		packetLen := int(binary.BigEndian.Uint32(peekData[1:PacketCloveHeaderLength]))
		packetLen = packetLen & PacketMaxLength

		if packetLen > l {
			return errors.New(ErrorDataIsDamage), pcks
		}

		in, n := b.Next(packetLen)
		if n != packetLen {
			return errors.New(ErrorDataIsDamage), pcks
		}

		err, pck, _ := p.ParseFromBytes(in)
		if err.Error() == ErrorDataNotReady {
			return nil, pcks
		}

		pcks = append(pcks, pck)
	}

}

func (p *ClovePacketPackager) Package(in []byte) (error, Packet, []byte) {

	return nil, nil, nil
}
