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

package notify

import (
	"errors"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"sync"
)

/*
事件订阅
每个事件包含subject，group，type信息。
每个type对应一个processor，taskQueue
每个processor包含subject
每个subject包含group
每个group包含多个组
每个group有多个订阅者

taskQueue里面有chan，保存事件
有多个worker对应事件处理回调

processor.run启动process，从队列中取事件，最终调用订阅者的onMessage
*/

// notify 管理所有的数据 给各个订阅者操作的权限
type Service struct {
	processors map[Type]*Processor
	mux        sync.RWMutex
	isClose    bool
}

// 创建并执行processor
func (s *Service) newProcessor(t Type) *Processor {
	s.mux.RLock()
	p, ok := s.processors[t]
	if ok {
		s.mux.RUnlock()
		return p
	}
	s.mux.RUnlock()

	s.mux.Lock()
	p, ok = s.processors[t]
	if ok {
		s.mux.Unlock()
		return p
	}

	//新建processor
	p = NewProcessor(t.String(), t.QueueSize())
	s.processors[t] = p
	s.mux.Unlock()

	p.Run() //初次添加执行processor
	return p
}

func (s *Service) Start() {
	if !s.Closed() {
		log.Warnf("notify service is already running")
		return
	}
	s.mux.Lock()
	s.isClose = false
	s.mux.Unlock()

	// 错误subscriber清理
	err := s.AddSubscriber(NewNotifyServiceHealthChecker())
	if err != nil {
		log.Error("", err)
	}

	log.Debugf("notify service is started")
}

// 添加一个subscriber 订阅者
func (s *Service) AddSubscriber(n Subscriber) error {
	if n == nil {
		err := errors.New("required Subscriber")
		log.Errorf(err, "add subscriber failed")
		return err
	}

	// 订阅者的notify类型不合法
	if !n.Type().IsValid() {
		err := errors.New("unknown subscribe type")
		log.Errorf(err, "add %s subscriber[%s/%s] failed", n.Type(), n.Subject(), n.Group())
		return err
	}

	p := s.newProcessor(n.Type())
	n.SetService(s) //把service设置到subscriber中，让订阅者具有管理功能
	n.OnAccept()    // 初次添加订阅者调用订阅者的OnAccept函数，做下必要的初始化工作

	// process 添加subscriber
	p.AddSubscriber(n)
	return nil
}

// 删除一个subscriber 会级联删除group，subject，不会删除processor
func (s *Service) RemoveSubscriber(n Subscriber) {
	s.mux.RLock()
	p, ok := s.processors[n.Type()]
	if !ok {
		s.mux.RUnlock()
		return
	}
	s.mux.RUnlock()

	p.Remove(n)
	// 调用订阅者的close方法
	n.Close()
}

// 终止各个processor
func (s *Service) stopProcessors() {
	s.mux.RLock()
	for _, p := range s.processors {
		p.Clear() //清空processor.subjects 主题
		p.Stop()  //终止processor的协程
	}
	s.mux.RUnlock()
}

// 通知内容塞到队列里
func (s *Service) Publish(job Event) error {
	if s.Closed() {
		return errors.New("add notify job failed for server shutdown")
	}

	s.mux.RLock()
	p, ok := s.processors[job.Type()]
	if !ok {
		s.mux.RUnlock()
		return errors.New("Unknown job type")
	}
	s.mux.RUnlock()
	// 写入事件
	p.Accept(job)
	return nil
}

// 是否已关闭
func (s *Service) Closed() (b bool) {
	s.mux.RLock()
	b = s.isClose
	s.mux.RUnlock()
	return
}

// 关闭notify
func (s *Service) Stop() {
	if s.Closed() {
		return
	}
	s.mux.Lock()
	s.isClose = true
	s.mux.Unlock()

	// 关闭所有的processor
	s.stopProcessors()

	log.Debug("notify service stopped")
}

func NewNotifyService() *Service {
	return &Service{
		processors: make(map[Type]*Processor),
		isClose:    true,
	}
}
