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
package event

import (
	"context"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/backoff"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/queue"
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/apache/servicecomb-service-center/server/mux"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	serviceUtil "github.com/apache/servicecomb-service-center/server/service/util"
	"time"
)

const defaultEventHandleInterval = 5 * time.Minute

// DependencyEventHandler add or remove the service dependencies
// when user call find instance api or dependence operation api
/*
	处理DependencyQueue事件
	事件的value是api接口添加的ConsumerDependency类型数据即consumer=>[]provider
	这里把consumer=>[]provider的数据进行转换
	转换成consumer=>GenerateConsumerDependencyRuleKey 即consumer=>[]provider
	provider=>GenerateProviderDependencyRuleKey 即provider=>[]consumer

	Override代表是分多次添加provider，还是一次性全部添加provider
	如果是一次性添加的会进行provider的校验，不存在的provider会分别从consumer，provider里删掉
	同一个consumer多次添加provider也会根据Override的不同，与现有的provider进行不同的判断过滤，进行数据的合并
*/
type DependencyEventHandler struct {
	signals *queue.UniQueue
}

func (h *DependencyEventHandler) Type() discovery.Type {
	return backend.DependencyQueue
}

// 删除事件不处理,因为处理完就直接删掉了
func (h *DependencyEventHandler) OnEvent(evt discovery.KvEvent) {
	action := evt.Type
	if action != pb.EVT_CREATE && action != pb.EVT_UPDATE && action != pb.EVT_INIT {
		return
	}
	h.notify()
}

// 写入事件，通知处理依赖记录
func (h *DependencyEventHandler) notify() {
	err := h.signals.Put(struct{}{})
	if err != nil {
		log.Error("", err)
	}
}

// 延迟重试
func (h *DependencyEventHandler) backoff(f func(), retries int) int {
	if f != nil {
		<-time.After(backoff.GetBackoff().Delay(retries))
		f() // 出现异常的重试函数
	}
	return retries + 1
}

func (h *DependencyEventHandler) tryWithBackoff(success func() error, backoff func(), retries int) (int, error) {
	defer log.Recover()
	// 尝试加锁，加不上说明别的节点在处理，此节点就忽略执行
	lock, err := mux.Try(mux.DepQueueLock)
	if err != nil {
		log.Errorf(err, "try to lock %s failed", mux.DepQueueLock)
		return h.backoff(backoff, retries), err
	}

	if lock == nil {
		return 0, nil
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			log.Error("", err)
		}
	}()
	err = success() // 加锁成功执行的函数，这里的err是etcd的
	if err != nil {
		log.Errorf(err, "handle dependency event failed")
		return h.backoff(backoff, retries), err // 重试
	}

	return 0, nil
}

// 处理事件
func (h *DependencyEventHandler) eventLoop() {
	gopool.Go(func(ctx context.Context) {
		// the events will lose, need to handle dependence records periodically
		// 因为etcd的watch失败肯能导致事件丢失，所以定时处理依赖记录
		period := defaultEventHandleInterval
		if core.ServerInfo.Config.CacheTTL > 0 {
			period = core.ServerInfo.Config.CacheTTL
		}
		timer := time.NewTimer(period)
		retries := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-h.signals.Chan():
				_, err := h.tryWithBackoff(h.Handle, h.notify, retries)
				if err != nil {
					log.Error("", err)
				}
				util.ResetTimer(timer, period)
			case <-timer.C:
				h.notify() // 写入事件
				timer.Reset(period)
			}
		}
	})
}

// 树中的node
type DependencyEventHandlerResource struct {
	dep           *pb.ConsumerDependency
	kv            *discovery.KeyValue
	domainProject string
}

func NewDependencyEventHandlerResource(dep *pb.ConsumerDependency, kv *discovery.KeyValue, domainProject string) *DependencyEventHandlerResource {
	return &DependencyEventHandlerResource{
		dep,
		kv,
		domainProject,
	}
}

// 左小右大  二叉树树的排序规则
func isAddToLeft(centerNode *util.Node, addRes interface{}) bool {
	res := addRes.(*DependencyEventHandlerResource)
	compareRes := centerNode.Res.(*DependencyEventHandlerResource)
	return res.kv.ModRevision <= compareRes.kv.ModRevision
}

