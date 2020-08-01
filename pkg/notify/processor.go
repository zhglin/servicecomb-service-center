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
	"context"
	"github.com/apache/servicecomb-service-center/pkg/queue"
	"github.com/apache/servicecomb-service-center/pkg/util"
)

// 同notify.type类型的所有subject(主题)
type Processor struct {
	*queue.TaskQueue	//task的处理

	name     string // notify.type的名称
	subjects *util.ConcurrentMap //subscriber的subject(主题)对应的subject结构，一个主题可以对应多个订阅者
}

// 当前notify.type的名称
func (p *Processor) Name() string {
	return p.name
}

// 接收事件
func (p *Processor) Accept(job Event) {
	p.Add(queue.Task{Object: job})
}

// 事件处理器函数，统一的事件处理
func (p *Processor) Handle(ctx context.Context, obj interface{}) {
	p.Notify(obj.(Event))
}

// 具体事件处理函数 可以进行独立通知
func (p *Processor) Notify(job Event) {
	// 根据Job的subject进行分发
	if itf, ok := p.subjects.Get(job.Subject()); ok {
		itf.(*Subject).Notify(job)
	}
}

func (p *Processor) Subjects(name string) *Subject {
	itf, ok := p.subjects.Get(name)
	if !ok {
		return nil
	}
	return itf.(*Subject)
}

// 添加subscriber
func (p *Processor) AddSubscriber(n Subscriber) {
	// 添加subscriber对应的subject
	item, _ := p.subjects.Fetch(n.Subject(), func() (interface{}, error) {
		return NewSubject(n.Subject()), nil
	})
	// subject处理subscriber
	item.(*Subject).GetOrNewGroup(n.Group()).AddSubscriber(n)
}

// 删除一个subscriber
func (p *Processor) Remove(n Subscriber) {
	itf, ok := p.subjects.Get(n.Subject())
	if !ok {
		return
	}

	s := itf.(*Subject)
	g := s.Groups(n.Group())
	if g == nil {
		return
	}

	g.Remove(n.ID())

	// 如果删除后的组中没有订阅者了，就删除整个组
	if g.Size() == 0 {
		s.Remove(g.Name())
	}

	// 如果删除后的主题下面没有订阅者组了，就删除主题
	if s.Size() == 0 {
		p.subjects.Remove(s.Name())
	}
}

// 清空subjects
func (p *Processor) Clear() {
	p.subjects.Clear()
}

func NewProcessor(name string, queueSize int) *Processor {
	p := &Processor{
		TaskQueue: queue.NewTaskQueue(queueSize),
		name:      name,
		subjects:  util.NewConcurrentMap(0),
	}

	// worker processor具有worker接口
	p.AddWorker(p)
	return p
}
