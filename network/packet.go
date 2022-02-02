package network

import (
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/packing/clove2/base"
)

const (
	PacketTypeBinary  = "binary"
	PacketTypeText    = "text"
	PacketTypeHTTP    = "http"
	PacketTypePing    = "ping"
	PacketTypeUnixMsg = "unixmsg"

	PacketMaxLength = 0xFFFFFF

	ErrorPacketUnsupported   = "Package format not supported"
	ErrorDataNotReady        = "The data does not ready"
	ErrorDataNotMatch        = "The data does not match any supported format"
	ErrorDataIsDamage        = "The data length does not match"
	ErrorClientReqDisconnect = "The client requested to disconnect"
)

type Packet interface {
	GetType() string
}

type BinaryPacket struct {
	From            string
	Encrypted       bool
	Compressed      bool
	ProtocolType    byte
	ProtocolVer     byte
	CompressSupport bool
	Raw             []byte
}

type TextPacket struct {
	From string
	Text string
}

type HTTPPacket struct {
	Request        *http.Request
	ResponseHeader http.Header
	Body           []byte
	StatusCode     int
	StatusText     string
	HTTPVer        string
}

type PingPacket struct {
	Data []byte
}

type UnixMsgPacket struct {
	Addr string
	B    []byte
	OOB  []byte
}

type PacketParser interface {
	TestMatchScore(base.Buffer) int
	ParseFromBytes([]byte) (error, Packet, int)
	ParseFromBuffer(base.Buffer) (error, []Packet)
}

type PacketPackager interface {
	Package(Packet) (error, []byte)
}

type PacketPreprocessor interface {
	HandshakeAck(base.Buffer) (error, []byte)
	Farewell() []byte
}

type PacketProcessor interface {
	Encrypt([]byte) []byte
	Decrypt([]byte) []byte
	Compress([]byte) []byte
	UnCompress([]byte) []byte
}

type PacketFormat struct {
	Tag          string
	Priority     uint32
	Type         string
	Default      bool
	Parser       PacketParser
	Packager     PacketPackager
	Preprocessor PacketPreprocessor
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

func (t *PingPacket) GetType() string {
	return PacketTypePing
}

func (t *UnixMsgPacket) GetType() string {
	return PacketTypeUnixMsg
}

type PacketFormatManager struct {
	mapFormats    map[string]*PacketFormat
	sortedFormats []*PacketFormat
	filters       []string
}

var instFormatManager *PacketFormatManager
var onceForFormatManagerSingleton sync.Once

func GetPacketFormatManager() *PacketFormatManager {
	onceForFormatManagerSingleton.Do(func() {
		instFormatManager = &PacketFormatManager{}
		instFormatManager.mapFormats = make(map[string]*PacketFormat)
		instFormatManager.sortedFormats = make([]*PacketFormat, 0)
		instFormatManager.AddPacketFormat(ClovePacketFormat)
		instFormatManager.AddPacketFormat(HTTPPacketFormat)
		instFormatManager.AddPacketFormat(TextPacketFormat)
		instFormatManager.AddPacketFormat(WSBPacketFormat)
		instFormatManager.AddPacketFormat(FdPacketFormat)

		sort.Slice(instFormatManager.sortedFormats, func(i, j int) bool {
			return instFormatManager.sortedFormats[i].Priority < instFormatManager.sortedFormats[j].Priority
		})
	})
	return instFormatManager
}

func NewPacketFormatManager(formats ...string) *PacketFormatManager {
	inst := new(PacketFormatManager)
	inst.mapFormats = make(map[string]*PacketFormat)
	inst.sortedFormats = make([]*PacketFormat, 0)
	for _, tag := range formats {
		format := GetPacketFormatManager().FindPacketFormat(tag)
		if format != nil {
			inst.AddPacketFormat(format)
		} else {
			base.LogError("PacketFormat '%s' not found", strings.ToLower(tag))
		}
	}
	return inst
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
	var defPf *PacketFormat
	for _, pf := range f.sortedFormats {
		if pf.Type == PacketTypeUnixMsg {
			continue
		}
		if pf.Default {
			defPf = pf
			continue
		}
		score := pf.Parser.TestMatchScore(buf)
		if score == 100 {
			return pf
		}
		if score > scorePrev {
			dstPf = pf
			scorePrev = score
		}
	}
	if scorePrev < 0 {
		return defPf
	} else if scorePrev >= 90 {
		return dstPf
	}
	return nil
}
