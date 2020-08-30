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

package discovery

import (
	"encoding/json"
	"fmt"
	simple "github.com/apache/servicecomb-service-center/pkg/time"
	"github.com/apache/servicecomb-service-center/pkg/types"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	"strconv"
	"time"
)

var (
	Types     []Type    //已注册的类型
	typeNames []string  //对应的已注册的类型名称
)

const (
	TypeError = Type(-1)
)
// etcd中数据类型
type Type int

// 转换成名称字符串
func (st Type) String() string {
	if int(st) < 0 {
		return "TypeError"
	}
	if int(st) < len(typeNames) {
		return typeNames[st]
	}
	return "TYPE" + strconv.Itoa(int(st))
}

// 注册数据类型
func RegisterType(name string) (newId Type, err error) {
	// 已注册
	for _, n := range Types {
		if n.String() == name {
			return TypeError, fmt.Errorf("redeclare store type '%s'", n)
		}
	}
	// name对应的id就是 已注册的类型的长度
	newId = Type(len(Types))
	Types = append(Types, newId)
	typeNames = append(typeNames, name)
	return
}
// 缓存中的数据
type KeyValue struct {
	Key            []byte
	Value          interface{}
	Version        int64 // 就是一个计数器，代表了 KeyValue 被修改了多少次
	CreateRevision int64 // KeyValue 在创建的时候生成的版本号 revision
	ModRevision    int64 // 数据被操作的时候对应的版本号 revision
	ClusterName    string
}

func (kv *KeyValue) String() string {
	b, _ := json.Marshal(kv.Value)
	return fmt.Sprintf("{key: '%s', value: %s, version: %d, cluster: '%s'}",
		util.BytesToStringWithNoCopy(kv.Key), util.BytesToStringWithNoCopy(b), kv.Version, kv.ClusterName)
}
// 创建cache的value
func NewKeyValue() *KeyValue {
	return &KeyValue{ClusterName: registry.Configuration().ClusterName}
}

type Response struct {
	Kvs   []*KeyValue
	Count int64
}
// 事件
type KvEvent struct {
	Revision int64 //版本号
	Type     types.EventType //事件类型
	KV       *KeyValue //数据
	CreateAt simple.Time //创建时间
}
// 事件处理函数
type KvEventFunc func(evt KvEvent)
// 事件处理器
type KvEventHandler interface {
	Type() Type	//数据类型
	OnEvent(evt KvEvent) //处理函数 类型就是KvEventFunc
}

// 创建事件
func NewKvEvent(action types.EventType, kv *KeyValue, rev int64) KvEvent {
	return KvEvent{Type: action, KV: kv, Revision: rev, CreateAt: simple.FromTime(time.Now())}
}