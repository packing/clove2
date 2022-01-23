package tcp

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

const WSDataMinLength = 2
const WSMagicStr = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
const WSRespFmt = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\nSec-WebSocket-Protocol: %s\r\n\r\n"
const WSRespFmtWithoutProtocol = "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n"

type WebsocketAccessController struct {
	origin []string
}

func (w *WebsocketAccessController) AddOrigin(origin ...string) {
	w.origin = append(w.origin, origin...)
}

func (w WebsocketAccessController) CheckOrigin(origin string) bool {
	if len(w.origin) == 0 {
		return true
	}
	for _, o := range w.origin {
		if o == origin {
			return true
		}
	}
	return false
}

var instWebsocketAccessController *WebsocketAccessController
var onceForWebsocketAccessControllerSingleton sync.Once

func GetWebsocketAccessController() *WebsocketAccessController {
	onceForWebsocketAccessControllerSingleton.Do(func() {
		instWebsocketAccessController = &WebsocketAccessController{}
		instWebsocketAccessController.origin = make([]string, 0)
	})
	return instWebsocketAccessController
}

type WSPacketPreprocessor struct {
}

func (w WSPacketPreprocessor) HandshakeAck(b base.Buffer) (error, []byte) {

	peekData, n := b.Peek(b.Len())
	if n < HttpHeaderMinLength {
		return errors.New(ErrorDataNotReady), nil
	}

	fB := base.ReadAsciiCode(peekData)
	if fB != 71 && fB != 80 {
		return errors.New(ErrorDataNotMatch), nil
	}

	pStr := string(peekData)

	iHeaderEnd := strings.Index(pStr, "\r\n\r\n")
	if iHeaderEnd == -1 {
		return errors.New(ErrorDataNotReady), nil
	}

	r := bytes.NewReader(peekData)
	br := bufio.NewReader(r)

	req, err := http.ReadRequest(br)

	if err != nil {
		return err, nil
	}

	if req.ContentLength > int64(b.Len()) {
		return errors.New(ErrorDataNotReady), nil
	}

	if req.ContentLength > HttpMaxLengthSupported {
		return errors.New(ErrorDataIsDamage), nil
	}

	lengthTotal := iHeaderEnd + 4
	if req.ContentLength > 0 {
		lengthTotal += int(req.ContentLength)
	}

	if b.Len() < lengthTotal {
		return errors.New(ErrorDataNotReady), nil
	}

	key := req.Header.Get("Sec-WebSocket-Key")
	pto := req.Header.Get("Sec-WebSocket-Protocol")
	ori := req.Header.Get("Origin")
	if pto != "" {
		return errors.New(ErrorDataIsDamage), nil
	}

	if !GetWebsocketAccessController().CheckOrigin(ori) {
		base.LogWarn("Websocket Origin %s is not allowed, Cancel the handshake.", ori)
		return errors.New(ErrorDataIsDamage), nil
	}

	accept := key + WSMagicStr
	hashcode := sha1.Sum([]byte(accept))
	fk := base64.StdEncoding.EncodeToString(hashcode[:])

	resp := fmt.Sprintf(WSRespFmtWithoutProtocol, fk)

	b.Reset()

	return nil, []byte(resp)
}

func (w WSPacketPreprocessor) Farewell() []byte {
	var opcode byte = 0x8
	var fin byte = 1
	raw := []byte("")

	rawLen := len(raw)
	header := make([]byte, 10)
	header[0] = fin<<7 | opcode
	header[1] = byte(rawLen)
	header = header[:2]

	finalBytes := bytes.Join([][]byte{header, raw}, []byte(""))
	return finalBytes
}

type WSPacketParser struct {
}

type WSPacketPackager struct {
}

var WSBPacketFormat = &PacketFormat{
	"ws",
	0xFF,
	"auto",
	false,
	&WSPacketParser{},
	&WSPacketPackager{},
	&WSPacketPreprocessor{},
}

func (p *WSPacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	defer func() {
		base.LogPanic(recover())
	}()

	var packet Packet = nil
	var readedLen = 0
	data := in
	recombining := false

	if !p.checkFinishFrame(in) {
		return nil, packet, readedLen
	}

	for {
		fin := data[0] >> 7
		opCode := data[0] & 0xF
		if fin == 0 {
			recombining = true
		}

		h, _, l := p.getFrameLengths(data)
		if l < 0 || len(data) < l {
			break
		}

		payloadData := p.getPayloadData(data, h, l)

		data = data[l:]
		readedLen += l

		switch opCode {
		case 0x0:
			//recombine
			if !recombining || packet == nil {
				return errors.New(ErrorDataIsDamage), nil, 0
			}
			switch packet.GetType() {
			case PacketTypeText:
				tp, ok := packet.(*TextPacket)
				if ok {
					tp.Text += string(payloadData)
				} else {
					return errors.New(ErrorDataIsDamage), nil, 0
				}
			case PacketTypeBinary:
				bp, ok := packet.(*BinaryPacket)
				if ok {
					bp.Raw = bytes.Join([][]byte{bp.Raw, payloadData}, []byte(""))
				} else {
					return errors.New(ErrorDataIsDamage), nil, 0
				}
			case PacketTypePing:
				bp, ok := packet.(*PingPacket)
				if ok {
					bp.Data = bytes.Join([][]byte{bp.Data, payloadData}, []byte(""))
				} else {
					return errors.New(ErrorDataIsDamage), nil, 0
				}
			default:
				return errors.New(ErrorDataIsDamage), nil, 0
			}
		case 0x1:
			//text
			tp := new(TextPacket)
			tp.Text = string(payloadData)
			packet = tp
		case 0x2:
			//binary
			bp := new(BinaryPacket)
			bp.Compressed = false
			bp.Encrypted = false
			bp.CompressSupport = false
			bp.ProtocolType = 0
			bp.ProtocolVer = 0
			bp.Raw = payloadData
			packet = bp
		case 0x8:
			//close
			return errors.New(ErrorClientReqDisconnect), nil, 0
		case 0x9:
			//ping
			pp := new(PingPacket)
			pp.Data = payloadData
			packet = pp
		default:
			return errors.New(ErrorDataIsDamage), nil, 0
		}

		if fin == 1 {
			break
		}
	}

	return nil, packet, readedLen
}

