package tcp

import (
	"sync"

	"github.com/packing/clove2/base"
)

const (
	PacketTypeBinary = "binary"
	PacketTypeText   = "text"

	PacketMaxLength = 0xFFFFFF

	ErrorDataNotReady = "The data does not ready."
	ErrorDataNotMatch = "The data does not match any supported format."
	ErrorDataIsDamage = "The data length does not match."
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
	//TryParse([]byte) (error, bool)
	//Prepare([]byte) (error, int, byte, byte, []byte)
	ParseFromBytes([]byte) (error, Packet, int)
	ParseFromBuffer(base.Buffer) (error, []Packet)
}

type PacketPackager interface {
	Package([]byte) (error, Packet, []byte)
}

type PacketFormat struct {
	Tag      string
	Priority int
	Type     string
	Parser   PacketParser
	Packager PacketPackager
}

func (b BinaryPacket) GetType() string {
	return PacketTypeBinary
}

func (t TextPacket) GetType() string {
	return PacketTypeText
}

type PacketFormatManager struct {
	mapFormats map[string]*PacketFormat
}

var instFormatManager *PacketFormatManager
var onceForFormatManagerSingleton sync.Once

func GetPacketFormatManager() *PacketFormatManager {
	onceForFormatManagerSingleton.Do(func() {
		instFormatManager = &PacketFormatManager{}
		instFormatManager.AddPacketFormat(&ClovePacketFormat)
	})
	return instFormatManager
}

func (f *PacketFormatManager) AddPacketFormat(format *PacketFormat) {
	f.mapFormats[format.Tag] = format
}

func (f PacketFormatManager) FindPacketFormat(tag string) *PacketFormat {
	format, ok := f.mapFormats[tag]
	if ok {
		return format
	}
	return nil
}
