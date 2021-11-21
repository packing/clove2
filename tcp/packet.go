package tcp

import "github.com/packing/clove2/base"

const (
	PacketTypeBinary = "binary"
	PacketTypeText   = "text"
)

type Packet interface {
	GetType() string
}

type BinaryPacket struct {
	Encrypted       bool
	Compressed      bool
	ProtocolType    byte
	ProtocolVer     byte
	CompressSupport bool
	Raw             []byte
}

type TextPacket struct {
	Text string
}

type PacketParser interface {
	TryParse([]byte) (error, bool)
	Prepare([]byte) (error, int, byte, byte, []byte)
	ParseFromBytes([]byte) (error, *Packet, int)
	ParseFromBuffer(base.Buffer) (error, *Packet, int)
}

type PacketPackager interface {
	Package(*Packet, []byte) (error, []byte)
}

type PacketFormat struct {
	Tag      string
	Priority int
	UnixNeed bool
	Parser   PacketParser
	Packager PacketPackager
}

func (b BinaryPacket) GetType() string {
	return PacketTypeBinary
}

func (t TextPacket) GetType() string {
	return PacketTypeText
}
