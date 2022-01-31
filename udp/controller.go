package udp

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
	"github.com/packing/clove2/network"
)

const (
	DefaultFragmentSize         = 512
	DefaultFragmentHeaderSize   = 8
	DefaultFragmentHeaderPrefix = 0x81
	DefaultFragmentHeaderSuffix = 0x80
	timeoutRead                 = time.Second * 5
	timeoutWrited               = time.Second * 3
)

type Controller struct {
	sessionId       base.CloveId
	addr            string
	conn            *net.UDPConn
	packetFmt       *network.PacketFormat
	packetProcessor network.PacketProcessor
	fragmentSize    int
	fragmentData    []byte
	fragmentHeader  []byte
	pipeline        map[string]*FragmentPipeline
	chReceived      base.ChannelQueue
	waitg           *sync.WaitGroup

	//注意，此处在目标连接的主逻辑线程内，里面任何可能导致挂起的调用都必须go
	OnPacketReceived func(network.Packet, *Controller) error
}

func CreateController(addr string, pfs string) *Controller {
	return CreateControllerWithFragmentSize(addr, DefaultFragmentSize, pfs)
}

func CreateControllerWithFragmentSize(addr string, fragmentSize int, pfs string) *Controller {
	c := new(Controller)
	err := c.bind()
	if err != nil {
		base.LogError("bind error: %s", err.Error())
		return nil
	}
	var seed uint = 0
	if pf, err := c.conn.File(); err == nil {
		seed = uint(pf.Fd())
	}
	if seed == 0 {
		return nil
	}
	c.addr = addr
	c.waitg = new(sync.WaitGroup)
	c.fragmentSize = fragmentSize
	c.fragmentData = make([]byte, c.fragmentSize)
	c.fragmentHeader = make([]byte, DefaultFragmentHeaderSize)
	c.packetFmt = network.GetPacketFormatManager().FindPacketFormat(pfs)
	if c.packetFmt == nil {
		base.LogError("There is no corresponding data format %s", pfs)
	}
	return c
}

func (c *Controller) GetId() base.CloveId {
	return c.sessionId
}

func (c *Controller) Close() {
	defer func() {
		recover()
	}()
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Controller) GetPacketProcessor() network.PacketProcessor {
	return c.packetProcessor
}

func (c *Controller) SetPacketProcessor(p network.PacketProcessor) {
	c.packetProcessor = p
}

func (c *Controller) PacketReceived(pck network.Packet) error {
	if c.OnPacketReceived != nil {
		return c.OnPacketReceived(pck, c)
	}
	return nil
}

func (c *Controller) getPipeline(addr string) *FragmentPipeline {
	f, ok := c.pipeline[addr]
	if !ok {
		c.pipeline[addr] = CreateFragmentPipeline(addr, c.packetFmt)
	}
	return f
}

func (c *Controller) delPipeline(addr string) {
	delete(c.pipeline, addr)
}

func (c *Controller) bind() error {
	address, err := net.ResolveUDPAddr("udp", c.addr)
	if err != nil {
		return err
	}
	c.conn, err = net.ListenUDP("udp", address)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) sendTo(dstAddr string, data []byte) error {
	gConn, err := net.Dial("udp", dstAddr)
	if err != nil {
		return err
	}
	sendConn, ok := gConn.(*net.UDPConn)
	if !ok {
		return errors.Errorf("The connection is not udp")
	}

	defer func() {
		_ = sendConn.Close()
	}()

	buf := bytes.NewReader(data)
	build := new(bytes.Buffer)

	//fragment

	var idx uint16 = 0
	var idxCount = uint16(len(data) / c.fragmentSize)
	if len(data)%c.fragmentSize > 0 {
		idxCount += 1
	}

	for buf.Len() > 0 {
		n, err := buf.Read(c.fragmentData)
		if err != nil {
			return err
		}
		if idx >= idxCount {
			return errors.Errorf("Fragmentation calculation error")
		}

		c.fragmentHeader[0] = DefaultFragmentHeaderPrefix

		_ = binary.Write(build, binary.BigEndian, DefaultFragmentHeaderPrefix)
		_ = binary.Write(build, binary.BigEndian, idx)
		_ = binary.Write(build, binary.BigEndian, idxCount)
		_ = binary.Write(build, binary.BigEndian, uint16(n))
		_ = binary.Write(build, binary.BigEndian, DefaultFragmentHeaderSuffix)
		_, _ = build.Write(c.fragmentData[:n])
		err = sendConn.SetWriteDeadline(time.Now().Add(timeoutWrited))
		if err != nil {
			return err
		}
		n, err = sendConn.Write(build.Bytes())
		if err != nil {
			return err
		}
		if n != build.Len() {
			return errors.Errorf("The data sent is incomplete")
		}
		idx += 1
	}

	return nil
}

