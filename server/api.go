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

package server

import (
	"context"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/grace"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/rest"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	rs "github.com/apache/servicecomb-service-center/server/rest"
	"github.com/apache/servicecomb-service-center/server/service"
	"net"
	"strconv"
)

var apiServer *APIServer

func init() {
	InitAPI()

	apiServer = &APIServer{
		isClose:   true, //初始关闭状态
		err:       make(chan error, 1),
		goroutine: gopool.New(context.Background()),
	}
}

func InitAPI() {
	core.ServiceAPI, core.InstanceAPI = service.AssembleResources()
}

type APIType int64

func (t APIType) String() string {
	switch t {
	case RPC:
		return "grpc" // support grpc
	case REST:
		return "rest"
	default:
		return "SCHEME" + strconv.Itoa(int(t))
	}
}

type APIServer struct {
	Listeners map[APIType]string // api协议对应的ip:port

	restSrv   *rest.Server
	isClose   bool
	forked    bool       // 标记是否重启
	err       chan error // 收集error 上游阻塞在chan上
	goroutine *gopool.Pool
}

const (
	RPC  APIType = 0
	REST APIType = 1
)

func (s *APIServer) Err() <-chan error {
	return s.err
}

// 优雅重启
func (s *APIServer) graceDone() {
	grace.Before(s.MarkForked)           // 进行fork重启先标记s.forked=true，不进行instance的下线操作
	grace.After(s.Stop)                  // 重启完关闭apiServer
	if err := grace.Done(); err != nil { // 关闭父进程
		log.Errorf(err, "server reload failed")
	}
}

func (s *APIServer) MarkForked() {
	s.forked = true
}

// 添加api协议类型对应的ip，port
func (s *APIServer) AddListener(t APIType, ip, port string) {
	if s.Listeners == nil {
		s.Listeners = map[APIType]string{}
	}
	if len(ip) == 0 {
		return
	}
	s.Listeners[t] = net.JoinHostPort(ip, port)
}

// 设置instance的api的ip端口号
func (s *APIServer) populateEndpoint(t APIType, ipPort string) {
	if len(ipPort) == 0 {
		return
	}
	address := fmt.Sprintf("%s://%s/", t, ipPort)
	if core.ServerInfo.Config.SslEnabled {
		address += "?sslEnabled=true"
	}
	core.Instance.Endpoints = append(core.Instance.Endpoints, address)
}

// 启动rest server
func (s *APIServer) startRESTServer() (err error) {
	addr, ok := s.Listeners[REST]
	if !ok {
		return
	}
	s.restSrv, err = rs.NewServer(addr)
	if err != nil {
		return
	}
	log.Infof("listen address: %s://%s", REST, s.restSrv.Listener.Addr().String())

	// 记录ip端口号信息
	s.populateEndpoint(REST, s.restSrv.Listener.Addr().String())

	// 协程开启server
	s.goroutine.Do(func(_ context.Context) {
		err := s.restSrv.Serve()
		if s.isClose {
			return
		}
		log.Errorf(err, "error to start REST API server %s", addr)
		s.err <- err
	})
	return
}

// 启动
func (s *APIServer) Start() {
	if !s.isClose {
		return
	}
	s.isClose = false

	core.Instance.Endpoints = nil

	// 启动rest api
	err := s.startRESTServer()
	if err != nil {
		s.err <- err
		return
	}

	// 优雅重启
	s.graceDone()

	defer log.Info("api server is ready")

	// 不需要自注册
	if !core.ServerInfo.Config.SelfRegister {
		log.Warnf("self register disabled")
		return
	}

	// 自注册
	err = backend.GetRegistryEngine().Start()
	if err != nil {
		s.err <- err
		return
	}
}

// 关闭apiServer
func (s *APIServer) Stop() {
	if s.isClose {
		return
	}
	s.isClose = true

	// 优雅重启状态不删除instance
	if !s.forked && core.ServerInfo.Config.SelfRegister {
		backend.GetRegistryEngine().Stop()
	}

	if s.restSrv != nil {
		s.restSrv.Shutdown()
	}

	close(s.err)

	s.goroutine.Close(true)

	log.Info("api server stopped")
}

func GetAPIServer() *APIServer {
	return apiServer
}
