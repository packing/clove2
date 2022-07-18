package unix

import (
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
	"github.com/packing/clove2/network"
)

const (
	DefaultPacketLengthLimit = 0xFFFF
	timeoutRead              = time.Second * 5
	timeoutWrited            = time.Second * 3
)

type Controller struct {
	sessionId       base.CloveId
	addr            string
	conn            *net.UnixConn
	packetFmt       *network.PacketFormat
	packetProcessor network.PacketProcessor
	packetLimit     int
	readBuf         []byte
	chReceived      base.ChannelQueue
	exit            bool
	waitg           *sync.WaitGroup

	//注意，此处在目标连接的主逻辑线程内，里面任何可能导致挂起的调用都必须go
	OnPacketReceived func(network.Packet, *Controller) error
}

func CreateController(addr string, pfs string) *Controller {
	return CreateControllerWithPacketLimit(addr, DefaultPacketLengthLimit, pfs)
}

func CreateControllerWithPacketLimit(addr string, packetLimit int, pfs string) *Controller {
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
	c.packetLimit = packetLimit
	c.readBuf = make([]byte, packetLimit)
	c.chReceived = make(base.ChannelQueue, 32)

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
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.waitg.Wait()

	_ = syscall.Unlink(c.addr)
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

	if c.exit {
		return false
	}

	if sendPck.GetType() == network.PacketTypeUnixMsg {
		return c.SendMsgPacket(addr, sendPck) == nil
	}

	var fmtPck = c.packetFmt
	if fmtPck == nil {
		fmtPck = network.GetPacketFormatManager().FindPacketFormat(sendPck.GetType())
	}
	if fmtPck != nil {
		err, raw := fmtPck.Packager.Package(sendPck)
		if err == nil {
			err := c.sendTo(addr, raw)
			if err != nil {
				base.LogError("send to %s error: %s", addr, err.Error())
				return false
			}
		}

	} else {
		base.LogError("没有包对应的格式处理器")
		return false
	}
	return true
}

func (c *Controller) SendBytes(dstAddr string, data []byte) error {
	return c.sendTo(dstAddr, data)
}

func (c *Controller) sendTo(dstAddr string, data []byte) error {
	defer func() {
	}()

	address, err := net.ResolveUnixAddr("unixgram", dstAddr)
	if err != nil {
		return err
	}

	err = c.conn.SetWriteDeadline(time.Now().Add(timeoutWrited))
	if err != nil {
		return err
	}
	n, err := c.conn.WriteToUnix(data, address)
	if err != nil {
		return err
	}
	if n != len(data) {
		return errors.Errorf("The data sent is incomplete")
	} else {
		base.LogVerbose("WriteToUnix %s Bytes %d", dstAddr, n)
	}

	return nil
}

func (c *Controller) SendMsgPacket(dstAddr string, sendPck network.Packet) error {
	defer func() {
	}()

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
	n, from, err := c.conn.ReadFromUnix(c.readBuf)
	if err != nil {
		return err
	}

	base.LogVerbose("ReadFromUnix %s Bytes %d", from.String(), n)

	var fmtPck = c.packetFmt
	if fmtPck != nil {
		err, pck, _ := fmtPck.Parser.ParseFromBytes(c.readBuf[:n])
		if err == nil {
			switch pck.GetType() {
			case network.PacketTypeBinary:
				binPck, ok := pck.(*network.BinaryPacket)
				if ok {
					binPck.From = from.String()
				}
			case network.PacketTypeText:
				txtPck, ok := pck.(*network.TextPacket)
				if ok {
					txtPck.From = from.String()
				}
			}

			go func() {
				c.chReceived <- pck
			}()
			return nil
		} else {
			return errors.Errorf(ErrorPacketFormat)
		}

	} else {
		base.LogError("没有包对应的格式处理器")
		return errors.Errorf(ErrorPacketFormat)
	}

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
		base.LogVerbose("%s 停止读取处理", c.addr)
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

func (c *Controller) process() {

	go c.processRead()

	go func() {
		defer func() {
			base.LogVerbose("%s 停止逻辑处理", c.addr)

			c.exit = true
			c.waitg.Done()
		}()
		c.waitg.Add(1)

		for {
			iReceived, ok := <-c.chReceived
			if !ok {
				//base.LogVerbose("Receive queue is closed.")
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
