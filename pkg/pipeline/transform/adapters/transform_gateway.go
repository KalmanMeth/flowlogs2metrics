/*
 * Copyright (C) 2021 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package adapters

import (
	"encoding/json"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/sirupsen/logrus"
)

const (
	defaultConnectionTimeout = time.Duration(1 * time.Minute)
	defaultCleanupLoopTime   = time.Duration(1 * time.Minute)
)

var mlog = logrus.WithField("component", "transform.Gateway")

type GatewayStruct struct {
	mu                sync.RWMutex
	gatewayInfo       *api.TransformGateway
	activeConnections map[string]*api.GatewayConnectionInfo
	conn              *net.TCPConn
	connTimeout       time.Duration
	cleanupLoopTime   time.Duration
}

// Transform transforms a flow to a new set of keys
func (m *GatewayStruct) Transform(entry config.GenericMap) (config.GenericMap, bool) {
	mlog.Tracef("Transform input = %v", entry)
	outputEntry := entry.Copy()
	addr1, ok := entry[m.gatewayInfo.Addr1].(string)
	if !ok {
		return outputEntry, true
	}
	addr2, ok := entry[m.gatewayInfo.Addr2].(string)
	if !ok {
		return outputEntry, true
	}
	port1, ok := entry[m.gatewayInfo.Port1]
	if !ok {
		return outputEntry, true
	}
	port2, ok := entry[m.gatewayInfo.Port2]
	if !ok {
		return outputEntry, true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, conn := range m.activeConnections {
		portA, err := strconv.ParseUint(conn.LocalSourcePort, 10, 32)
		if err != nil {
			mlog.Errorf("error converting LocalSourcePort = %v, err = %v", conn.LocalSourcePort, err)
			continue
		}
		portB, err := strconv.ParseUint(conn.GatewayPort, 10, 32)
		if err != nil {
			mlog.Errorf("error converting GatewayPort = %v, err = %v", conn.GatewayPort, err)
			continue
		}
		if conn.LocalSourceIP == addr1 &&
			portA == uint64(port1.(uint32)) &&
			conn.GatewayIP == addr2 &&
			portB == uint64(port2.(uint32)) {
			outputEntry["ConnectionToken"] = conn.ConnectionToken
			mlog.Debugf("GatewayStruct Transform, outputEntry = %v \n", outputEntry)
			break
		} else if conn.LocalSourceIP == addr2 &&
			portA == uint64(port2.(uint32)) &&
			conn.GatewayIP == addr1 &&
			portB == uint64(port1.(uint32)) {
			outputEntry["ConnectionToken"] = conn.ConnectionToken
			mlog.Debugf("GatewayStruct Transform, outputEntry = %v \n", outputEntry)
			break
		}
	}
	mlog.Tracef("Transform output = %v", outputEntry)
	return outputEntry, true
}

func (m *GatewayStruct) receiveMessages() {
	// TBD prepare elegant exit
	mlog.Debugf("entering GatewayStruct receiveMessages")
	for {
		cInfo := api.GatewayConnectionInfo{}
		jsonStr := make([]byte, 4096)
		nBytes, err := m.conn.Read(jsonStr)
		mlog.Debugf("GatewayStruct receiveMessages, received data, nBytes = %d \n", nBytes)
		if err != nil {
			mlog.Errorf("error reading from gateway server: %v", err)
			if err == io.EOF {
				return
			}
		}
		// TODO handle receiving multiple ConnectionInfo structs together
		jsonStr2 := jsonStr[0:nBytes]
		err = json.Unmarshal([]byte(jsonStr2), &cInfo)
		if err != nil {
			mlog.Errorf("error unmarshalling data: err = %v, data = %s", err, string(jsonStr2))
			continue
		}
		// verify that we received a complete record. If not, discard it.
		// TBD improve this verification mechanism
		if cInfo.LocalSourcePort == "" {
			continue
		}
		mlog.Debugf("cInfo = %v \n", cInfo)
		m.processMessage(&cInfo)
	}
}

func (m *GatewayStruct) processMessage(cInfo *api.GatewayConnectionInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeConnections[cInfo.ConnectionToken] = cInfo
	if !cInfo.ConnectionActive {
		connTimeout := time.Now().Add(m.connTimeout)
		cInfo.ExpireTime = connTimeout
	}
}

func (m *GatewayStruct) cleanupExpiredEntries() {
	m.mu.Lock()
	defer m.mu.Unlock()
	currentTime := time.Now()
	for key, cInfo := range m.activeConnections {
		if cInfo.ExpireTime.Before(currentTime) {
			delete(m.activeConnections, key)
		}
	}
}

func (m *GatewayStruct) cleanupExpiredEntriesLoop() {
	ticker := time.NewTicker(m.cleanupLoopTime)
	go func() {
		for {
			select {
			case <-utils.ExitChannel():
				return
			case <-ticker.C:
				m.cleanupExpiredEntries()
			}
		}
	}()
}

// NewTransformGateway create a new transform
func NewTransformGateway(params config.StageParam) (transform.Transformer, error) {
	mlog.Debugf("entering NewTransformGateway")
	var serverAddr string
	if params.Transform != nil && params.Transform.Gateway != nil {
		serverAddr = params.Transform.Gateway.ServerAddr
	}

	// connect to server address
	mlog.Debugf("before connectToGatewayServer")
	conn, err := connectToGatewayServer(serverAddr)
	mlog.Debugf("after connectToGatewayServer, err = %v", err)
	if err != nil {
		return nil, err
	}

	transformGateway := &GatewayStruct{
		gatewayInfo:       params.Transform.Gateway,
		activeConnections: make(map[string]*api.GatewayConnectionInfo),
		conn:              conn,
		connTimeout:       defaultConnectionTimeout,
		cleanupLoopTime:   defaultCleanupLoopTime,
	}
	mlog.Debugf("transformGateway = %v", transformGateway)

	// set up go thread to receive inputs
	go transformGateway.receiveMessages()

	transformGateway.cleanupExpiredEntriesLoop()

	return transformGateway, nil
}

func connectToGatewayServer(serverAddr string) (*net.TCPConn, error) {
	mlog.Debugf("entering connectToGatewayServer")
	mlog.Debugf("serverAddr = %s", serverAddr)
	tcpServer, err := net.ResolveTCPAddr("tcp", serverAddr)
	mlog.Debugf("after ResolveTCPAddr")
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, tcpServer)
	mlog.Debugf("after DialTCP")
	if err != nil {
		return nil, err
	}
	return conn, nil
}