func (c *Controller) read() error {
	buf := make([]byte, c.fragmentSize+DefaultFragmentHeaderSize)
	//_ = c.conn.SetReadDeadline(time.Now().Add(timeoutRead))
	n, from, err := c.conn.ReadFrom(buf)
	if err != nil {
		return err
	}
	isFrag := false
	if n < DefaultFragmentHeaderSize {
		isFrag = false
	} else {
		if buf[0] != DefaultFragmentHeaderPrefix || buf[DefaultFragmentHeaderSize-1] != DefaultFragmentHeaderSuffix {
			isFrag = false
		} else {
			frag := new(Fragment)
			frag.idx = binary.BigEndian.Uint16(buf[1:])
			frag.count = binary.BigEndian.Uint16(buf[3:])
			frag.length = binary.BigEndian.Uint16(buf[5:])
			frag.data = buf[:n][DefaultFragmentHeaderSize:]
			if frag.length+DefaultFragmentHeaderSize != uint16(n) {
				isFrag = false
			} else {
				pipeline := c.getPipeline(from.String())
				pipeline.addFragment(frag)
				isFrag = true
			}
		}
	}

	if isFrag {
		pipeline := c.getPipeline(from.String())
		pcks, err := pipeline.Combine()
		if err != nil {
			if err.Error() != ErrorDataFragmentNotEnough {
				c.delPipeline(from.String())
				return err
			}
		} else {
			go func() {
				for _, pck := range pcks {
					c.chReceived <- pck
				}
			}()
		}
	} else {
		pck := new(network.BinaryPacket)
		pck.From = from.String()
		pck.Raw = buf[:n]

		go func() {
			c.chReceived <- pck
		}()
	}
	return nil
}

func (c *Controller) processRead() {
	defer func() {
		base.LogVerbose("The connection %d (%s) stops reading.", c.addr, c.addr)
		c.waitg.Done()
		base.LogPanic(recover())
	}()
	c.waitg.Add(1)

	for {
		err := c.read()
		if err != nil {
			base.LogError("read error: %s", err.Error())
			break
		}
	}
}

func (c *Controller) process() {
	c.waitg = new(sync.WaitGroup)

	go c.processRead()

	go func() {
		defer func() {
			base.LogVerbose("The connection %d (%s) stops processing.", c.GetId().Integer(), c.addr)

			c.waitg.Done()
			c.Close()
		}()
		c.waitg.Add(1)

		for {
			iReceived, ok := <-c.chReceived
			pckReceived, ok := iReceived.(network.Packet)
			if !ok || pckReceived == nil {
				base.LogVerbose("A error value [%s] on [processData].", iReceived)
				break
			} else {
				err := c.PacketReceived(pckReceived)
				if err != nil {
					base.LogError("The connection %d (%s) will be closed because of '%s'.", c.GetId().Integer(), c.addr, err.Error())
					//base.LogVerbose("The full stack:\n\n%+v\n\n", err)
					break
				}
			}
		}
	}()
}
