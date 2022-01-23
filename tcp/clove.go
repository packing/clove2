package tcp

import (
	"bytes"
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

var ClovePacketFormat = &PacketFormat{
	"clove",
	0,
	"binary",
	false,
	&ClovePacketParser{},
	&ClovePacketPackager{},
	nil,
}

func (p *ClovePacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	defer func() {
		base.LogPanic(recover())
	}()

	if len(in) < PacketCloveHeaderLength {
		return errors.New(ErrorDataNotReady), nil, 0
	}

	peekData := in
	opFlag := int(peekData[0])
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
	packet.Compressed = base.TestMask(opFlag, MaskCloveCompressed)
	packet.Encrypted = base.TestMask(opFlag, MaskCloveEncrypt)
	packet.CompressSupport = base.TestMask(opFlag, MaskCloveCompressSupport)
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

		if err != nil {
			return err, pcks
		}

		if err.Error() == ErrorDataNotReady {
			return nil, pcks
		}

		pcks = append(pcks, pck)
	}

}

func (p *ClovePacketParser) TestMatchScore(b base.Buffer) int {
	defer func() {
		base.LogPanic(recover())
	}()

	score := -1

	l := b.Len()
	peekData, n := b.Peek(PacketCloveHeaderLength)
	if n < PacketCloveHeaderLength {
		return score
	}
	score = 0

	opFlag := int(peekData[0])
	mask := opFlag & MaskCloveFeature
	packetLen := int(binary.BigEndian.Uint32(peekData[1:PacketCloveHeaderLength]))
	ptop := byte((packetLen & 0xF0000000) >> 28)
	ptov := byte((packetLen & 0xF000000) >> 24)

	packetLen = packetLen & PacketMaxLength

	if ptop == 0 || ptov == 0 {
		return score
	}

	if packetLen > PacketMaxLength || packetLen < PacketCloveHeaderLength {
		return score
	}

	if mask != MaskCloveFeature {
		return score
	}

	if packetLen > l {
		score = 50
	} else {
		score = 99
	}

	return score
}

func (p *ClovePacketPackager) Package(dst Packet) (error, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()

	header := make([]byte, PacketCloveHeaderLength)

	if dst.GetType() != "binary" {
		return errors.New(ErrorPacketUnsupported), nil
	}

	pck, ok := dst.(*BinaryPacket)
	if !ok {
		return errors.New(ErrorPacketUnsupported), nil
	}

	var opFlag byte = 0
	if pck.Compressed {
		opFlag |= MaskCloveCompressed
	}
	if pck.CompressSupport {
		opFlag |= MaskCloveCompressSupport
	}
	if pck.Encrypted {
		opFlag |= MaskCloveEncrypt
	}

	header[0] = MaskCloveFeature | opFlag

	packetLen := uint32(len(pck.Raw)) + PacketCloveHeaderLength
	ptoInfo := uint32((pck.ProtocolType<<4)|pck.ProtocolVer) << 24
	packetLen |= ptoInfo
	binary.BigEndian.PutUint32(header[1:], packetLen)

	return nil, bytes.Join([][]byte{header, pck.Raw}, []byte(""))
}