func (p *WSPacketParser) checkFinishFrame(in []byte) bool {
	if len(in) < WSDataMinLength {
		return false
	}

	data := in
	for {
		fin := data[0] >> 7
		if fin == 1 {
			return true
		}
		_, _, l := p.getFrameLengths(data)
		if l < 0 || len(data) < l {
			break
		}
		data = data[l:]
	}
	return false
}

func (p *WSPacketParser) getFrameLengths(in []byte) (int, uint64, int) {
	maskFlag := in[1] >> 7
	maskBits := 4
	if maskFlag != 1 {
		maskBits = 0
	}
	payloadLen := int(in[1] & 0x7F)
	headlen := WSDataMinLength
	dataLen := uint64(0)

	switch payloadLen {
	case 126:
		headlen = 4 + maskBits
	case 127:
		headlen = 10 + maskBits
	default:
		headlen = WSDataMinLength + maskBits
	}

	if len(in) < headlen {
		return headlen, 0, -1
	}

	switch payloadLen {
	case 126:
		dataLen = uint64(binary.BigEndian.Uint16(in[2:4]))
	case 127:
		dataLen = binary.BigEndian.Uint64(in[2:10])
	default:
		dataLen = uint64(payloadLen)
	}

	totalLen := headlen + int(dataLen&0xFFFFFFFF)
	return headlen, dataLen, totalLen
}

func (p *WSPacketParser) getPayloadData(in []byte, h, l int) []byte {
	maskFlag := in[1] >> 7
	maskBits := 4
	if maskFlag != 1 {
		maskBits = 0
	}

	payloadData := in[:l]
	payloadData = payloadData[h:]

	if maskFlag == 1 {
		mask := make([]byte, maskBits)
		payloadLen := int(in[1] & 0x7F)
		switch payloadLen {
		case 126:
			mask = in[4:8]
		case 127:
			mask = in[10:14]
		default:
			mask = in[2:6]
		}
		for i := 0; i < len(payloadData); i++ {
			payloadData[i] = payloadData[i] ^ mask[i%4]
		}
	}

	return payloadData
}

func (p *WSPacketParser) ParseFromBuffer(b base.Buffer) (error, []Packet) {
	defer func() {
		base.LogPanic(recover())
	}()
	var pcks = make([]Packet, 0)

	var err error

	for {
		in, _ := b.Peek(b.Len())
		e, pck, l := p.ParseFromBytes(in)
		if l > 0 {
			_, _ = b.Next(l)
		}
		if e == nil {
			if pck != nil {
				pcks = append(pcks, pck)
			} else {
				break
			}
		} else if e.Error() == ErrorDataNotReady {
			break
		} else {
			err = e
			break
		}
	}

	return err, pcks
}

func (p *WSPacketParser) TestMatchScore(b base.Buffer) int {
	defer func() {
		base.LogPanic(recover())
	}()

	score := -1

	peekData, n := b.Peek(b.Len())
	if n < HttpHeaderMinLength {
		return score
	}
	fB := base.ReadAsciiCode(peekData)
	if fB != 71 && fB != 80 {
		return score
	}
	score = 0
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(peekData)))
	if err != nil {
		return score
	}

	if req.ContentLength > int64(b.Len()) {
		return score
	} else if req.ContentLength > HttpMaxLengthSupported {
		return score
	}

	key := req.Header.Get("Sec-WebSocket-Key")
	ver := req.Header.Get("Sec-WebSocket-Version")
	upg := req.Header.Get("Upgrade")
	cnn := req.Header.Get("Connection")
	if strings.ToLower(upg) != "websocket" || cnn != "Upgrade" || key == "" || ver == "" {
		return score
	}

	return 99
}

func (p *WSPacketPackager) Package(dst Packet) (error, []byte) {
	defer func() {
		base.LogPanic(recover())
	}()
	var opcode byte = 0x0
	var fin byte = 1
	raw := []byte("")

	switch dst.GetType() {
	case "text":
		packetTxt, ok := dst.(*TextPacket)
		if !ok {
			return errors.New(ErrorPacketUnsupported), nil
		}
		raw = []byte(packetTxt.Text)
		opcode = 0x1
	case "binary":
		packetBin, ok := dst.(*BinaryPacket)
		if !ok {
			return errors.New(ErrorPacketUnsupported), nil
		}
		raw = packetBin.Raw
		opcode = 0x2
	case "ping":
		packetPing, ok := dst.(*PingPacket)
		if !ok {
			return errors.New(ErrorPacketUnsupported), nil
		}
		raw = packetPing.Data
		opcode = 0xa
	}

	rawLen := len(raw)
	header := make([]byte, 10)
	header[0] = fin<<7 | opcode

	switch {
	case rawLen <= 125:
		header[1] = byte(rawLen)
		header = header[:2]
	case rawLen <= 0xFFFF:
		header[1] = 0x7e
		binary.BigEndian.PutUint16(header[2:4], uint16(rawLen))
		header = header[:4]
	default:
		header[1] = 0x7f
		binary.BigEndian.PutUint64(header[2:10], uint64(rawLen))
		header = header[:10]
	}

	finalBytes := bytes.Join([][]byte{header, raw}, []byte(""))

	return nil, finalBytes
}
