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
	manager     ControllerManager
	waitg       *sync.WaitGroup
	queueSend   base.ChannelQueue
	bufRecv     *base.SyncBuffer
}

const (
	cFlagOpened  = 0x0
	cFlagClosing = 0x1
	cFlagClosed  = 0x2
)

func CreateController(conn net.Conn, m ControllerManager) *Controller {
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
	c.sessionId, ok = <-base.GetIdGenerator().NextIdWithSeed(seed)
	c.conn = conn
	c.manager = m
	c.flag = cFlagOpened
	c.queueSend = make(base.ChannelQueue)
	c.bufRecv = new(base.SyncBuffer)
	c.process()
	return c
}

func (controller *Controller) GetId() *base.CloveId {
	return &controller.sessionId
}

func (controller *Controller) GetRemoteHostName() string {
	return controller.conn.RemoteAddr().String()
}

func (controller *Controller) GetLocalHostName() string {
	return controller.conn.LocalAddr().String()
}

func (controller Controller) GetTotalRecvBytes() int64 {
	return controller.totalRBytes
}

func (controller Controller) GetTotalSentBytes() int64 {
	return controller.totalSBytes
}

func (controller Controller) GetTotalRecvPackets() int64 {
	return controller.totalRPcks
}

func (controller Controller) GetTotalSentPackets() int64 {
	return controller.totalSPcks
}

func (controller *Controller) Close() {
	if controller.flag == cFlagOpened {
		controller.flag = cFlagClosing
		controller.manager.ControllerClosing(controller)
	}
}

func (controller *Controller) CheckAlive() bool {
	if controller.flag == cFlagClosing {

		controller.queueSend.Close()
		controller.waitg.Wait()

		controller.manager.ControllerLeave(controller)
		_ = controller.conn.Close()
		controller.flag = cFlagClosed

		return false
	}
	return controller.flag == cFlagOpened
}

func (controller Controller) SendBytes(sentBytes []byte) bool {
	if controller.flag != cFlagOpened {
		return false
	}

	go func() { controller.queueSend <- sentBytes }()

	return true
}

func (controller *Controller) processRead() {
	defer func() {
		base.LogPanic(recover())
	}()

	var buf = make([]byte, 512)

	for {
		n, err := controller.conn.Read(buf)
		if err == nil && n > 0 {
			_, err = controller.bufRecv.Write(buf)
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

	for {
		isentBytes, ok := <-controller.queueSend
		if ok {
			sentBytes, ok := isentBytes.([]byte)
			if ok {
				for {
					e := controller.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
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

	controller.waitg.Done()
}

func (controller *Controller) process() {
	controller.waitg = new(sync.WaitGroup)
	controller.waitg.Add(3)
	go controller.processRead()
	go controller.processWrite()
}
