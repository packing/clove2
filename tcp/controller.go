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
	conn            net.Conn
	sessionId       base.CloveId
	flag            int
	totalRBytes     int64
	totalSBytes     int64
	totalRPcks      int64
	totalSPcks      int64
	packetFmt       *PacketFormat
	packetProcessor PacketProcessor
	manager         ControllerManager
	waitg           *sync.WaitGroup
	queueSend       base.ChannelQueue
	bufRecv         *base.StandardBuffer
	chReceived      base.ChannelQueue
	timeOfBirth     time.Time
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
	c := new(Controller)
	c.flag = cFlagOpened
	c.timeOfBirth = time.Now()
	c.sessionId, ok = <-base.GetIdGenerator().NextIdWithSeed(seed)
	c.conn = conn
	c.manager = m
	c.packetFmt = c.manager.GetPacketFormat()
	if c.packetFmt != nil {
		c.setReadyedFlag()
	}
	c.packetProcessor = c.manager.GetPacketProcessor()
	if packetProcessor != nil {
		c.packetProcessor = packetProcessor
	}
	c.queueSend = make(base.ChannelQueue)
	c.bufRecv = new(base.StandardBuffer)
	c.chReceived = make(base.ChannelQueue, 32)
	c.process()
	return c
}

func (controller *Controller) GetId() *base.CloveId {
	return &controller.sessionId
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
		controller.packetFmt = GetPacketFormatManager().DetermineFromBuffer(controller.bufRecv)
		if controller.packetFmt != nil {
			controller.setReadyedFlag()
			return controller.parsePacket()
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
		base.LogVerbose("Determine PacketFormat => %s", controller.packetFmt.Tag)
	}
}

func (controller *Controller) CheckTimeout() {
	if base.TestMask(controller.flag, cFlagOpened) {

		if !base.TestMask(controller.flag, cFlagDataReceived) {
			if time.Now().Sub(controller.timeOfBirth) > timeoutReceived {
				controller.Close()
			}
		}

		if !base.TestMask(controller.flag, cFlagReadyed) {
			if time.Now().Sub(controller.timeOfBirth) > timeoutDeterminePacketFormat {
				controller.Close()
			}
		}

	}
}

func (controller *Controller) Close() {
	if base.TestMask(controller.flag, cFlagOpened) {
		controller.flag = cFlagClosing
		controller.manager.ControllerClosing(controller)
	}
}

func (controller *Controller) CheckAlive() bool {
	if controller.flag == cFlagClosing {
		controller.manager.ControllerLeave(controller)
		_ = controller.conn.Close()
		controller.queueSend.Close()
		controller.chReceived.Close()
		controller.waitg.Wait()
		controller.flag = cFlagClosed
		return false
	}
	return base.TestMask(controller.flag, cFlagOpened)
}

func (controller *Controller) SendBytes(sentBytes []byte) bool {
	defer func() {
		base.LogPanic(recover())
	}()

	if controller.flag != cFlagOpened {
		return false
	}

	go func() { controller.queueSend <- sentBytes }()

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
		for _, pck := range pcks {
			controller.chReceived <- pck
		}
	}()
}

func (controller *Controller) processRead() {
	defer func() {
		base.LogPanic(recover())
	}()
	controller.waitg.Add(1)

	var buf = make([]byte, 512)

	for {
		n, err := controller.conn.Read(buf)
		if err == nil && n > 0 {
			_, err = controller.bufRecv.Write(buf[:n])
			if err != nil {
				controller.Close()
				break
			}
			controller.setDataReceivedFlag()
			err = controller.parsePacket()
			if err != nil {
				base.LogError("The connection %d (%s) will be closed because of a data problem.", controller.GetId().Integer(), controller.GetRemoteHostName())
				base.LogVerbose("The full stack:\n\n%+v\n\n", err)
				controller.Close()
				break
			}
			atomic.AddInt64(&controller.totalRBytes, int64(n))
		}
		if err != nil || n == 0 {
			controller.Close()
			break
		}
	}

	controller.waitg.Done()
}

func (controller *Controller) processWrite() {
	defer func() {
		base.LogPanic(recover())
	}()
	controller.waitg.Add(1)

	for {
		isentBytes, ok := <-controller.queueSend
		if ok {
			sentBytes, ok := isentBytes.([]byte)
			if ok {
				for {
					e := controller.conn.SetWriteDeadline(time.Now().Add(timeoutWrited))
					if e != nil {
						controller.Close()
						break
					}
					n, e := controller.conn.Write(sentBytes)
					if e != nil {
						controller.Close()
						break
					}
					if n < len(sentBytes) {
						sentBytes = sentBytes[n:]
						continue
					}
				}
			}
		} else {
			break
		}
	}
	controller.waitg.Done()
}

func (controller *Controller) processData() {
	defer func() {
		base.LogPanic(recover())
	}()
	controller.waitg.Add(1)

	for {
		iReceived, ok := <-controller.chReceived
		if !ok {
			break
		}
		pckReceived, ok := iReceived.(Packet)
		if !ok || pckReceived == nil {
			base.LogVerbose("A error value [%s] on [processData].", iReceived)
			continue
		}

		err := controller.manager.ControllerPacketReceived(pckReceived, controller)
		if err != nil {
			base.LogError("The connection %d (%s) will be closed because of '%s'.", controller.GetId().Integer(), controller.GetRemoteHostName(), err.Error())
			base.LogVerbose("The full stack:\n\n%+v\n\n", err)
			controller.Close()
		}

	}

	controller.waitg.Done()
}

func (controller *Controller) process() {
	controller.waitg = new(sync.WaitGroup)
	go controller.processRead()
	go controller.processWrite()
	go controller.processData()
}
