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
	"github.com/packing/clove2/codec"
	"github.com/packing/clove2/errors"
)

// Server           TCP-Server class
type Server struct {
	byteOrder binary.ByteOrder
	Codec     *codec.Codec

	OnControllerCome  func(*Controller) error
	OnControllerLeave func(*Controller)

	limit int
	top   int
	total int64

	closed bool

	controllers base.CloveMap

	mutex sync.Mutex
}

// Create a default tcp server instance.
func CreateServer() *Server {
	srv := new(Server)
	srv.controllers = base.NewSyncCloveMap()
	return srv
}

// Create a server instance that supports a maximum of n clients.
func CreateServerWithLimit(n int) *Server {
	srv := CreateServer()
	srv.limit = n
	return srv
}

// Create a server instance that supports up to n clients and parses data using byte order b
func CreateServerWithByteOrder(b binary.ByteOrder, n int) *Server {
	srv := CreateServer()
	srv.limit = n
	srv.byteOrder = b
	return srv
}

func (server *Server) ConnectAccepted(conn net.Conn) error {
	if server.controllers.Count() < server.limit {
		c := CreateController(conn, server)
		if c == nil {
			return errors.New("Failed to create controller.")
		}
		err, ok := <-server.ControllerCome(c)
		if !ok || err != nil {
			return err
		}
		server.controllers.Set(c.GetId().Integer(), c)
		return nil
	} else {
		return errors.New("Connection limit.")
	}
}

func (server *Server) ControllerCome(controller *Controller) <-chan error {
	iv := make(chan error)
	if server.OnControllerCome != nil {
		go func() {
			iv <- server.OnControllerCome(controller)
		}()
	}
	return iv
}

func (server *Server) ControllerLeave(controller *Controller) {
	if server.OnControllerLeave != nil {
		go server.OnControllerLeave(controller)
	}
}

func (server *Server) lookupController() {
	go func() {
		for server.closed {
			v, ok := <-server.controllers.IterItems()
			if ok && v.Value != nil {
				c, ok := v.Value.(*Controller)
				if ok && c != nil {
					c.ProcessRead()
					c.ProcessWrite()
					if !c.CheckAlive() {
						server.controllers.Pop(v.Key)
					}
				}
			}
		}
	}()
}
