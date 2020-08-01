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
	"github.com/apache/servicecomb-service-center/pkg/util"
)

// 订阅者组
type Group struct {
	name        string // 组名称
	// 相同group的订阅者  订阅者id=>订阅者  一个group中多个订阅者
	subscribers *util.ConcurrentMap
}

func (g *Group) Name() string {
	return g.name
}

// 调用订阅者的onMessage函数处理事件
func (g *Group) Notify(job Event) {
	g.subscribers.ForEach(func(item util.MapItem) (next bool) {
		item.Value.(Subscriber).OnMessage(job)
		return true
	})
}

//获取指定的订阅者
func (g *Group) Subscribers(name string) Subscriber {
	s, ok := g.subscribers.Get(name)
	if !ok {
		return nil
	}
	return s.(Subscriber)
}

// 在当前组中添加订阅者
func (g *Group) AddSubscriber(subscriber Subscriber) Subscriber {
	return g.subscribers.PutIfAbsent(subscriber.ID(), subscriber).(Subscriber)
}

//重组中删除name对应的订阅者
func (g *Group) Remove(name string) {
	g.subscribers.Remove(name)
}

// 当前组中的订阅者数量
func (g *Group) Size() int {
	return g.subscribers.Size()
}

func NewGroup(name string) *Group {
	return &Group{
		name:        name,
		subscribers: util.NewConcurrentMap(0),
	}
}
