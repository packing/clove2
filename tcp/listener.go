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

// Implementation of TCP protocol listener processing details.

package tcp

import (
	"net"

	"github.com/packing/clove2/base"
	"github.com/packing/clove2/network"
)

// Listener           TCP-Listener class
type Listener struct {
	addr string

	l *net.TCPListener
}

// Create a default tcp listener instance.
func CreateListener() *Listener {
	listener := new(Listener)
	return listener
}

func (listener Listener) Start(addr string, processor ConnectionProcessor, packetProcessor network.PacketProcessor) error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return err
	}

	l, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	base.LogVerbose("Listen on: %s", addr)

	listener.addr = addr
	listener.l = l

	go func() {
		for {
			conn, err := listener.l.Accept()
			if err == nil {
				err = processor.AddConnection(conn, packetProcessor)
				if err != nil {
					base.LogWarn("Failed to accept connection. error: %s", err.Error())
					_ = conn.Close()
				}
			} else {
				base.LogWarn("Failed to accept connection. error: %s", err.Error())
			}
		}
	}()

	return nil
}

func (listener Listener) Stop() {
	if listener.l != nil {
		_ = listener.l.Close()
	}
}
