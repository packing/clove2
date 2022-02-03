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
	Length uint16
	Idx    uint16
	Count  uint16
	Data   []byte
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

func (p *FragmentPipeline) AddFragment(fragment *Fragment) {
	p.fragments[fragment.Idx] = fragment
	if p.count != fragment.Count {
		p.count = fragment.Count
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
		//base.LogVerbose("合并分片数据 =>", frag.Data)
		_, _ = buf.Write(frag.Data)
	}

	//peek, _ := buf.Peek(buf.Len())
	//base.LogVerbose("读取完整数据 =>", peek)

	err, pcks := p.packetFmt.Parser.ParseFromBuffer(buf)
	if err != nil {
		return nil, err
	}

	topcks := make([]network.Packet, len(pcks))
	for i, pck := range pcks {
		bin, ok := pck.(*network.BinaryPacket)
		if ok {
			bin.From = p.addr
			topcks[i] = bin
		}
	}
	return topcks, nil
}
