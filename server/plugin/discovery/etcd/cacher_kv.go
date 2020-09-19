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
package etcd

import (
	"context"
	"errors"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/backoff"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	rmodel "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"sync"
	"time"
)

// KvCacher implements discovery.Cacher.
// KvCacher manages etcd cache.
// To update cache, KvCacher watch etcd event and pull data periodly from etcd.
// When the cache data changes, KvCacher creates events and notifies it's
// subscribers.
// Use Cfg to set it's behaviors.
// 缓存管理器 同步etcd数据写入KvCacher
// 如果出现panic会导致终止
type KvCacher struct {
	// discovery.Type的配置
	Cfg *discovery.Config

	reListCount int

	ready chan struct{}
	// 关联register提供 list watch接口
	lw        ListWatch
	mux       sync.Mutex
	once      sync.Once
	cache     discovery.Cache
	goroutine *gopool.Pool
}

func (c *KvCacher) Config() *discovery.Config {
	return c.Cfg
}

// 是否需要拉取全量数据
func (c *KvCacher) needList() bool {
	// listWatch中的版本号为0需要全量拉取，watch的异常会重置rev=0
	rev := c.lw.Revision()
	if rev == 0 {
		c.reListCount = 0
		return true
	}
	// 每DefaultForceListInterval次 返回tre 需要全量拉取
	c.reListCount++
	if c.reListCount < DefaultForceListInterval {
		return false
	}
	c.reListCount = 0
	return true
}

// 从listWatch中获取全量数据，需要跟本地cache比较生成事件进行同步
func (c *KvCacher) doList(cfg ListWatchConfig) error {
	resp, err := c.lw.List(cfg)
	if err != nil {
		return err
	}

	rev := c.lw.Revision()
	kvs := resp.Kvs
	start := time.Now()
	defer log.DebugOrWarnf(start, "finish to cache key %s, %d items, rev: %d",
		c.Cfg.Key, len(kvs), rev)

	// just reset the cacher if cache marked dirty
	// 如果cache被设置成脏的 需要重置cache 定时设置 不会进行事件通知
	if c.cache.Dirty() {
		c.reset(rev, kvs)
		log.Warnf("Cache[%s] is reset!", c.cache.Name())
		return nil
	}

	// calc and return the diff between cache and ETCD
	// 过滤生成变更事件，跟本地cache比较生成事件
	evts := c.filter(rev, kvs)
	// there is no change between List() and cache, then stop the self preservation
	// etcd的ttl的过期会触发自我保护，重新续约会才取消，这个时候一定会有事件产生
	if ec, kc := len(evts), len(kvs); c.Cfg.DeferHandler != nil && ec == 0 && kc != 0 &&
		c.Cfg.DeferHandler.Reset() {
		log.Warnf("most of the protected data(%d/%d) are recovered",
			kc, c.cache.GetAll(nil))
	}

	// notify the subscribers
	c.sync(evts)
	return nil
}

// 全量重置cache，不会进行事件通知，可能会丢事件
func (c *KvCacher) reset(rev int64, kvs []*mvccpb.KeyValue) {
	// 重置DeferHandler
	if c.Cfg.DeferHandler != nil {
		c.Cfg.DeferHandler.Reset()
	}
	// clear cache before Set is safe, because the watch operation is stop,
	// but here will make all API requests go to ETCD directly. 这里将使所有API请求直接到ETCD。
	c.cache.Clear()
	// do not notify when cacher is dirty status,		当cacher是脏状态时不通知
	// otherwise, too many events will notify to downstream. 否则，将有太多事件通知下游。
	c.buildCache(c.filter(rev, kvs))
}

// 返回的是innerWatcher
func (c *KvCacher) doWatch(cfg ListWatchConfig) error {
	if watcher := c.lw.Watch(cfg); watcher != nil {
		return c.handleWatcher(watcher)
	}
	return fmt.Errorf("handle a nil watcher")
}

