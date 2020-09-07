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
	"net"
	"os"
	"syscall"
)

// 系统的net.Listener转换为TCPListener，控制conn链接数
type TCPListener struct {
	net.Listener
	stopCh chan error
	closed bool
	server *Server
}

// 内部的TCPListener 管理net.Listener
func NewTCPListener(l net.Listener, srv *Server) (el *TCPListener) {
	el = &TCPListener{
		Listener: l,
		stopCh:   make(chan error),
		server:   srv,
	}
	go func() {
		<-el.stopCh // 阻塞chain，Close的调用会解除阻塞并调用Listen.close
		el.stopCh <- el.Listener.Close()
	}()
	return
}

// 接受请求
func (rl *TCPListener) Accept() (c net.Conn, err error) {
	tc, err := rl.Listener.(*net.TCPListener).AcceptTCP()
	if err != nil {
		return
	}

	// 设置tcp链接的keepalive
	if rl.server.KeepaliveTimeout > 0 {
		if err := tc.SetKeepAlive(true); err != nil {
			return nil, err
		}
		err = tc.SetKeepAlivePeriod(rl.server.KeepaliveTimeout)
		if err != nil {
			return nil, err
		}
	}

	c = restConn{
		Conn:   tc,
		server: rl.server,
	}

	// 设置链接数
	rl.server.AcceptOne()
	return
}

// 关闭链接
func (rl *TCPListener) Close() error {
	if rl.closed {
		return syscall.EINVAL
	}
	rl.closed = true
	rl.stopCh <- nil   // 触发rl.Listener.Close()
	return <-rl.stopCh // 阻塞等待rl.Listener.Close()
}

func (rl *TCPListener) File() *os.File {
	tl := rl.Listener.(*net.TCPListener)
	fl, _ := tl.File()
	return fl
}
