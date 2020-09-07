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

package rest

import (
	"compress/gzip"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/servicecomb-service-center/pkg/grace"
	"github.com/apache/servicecomb-service-center/pkg/log"

	"github.com/NYTimes/gziphandler"
)

const (
	serverStateInit        = iota //server 初始化
	serverStateRunning            //server 已启动
	serverStateTerminating        //server 关闭中
	serverStateClosed             //server 已关闭
)

// rest server配置
type ServerConfig struct {
	Addr              string
	Handler           http.Handler
	ReadTimeout       time.Duration //读取相应超时时间，包括body
	ReadHeaderTimeout time.Duration //读取相应头的超时时间
	IdleTimeout       time.Duration //等待的最大时间 keep-live
	WriteTimeout      time.Duration //写入response的超时时间
	KeepAliveTimeout  time.Duration //tcp链接的keepAlive时长
	GraceTimeout      time.Duration //server关闭的延迟时间 非配置项
	MaxHeaderBytes    int           //请求的头最大长度
	TLSConfig         *tls.Config   //ssl配置
	Compressed        bool          // 是否开启压缩  非配置项 默认开启
	CompressMinBytes  int           // 使用压缩的临界值 非配置项 默认1.4k
}

// 默认的rest server配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadTimeout:       60 * time.Second,
		ReadHeaderTimeout: 60 * time.Second,
		IdleTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		KeepAliveTimeout:  1 * time.Minute,
		GraceTimeout:      3 * time.Second,
		MaxHeaderBytes:    16384,
		Compressed:        true,
		CompressMinBytes:  1400, // 1.4KB
	}
}

func NewServer(srvCfg *ServerConfig) *Server {
	if srvCfg == nil {
		srvCfg = DefaultServerConfig()
	}
	s := &Server{
		Server: &http.Server{
			Addr:              srvCfg.Addr,
			Handler:           srvCfg.Handler,
			TLSConfig:         srvCfg.TLSConfig,
			ReadTimeout:       srvCfg.ReadTimeout,
			ReadHeaderTimeout: srvCfg.ReadHeaderTimeout,
			IdleTimeout:       srvCfg.IdleTimeout,
			WriteTimeout:      srvCfg.WriteTimeout,
			MaxHeaderBytes:    srvCfg.MaxHeaderBytes,
		},
		KeepaliveTimeout: srvCfg.KeepAliveTimeout,
		GraceTimeout:     srvCfg.GraceTimeout,
		state:            serverStateInit,
		Network:          "tcp",
	}
	if srvCfg.Compressed && srvCfg.CompressMinBytes > 0 && srvCfg.Handler != nil {
		wrapper, _ := gziphandler.NewGzipLevelAndMinSize(gzip.DefaultCompression, srvCfg.CompressMinBytes)
		s.Handler = wrapper(srvCfg.Handler)
	}
	return s
}

type Server struct {
	*http.Server

	Network          string        // 网络类型 tcp
	KeepaliveTimeout time.Duration // conn的keepalive时间
	GraceTimeout     time.Duration //延迟关闭时间

	Listener    net.Listener // http.Server使用的listener
	netListener net.Listener
	tcpListener *TCPListener

	conns int64 // 链接数
	wg    sync.WaitGroup
	state uint8 // server端链接状态 关闭 开启
}

// serve listen  tcp的listener
func (srv *Server) Serve() (err error) {
	defer func() {
		srv.state = serverStateClosed
	}()
	defer log.Recover()
	srv.state = serverStateRunning
	// 转成http 并for调用srv.Listener.Accept
	err = srv.Server.Serve(srv.Listener)
	log.Errorf(err, "rest server serve failed")
	// accept的异常以及关闭，阻塞等待现有conn关闭
	srv.wg.Wait()
	return
}

// 增加链接数
func (srv *Server) AcceptOne() {
	defer log.Recover()
	srv.wg.Add(1)
	atomic.AddInt64(&srv.conns, 1)
}

