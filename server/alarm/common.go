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
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/notify"
	"github.com/apache/servicecomb-service-center/server/alarm/model"
)

const (
	// 异常状态
	Activated model.Status = "ACTIVATED" // 激活
	Cleared   model.Status = "CLEARED"   // 恢复
)

const (
	// 异常类型
	IDBackendConnectionRefuse model.ID = "BackendConnectionRefuse"  // 标记依赖系统的异常 比如etcd
	IDInternalError           model.ID = "InternalError" // api接口异常 比如添加数据接口
)

const (
	FieldAdditionalContext = "detail" // 异常信息的key
)

const (
	Subject = "__ALARM_SUBJECT__" // 订阅者主题
	Group   = "__ALARM_GROUP__"   // 订阅者组
)

// 注册notify类型
var ALARM = notify.RegisterType("ALARM", 0)

func FieldBool(key string, v bool) model.Field {
	return model.Field{Key: key, Value: v}
}

// 创建model.field
func FieldString(key string, v string) model.Field {
	return model.Field{Key: key, Value: v}
}

func FieldInt64(key string, v int64) model.Field {
	return model.Field{Key: key, Value: v}
}

func FieldInt(key string, v int) model.Field {
	return model.Field{Key: key, Value: v}
}

func FieldFloat64(key string, v float64) model.Field {
	return model.Field{Key: key, Value: v}
}

//创建个有异常信息的model.Field
func AdditionalContext(format string, args ...interface{}) model.Field {
	return FieldString(FieldAdditionalContext, fmt.Sprintf(format, args...))
}

func ListAll() []*model.AlarmEvent {
	return Center().ListAll()
}

// 发布事件
func Raise(id model.ID, fields ...model.Field) error {
	return Center().Raise(id, fields...)
}

// 发布事件
func Clear(id model.ID) error {
	return Center().Clear(id)
}

// 清空所有异常
func ClearAll() {
	Center().ClearAll()
}