// 拉取register中数据
func (c *KvCacher) ListAndWatch(ctx context.Context) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	// 忽略panic
	defer log.Recover() // ensure ListAndWatch never raise panic

	cfg := ListWatchConfig{
		Timeout: c.Cfg.Timeout,
		Context: ctx, //ctx是goPool中的ctx, goPool关闭会导致watch链接的关闭
	}

	// the scenario need to list etcd:
	// 1. Initial: cache is building, the lister's revision is 0.
	// 2. Runtime: error occurs in previous watch operation, the lister's revision is set to 0.
	// 3. Runtime: watch operation timed out over DEFAULT_FORCE_LIST_INTERVAL times.
	// watch DefaultForceListInterval次或者watch异常会全量拉取，watch异常全量拉取的时候需要跟cache比较创建事件
	if c.needList() {
		// 全量获取 cache的定期全量生成 只在doList阶段处理
		if err := c.doList(cfg); err != nil && (!c.IsReady() || c.lw.Revision() == 0) {
			return err // do retry to list etcd
		}
		// keep going to next step:
		// 1. doList return OK.
		// 2. some traps in etcd client, like the limitation of max response body(4MB),
		//    doList always return error. So call doWatch to compensate it if cacher is ready.
	}
	// 已初始化好
	util.SafeCloseChan(c.ready)

	// watch
	return c.doWatch(cfg)
}

// 处理watch返回的数据 转换kvEvent并进行sync
func (c *KvCacher) handleWatcher(watcher Watcher) error {
	defer watcher.Stop() //watch的异常 退出for
	// 阻塞在这里直到EventBus关闭
	for resp := range watcher.EventBus() {
		// watch异常 直接退出
		if resp == nil {
			return errors.New("handle watcher error")
		}

		start := time.Now()
		rev := resp.Revision
		evts := make([]discovery.KvEvent, 0, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			evt := discovery.NewKvEvent(rmodel.EVT_CREATE, nil, kv.ModRevision)
			switch {
			case resp.Action == registry.Put && kv.Version == 1:
				evt.Type, evt.KV = rmodel.EVT_CREATE, c.doParse(kv)
			case resp.Action == registry.Put:
				evt.Type, evt.KV = rmodel.EVT_UPDATE, c.doParse(kv)
			case resp.Action == registry.Delete:
				evt.Type = rmodel.EVT_DELETE
				if kv.Value == nil {
					// it will happen in embed mode, and then need to get the cache value not unmarshal
					evt.KV = c.cache.Get(util.BytesToStringWithNoCopy(kv.Key))
				} else {
					evt.KV = c.doParse(kv)
				}
			default:
				log.Errorf(nil, "unknown KeyValue %v", kv)
				continue
			}
			if evt.KV == nil {
				log.Errorf(nil, "failed to parse KeyValue %v", kv)
				continue
			}
			evts = append(evts, evt)
		}
		c.sync(evts)
		log.DebugOrWarnf(start, "finish to handle %d events, prefix: %s, rev: %d",
			len(evts), c.Cfg.Key, rev)
	}
	return nil
}

// 应用DeferHandle
func (c *KvCacher) needDeferHandle(evts []discovery.KvEvent) bool {
	// 启动阶段不执行
	if c.Cfg.DeferHandler == nil || !c.IsReady() {
		return false
	}

	return c.Cfg.DeferHandler.OnCondition(c.Cache(), evts)
}

// cache同步 定时watch，重置cache
func (c *KvCacher) refresh(ctx context.Context) {
	log.Debugf("start to list and watch %s", c.Cfg)
	retries := 0

	timer := time.NewTimer(minWaitInterval)
	defer timer.Stop()
	for {
		nextPeriod := minWaitInterval
		if err := c.ListAndWatch(ctx); err != nil {
			retries++
			nextPeriod = backoff.GetBackoff().Delay(retries)
		} else {
			retries = 0
		}

		select {
		case <-ctx.Done():
			log.Debugf("stop to list and watch %s", c.Cfg)
			return
		case <-timer.C:
			timer.Reset(nextPeriod)
		}
	}
}

// keep the evts valid when call sync
// 集中处理事件
func (c *KvCacher) sync(evts []discovery.KvEvent) {
	if len(evts) == 0 {
		return
	}

	// 如果被deferHandle处理，需要从deferHandle里取数据，不在直接onEvents
	if c.needDeferHandle(evts) {
		return
	}

	// 处理cache notify
	c.onEvents(evts)
}

