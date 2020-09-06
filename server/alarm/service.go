// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alarm

import (
	"github.com/apache/servicecomb-service-center/pkg/log"
	nf "github.com/apache/servicecomb-service-center/pkg/notify"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/alarm/model"
	"github.com/apache/servicecomb-service-center/server/notify"
	"sync"
)

var (
	service *Service
	once    sync.Once
)

// 报警服务
// 利用notify组件 收集系统内部异常
type Service struct {
	nf.Subscriber
	alarms util.ConcurrentMap  // 保存出现的异常
}

// 发布异常事件（出现异常）
func (ac *Service) Raise(id model.ID, fields ...model.Field) error {
	ae := &model.AlarmEvent{
		Event:  nf.NewEvent(ALARM, Subject, ""),
		Status: Activated,	// 已出现异常
		ID:     id,
		Fields: util.NewJSONObject(), // 异常信息
	}

	// 设置异常信息
	for _, f := range fields {
		ae.Fields[f.Key] = f.Value
	}
	return notify.GetNotifyCenter().Publish(ae) //发布事件
}

// 发布清空异常事件 （异常恢复）
func (ac *Service) Clear(id model.ID) error {
	ae := &model.AlarmEvent{
		Event:  nf.NewEvent(ALARM, Subject, ""),
		Status: Cleared,
		ID:     id,
	}
	return notify.GetNotifyCenter().Publish(ae)
}

// 获取所有异常
func (ac *Service) ListAll() (ls []*model.AlarmEvent) {
	ac.alarms.ForEach(func(item util.MapItem) (next bool) {
		ls = append(ls, item.Value.(*model.AlarmEvent))
		return true
	})
	return
}

// 清空异常
func (ac *Service) ClearAll() {
	ac.alarms = util.ConcurrentMap{}
}

// 处理事件
func (ac *Service) OnMessage(evt nf.Event) {
	alarm := evt.(*model.AlarmEvent)
	switch alarm.Status {
	case Cleared: // cleared 异常已恢复状态 变更异常状态
		if itf, ok := ac.alarms.Get(alarm.ID); ok {
			if exist := itf.(*model.AlarmEvent); exist.Status != Cleared {
				exist.Status = Cleared
				alarm = exist
			}
		}
	default:
		ac.alarms.Put(alarm.ID, alarm)  // 记录异常
	}
	log.Debugf("alarm[%s] %s, %v", alarm.ID, alarm.Status, alarm.Fields)
}

// 创建alarm实例
func NewAlarmService() *Service {
	c := &Service{
		// 创建一个订阅者(订阅者类型，订阅主题，订阅者组)
		Subscriber: nf.NewSubscriber(ALARM, Subject, Group),
	}

	// 添加订阅者
	err := notify.GetNotifyCenter().AddSubscriber(c)
	if err != nil {
		log.Error("", err)
	}
	return c
}

// 获取一个alarm实例
func Center() *Service {
	once.Do(func() {
		service = NewAlarmService()
	})
	return service
}
