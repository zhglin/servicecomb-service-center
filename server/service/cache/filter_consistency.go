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

package cache

import (
	"context"
	"github.com/apache/servicecomb-service-center/pkg/cache"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/core/backend"
)

// ConsistencyFilter improves consistency.
// Scenario: cache maybe different between several service-centers.
// 不同的service_center节点的缓存可能会不一致，如果切换节点导致requestRev不一致就会进行重新获取
type ConsistencyFilter struct {
	InstancesFilter
}

func (f *ConsistencyFilter) Name(ctx context.Context, parent *cache.Node) string {
	item := parent.Cache.Get(Find).(*VersionRuleCacheItem)
	requestRev := ctx.Value(CtxFindRequestRev).(string)
	// 查不到name会重新执行init
	if len(requestRev) == 0 || requestRev == item.Rev {
		return ""
	}
	return requestRev
}

// Init generates cache.
// We think cache inconsistency happens and correction is needed only when the
// revision in the request is not empty and different from the revision of
// parent cache. To correct inconsistency, RevisionFilter skips cache and get
// data from the backend directly to response.
// It's impossible to guarantee consistency if the backend is not creditable,
// thus in this condition RevisionFilter uses cache only.
func (f *ConsistencyFilter) Init(ctx context.Context, parent *cache.Node) (node *cache.Node, err error) {
	pCache := parent.Cache.Get(Find).(*VersionRuleCacheItem) // 这个是引用
	requestRev := ctx.Value(CtxFindRequestRev).(string)
	// 不可信的后端存储不进行修正
	if len(requestRev) == 0 || requestRev == pCache.Rev ||
		!(backend.Store().Instance().Creditable()) {
		node = cache.NewNode()
		node.Cache.Set(Find, pCache)
		return
	}

	// 第一次调filter_consistency，直接使用
	// 并发会阻塞这里，修正完的pCace.Broken关闭chain，解除阻塞时因为pCache是引用数据已更新，所以不存在并发问题
	if pCache.BrokenWait() {
		node = cache.NewNode()
		node.Cache.Set(Find, pCache)
		return
	}

	cloneCtx := util.CloneContext(ctx)
	cloneCtx = util.SetContext(cloneCtx, util.CtxNocache, "1") //不使用缓存进行获取
	insts, _, err := f.Find(cloneCtx, parent)
	if err != nil { // 出现异常后的第一次跳过直接使用cache
		pCache.InitBrokenQueue()
		return nil, err
	}

	log.Warnf("the cache of finding instances api is broken, req[%s]!=cache[%s][%s]",
		requestRev, pCache.Rev, parent.Name)
	pCache.Instances = insts
	pCache.Broken() // 表示修正完了，关闭chain

	node = cache.NewNode()
	node.Cache.Set(Find, pCache)
	return
}
