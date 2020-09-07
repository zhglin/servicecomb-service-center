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

// 主题
type Subject struct {
	name string //主题名称
	//相同主题可以通知到不同的订阅者组，组名称=>Group，一个主题对应多个group
	groups *util.ConcurrentMap
}

func (s *Subject) Name() string {
	return s.name
}

// 通知各个订阅者
func (s *Subject) Notify(job Event) {
	// job不区分组，全部通知
	if len(job.Group()) == 0 {
		s.groups.ForEach(func(item util.MapItem) (next bool) {
			item.Value.(*Group).Notify(job)
			return true
		})
		return
	}

	// 获取特定组进行通知
	itf, ok := s.groups.Get(job.Group())
	if !ok {
		return
	}
	itf.(*Group).Notify(job)
}

// 获取name对应的订阅者组
func (s *Subject) Groups(name string) *Group {
	g, ok := s.groups.Get(name)
	if !ok {
		return nil
	}
	return g.(*Group)
}

// 添加并返回group
func (s *Subject) GetOrNewGroup(name string) *Group {
	item, _ := s.groups.Fetch(name, func() (interface{}, error) {
		return NewGroup(name), nil
	})
	return item.(*Group)
}

// 删除指定的组
func (s *Subject) Remove(name string) {
	s.groups.Remove(name)
}

// 当前主题下的组的数量
func (s *Subject) Size() int {
	return s.groups.Size()
}

func NewSubject(name string) *Subject {
	return &Subject{
		name:   name,
		groups: util.NewConcurrentMap(0),
	}
}
