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
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	"sync"
)

// 这里管理watch链接,cacher定时watch,watch的链接要能超时关闭
type innerWatcher struct {
	Cfg    ListWatchConfig
	lw     ListWatch
	bus    chan *registry.PluginResponse
	stopCh chan struct{}
	stop   bool
	mux    sync.Mutex
}

// watch返回给外部
func (w *innerWatcher) EventBus() <-chan *registry.PluginResponse {
	return w.bus
}

func (w *innerWatcher) process(_ context.Context) {
	stopCh := make(chan struct{})
	// 设置watch链接的 链接时间
	ctx, cancel := context.WithTimeout(w.Cfg.Context, w.Cfg.Timeout)
	gopool.Go(func(_ context.Context) {
		defer close(stopCh)
		//DoWatch异常会return,ctx超时会return
		w.lw.DoWatch(ctx, w.sendEvent)
	})

	select {
	case <-stopCh: //主动取消
		// timed out or exception
		w.Stop() // 关闭EventBus 解除上层的for
		cancel()
	case <-w.stopCh: // 上层的退出导致这里取消
		cancel()
	}

}

// watch回调函数 数据写入bus
func (w *innerWatcher) sendEvent(resp *registry.PluginResponse) {
	defer log.Recover()
	w.bus <- resp
}

// 关闭
func (w *innerWatcher) Stop() {
	w.mux.Lock()
	// 已经关闭了，会重复stop
	if w.stop {
		w.mux.Unlock()
		return
	}
	w.stop = true
	close(w.stopCh)
	close(w.bus)
	w.mux.Unlock()
}

func newInnerWatcher(lw ListWatch, cfg ListWatchConfig) *innerWatcher {
	w := &innerWatcher{
		Cfg:    cfg,
		lw:     lw,
		bus:    make(chan *registry.PluginResponse, eventBusSize),
		stopCh: make(chan struct{}),
	}
	// 开始watch
	gopool.Go(w.process)
	return w
}
