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
	"sync/atomic"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/codec"
	"github.com/packing/clove2/errors"
)

// StandardPool           TCP-StandardPool class
type StandardPool struct {
	byteOrder binary.ByteOrder
	Codec     *codec.Codec

	OnControllerEnter func(*Controller) error
	OnControllerLeave func(*Controller)

	limit int
	top   int32
	curr  int32
	total int64

	controllers *sync.Map

	waitg *sync.WaitGroup
	mutex sync.Mutex

	chLookup chan *base.CloveId
}

// Create a default tcp StandardPool instance.
func CreateStandardPool() *StandardPool {
	p := new(StandardPool)
	p.controllers = new(sync.Map)
	p.limit = -1
	p.waitg = new(sync.WaitGroup)
	p.chLookup = make(chan *base.CloveId)
	return p
}

// Create a StandardPool instance that supports a maximum of n controllers.
func CreateStandardPoolWithLimit(n int) *StandardPool {
	p := CreateStandardPool()
	p.limit = n
	return p
}

// Create a StandardPool instance that supports up to n controllers and parses data using byte order b
func CreateStandardPoolWithByteOrder(b binary.ByteOrder, n int) *StandardPool {
	p := CreateStandardPool()
	p.limit = n
	p.byteOrder = b
	return p
}

func (pool *StandardPool) AddConnection(conn net.Conn) error {
	if pool.limit <= 0 || pool.curr < int32(pool.limit) {
		c := CreateController(conn, pool)
		if c == nil {
			return errors.New("Failed to create controller.")
		}
		err, ok := <-pool.ControllerEnter(c)
		if !ok || err != nil {
			return err
		}
		pool.controllers.Store(c.GetId().Integer(), c)

		atomic.AddInt32(&pool.curr, 1)
		atomic.AddInt64(&pool.total, 1)
		if pool.curr > pool.top {
			atomic.StoreInt32(&pool.top, pool.curr)
		}
		base.LogVerbose("current/top/total: %d / %d / %d", pool.curr, pool.top, pool.total)

		return nil
	} else {
		return errors.New("Connection limit.")
	}
}

func (pool *StandardPool) ControllerEnter(controller *Controller) <-chan error {
	iv := make(chan error)
	if pool.OnControllerEnter != nil {
		go func() {
			iv <- pool.OnControllerEnter(controller)
		}()
	}
	return iv
}

func (pool *StandardPool) ControllerLeave(controller *Controller) {
	if pool.OnControllerLeave != nil {
		go pool.OnControllerLeave(controller)
	}
}

func (pool *StandardPool) ControllerClosing(controller *Controller) {
	go func() {
		base.LogVerbose("ControllerClosing ", controller.GetId())
		pool.chLookup <- controller.GetId()
	}()
}

func (pool *StandardPool) Lookup() {
	pool.waitg.Add(1)

	go func() {

		defer func() {
			pool.waitg.Done()
			base.LogPanic(recover())
		}()

		for {
			id, ok := <-pool.chLookup
			if !ok {
				break
			}
			base.LogVerbose("process ", id)
			if id != nil {
				ic, ok := pool.controllers.Load(id.Integer())
				if ok {
					c, ok := ic.(*Controller)
					if ok && c != nil {
						if !c.CheckAlive() {
							pool.controllers.Delete(id.Integer())
							atomic.AddInt32(&pool.curr, -1)
						}
					}
				}
			}
		}

	}()
}

func (pool *StandardPool) Close() {
	close(pool.chLookup)
	pool.waitg.Wait()

	pool.controllers.Range(func(key, value interface{}) bool {
		pool.controllers.Delete(key)
		return true
	})
}

func (pool *StandardPool) CloseController(id *base.CloveId) {
	ic, ok := pool.controllers.Load(id.Integer())
	if ok && ic != nil {
		c, ok := ic.(*Controller)
		if ok {
			c.Close()
		}
	}
}
