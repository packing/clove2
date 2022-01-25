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

// Implementation of TCP protocol server processing details.

package tcp

import (
	"encoding/binary"
	"net"
	"sync"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/errors"
)

// StandardPool           TCP-StandardPool class
type StandardPool struct {
	byteOrder       binary.ByteOrder
	packetFormatMgr *PacketFormatManager
	packetProcessor PacketProcessor

	OnControllerEnter func(*Controller) error
	OnControllerLeave func(*Controller)

	//注意，此处在目标连接的主逻辑线程内，里面任何可能导致挂起的调用都必须go
	OnControllerPacketReceived func(Packet, *Controller) error

	limit int
	top   int32
	curr  int32
	total int64

	controllers *sync.Map

	waitg *sync.WaitGroup
	mutex sync.Mutex
}

// Create a default tcp StandardPool instance.
func CreateStandardPool(pfs ...string) *StandardPool {
	p := new(StandardPool)
	if len(pfs) > 0 {
		p.packetFormatMgr = NewPacketFormatManager(pfs...)
	}
	p.limit = -1
	p.controllers = new(sync.Map)
	p.waitg = new(sync.WaitGroup)
	return p
}

// Create a StandardPool instance that supports a maximum of n controllers.
func CreateStandardPoolWithLimit(n int, pfs ...string) *StandardPool {
	p := CreateStandardPool(pfs...)
	p.limit = n
	return p
}

// Create a StandardPool instance that supports up to n controllers and parses data using byte order b
func CreateStandardPoolWithByteOrder(b binary.ByteOrder, n int, pfs ...string) *StandardPool {
	p := CreateStandardPoolWithLimit(n, pfs...)
	p.byteOrder = b
	return p
}

func (pool *StandardPool) addTotalNum(n int64) {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	pool.total += n
}

func (pool *StandardPool) addCurrNum(n int32) {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	pool.curr += n
}

func (pool *StandardPool) resetTopNum() {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	if pool.curr > pool.top {
		pool.top = pool.curr
	}
}

func (pool *StandardPool) checkLimit() bool {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	return pool.limit <= 0 || pool.curr < int32(pool.limit)
}

func (pool *StandardPool) getCurrNum() int32 {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	return pool.curr
}

func (pool *StandardPool) getTotalNum() int64 {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	return pool.total
}

func (pool *StandardPool) getTopNum() int32 {
	defer pool.mutex.Unlock()
	pool.mutex.Lock()
	return pool.top
}

func (pool *StandardPool) AddConnection(conn net.Conn, packetProcessor PacketProcessor) error {
	if pool.checkLimit() {
		c := CreateController(conn, pool, packetProcessor)
		if c == nil {
			return errors.New("Failed to create controller.")
		}
		err, ok := <-pool.ControllerEnter(c)
		if !ok || err != nil {
			return err
		}
		pool.controllers.Store(c.GetId().Integer(), c)

		base.LogVerbose("current/top/total: %d / %d / %d", pool.getCurrNum(), pool.getTopNum(), pool.getTotalNum())

		return nil
	} else {
		return errors.New("Connection limit.")
	}
}

func (pool *StandardPool) ControllerEnter(controller *Controller) <-chan error {
	iv := make(chan error)
	go func() {
		if pool.OnControllerEnter != nil {
			iv <- pool.OnControllerEnter(controller)
		} else {
			iv <- nil
		}
		close(iv)
	}()
	return iv
}

func (pool *StandardPool) ControllerLeave(controller *Controller) {
	pool.controllers.Delete(controller.GetId().Integer())
	pool.addCurrNum(-1)
	if pool.OnControllerLeave != nil {
		go pool.OnControllerLeave(controller)
	}
}

func (pool *StandardPool) ControllerPacketReceived(pck Packet, controller *Controller) error {
	if pool.OnControllerPacketReceived != nil {
		return pool.OnControllerPacketReceived(pck, controller)
	}
	return nil
}

func (pool *StandardPool) Lookup() {
}

func (pool *StandardPool) Close() {
	pool.waitg.Wait()
	pool.controllers.Range(func(key, value interface{}) bool {
		pool.controllers.Delete(key)
		return true
	})
}

func (pool *StandardPool) CloseController(id uint64) {
	ic, ok := pool.controllers.Load(id)
	if ok && ic != nil {
		c, ok := ic.(*Controller)
		if ok {
			c.Exit()
		}
	}
}

func (pool *StandardPool) GetController(id uint64) *Controller {
	ic, ok := pool.controllers.Load(id)
	if ok && ic != nil {
		c, ok := ic.(*Controller)
		if ok {
			return c
		}
	}
	return nil
}

func (pool *StandardPool) GetPacketFormatManager() *PacketFormatManager {
	return pool.packetFormatMgr
}

func (pool *StandardPool) GetPacketProcessor() PacketProcessor {
	return pool.packetProcessor
}

func (pool *StandardPool) SetPacketProcessor(pp PacketProcessor) {
	pool.packetProcessor = pp
}
