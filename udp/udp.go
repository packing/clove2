package udp

import (
	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
	"github.com/packing/clove2/network"
)

const (
	ErrorDataFragmentNotEnough = "Data fragment is not enough"
	ErrorDataFragmentCorrupted = "Data fragment is corrupted"
	ErrorPacketFormatNil       = "No packet format is specified"
)

type DatagramManager interface {
	GetPacketProcessor() network.PacketProcessor
	SetPacketProcessor(network.PacketProcessor)
	PacketReceived(network.Packet) error
}

type Fragment struct {
	length uint16
	idx    uint16
	count  uint16
	data   []byte
}

type FragmentPipeline struct {
	addr      string
	count     uint16
	packetFmt *network.PacketFormat
	fragments map[uint16]*Fragment
}

func CreateFragmentPipeline(addr string, packetFmt *network.PacketFormat) *FragmentPipeline {
	frags := new(FragmentPipeline)
	frags.fragments = make(map[uint16]*Fragment)
	frags.count = 0
	frags.addr = addr
	frags.packetFmt = packetFmt
	return frags
}

func (p *FragmentPipeline) addFragment(fragment *Fragment) {
	p.fragments[fragment.idx] = fragment
	if p.count != fragment.count {
		p.count = fragment.count
	}
}

func (p *FragmentPipeline) Combine() ([]network.Packet, error) {
	if len(p.fragments) < int(p.count) {
		return nil, errors.Errorf(ErrorDataFragmentNotEnough)
	}
	if p.packetFmt == nil {
		return nil, errors.Errorf(ErrorPacketFormatNil)
	}

	buf := new(base.StandardBuffer)
	var i uint16
	for i = 0; i < p.count; i++ {
		frag, ok := p.fragments[i]
		if !ok {
			return nil, errors.Errorf(ErrorDataFragmentCorrupted)
		}
		_, _ = buf.Write(frag.data)
	}

	err, pcks := p.packetFmt.Parser.ParseFromBuffer(buf)
	if err != nil {
		return nil, err
	}
	return pcks, nil
}
