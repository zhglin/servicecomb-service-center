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
	simple "github.com/apache/servicecomb-service-center/pkg/time"
	"time"
)

// 事件接口
type Event interface {
	// notify.type类型,区分processor
	Type() Type
	Subject() string // 当前事件对应的主题 required!
	Group() string   // 当前事件对应的组，不填就通知所有主题下的订阅者，broadcast all the subscriber of the same subject if group is empty
	CreateAt() time.Time // 创建时间
}

type baseEvent struct {
	nType    Type	//notify 类型
	subject  string
	group    string
	createAt simple.Time
}

func (s *baseEvent) Type() Type {
	return s.nType
}

func (s *baseEvent) Subject() string {
	return s.subject
}

func (s *baseEvent) Group() string {
	return s.group
}

func (s *baseEvent) CreateAt() time.Time {
	return s.createAt.Local()
}

func NewEvent(t Type, s, g string) Event {
	return NewEventWithTime(t, s, g, simple.FromTime(time.Now()))
}

func NewEventWithTime(t Type, s, g string, now simple.Time) Event {
	return &baseEvent{t, s, g, now}
}
