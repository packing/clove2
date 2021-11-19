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

// Implementation of TCP protocol connector processing details.

package tcp

import (
	"net"
	"sync"
	"time"
)

// Listener           TCP-Connector class
type Connector struct {
}

const DefaultTimeout = time.Second * 30

// Create a default tcp connector instance.
func CreateConnector() *Connector {
	connector := new(Connector)
	return connector
}

// try to connect to the specified address within a timeout time.
func (connector Connector) ConnectWithTimeout(addr string, timeout time.Duration, processor ConnectionProcessor) error {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}

	if processor.AddConnection(conn) != nil {
		_ = conn.Close()
	}

	return nil
}

// try to connect to the specified address within 30 seconds.
func (connector Connector) Connect(addr string, processor ConnectionProcessor) error {
	return connector.ConnectWithTimeout(addr, DefaultTimeout, processor)
}

var instConnector *Connector
var onceForConnectorSingleton sync.Once

func GetConnector() *Connector {
	onceForConnectorSingleton.Do(func() {
		instConnector = &Connector{}
	})
	return instConnector
}
