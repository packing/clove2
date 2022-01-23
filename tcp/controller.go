/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Implementation of TCP protocol controller processing details.

package tcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/packing/clove2/base"
)

type Controller struct {
	conn        net.Conn
	sessionId   base.CloveId
	flag        int
	totalRBytes int64
	totalSBytes int64
	totalRPcks  int64
	totalSPcks  int64

	aliveChecking   bool
	packetFmtMgr    *PacketFormatManager
	packetFmt       *PacketFormat
	packetProcessor PacketProcessor
	manager         ControllerManager
	waitg           *sync.WaitGroup
	bufRecv         *base.StandardBuffer
	bufSend         *base.SyncBuffer
	chReceived      base.ChannelQueue
	timeOfBirth     time.Time
	chClosed        chan bool
	chSend          chan int
}

const (
	cFlagOpened       = 0x1
	cFlagClosing      = 0x2
	cFlagClosed       = 0x4
	cFlagDataReceived = 0x8
	cFlagReadyed      = 0x10

	timeoutDeterminePacketFormat = time.Second * 10
	timeoutReceived              = time.Second * 5
	timeoutWrited                = time.Second * 3
)

func CreateController(conn net.Conn, m ControllerManager, packetProcessor PacketProcessor) *Controller {
	var seed uint = 0
	tc, ok := conn.(*net.TCPConn)
	if ok {
		if pf, err := tc.File(); err == nil {
			seed = uint(pf.Fd())
		}
	}
	if seed == 0 {
		return nil
	}
	_ = tc.SetLinger(-1)
	_ = tc.SetNoDelay(true)
	c := new(Controller)
	c.flag = cFlagOpened
	c.aliveChecking = true
	c.timeOfBirth = time.Now()
	c.sessionId, ok = <-base.GetIdGenerator().NextIdWithSeed(seed)
	c.conn = conn
	c.manager = m
	c.packetFmt = nil
	c.packetFmtMgr = c.manager.GetPacketFormatManager()
	c.packetProcessor = c.manager.GetPacketProcessor()
	if packetProcessor != nil {
		c.packetProcessor = packetProcessor
	}
	c.bufSend = new(base.SyncBuffer)
	c.bufRecv = new(base.StandardBuffer)
	c.chReceived = make(base.ChannelQueue, 32)
	c.chClosed = make(chan bool)
	c.chSend = make(chan int)
	c.process()
	return c
}

func (controller *Controller) GetId() base.CloveId {
	return controller.sessionId
}

func (controller *Controller) GetPacketProcessor() PacketProcessor {
	return controller.packetProcessor
}

func (controller *Controller) SetPacketProcessor(pp PacketProcessor) {
	controller.packetProcessor = pp
}

func (controller *Controller) GetRemoteHostName() string {
	return controller.conn.RemoteAddr().String()
}

func (controller *Controller) GetLocalHostName() string {
	return controller.conn.LocalAddr().String()
}

func (controller *Controller) GetTotalRecvBytes() int64 {
	return controller.totalRBytes
}

func (controller *Controller) GetTotalSentBytes() int64 {
	return controller.totalSBytes
}

func (controller *Controller) GetTotalRecvPackets() int64 {
	return controller.totalRPcks
}

func (controller *Controller) GetTotalSentPackets() int64 {
	return controller.totalSPcks
}

func (controller *Controller) parsePacket() error {
	if controller.packetFmt != nil {
		err, pcks := controller.packetFmt.Parser.ParseFromBuffer(controller.bufRecv)
		if err != nil {
			return err
		}
		controller.ReceivePackets(pcks...)
	} else {
		pfMgr := controller.packetFmtMgr
		if pfMgr == nil {
			pfMgr = GetPacketFormatManager()
		}
		controller.packetFmt = pfMgr.DetermineFromBuffer(controller.bufRecv)
		if controller.packetFmt != nil {
			if controller.packetFmt.Preprocessor != nil {
				err, ack := controller.packetFmt.Preprocessor.HandshakeAck(controller.bufRecv)
				if err != nil {
					return err
				}
				controller.SendBytes(ack)
				controller.setReadyedFlag()
			} else {
				controller.setReadyedFlag()
				return controller.parsePacket()
			}
		}
	}
	return nil
}