// 过滤etcd相应的数据
func (c *KvCacher) filter(rev int64, items []*mvccpb.KeyValue) []discovery.KvEvent {
	nc := len(items)
	newStore := make(map[string]*mvccpb.KeyValue, nc)
	for _, kv := range items {
		newStore[util.BytesToStringWithNoCopy(kv.Key)] = kv
	}
	// 协调两个go执行完
	filterStopCh := make(chan struct{})
	// 事件通道
	eventsCh := make(chan [eventBlockSize]discovery.KvEvent, 2)
	// 过滤删除事件
	go c.filterDelete(newStore, rev, eventsCh, filterStopCh)
	// 过滤添加、修改事件
	go c.filterCreateOrUpdate(newStore, rev, eventsCh, filterStopCh)

	// 事件写入evts 返回
	evts := make([]discovery.KvEvent, 0, nc)
	for block := range eventsCh {
		for _, e := range block {
			// 跳过不足eventBlockSize长度的block
			if e.KV == nil {
				break
			}
			evts = append(evts, e)
		}
	}
	return evts
}

// 过滤出来删除的数据
func (c *KvCacher) filterDelete(newStore map[string]*mvccpb.KeyValue,
	rev int64, eventsCh chan [eventBlockSize]discovery.KvEvent, filterStopCh chan struct{}) {
	var block [eventBlockSize]discovery.KvEvent
	i := 0

	c.cache.ForEach(func(k string, v *discovery.KeyValue) (next bool) {
		// 全量扫描 一直是true
		next = true
		// 存在跳过
		_, ok := newStore[k]
		if ok {
			return
		}
		// 一次处理eventBlockSize个事件 把block写入eventsCh并重置block、计数器
		if i >= eventBlockSize {
			eventsCh <- block
			block = [eventBlockSize]discovery.KvEvent{}
			i = 0
		}

		// 转换delete事件
		block[i] = discovery.NewKvEvent(rmodel.EVT_DELETE, v, rev)
		i++
		return
	})

	// 不足eventBlockSize的数据接着写入
	if i > 0 {
		eventsCh <- block
	}
	// 关闭
	close(filterStopCh)
}

// 过滤写入、修改事件
func (c *KvCacher) filterCreateOrUpdate(newStore map[string]*mvccpb.KeyValue,
	rev int64, eventsCh chan [eventBlockSize]discovery.KvEvent, filterStopCh chan struct{}) {
	var block [eventBlockSize]discovery.KvEvent
	i := 0

	for k, v := range newStore {
		ov := c.cache.Get(k)
		// 写入事件
		if ov == nil {
			// 批量处理
			if i >= eventBlockSize {
				eventsCh <- block
				block = [eventBlockSize]discovery.KvEvent{}
				i = 0
			}
			// 解析成写入事件
			if kv := c.doParse(v); kv != nil {
				block[i] = discovery.NewKvEvent(rmodel.EVT_CREATE, kv, rev)
				i++
			}
			continue
		}
		// 数据没变 跳过
		if ov.CreateRevision == v.CreateRevision && ov.ModRevision == v.ModRevision {
			continue
		}

		// 修改事件
		if i >= eventBlockSize {
			eventsCh <- block
			block = [eventBlockSize]discovery.KvEvent{}
			i = 0
		}

		if kv := c.doParse(v); kv != nil {
			block[i] = discovery.NewKvEvent(rmodel.EVT_UPDATE, kv, rev)
			i++
		}
	}

	// 不足eventBlockSize
	if i > 0 {
		eventsCh <- block
	}
	// 阻塞等待 filterDelete
	<-filterStopCh
	// 关闭事件通道
	close(eventsCh)
}

// 处理deferHandler数据
func (c *KvCacher) deferHandle(ctx context.Context) {
	if c.Cfg.DeferHandler == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.handleDeferEvents(ctx)
		}
	}
}

// 批量处理deferHandle数据
func (c *KvCacher) handleDeferEvents(ctx context.Context) {
	defer log.Recover()
	var (
		evts = make([]discovery.KvEvent, eventBlockSize)
		i    int
	)
	interval := 300 * time.Millisecond //累积interval时间段的数据一起处理
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-c.Cfg.DeferHandler.HandleChan():
			if !ok { //chan未初始化 || 空数据 chan有缓冲空间
				return
			}

			if i >= eventBlockSize {
				c.onEvents(evts[:i])
				evts = make([]discovery.KvEvent, eventBlockSize)
				i = 0
			}

			evts[i] = evt
			i++

			util.ResetTimer(timer, interval)
		case <-timer.C:
			timer.Reset(interval)

			if i == 0 {
				continue
			}

			c.onEvents(evts[:i])
			evts = make([]discovery.KvEvent, eventBlockSize)
			i = 0
		}
	}
}

