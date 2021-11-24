package tcp

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/packing/clove2/base"
)

const (
	PacketTypeBinary = "binary"
	PacketTypeText   = "text"
	PacketTypeHTTP   = "http"

	PacketMaxLength = 0xFFFFFF

	ErrorPacketUnsupported = "Package format not supported."
	ErrorDataNotReady      = "The data does not ready."
	ErrorDataNotMatch      = "The data does not match any supported format."
	ErrorDataIsDamage      = "The data length does not match."
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

type HTTPPacket struct {
	Request *http.Request
	Body    string
}

type PacketParser interface {
	//TryParse([]byte) (error, bool)
	//Prepare([]byte) (error, int, byte, byte, []byte)
	TestMatchScore(base.Buffer) int
	ParseFromBytes([]byte) (error, Packet, int)
	ParseFromBuffer(base.Buffer) (error, []Packet)
}

type PacketPackager interface {
	Package(Packet, []byte) (error, []byte)
}

type PacketProcessor interface {
	Encrypt([]byte) []byte
	Decrypt([]byte) []byte
	Compress([]byte) []byte
	UnCompress([]byte) []byte
}

type PacketFormat struct {
	Tag      string
	Priority int
	Type     string
	Parser   PacketParser
	Packager PacketPackager
}

func (b *BinaryPacket) GetType() string {
	return PacketTypeBinary
}

func (t *TextPacket) GetType() string {
	return PacketTypeText
}

func (t *HTTPPacket) GetType() string {
	return PacketTypeHTTP
}

type PacketFormatManager struct {
	mapFormats    map[string]*PacketFormat
	sortedFormats []*PacketFormat
}

var instFormatManager *PacketFormatManager
var onceForFormatManagerSingleton sync.Once

func GetPacketFormatManager() *PacketFormatManager {
	onceForFormatManagerSingleton.Do(func() {
		instFormatManager = &PacketFormatManager{}
		instFormatManager.mapFormats = make(map[string]*PacketFormat)
		instFormatManager.sortedFormats = make([]*PacketFormat, 0)
		instFormatManager.AddPacketFormat(&ClovePacketFormat)
		instFormatManager.AddPacketFormat(&HTTPPacketFormat)

		sort.Slice(instFormatManager.sortedFormats, func(i, j int) bool {
			return instFormatManager.sortedFormats[i].Priority < instFormatManager.sortedFormats[j].Priority
		})
	})
	return instFormatManager
}

func (f *PacketFormatManager) AddPacketFormat(format *PacketFormat) {
	f.mapFormats[strings.ToLower(format.Tag)] = format
	f.sortedFormats = append(f.sortedFormats, format)
}

func (f PacketFormatManager) FindPacketFormat(tag string) *PacketFormat {
	format, ok := f.mapFormats[strings.ToLower(tag)]
	if ok {
		return format
	}
	return nil
}

func (f PacketFormatManager) DetermineFromBuffer(buf base.Buffer) *PacketFormat {
	var scorePrev = -1
	var dstPf *PacketFormat
	for _, pf := range f.sortedFormats {
		score := pf.Parser.TestMatchScore(buf)
		if score == 100 {
			return pf
		}
		if score > scorePrev {
			dstPf = pf
		}
	}
	return dstPf
}
