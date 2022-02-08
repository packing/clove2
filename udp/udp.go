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
	_, ok := p.fragments[fragment.Idx]
	if ok {
		//存在冲突序号，重新初始化当前序列容器
		p.fragments = make(map[uint16]*Fragment)
	}

	if p.count > 0 && p.count != fragment.Count {
		//序列总数不匹配，重新初始化当前序列容器
		p.fragments = make(map[uint16]*Fragment)
		p.count = fragment.Count
	}

	p.fragments[fragment.Idx] = fragment

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
