package unix

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
	"github.com/packing/clove2/network"
	"github.com/packing/clove2/udp"
)

const (
	DefaultFragmentSize              = 512
	DefaultFragmentHeaderSize        = 8
	DefaultFragmentHeaderPrefix byte = 0x81
	DefaultFragmentHeaderSuffix byte = 0x80
	timeoutRead                      = time.Second * 5
	timeoutWrited                    = time.Second * 3
)

type Controller struct {
	sessionId       base.CloveId
	addr            string
	conn            *net.UnixConn
	packetFmt       *network.PacketFormat
	packetProcessor network.PacketProcessor
	fragmentSize    int
	fragmentData    []byte
	fragmentHeader  []byte
	pipeline        map[string]*udp.FragmentPipeline
	chReceived      base.ChannelQueue
	chSend          chan bool
	exit            bool
	waitg           *sync.WaitGroup

	//注意，此处在目标连接的主逻辑线程内，里面任何可能导致挂起的调用都必须go
	OnPacketReceived func(network.Packet, *Controller) error
}

func CreateController(addr string, pfs string) *Controller {
	return CreateControllerWithFragmentSize(addr, DefaultFragmentSize, pfs)
}

func CreateControllerWithFragmentSize(addr string, fragmentSize int, pfs string) *Controller {
	c := new(Controller)
	c.sessionId = <-base.GetIdGenerator().NextId()
	c.addr = addr
	c.exit = false
	c.packetFmt = network.GetPacketFormatManager().FindPacketFormat(pfs)
	if c.packetFmt == nil {
		base.LogError("There is no corresponding data format %s", pfs)
	}
	err := c.bind()
	if err != nil {
		base.LogError("bind error: %s", err.Error())
		return nil
	}
	c.waitg = new(sync.WaitGroup)
	c.fragmentSize = fragmentSize
	c.fragmentData = make([]byte, c.fragmentSize)
	c.fragmentHeader = make([]byte, DefaultFragmentHeaderSize)
	c.chReceived = make(base.ChannelQueue, 32)
	c.chSend = make(chan bool)
	c.pipeline = make(map[string]*udp.FragmentPipeline)

	c.process()
	return c
}

func (c *Controller) GetId() base.CloveId {
	return c.sessionId
}

func (c *Controller) Close() {
	defer func() {
		base.LogPanic(recover())
	}()

	c.exit = true
	go func() {
		select {
		case _, _ = <-c.chSend:
		default:

		}
	}()
	if c.conn != nil {
		base.LogError("close socket")
		e := c.conn.Close()
		base.LogError("close socket error >", e)
	}
	c.waitg.Wait()
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

func (c *Controller) getPipeline(addr string) *udp.FragmentPipeline {
	f, ok := c.pipeline[addr]
	if !ok {
		f = udp.CreateFragmentPipeline(addr, c.packetFmt)
		c.pipeline[addr] = f
	}
	return f
}

func (c *Controller) delPipeline(addr string) {
	delete(c.pipeline, addr)
}

func (c *Controller) bind() error {
	address, err := net.ResolveUnixAddr("unixgram", c.addr)
	if err != nil {
		return err
	}
	c.conn, err = net.ListenUnixgram("unixgram", address)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) SendPacket(addr string, sendPck network.Packet) bool {
	defer func() {
		base.LogPanic(recover())
	}()

	go func() {
		var fmtPck = c.packetFmt
		if fmtPck == nil {
			fmtPck = network.GetPacketFormatManager().FindPacketFormat(sendPck.GetType())
		}
		if fmtPck != nil {
			err, raw := fmtPck.Packager.Package(sendPck)
			if err == nil {
				base.LogVerbose("尝试获取发送许可")
				sendAllowed, ok := <-c.chSend
				if !ok {
					base.LogVerbose("获取发送许可失败")
					return
				}
				if sendAllowed {
					base.LogVerbose("成功获取发送许可")
					base.LogVerbose("发送完整数据 =>", addr, raw)
					err := c.sendTo(addr, raw)
					if err != nil {
						base.LogError("send to %s error: %s", addr, err.Error())
					}
				}
			}
		} else {
			base.LogError("没有包对应的格式处理器")
		}
	}()
	return true
}

func (c *Controller) sendTo(dstAddr string, data []byte) error {
	defer func() {
	}()

	address, err := net.ResolveUnixAddr("unixgram", dstAddr)
	if err != nil {
		return err
	}

	buf := bytes.NewReader(data)

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

		build := new(bytes.Buffer)
		c.fragmentHeader[0] = DefaultFragmentHeaderPrefix

		_ = binary.Write(build, binary.BigEndian, DefaultFragmentHeaderPrefix)
		_ = binary.Write(build, binary.BigEndian, idx)
		_ = binary.Write(build, binary.BigEndian, idxCount)
		_ = binary.Write(build, binary.BigEndian, uint16(n))
		_ = binary.Write(build, binary.BigEndian, DefaultFragmentHeaderSuffix)
		_, _ = build.Write(c.fragmentData[:n])
		err = c.conn.SetWriteDeadline(time.Now().Add(timeoutWrited))
		if err != nil {
			return err
		}
		n, err = c.conn.WriteToUnix(build.Bytes(), address)
		if err != nil {
			return err
		}
		if n != build.Len() {
			return errors.Errorf("The data sent is incomplete")
		} else {
			base.LogVerbose("成功发送 %d 字节", n)
			base.LogVerbose("发送分片数据 =>", build.Bytes())
		}
		idx += 1
	}

	return nil
}

