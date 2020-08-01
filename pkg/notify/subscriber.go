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
	"github.com/apache/servicecomb-service-center/pkg/util"
)

//订阅者接口
type Subscriber interface {
	// 订阅者标识
	ID() string
	// 订阅的主题
	Subject() string
	// 订阅者所在组
	Group() string
	// notify类型
	Type() Type
	// notification_service 通过service操作所有的notify数据
	Service() *Service
	SetService(*Service)

	Err() error
	SetError(err error)

	// 关闭
	Close()
	OnAccept()
	// The event bus will callback this function, so it must be non-blocked.
	// 事件的回调函数 上层同步调用
	OnMessage(Event)
}

// 订阅者
type baseSubscriber struct {
	// notify类型
	nType   Type
	// 订阅者id
	id      string
	// 订阅主题
	subject string
	// 主题组
	group   string
	// notifyService
	service *Service
	err     error
}

func (s *baseSubscriber) ID() string              { return s.id }
func (s *baseSubscriber) Subject() string         { return s.subject }
func (s *baseSubscriber) Group() string           { return s.group }
func (s *baseSubscriber) Type() Type              { return s.nType }
func (s *baseSubscriber) Service() *Service       { return s.service }
func (s *baseSubscriber) SetService(svc *Service) { s.service = svc }
func (s *baseSubscriber) Err() error              { return s.err }
func (s *baseSubscriber) SetError(err error)      { s.err = err }
func (s *baseSubscriber) Close()                  {}
func (s *baseSubscriber) OnAccept()               {}
func (s *baseSubscriber) OnMessage(job Event) {
	s.SetError(errors.New("do not call base notifier OnMessage method"))
}

// 创建订阅者 消费消息
func NewSubscriber(nType Type, subject, group string) Subscriber {
	return &baseSubscriber{
		id:      util.GenerateUUID(),
		group:   group,
		subject: subject,
		nType:   nType,
	}
}