func (controller *Controller) setDataReceivedFlag() {
	if base.TestMask(controller.flag, cFlagOpened) && !base.TestMask(controller.flag, cFlagDataReceived) {
		controller.flag |= cFlagDataReceived
	}
}

func (controller *Controller) setReadyedFlag() {
	if base.TestMask(controller.flag, cFlagOpened) && !base.TestMask(controller.flag, cFlagReadyed) {
		controller.flag |= cFlagReadyed
		controller.aliveChecking = false
		base.LogVerbose("Determine PacketFormat => %s (%d)", controller.packetFmt.Tag, controller.GetId().Integer())
	}
}

func (controller *Controller) forceClose() {
	if controller.packetFmt != nil && controller.packetFmt.Preprocessor != nil {
		fw := controller.packetFmt.Preprocessor.Farewell()
		if fw != nil && len(fw) > 0 {
			_ = controller.conn.SetWriteDeadline(time.Now().Add(timeoutWrited))
			_, _ = controller.conn.Write(fw)
			base.LogVerbose("Farewell Data %d (%d)", len(fw), controller.GetId().Integer())
		}
	}
	t, _ := controller.conn.(*net.TCPConn)
	_ = t.CloseRead()
	_ = t.Close()
}

func (controller *Controller) ExitAndNotify() {
	base.LogVerbose("ExitAndNotify %d", controller.GetId().Integer())
	if base.TestMask(controller.flag, cFlagOpened) {
		controller.aliveChecking = false
		controller.forceClose()
		go func() {
			controller.chClosed <- true
		}()
	}
}

func (controller *Controller) Exit() {
	base.LogVerbose("Exit %d", controller.GetId().Integer())
	if base.TestMask(controller.flag, cFlagOpened) {
		controller.aliveChecking = false
		controller.forceClose()
	}
}

func (controller *Controller) Close() {
	//base.LogVerbose("Close")
	if base.TestMask(controller.flag, cFlagClosing) {
		controller.bufRecv.Reset()
		controller.bufSend.Reset()
		controller.chReceived.Close()
		close(controller.chSend)
		close(controller.chClosed)
		controller.waitg.Wait()
		controller.flag = cFlagClosed
		controller.manager.ControllerLeave(controller)
		controller.manager = nil
	}
}

func (controller *Controller) SendBytes(sendBytes []byte) bool {
	defer func() {
		base.LogPanic(recover())
	}()

	if !base.TestMask(controller.flag, cFlagOpened) {
		return false
	}

	n, err := controller.bufSend.Write(sendBytes)
	if err != nil || n != len(sendBytes) {
		controller.Exit()
		return false
	}

	controller.chSend <- len(sendBytes)

	return true
}

func (controller *Controller) SendPackets(sendPcks ...Packet) bool {
	return controller.SendPacketsAndClose(false, sendPcks...)
}

func (controller *Controller) SendPacketsAndClose(close bool, sendPcks ...Packet) bool {
	defer func() {
		base.LogPanic(recover())
	}()

	if !base.TestMask(controller.flag, cFlagOpened) {
		return false
	}

	go func() {
		for _, pck := range sendPcks {
			var fmtPck = controller.packetFmt
			if fmtPck == nil {
				fmtPck = GetPacketFormatManager().FindPacketFormat(pck.GetType())
			}
			if fmtPck != nil {
				err, raw := fmtPck.Packager.Package(pck)
				if err == nil {
					controller.SendBytes(raw)
				}
			}
		}
		if close {
			controller.SendBytes([]byte(""))
		}
	}()
	return true
}

func (controller *Controller) ReceivePackets(pcks ...Packet) {
	defer func() {
		base.LogPanic(recover())
	}()

	if len(pcks) == 0 {
		return
	}

	go func() {
		defer func() {
			recover()
		}()
		for _, pck := range pcks {
			controller.chReceived <- pck
		}
	}()
}