// conn关闭减少链接数
func (srv *Server) CloseOne() bool {
	defer log.Recover()
	for {
		left := atomic.LoadInt64(&srv.conns)
		if left <= 0 {
			return false
		}
		if atomic.CompareAndSwapInt64(&srv.conns, left, left-1) {
			srv.wg.Done()
			return true
		}
	}
}

// 获取tcp的listener
func (srv *Server) Listen() error {
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}

	l, err := srv.getOrCreateListener(addr)
	if err != nil {
		return err
	}

	srv.Listener = NewTCPListener(l, srv)
	grace.RegisterFiles(addr, srv.File())
	return nil
}

func (srv *Server) ListenTLS() error {
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	l, err := srv.getOrCreateListener(addr)
	if err != nil {
		return err
	}

	srv.tcpListener = NewTCPListener(l, srv)
	srv.Listener = tls.NewListener(srv.tcpListener, srv.TLSConfig)
	grace.RegisterFiles(addr, srv.File())
	return nil
}

func (srv *Server) ListenAndServe() (err error) {
	err = srv.Listen()
	if err != nil {
		return
	}
	return srv.Serve()
}

func (srv *Server) ListenAndServeTLS(certFile, keyFile string) (err error) {
	if srv.TLSConfig == nil {
		srv.TLSConfig = &tls.Config{}
		srv.TLSConfig.Certificates = make([]tls.Certificate, 1)
		srv.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return
		}
	}
	if srv.TLSConfig.NextProtos == nil {
		srv.TLSConfig.NextProtos = []string{"h2", "http/1.1"}
	}

	err = srv.ListenTLS()
	if err != nil {
		return
	}
	return srv.Serve()
}

// RegisterListener register the instance created outside by net.Listen() in server
func (srv *Server) RegisterListener(l net.Listener) {
	srv.netListener = l
}

// 创建||获取(优雅重启) net.Listener
func (srv *Server) getOrCreateListener(addr string) (l net.Listener, err error) {
	if !grace.IsFork() {
		return srv.newListener(addr)
	}

	offset := grace.ExtraFileOrder(addr)
	if offset < 0 {
		return srv.newListener(addr)
	}

	//索引位置为i的文件描述符传过去，最终会变为值为i+3的文件描述符。ie: 索引为0的文件描述符, 最终变为文件描述符3
	f := os.NewFile(uintptr(3+offset), "")
	l, err = net.FileListener(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return
}

// 创建服务器端链接
func (srv *Server) newListener(addr string) (net.Listener, error) {
	l := srv.netListener
	if l != nil {
		return l, nil
	}
	return net.Listen(srv.Network, addr)
}

// 关闭server
func (srv *Server) Shutdown() {
	if srv.state != serverStateRunning {
		return
	}

	srv.state = serverStateTerminating
	// 关闭服务端
	err := srv.Listener.Close()
	if err != nil {
		log.Errorf(err, "server listener close failed")
	}
	//优雅关闭，等待现有conn处理完关闭
	if srv.GraceTimeout >= 0 {
		srv.gracefulStop(srv.GraceTimeout)
	}
}

// 优雅关闭
func (srv *Server) gracefulStop(d time.Duration) {
	if srv.state != serverStateTerminating {
		return
	}

	<-time.After(d)

	n := 0
	for {
		if srv.state == serverStateClosed {
			break
		}

		if srv.CloseOne() {
			n++
			continue
		}
		break
	}

	if n != 0 {
		log.Warnf("%s timed out, force close %d connection(s)", d, n)
		// 关闭server，并关闭conn
		err := srv.Server.Close()
		if err != nil {
			log.Errorf(err, "server close failed")
		}
	}
}

// 这里获取的是复制的File
func (srv *Server) File() *os.File {
	switch srv.Listener.(type) {
	case *TCPListener:
		return srv.Listener.(*TCPListener).File()
	default:
		return srv.tcpListener.File()
	}
}
