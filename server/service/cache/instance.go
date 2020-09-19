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
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"math"
	"time"
)

var FindInstances = &FindInstancesCache{
	Tree: cache.NewTree(cache.Configure().
		WithTTL(2 * time.Minute).
		WithMaxSize(math.MaxInt64))}

func init() {
	FindInstances.AddFilter(
		&ServiceFilter{},     // 指定的provider信息  /Tenant/Environment/AppId/ServiceName
		&VersionRuleFilter{}, // provider对应versionRule的serviceIds
		&TagsFilter{},        // 符合tags的serviceIds
		&AccessibleFilter{},  // 符合权限的serviceIds 黑白名单
		&InstancesFilter{},   // 查找instance
		&ConsistencyFilter{}, // 修正
	)
}

type VersionRuleCacheItem struct {
	ServiceIds []string
	Instances  []*pb.MicroServiceInstance
	Rev        string  // 集群中的数据拼成的版本号，会返回给上游，上游下次请求带着进行校验

	broken bool
	queue  chan struct{}
}

// 标记是首次生成的
func (vi *VersionRuleCacheItem) InitBrokenQueue() {
	if vi.queue == nil {
		vi.queue = make(chan struct{}, 1)
	}
	vi.broken = false
	vi.queue <- struct{}{}
}

// 修正过程锁等待
func (vi *VersionRuleCacheItem) BrokenWait() bool {
	<-vi.queue
	return vi.broken
}

// 修成完关闭chain
func (vi *VersionRuleCacheItem) Broken() {
	vi.broken = true
	close(vi.queue)
}

type FindInstancesCache struct {
	*cache.Tree
}

// 获取所有provider的instance
func (f *FindInstancesCache) Get(ctx context.Context, consumer *pb.MicroService, provider *pb.MicroServiceKey,
	tags []string, rev string) (*VersionRuleCacheItem, error) {
	cloneCtx := context.WithValue(context.WithValue(context.WithValue(context.WithValue(ctx,
		CtxFindConsumer, consumer),
		CtxFindProvider, provider),
		CtxFindTags, tags),
		CtxFindRequestRev, rev)

	node, err := f.Tree.Get(cloneCtx, cache.Options().Temporary(ctx.Value(util.CtxNocache) == "1"))
	if node == nil {
		return nil, err
	}
	return node.Cache.Get(Find).(*VersionRuleCacheItem), nil
}

// 获取指定provider的instance
func (f *FindInstancesCache) GetWithProviderID(ctx context.Context, consumer *pb.MicroService, provider *pb.MicroServiceKey,
	instanceKey *pb.HeartbeatSetElement, tags []string, rev string) (*VersionRuleCacheItem, error) {
	cloneCtx := context.WithValue(ctx, CtxFindProviderInstance, instanceKey)
	return f.Get(cloneCtx, consumer, provider, tags, rev)
}

func (f *FindInstancesCache) Remove(provider *pb.MicroServiceKey) {
	f.Tree.Remove(context.WithValue(context.Background(), CtxFindProvider, provider))
	if len(provider.Alias) > 0 {
		copy := *provider
		copy.ServiceName = copy.Alias
		f.Tree.Remove(context.WithValue(context.Background(), CtxFindProvider, &copy))
	}
}