func (controller *Controller) processRead() {
	defer func() {
		base.LogVerbose("The connection %d (%s) stops reading.", controller.GetId().Integer(), controller.GetRemoteHostName())
		controller.waitg.Done()
		base.LogPanic(recover())
	}()
	controller.waitg.Add(1)

	var buf = make([]byte, 512)

	for {
		n, err := controller.conn.Read(buf)
		if err == nil && n > 0 {
			_, err = controller.bufRecv.Write(buf[:n])
			if err != nil {
				controller.ExitAndNotify()
				break
			}
			controller.setDataReceivedFlag()
			err = controller.parsePacket()
			if err != nil {
				base.LogError("The connection %d (%s) will be closed because of a data problem.", controller.GetId().Integer(), controller.GetRemoteHostName())
				//base.LogVerbose("The full stack:\n\n%+v\n\n", err)
				controller.ExitAndNotify()
				break
			}
			atomic.AddInt64(&controller.totalRBytes, int64(n))
		}
		if err != nil || n == 0 {
			controller.ExitAndNotify()
			break
		}
	}
}

func (controller *Controller) tryAlive() {
	defer func() {
		controller.waitg.Done()
		base.LogPanic(recover())
	}()
	controller.waitg.Add(1)
	for controller.aliveChecking {
		if !base.TestMask(controller.flag, cFlagDataReceived) {
			if time.Now().Sub(controller.timeOfBirth) > timeoutReceived {
				base.LogVerbose("Waiting for data timeout. %d (%s)", controller.GetId().Integer(), controller.GetRemoteHostName())
				controller.Exit()
				break
			}
		} else if !base.TestMask(controller.flag, cFlagReadyed) {
			if time.Now().Sub(controller.timeOfBirth) > timeoutDeterminePacketFormat {
				base.LogVerbose("Identify protocol timeout.", controller.flag)
				controller.Exit()
				break
			}
		} else {
			break
		}
		time.Sleep(time.Millisecond * 500)
	}

}

func (controller *Controller) process() {
	controller.waitg = new(sync.WaitGroup)

	go controller.processRead()
	go controller.tryAlive()

	go func() {
		defer func() {
			base.LogVerbose("The connection %d (%s) stops processing.", controller.GetId().Integer(), controller.GetRemoteHostName())

			controller.waitg.Done()
			controller.Close()
		}()
		controller.waitg.Add(1)

		for base.TestMask(controller.flag, cFlagOpened) {
			//base.LogVerbose("loop >>>>>>>>>")
			select {
			case isentBytes, ok := <-controller.chSend:
				if ok {
					if isentBytes <= 0 {
						base.LogVerbose("Connection send close")
						controller.Exit()
						break
					}
					sentBytes, n := controller.bufSend.Next(isentBytes)
					if n != isentBytes {
						controller.Exit()
						break
					}

					if len(sentBytes) == 0 {
						controller.Exit()
						break
					}

					if !func() bool {
						for {
							e := controller.conn.SetWriteDeadline(time.Now().Add(timeoutWrited))
							if e != nil {
								return false
							}
							n, e := controller.conn.Write(sentBytes)
							if e != nil {
								return false
							}
							if n < len(sentBytes) {
								sentBytes = sentBytes[n:]
								continue
							}
							//base.LogVerbose("Connection write %d", n)
							break
						}
						return true
					}() {
						controller.Exit()
						break
					}
				} else {
					controller.Exit()
					break
				}
			case iReceived, ok := <-controller.chReceived:
				pckReceived, ok := iReceived.(Packet)
				if !ok || pckReceived == nil {
					base.LogVerbose("A error value [%s] on [processData].", iReceived)
					controller.Exit()
					break
				} else {
					err := controller.manager.ControllerPacketReceived(pckReceived, controller)
					if err != nil {
						base.LogError("The connection %d (%s) will be closed because of '%s'.", controller.GetId().Integer(), controller.GetRemoteHostName(), err.Error())
						//base.LogVerbose("The full stack:\n\n%+v\n\n", err)
						controller.Exit()
						break
					}
				}

			case closed, ok := <-controller.chClosed:
				if !ok || closed {
					controller.flag = cFlagClosing
					break
				}

			}
		}
	}()
}