func (c *Controller) sendMsgPacket(dstAddr string, sendPck network.Packet) error {
	defer func() {
	}()

	base.LogVerbose("尝试获取发送许可")
	sendAllowed, ok := <-c.chSend
	if !ok || !sendAllowed {
		base.LogVerbose("获取发送许可失败")
		return errors.Errorf(ErrorDisallowSend)
	}

	address, err := net.ResolveUnixAddr("unixgram", dstAddr)
	if err != nil {
		return err
	}
	unixMsgPck, ok := sendPck.(*network.UnixMsgPacket)
	if !ok {
		return errors.Errorf(ErrorPacketNotIsUnixMsg)
	}

	n, oobn, err := c.conn.WriteMsgUnix(unixMsgPck.B, unixMsgPck.OOB, address)
	if err != nil {
		return err
	}
	if n != len(unixMsgPck.B) || oobn != len(unixMsgPck.OOB) {
		return errors.Errorf("The unixmsg sent is incomplete")
	}

	return nil
}

func (c *Controller) read() error {
	buf := make([]byte, c.fragmentSize+DefaultFragmentHeaderSize)
	//_ = c.conn.SetReadDeadline(time.Now().Add(timeoutRead))
	base.LogVerbose("尝试读取数据")
	n, from, err := c.conn.ReadFromUnix(buf)
	if err != nil {
		return err
	}
	isFrag := false
	base.LogVerbose("成功读取 %d 字节", n)
	if n < DefaultFragmentHeaderSize {
		isFrag = false
	} else {
		if buf[0] != DefaultFragmentHeaderPrefix || buf[DefaultFragmentHeaderSize-1] != DefaultFragmentHeaderSuffix {
			isFrag = false
		} else {
			frag := new(udp.Fragment)
			frag.Idx = binary.BigEndian.Uint16(buf[1:])
			frag.Count = binary.BigEndian.Uint16(buf[3:])
			frag.Length = binary.BigEndian.Uint16(buf[5:])
			frag.Data = buf[:n][DefaultFragmentHeaderSize:]
			if frag.Length+DefaultFragmentHeaderSize != uint16(n) {
				isFrag = false
			} else {
				pipeline := c.getPipeline(from.String())
				pipeline.AddFragment(frag)
				isFrag = true
			}
		}
	}

	if isFrag {
		pipeline := c.getPipeline(from.String())
		pcks, err := pipeline.Combine()
		if err != nil {
			if err.Error() != udp.ErrorDataFragmentNotEnough {
				c.delPipeline(from.String())
				return err
			}
		} else {
			for _, pck := range pcks {
				c.chReceived <- pck
			}
		}
	} else {
		pck := new(network.BinaryPacket)
		pck.From = from.String()
		pck.Raw = buf[:n]

		//go func() {
		c.chReceived <- pck
		//}()
	}
	return nil
}

func (c *Controller) readMsg() error {
	base.LogVerbose("尝试读取数据消息报文")

	b := make([]byte, 4)
	oob := make([]byte, 1024)
	bn, oobn, _, addr, err := c.conn.ReadMsgUnix(b, oob)
	if err != nil {
		return err
	}
	base.LogVerbose("成功读取 %d 字节报文数据及 %d 字节带外数据", bn, oobn)

	pck := new(network.UnixMsgPacket)
	pck.Addr = addr.String()
	pck.B = b[:bn]
	pck.OOB = oob[:oobn]

	c.chReceived <- pck

	return nil
}

func (c *Controller) processRead() {
	defer func() {
		base.LogVerbose("The connection %d (%s) stops reading.", c.GetId().Integer(), c.addr)
		c.waitg.Done()
		base.LogPanic(recover())
	}()
	c.waitg.Add(1)

	for {
		if c.packetFmt.Type == network.PacketTypeBinary {
			err := c.read()
			if err != nil {
				base.LogError("read datagram error: %s", err.Error())
				break
			}
		} else if c.packetFmt.Type == network.PacketTypeUnixMsg {
			err := c.readMsg()
			if err != nil {
				base.LogError("read unixmsg error: %s", err.Error())
				break
			}
		} else {
			base.LogError("unixgram's type %s is not support", c.packetFmt.Type)
			break
		}
	}

	c.chReceived.Close()
}

func (c *Controller) processWrite() {
	defer func() {
		base.LogVerbose("The connection %d (%s) stops writing.", c.GetId().Integer(), c.addr)
		c.waitg.Done()
		base.LogPanic(recover())
	}()
	c.waitg.Add(1)

	base.LogError("closeing?", c.exit)
	for !c.exit {
		c.chSend <- true
	}

	close(c.chSend)
}

func (c *Controller) process() {

	go c.processRead()
	go c.processWrite()

	go func() {
		defer func() {
			base.LogVerbose("The connection %d (%s) stops processing.", c.GetId().Integer(), c.addr)

			c.exit = true
			c.waitg.Done()
			c.Close()
		}()
		c.waitg.Add(1)

		for {
			iReceived, ok := <-c.chReceived
			if !ok {
				base.LogVerbose("Receive queue is closed.")
				break
			}
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