// 应用事件 cache notify
func (c *KvCacher) onEvents(evts []discovery.KvEvent) {
	c.buildCache(evts) // 事件类型已被重置
	c.notify(evts)
}

// 修改cache中数据
func (c *KvCacher) buildCache(evts []discovery.KvEvent) {
	// 是否启动初始化阶段
	init := !c.IsReady()
	for i, evt := range evts {
		key := util.BytesToStringWithNoCopy(evt.KV.Key)
		prevKv := c.cache.Get(key)
		ok := prevKv != nil

		// 重置事件类型 只是为了上报
		switch evt.Type {
		case rmodel.EVT_CREATE, rmodel.EVT_UPDATE:
			switch {
			case init: // 初始化阶段重置成init事件
				evt.Type = rmodel.EVT_INIT
			case !ok && evt.Type != rmodel.EVT_CREATE: // cache不存在,并且不是create事件,重置为create
				log.Warnf("unexpected %s event! it should be %s key %s",
					evt.Type, rmodel.EVT_CREATE, key)
				evt.Type = rmodel.EVT_CREATE
			case ok && evt.Type != rmodel.EVT_UPDATE: // cache存在,并且不是update事件,重置为update
				log.Warnf("unexpected %s event! it should be %s key %s",
					evt.Type, rmodel.EVT_UPDATE, key)
				evt.Type = rmodel.EVT_UPDATE
			}

			// 修改cache
			c.cache.Put(key, evt.KV)
			evts[i] = evt // 重置类型
		case rmodel.EVT_DELETE: //删除事件 不存在报警
			if !ok {
				log.Warnf("unexpected %s event! key %s does not cache",
					evt.Type, key)
			} else {
				evt.KV = prevKv //kv修改为当前cache的值 为了上报
				c.cache.Remove(key)
			}
			evts[i] = evt // 重置类型
		}
	}

	// 监控
	discovery.ReportProcessEventCompleted(c.Cfg.Key, evts)
}

// OnEvent处理链
func (c *KvCacher) notify(evts []discovery.KvEvent) {
	if c.Cfg.OnEvent == nil {
		return
	}

	defer log.Recover()
	for _, evt := range evts {
		c.Cfg.OnEvent(evt)
	}
}

// etcd中的json数据 解析成 struct数据
func (c *KvCacher) doParse(src *mvccpb.KeyValue) (kv *discovery.KeyValue) {
	kv = discovery.NewKeyValue()
	if err := FromEtcdKeyValue(kv, src, c.Cfg.Parser); err != nil {
		log.Errorf(err, "parse %s value failed", util.BytesToStringWithNoCopy(src.Key))
		return nil
	}
	return
}

// 只是返回c.cache
func (c *KvCacher) Cache() discovery.CacheReader {
	return c.cache
}

// 启动函数
func (c *KvCacher) Run() {
	c.once.Do(func() {
		c.goroutine.Do(c.refresh)       //cache同步
		c.goroutine.Do(c.deferHandle)   //deferHandle结果处理
		c.goroutine.Do(c.reportMetrics) // 上报监控
	})
}

// 关闭cacher
func (c *KvCacher) Stop() {
	c.goroutine.Close(true)

	util.SafeCloseChan(c.ready)
}

func (c *KvCacher) Ready() <-chan struct{} {
	return c.ready
}

//是否准备就绪
func (c *KvCacher) IsReady() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

// 上报cache大小
func (c *KvCacher) reportMetrics(ctx context.Context) {
	if !core.ServerInfo.Config.EnablePProf {
		return
	}
	timer := time.NewTimer(DefaultMetricsInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			ReportCacheSize(c.cache.Name(), "raw", c.cache.Size())
			timer.Reset(DefaultMetricsInterval)
		}
	}
}

// 创建KvCacher
func NewKvCacher(cfg *discovery.Config, cache discovery.Cache) *KvCacher {
	return &KvCacher{
		Cfg:   cfg,
		cache: cache,
		ready: make(chan struct{}),
		lw: &innerListWatch{
			Client: backend.Registry(),
			Prefix: cfg.Key,
		},
		goroutine: gopool.New(context.Background()),
	}
}
