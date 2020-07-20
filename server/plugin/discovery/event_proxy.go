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
package discovery

import (
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"sync"
)

var (
	// 不同类型的事件处理函数
	eventProxies = make(map[Type]*KvEventProxy)
)
//事件的处理函数  这里的处理函数是暂存的
//因为addOn的config可能还未创建
//创建之后需要把这里的OnEvent添加到addOn的config中
type KvEventProxy struct {
	evtHandleFuncs []KvEventFunc
	lock           sync.RWMutex
}
// 添加事件处理函数
func (h *KvEventProxy) AddHandleFunc(f KvEventFunc) {
	h.lock.Lock()
	h.evtHandleFuncs = append(h.evtHandleFuncs, f)
	h.lock.Unlock()
}
// 顺序执行所有的处理函数
func (h *KvEventProxy) OnEvent(evt KvEvent) {
	h.lock.RLock()
	for _, f := range h.evtHandleFuncs {
		f(evt)
	}
	h.lock.RUnlock()
}

// InjectConfig will inject a resource changed event callback function in Config
// 把这里的事件处理函数 添加到addOn的config中 因为config中的OnEvent与这里的OnEvent都是KvEventFunc类型
func (h *KvEventProxy) InjectConfig(cfg *Config) *Config {
	return cfg.AppendEventFunc(h.OnEvent)
}

// unsafe
// 获取并设置指定类型的eventProxies
func EventProxy(t Type) *KvEventProxy {
	proxy, ok := eventProxies[t]
	if !ok {
		proxy = &KvEventProxy{}
		eventProxies[t] = proxy
	}
	return proxy
}

// the event handler/func must be good performance, or will block the event bus.
// 同样是添加处理函数 只是不是struct类型
func AddEventHandleFunc(t Type, f KvEventFunc) {
	EventProxy(t).AddHandleFunc(f)
	log.Infof("register event handle function[%s] %s", t, util.FuncName(f))
}
// 添加处理函数 struct类型
func AddEventHandler(h KvEventHandler) {
	EventProxy(h.Type()).AddHandleFunc(h.OnEvent)
	log.Infof("register event handler[%s] %s", h.Type(), util.Reflect(h).Name())
}
