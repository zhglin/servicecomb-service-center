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
	"errors"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/task"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/plugin"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	"time"
)

var store = &KvStore{}

func init() {
	store.Initialize()   // 创建KvStore结构体
	registerInnerTypes() // 注册各个discovery.Type
}

// discovery etcd中的数据管理器
type KvStore struct {
	// 各个类型的addOn配置
	AddOns map[discovery.Type]AddOn
	// 所有类型的adaptors实例
	adaptors util.ConcurrentMap
	// 异步任务处理器
	taskService task.Service
	// 是否就绪
	ready     chan struct{}
	goroutine *gopool.Pool
	isClose   bool
	// 最新的版本号
	rev int64
}

func (s *KvStore) Initialize() {
	s.AddOns = make(map[discovery.Type]AddOn)
	s.taskService = task.NewTaskService()
	s.ready = make(chan struct{})
	s.goroutine = gopool.New(context.Background())
}

// kvStore的事件处理函数 记录最新的版本号
func (s *KvStore) OnCacheEvent(evt discovery.KvEvent) {
	if s.rev < evt.Revision {
		s.rev = evt.Revision
	}
}

//为addOn添加自身的事件处理函数
func (s *KvStore) InjectConfig(cfg *discovery.Config) *discovery.Config {
	return cfg.AppendEventFunc(s.OnCacheEvent)
}

// 获取discovery组件实例
func (s *KvStore) repo() discovery.AdaptorRepository {
	return plugin.Plugins().Discovery()
}

// 获取或创建指定类型的adaptor
func (s *KvStore) getOrCreateAdaptor(t discovery.Type) discovery.Adaptor {
	v, _ := s.adaptors.Fetch(t, func() (interface{}, error) {
		addOn, ok := s.AddOns[t]
		if ok {
			// 创建并指定对应的adaptor  每个类型一个adaptor实例
			adaptor := s.repo().New(t, addOn.(AddOn).Config())
			adaptor.Run()
			return adaptor, nil
		}
		log.Warnf("type '%s' not found", t)
		return nil, nil
	})
	return v.(discovery.Adaptor)
}

// 启动函数 server启动时调用
func (s *KvStore) Run() {
	s.goroutine.Do(s.store)
	s.goroutine.Do(s.autoClearCache)
	s.taskService.Run()
}

// 创建所有类型的adaptor
func (s *KvStore) store(ctx context.Context) {
	// new all types
	for _, t := range discovery.Types {
		select {
		case <-ctx.Done():
			return
			// 一直阻塞 直到adaptor准备好数据
			// 如果启动过程中有panic，会触发panic
		case <-s.getOrCreateAdaptor(t).Ready():
		}
	}
	// 所有的adaptor都已初始化好数据
	util.SafeCloseChan(s.ready)

	log.Debugf("all adaptors are ready")
}

// 定时清理cache
// 会先把cache清空，本次cache获取不到数据，会到etcd进行重试
// cache会进行全量数据的同步
func (s *KvStore) autoClearCache(ctx context.Context) {
	// 未设置清理间隔
	if core.ServerInfo.Config.CacheTTL == 0 {
		return
	}

	log.Infof("start auto clear cache in %v", core.ServerInfo.Config.CacheTTL)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(core.ServerInfo.Config.CacheTTL):
			for _, t := range discovery.Types {
				cache, ok := s.getOrCreateAdaptor(t).Cache().(discovery.Cache)
				if !ok {
					log.Error("the discovery adaptor does not implement the Cache", nil)
					continue
				}
				//标记缓存为脏数据
				cache.MarkDirty()
			}
			log.Warnf("caches are marked dirty!")
		}
	}
}

func (s *KvStore) Stop() {
	if s.isClose {
		return
	}
	s.isClose = true

	s.adaptors.ForEach(func(item util.MapItem) bool {
		item.Value.(discovery.Adaptor).Stop()
		return true
	})

	s.taskService.Stop()

	s.goroutine.Close(true)

	util.SafeCloseChan(s.ready)

	log.Debugf("store daemon stopped")
}

