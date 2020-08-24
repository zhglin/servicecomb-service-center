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

package backend

import (
	"context"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"sync"
	"time"
)

type deferItem struct {
	ttl   int32 // in seconds 过期时间
	event discovery.KvEvent
}

// 自我保护handler
// 统计deferCheckWindow事件段内的delete事件占比是否大于Percent
type InstanceEventDeferHandler struct {
	Percent float64  // 生效的比例
	// 只读cache
	cache     discovery.CacheReader
	once      sync.Once
	enabled   bool
	// 被保护的event(过期删除的instance)
	items     map[string]*deferItem
	// 追加事件的chan eventBlockSize的换成
	pendingCh chan []discovery.KvEvent
	// 处理后的可返回的事件
	deferCh   chan discovery.KvEvent
	// 重置事件
	resetCh   chan struct{}
}

// 接受数据
func (iedh *InstanceEventDeferHandler) OnCondition(cache discovery.CacheReader, evts []discovery.KvEvent) bool {
	// 生效比例为0 不处理
	if iedh.Percent <= 0 {
		return false
	}

	iedh.once.Do(func() {
		iedh.cache = cache
		iedh.items = make(map[string]*deferItem)
		iedh.pendingCh = make(chan []discovery.KvEvent, eventBlockSize)
		iedh.deferCh = make(chan discovery.KvEvent, eventBlockSize)
		iedh.resetCh = make(chan struct{})
		// 处理事件的协程
		gopool.Go(iedh.check)
	})

	// 写入事件
	iedh.pendingCh <- evts
	return true
}

// 处理事件 生成iedh.items数据
func (iedh *InstanceEventDeferHandler) recoverOrDefer(evt discovery.KvEvent) {
	if evt.KV == nil {
		log.Errorf(nil, "defer or recover a %s nil KV", evt.Type)
		return
	}
	kv := evt.KV
	key := util.BytesToStringWithNoCopy(kv.Key)
	_, ok := iedh.items[key]
	switch evt.Type {
	// 添加 修改事件,直接recover  被保护期间注册上了,直接recover
	case registry.EVT_CREATE, registry.EVT_UPDATE:
		if ok {
			log.Infof("recovered key %s events", key)
			// return nil // no need to publish event to subscribers?
		}
		iedh.recover(evt)
	case registry.EVT_DELETE:
		if ok {
			return
		}

		instance := kv.Value.(*registry.MicroServiceInstance)
		if instance == nil {
			log.Errorf(nil, "defer or recover a %s nil Value, KV is %v", evt.Type, kv)
			return
		}

		// 保护周期  etcd租约过期时间
		ttl := instance.HealthCheck.Interval * (instance.HealthCheck.Times + 1)
		if ttl <= 0 || ttl > selfPreservationMaxTTL {
			ttl = selfPreservationMaxTTL
		}

		// 删除的事件添加到items
		iedh.items[key] = &deferItem{
			ttl:   ttl,
			event: evt,
		}
	}
}

// 返回处理后的数据
func (iedh *InstanceEventDeferHandler) HandleChan() <-chan discovery.KvEvent {
	return iedh.deferCh
}

func (iedh *InstanceEventDeferHandler) check(ctx context.Context) {
	defer log.Recover()

	t, n := time.NewTimer(deferCheckWindow), false
	interval := int32(deferCheckWindow / time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case evts := <-iedh.pendingCh: // 处理新生成的事件 收集delete事件
			// 处理事件
			for _, evt := range evts {
				iedh.recoverOrDefer(evt)
			}

			// 无删除事件
			del := len(iedh.items)
			if del == 0 {
				continue
			}

			// 已开启 跳过
			if iedh.enabled {
				continue
			}

			//是否达到开启条件
			total := iedh.cache.GetAll(nil)
			if total > selfPreservationInitCount && float64(del) >= float64(total)*iedh.Percent {
				iedh.enabled = true
				log.Warnf("self preservation is enabled, caught %d/%d(>=%.0f%%) DELETE events",
					del, total, iedh.Percent*100)
			}

			// 是否收集过delete数据 未收集过就延迟deferCheckWindow时间
			// 一次收集deferCheckWindow时间段的delete事件
			if !n {
				util.ResetTimer(t, deferCheckWindow)
				// 标记收集过
				n = true
			}
		case <-t.C: // 处理items中的delete事件
			n = false
			t.Reset(deferCheckWindow)

			// 未开启时全部recover
			if !iedh.enabled {
				for _, item := range iedh.items {
					iedh.recover(item.event)
				}
				continue
			}

			// 已开启 依次减少ttl ttl<0 recover
			for key, item := range iedh.items {
				item.ttl -= interval
				if item.ttl > 0 {
					continue
				}
				log.Warnf("defer handle timed out, removed key is %s", key)
				iedh.recover(item.event)
			}

			// 没有需要保护的event renew
			if len(iedh.items) == 0 {
				iedh.renew()
				log.Warnf("self preservation is stopped")
			}
		case <-iedh.resetCh:
			iedh.renew()
			log.Warnf("self preservation is reset")

			util.ResetTimer(t, deferCheckWindow)
		}
	}
}

// 过滤通过   (EVT_CREATE，EVT_UPDATE会替换掉EVT_DELETE事件)
func (iedh *InstanceEventDeferHandler) recover(evt discovery.KvEvent) {
	key := util.BytesToStringWithNoCopy(evt.KV.Key)
	delete(iedh.items, key)
	iedh.deferCh <- evt
}

// 关闭 并重新创建items(之前的items可能会很大)
func (iedh *InstanceEventDeferHandler) renew() {
	iedh.enabled = false
	iedh.items = make(map[string]*deferItem)
}

// 重置
// cache与etcd的数据对比,如果cache与etcd数据一致就reset
// 如果cache已经Dirty，被保护的数据也可能是dirty，也会被reset
func (iedh *InstanceEventDeferHandler) Reset() bool {
	// 开启状态设置关闭并丢弃现有的items数据
	if iedh.enabled {
		iedh.resetCh <- struct{}{}
		return true
	}
	return false
}

func NewInstanceEventDeferHandler() *InstanceEventDeferHandler {
	return &InstanceEventDeferHandler{Percent: selfPreservationPercentage}
}
