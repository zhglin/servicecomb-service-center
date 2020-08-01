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

package notify

import "strconv"

type Type int

func (nt Type) String() string {
	if nt.IsValid() {
		return typeNames[nt]
	}
	return "Type" + strconv.Itoa(int(nt))
}

// 获取type对应的queue长度
func (nt Type) QueueSize() (s int) {
	if nt.IsValid() {
		s = typeQueues[nt]
	}
	if s <= 0 {
		s = DefaultQueueSize
	}
	return
}

// 是否合法
func (nt Type) IsValid() bool {
	return nt >= 0 && int(nt) < len(typeQueues)
}

// 全部的notify类型
var typeNames = []string{
	NOTIFTY: "NOTIFTY",
}

//每中notify类型对应的queues长度
var typeQueues = []int{
	NOTIFTY: 0,
}

func Types() (ts []Type) {
	for i := range typeNames {
		ts = append(ts, Type(i))
	}
	return
}

// 注册notify类型
func RegisterType(name string, size int) Type {
	l := len(typeNames)
	typeNames = append(typeNames, name)
	typeQueues = append(typeQueues, size)
	return Type(l)
}