func (h *DependencyEventHandler) Handle() error {
	// 所有的依赖关系
	key := core.GetServiceDependencyQueueRootKey("")
	resp, err := backend.Store().DependencyQueue().Search(context.Background(),
		registry.WithStrKey(key),
		registry.WithPrefix())
	if err != nil {
		return err
	}

	// maintain dependency rules.
	l := len(resp.Kvs)
	if l == 0 {
		return nil
	}

	// 创建排序树
	dependencyTree := util.NewTree(isAddToLeft)

	cleanUpDomainProjects := make(map[string]struct{})
	defer h.CleanUp(cleanUpDomainProjects)

	// 添加到树中
	for _, kv := range resp.Kvs {
		r := kv.Value.(*pb.ConsumerDependency)

		_, domainProject, uuid := core.GetInfoFromDependencyQueueKV(kv.Key)
		if uuid == core.DepsQueueUUID { // 即Override=true
			cleanUpDomainProjects[domainProject] = struct{}{}
		}

		//添加到tree中
		res := NewDependencyEventHandlerResource(r, kv, domainProject)
		dependencyTree.AddNode(res)
	}

	// 遍历树中节点
	return dependencyTree.InOrderTraversal(dependencyTree.GetRoot(), h.dependencyRuleHandle)
}

// 处理每一个依赖关系
func (h *DependencyEventHandler) dependencyRuleHandle(res interface{}) error {
	ctx := context.WithValue(context.Background(), util.CtxGlobal, "1")
	dependencyEventHandlerRes := res.(*DependencyEventHandlerResource)
	r := dependencyEventHandlerRes.dep
	consumerFlag := util.StringJoin([]string{r.Consumer.Environment, r.Consumer.AppId, r.Consumer.ServiceName, r.Consumer.Version}, "/")

	// 填充consumer provider的Tenant，并没有强制改成一致
	domainProject := dependencyEventHandlerRes.domainProject
	consumerInfo := proto.DependenciesToKeys([]*pb.MicroServiceKey{r.Consumer}, domainProject)[0]
	providersInfo := proto.DependenciesToKeys(r.Providers, domainProject)

	var dep serviceUtil.Dependency
	var err error
	dep.DomainProject = domainProject
	dep.Consumer = consumerInfo
	dep.ProvidersRule = providersInfo
	if r.Override { // 唯一
		err = serviceUtil.CreateDependencyRule(ctx, &dep)
	} else { // 不唯一
		err = serviceUtil.AddDependencyRule(ctx, &dep)
	}

	if err != nil {
		log.Errorf(err, "modify dependency rule failed, override: %t, consumer %s", r.Override, consumerFlag)
		return fmt.Errorf("override: %t, consumer is %s, %s", r.Override, consumerFlag, err.Error())
	}

	// 处理完删掉dependencyQueue
	if err = h.removeKV(ctx, dependencyEventHandlerRes.kv); err != nil {
		log.Errorf(err, "remove dependency rule failed, override: %t, consumer %s", r.Override, consumerFlag)
		return err
	}

	log.Infof("maintain dependency [%v] successfully", r)
	return nil
}

// 用完就删
func (h *DependencyEventHandler) removeKV(ctx context.Context, kv *discovery.KeyValue) error {
	// 保证版本一致
	dResp, err := backend.Registry().TxnWithCmp(ctx, []registry.PluginOp{registry.OpDel(registry.WithKey(kv.Key))},
		[]registry.CompareOp{registry.OpCmp(registry.CmpVer(kv.Key), registry.CmpEqual, kv.Version)},
		nil)
	if err != nil {
		return fmt.Errorf("can not remove the dependency %s request, %s", util.BytesToStringWithNoCopy(kv.Key), err.Error())
	}
	if !dResp.Succeeded {
		log.Infof("the dependency %s request is changed", util.BytesToStringWithNoCopy(kv.Key))
	}
	return nil
}

// 清理domainProjects下 不存在的provider
func (h *DependencyEventHandler) CleanUp(domainProjects map[string]struct{}) {
	for domainProject := range domainProjects {
		ctx := context.WithValue(context.Background(), util.CtxGlobal, "1")
		if err := serviceUtil.CleanUpDependencyRules(ctx, domainProject); err != nil {
			log.Errorf(err, "clean up '%s' dependency rules failed", domainProject)
		}
	}
}

// consumer provider依赖关系
func NewDependencyEventHandler() *DependencyEventHandler {
	h := &DependencyEventHandler{
		signals: queue.NewUniQueue(),
	}
	h.eventLoop()
	return h
}
