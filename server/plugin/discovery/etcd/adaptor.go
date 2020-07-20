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
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
)

// Adaptor implements discovery.Adaptor.
// Adaptor does service discovery with etcd as it's registry.
type Adaptor struct {
	discovery.Cacher
	discovery.Indexer
}

func (se *Adaptor) Run() {
	if r, ok := se.Cacher.(discovery.Runnable); ok {
		r.Run()
	}
}

func (se *Adaptor) Stop() {
	if r, ok := se.Cacher.(discovery.Runnable); ok {
		r.Stop()
	}
}

func (se *Adaptor) Ready() <-chan struct{} {
	if r, ok := se.Cacher.(discovery.Runnable); ok {
		return r.Ready()
	}
	return closedCh
}

func NewEtcdAdaptor(name string, cfg *discovery.Config) *Adaptor {
	var adaptor Adaptor
	switch {
	case core.ServerInfo.Config.EnableCache && cfg.InitSize > 0:
		cache := discovery.NewKvCache(name, cfg)
		adaptor.Cacher = NewKvCacher(cfg, cache)
		adaptor.Indexer = NewCacheIndexer(cfg, cache)
	default:
		log.Infof(
			"core will not cache '%s' and ignore all events of it, cache enabled: %v, init size: %d",
			name, core.ServerInfo.Config.EnableCache, cfg.InitSize)
		adaptor.Cacher = discovery.NullCacher
		adaptor.Indexer = NewEtcdIndexer(cfg.Key, cfg.Parser)
	}
	return &adaptor
}
