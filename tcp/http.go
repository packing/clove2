package tcp

import (
	"bufio"
	"bytes"
	"net/http"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

const HttpHeaderMinLength = 16
const HttpMaxLengthSupported = 1024 * 1024

type HTTPPacketParser struct {
}

type HTTPPacketPackager struct {
}

var HTTPPacketFormat = PacketFormat{
	"http",
	0,
	"http",
	&HTTPPacketParser{},
	&HTTPPacketPackager{},
}

func (p *HTTPPacketParser) ParseFromBytes(in []byte) (error, Packet, int) {
	defer func() {
		base.LogPanic(recover())
	}()

	fB := base.ReadAsciiCode(in)
	if fB != 71 && fB != 80 {
		return errors.New(ErrorDataNotMatch), nil, 0
	}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(in)))
	if err != nil {
		return errors.New(ErrorDataNotMatch), nil, 0
	}

	if int(req.ContentLength) > len(in) {
		return errors.New(ErrorDataNotReady), nil, 0
	}

	packet := new(HTTPPacket)
	packet.Request = req.Clone(req.Context())

	return nil, packet, int(req.ContentLength)
}

func (p *HTTPPacketParser) ParseFromBuffer(b base.Buffer) (error, []Packet) {
	defer func() {
		base.LogPanic(recover())
	}()

	var pcks = make([]Packet, 0)

	for {

		peekData, n := b.Peek(4096)
		if n < HttpHeaderMinLength {
			break
		}

		fB := base.ReadAsciiCode(peekData)
		if fB != 71 && fB != 80 {
			return errors.New(ErrorDataNotMatch), nil
		}

		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(peekData)))
		if err != nil {
			return errors.New(ErrorDataNotMatch), nil
		}

		if req.ContentLength > int64(b.Len()) {
			break
		}

		if req.ContentLength > HttpMaxLengthSupported {
			return errors.New(ErrorDataIsDamage), nil
		}

		in, n := b.Next(int(req.ContentLength))
		if n != int(req.ContentLength) {
			return errors.New(ErrorDataIsDamage), nil
		}

		err, pck, n := p.ParseFromBytes(in)
		if err != nil {
			if err.Error() != ErrorDataNotReady {
				return err, nil
			}
			break
		}

		pcks = append(pcks, pck)
	}

	return nil, pcks
}

func (p *HTTPPacketParser) TestMatchScore(b base.Buffer) int {
	defer func() {
		base.LogPanic(recover())
	}()

	score := -1

	peekData, n := b.Peek(4096)
	if n < HttpHeaderMinLength {
		return score
	}
	fB := base.ReadAsciiCode(peekData)
	if fB != 71 && fB != 80 {
		return score
	}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(peekData)))
	if err != nil {
		return score
	}

	if req.ContentLength > int64(b.Len()) {
		score = 20
	} else {
		score = 90
	}

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