// 是否已就绪
func (s *KvStore) Ready() <-chan struct{} {
	<-s.taskService.Ready()
	return s.ready
}

// 注册addOn
func (s *KvStore) Install(addOn AddOn) (id discovery.Type, err error) {
	if addOn == nil || len(addOn.Name()) == 0 || addOn.Config() == nil {
		return discovery.TypeError, errors.New("invalid parameter")
	}
	// 注册name 并转换成数字标识
	id, err = discovery.RegisterType(addOn.Name())
	if err != nil {
		return
	}
	// 合并EventProxy的onEvent以及addOn的onEvent
	discovery.EventProxy(id).InjectConfig(addOn.Config())
	//注册kvStore的事件处理函数
	s.InjectConfig(addOn.Config())
	//注册到AddOns
	s.AddOns[id] = addOn

	log.Infof("install new type %d:%s->%s", id, addOn.Name(), addOn.Config().Key)
	return
}

// 注册类型配置 异常直接panic
func (s *KvStore) MustInstall(addOn AddOn) discovery.Type {
	id, err := s.Install(addOn)
	if err != nil {
		panic(err)
	}
	return id
}

func (s *KvStore) Adaptors(id discovery.Type) discovery.Adaptor { return s.getOrCreateAdaptor(id) }
func (s *KvStore) Service() discovery.Adaptor                   { return s.Adaptors(SERVICE) }
func (s *KvStore) SchemaSummary() discovery.Adaptor             { return s.Adaptors(SchemaSummary) }
func (s *KvStore) Instance() discovery.Adaptor                  { return s.Adaptors(INSTANCE) }
func (s *KvStore) Lease() discovery.Adaptor                     { return s.Adaptors(LEASE) }
func (s *KvStore) ServiceIndex() discovery.Adaptor              { return s.Adaptors(ServiceIndex) }
func (s *KvStore) ServiceAlias() discovery.Adaptor              { return s.Adaptors(ServiceAlias) }
func (s *KvStore) ServiceTag() discovery.Adaptor                { return s.Adaptors(ServiceTag) }
func (s *KvStore) Rule() discovery.Adaptor                      { return s.Adaptors(RULE) }
func (s *KvStore) RuleIndex() discovery.Adaptor                 { return s.Adaptors(RuleIndex) }
func (s *KvStore) Schema() discovery.Adaptor                    { return s.Adaptors(SCHEMA) }
func (s *KvStore) DependencyRule() discovery.Adaptor            { return s.Adaptors(DependencyRule) }
func (s *KvStore) DependencyQueue() discovery.Adaptor           { return s.Adaptors(DependencyQueue) }
func (s *KvStore) Domain() discovery.Adaptor                    { return s.Adaptors(DOMAIN) }
func (s *KvStore) Project() discovery.Adaptor                   { return s.Adaptors(PROJECT) }

// KeepAlive will always return ok when registry is unavailable
// unless the registry response is LeaseNotFound
// 续租 支持异步
func (s *KvStore) KeepAlive(ctx context.Context, opts ...registry.PluginOpOption) (int64, error) {
	op := registry.OpPut(opts...)

	t := NewLeaseAsyncTask(op)
	if op.Mode == registry.ModeNoCache {
		log.Debugf("keep alive lease WitchNoCache, request etcd server, op: %s", op)
		err := t.Do(ctx)
		ttl := t.TTL
		return ttl, err
	}

	// service_center节点异步
	err := s.taskService.Add(ctx, t)
	if err != nil {
		return 0, err
	}
	//t.key = domain/project/serviceId/instanceId
	itf, err := s.taskService.LatestHandled(t.Key())
	if err != nil {
		return 0, err
	}
	pt := itf.(*LeaseTask)
	return pt.TTL, pt.Err()
}

func Store() *KvStore {
	return store
}

func Revision() int64 {
	return store.rev
}
