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

	"github.com/packing/clove2/base"
)

type Controller struct {
	conn      net.Conn
	sessionId base.CloveId
	flag      int
	manager   ControllerManager
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
	return c
}

func (controller *Controller) GetId() base.CloveId {
	return controller.sessionId
}

func (controller *Controller) Close() {
	controller.flag = cFlagClosing
}

func (controller *Controller) CheckAlive() bool {
	if controller.flag == cFlagClosing {
		controller.manager.ControllerLeave(controller)
		_ = controller.conn.Close()
		controller.flag = cFlagClosed
		return false
	}
	return controller.flag == cFlagOpened
}

func (controller *Controller) ProcessRead() {

}

func (controller *Controller) ProcessWrite() {

}
