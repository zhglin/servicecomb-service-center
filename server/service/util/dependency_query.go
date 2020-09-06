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
package util

import (
	"context"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/log"
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	apt "github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	"strings"
)

// 选项
type DependencyRelationFilterOpt struct {
	SameDomainProject bool
	NonSelf           bool
}

// 选项函数
type DependencyRelationFilterOption func(opt DependencyRelationFilterOpt) DependencyRelationFilterOpt

// 设置SameDomainProject
func WithSameDomainProject() DependencyRelationFilterOption {
	return func(opt DependencyRelationFilterOpt) DependencyRelationFilterOpt {
		opt.SameDomainProject = true
		return opt
	}
}

// 设置NonSelf
func WithoutSelfDependency() DependencyRelationFilterOption {
	return func(opt DependencyRelationFilterOpt) DependencyRelationFilterOpt {
		opt.NonSelf = true
		return opt
	}
}

// 执行opts 返回op
func toDependencyRelationFilterOpt(opts ...DependencyRelationFilterOption) (op DependencyRelationFilterOpt) {
	for _, opt := range opts {
		op = opt(op)
	}
	return
}

type DependencyRelation struct {
	ctx           context.Context
	domainProject string
	consumer      *pb.MicroService
	provider      *pb.MicroService
}

// 获取consumer对应的provider
func (dr *DependencyRelation) GetDependencyProviders(opts ...DependencyRelationFilterOption) ([]*pb.MicroService, error) {
	// 获取provider
	keys, err := dr.getProviderKeys()
	if err != nil {
		return nil, err
	}
	services := make([]*pb.MicroService, 0, len(keys))
	op := toDependencyRelationFilterOpt(opts...)
	for _, key := range keys {
		// 必须相同的domainProject
		if op.SameDomainProject && key.Tenant != dr.domainProject {
			continue
		}

		// 获取provider的serviceId
		providerIDs, err := dr.parseDependencyRule(key)
		if err != nil {
			return nil, err
		}

		// provider里既有*，又有非*，此key==* 就忽略非*的结果
		if key.ServiceName == "*" {
			services = services[:0]
		}

		for _, providerID := range providerIDs {
			// 校验serviceId是否存在
			provider, err := GetService(dr.ctx, key.Tenant, providerID)
			if err != nil {
				log.Warnf("get provider[%s/%s/%s/%s] failed",
					key.Environment, key.AppId, key.ServiceName, key.Version)
				continue
			}
			if provider == nil {
				log.Warnf("provider[%s/%s/%s/%s] does not exist",
					key.Environment, key.AppId, key.ServiceName, key.Version)
				continue
			}

			// 不允许自己依赖自己
			if op.NonSelf && providerID == dr.consumer.ServiceId {
				continue
			}
			services = append(services, provider)
		}

		// 不再处理剩下的provider
		if key.ServiceName == "*" {
			break
		}
	}
	return services, nil
}

func (dr *DependencyRelation) GetDependencyProviderIds() ([]string, error) {
	keys, err := dr.getProviderKeys()
	if err != nil {
		return nil, err
	}
	return dr.getDependencyProviderIds(keys)
}

// 获取etcd中consumer对应的provider
func (dr *DependencyRelation) getProviderKeys() ([]*pb.MicroServiceKey, error) {
	if dr.consumer == nil {
		return nil, fmt.Errorf("Invalid consumer")
	}

	// consumer的service信息转换成serviceKey
	consumerMicroServiceKey := proto.MicroServiceToKey(dr.domainProject, dr.consumer)

	// 生成etcd key
	conKey := apt.GenerateConsumerDependencyRuleKey(dr.domainProject, consumerMicroServiceKey)

	// 获取etcd value
	consumerDependency, err := TransferToMicroServiceDependency(dr.ctx, conKey)
	if err != nil {
		return nil, err
	}
	return consumerDependency.Dependency, nil
}

func (dr *DependencyRelation) getDependencyProviderIds(providerRules []*pb.MicroServiceKey) ([]string, error) {
	provideServiceIds := make([]string, 0, len(providerRules))
	for _, provider := range providerRules {
		serviceIDs, err := dr.parseDependencyRule(provider)
		switch {
		case provider.ServiceName == "*":
			if err != nil {
				log.Errorf(err, "get all serviceIDs failed")
				return provideServiceIds, err
			}
			return serviceIDs, nil
		default:
			if err != nil {
				log.Errorf(err, "get service[%s/%s/%s/%s]'s providerIDs failed",
					provider.Environment, provider.AppId, provider.ServiceName, provider.Version)
				return provideServiceIds, err
			}
			if len(serviceIDs) == 0 {
				log.Warnf("get service[%s/%s/%s/%s]'s providerIDs is empty",
					provider.Environment, provider.AppId, provider.ServiceName, provider.Version)
				continue
			}
			provideServiceIds = append(provideServiceIds, serviceIDs...)
		}
	}
	return provideServiceIds, nil
}

// 获取指定service的serviceId
func (dr *DependencyRelation) parseDependencyRule(dependencyRule *pb.MicroServiceKey) (serviceIDs []string, err error) {
	opts := FromContext(dr.ctx)
	switch {
	// *代表domain/project下的所有service
	case dependencyRule.ServiceName == "*":
		log.Infof("service[%s/%s/%s/%s] rely all service",
			dr.consumer.Environment, dr.consumer.AppId, dr.consumer.ServiceName, dr.consumer.Version)
		splited := strings.Split(apt.GenerateServiceIndexKey(dependencyRule), "/")
		// /cse-sr/ms/indexes/{domin/project}/{environment}
		allServiceKey := util.StringJoin(splited[:len(splited)-3], "/") + "/"
		// 获取domainProject下所有的serviceId
		sopts := append(opts,
			registry.WithStrKey(allServiceKey),
			registry.WithPrefix())
		resp, err := backend.Store().ServiceIndex().Search(dr.ctx, sopts...)
		if err != nil {
			return nil, err
		}

		for _, kv := range resp.Kvs {
			serviceIDs = append(serviceIDs, kv.Value.(string))
		}
	default:
		serviceIDs, _, err = FindServiceIds(dr.ctx, dependencyRule.Version, dependencyRule)
	}
	return
}

// 获取provider对应的consumer
func (dr *DependencyRelation) GetDependencyConsumers(opts ...DependencyRelationFilterOption) ([]*pb.MicroService, error) {
	// 获取所有consumer
	consumerDependAllList, err := dr.getDependencyConsumersOfProvider()
	if err != nil {
		log.Errorf(err, "get service[%s]'s consumers failed", dr.provider.ServiceId)
		return nil, err
	}
	consumers := make([]*pb.MicroService, 0)
	op := toDependencyRelationFilterOpt(opts...)
	for _, consumer := range consumerDependAllList {
		// 必须domainProject一致
		if op.SameDomainProject && consumer.Tenant != dr.domainProject {
			continue
		}

		// 校验service是否存在
		service, err := dr.getServiceByMicroServiceKey(consumer)
		if err != nil {
			return nil, err
		}
		if service == nil {
			log.Warnf("consumer[%s/%s/%s/%s] does not exist",
				consumer.Environment, consumer.AppId, consumer.ServiceName, consumer.Version)
			continue
		}

		// 不允许provider依赖provider
		if op.NonSelf && service.ServiceId == dr.provider.ServiceId {
			continue
		}

		consumers = append(consumers, service)
	}
	return consumers, nil
}

// 获取serviceKey对应的serviceId
func (dr *DependencyRelation) getServiceByMicroServiceKey(service *pb.MicroServiceKey) (*pb.MicroService, error) {
	serviceID, err := GetServiceID(dr.ctx, service)
	if err != nil {
		return nil, err
	}
	if len(serviceID) == 0 {
		log.Warnf("service[%s/%s/%s/%s] not exist",
			service.Environment, service.AppId, service.ServiceName, service.Version)
		return nil, nil
	}
	return GetService(dr.ctx, service.Tenant, serviceID)
}

// 获取provider被依赖的consumer的serviceId
func (dr *DependencyRelation) GetDependencyConsumerIds() ([]string, error) {
	consumerDependAllList, err := dr.getDependencyConsumersOfProvider()
	if err != nil {
		return nil, err
	}
	consumerIDs := make([]string, 0, len(consumerDependAllList))
	for _, consumer := range consumerDependAllList {
		consumerID, err := GetServiceID(dr.ctx, consumer)
		if err != nil {
			log.Errorf(err, "get consumer[%s/%s/%s/%s] failed",
				consumer.Environment, consumer.AppId, consumer.ServiceName, consumer.Version)
			return nil, err
		}
		if len(consumerID) == 0 {
			log.Warnf("get consumer[%s/%s/%s/%s] not exist",
				consumer.Environment, consumer.AppId, consumer.ServiceName, consumer.Version)
			continue
		}
		consumerIDs = append(consumerIDs, consumerID)
	}
	return consumerIDs, nil

}

// 根据provider查consumer 有*的存在
func (dr *DependencyRelation) getDependencyConsumersOfProvider() ([]*pb.MicroServiceKey, error) {
	if dr.provider == nil {
		return nil, fmt.Errorf("Invalid provider")
	}
	providerService := proto.MicroServiceToKey(dr.domainProject, dr.provider)
	// 处理* 不管version
	consumerDependAllList, err := dr.getConsumerOfDependAllServices()
	if err != nil {
		log.Errorf(err, "get consumers that depend on all services failed, %s", dr.provider.ServiceId)
		return nil, err
	}

	// 处理非* version
	consumerDependList, err := dr.getConsumerOfSameServiceNameAndAppID(providerService)
	if err != nil {
		log.Errorf(err, "get consumers that depend on rule[%s/%s/%s/%s] failed",
			dr.provider.Environment, dr.provider.AppId, dr.provider.ServiceName, dr.provider.Version)
		return nil, err
	}

	// 合并
	consumerDependAllList = append(consumerDependAllList, consumerDependList...)
	return consumerDependAllList, nil
}

// 获取consumer中provider==*的consumer
func (dr *DependencyRelation) getConsumerOfDependAllServices() ([]*pb.MicroServiceKey, error) {
	providerService := proto.MicroServiceToKey(dr.domainProject, dr.provider)
	providerService.ServiceName = "*"
	relyAllKey := apt.GenerateProviderDependencyRuleKey(dr.domainProject, providerService)
	opts := append(FromContext(dr.ctx), registry.WithStrKey(relyAllKey))
	rsp, err := backend.Store().DependencyRule().Search(dr.ctx, opts...)
	if err != nil {
		log.Errorf(err, "get consumers that rely all service failed, %s/%s/%s/%s",
			dr.provider.Environment, dr.provider.AppId, dr.provider.ServiceName, dr.provider.Version)
		return nil, err
	}
	if len(rsp.Kvs) != 0 {
		log.Infof("exist consumers that rely all service, %s/%s/%s/%s",
			dr.provider.Environment, dr.provider.AppId, dr.provider.ServiceName, dr.provider.Version)
		return rsp.Kvs[0].Value.(*pb.MicroServiceDependency).Dependency, nil
	}
	return nil, nil
}

// 获取provider的consumer 非* 处理版本
func (dr *DependencyRelation) getConsumerOfSameServiceNameAndAppID(provider *pb.MicroServiceKey) ([]*pb.MicroServiceKey, error) {
	providerVersion := provider.Version
	// 去掉了version查询etcd
	provider.Version = ""
	prefix := apt.GenerateProviderDependencyRuleKey(dr.domainProject, provider)
	provider.Version = providerVersion

	opts := append(FromContext(dr.ctx),
		registry.WithStrKey(prefix),
		registry.WithPrefix())
	rsp, err := backend.Store().DependencyRule().Search(dr.ctx, opts...)
	if err != nil {
		log.Errorf(err, "get service[%s/%s/%s]'s dependency rules failed",
			provider.Environment, provider.AppId, provider.ServiceName)
		return nil, err
	}

	allConsumers := make([]*pb.MicroServiceKey, 0, len(rsp.Kvs))
	var latestServiceID []string

	for _, kv := range rsp.Kvs {
		// 判断版本号  consumer的
		providerVersionRuleArr := strings.Split(util.BytesToStringWithNoCopy(kv.Key), "/")
		providerVersionRule := providerVersionRuleArr[len(providerVersionRuleArr)-1]
		// 取provider最后的版本 查一次就可以了
		if providerVersionRule == "latest" {
			if latestServiceID == nil {
				latestServiceID, _, err = FindServiceIds(dr.ctx, providerVersionRule, provider)
				if err != nil {
					log.Errorf(err, "get service[%s/%s/%s/%s]'s serviceID failed",
						provider.Environment, provider.AppId, provider.ServiceName, providerVersionRule)
					return nil, err
				}
			}
			if len(latestServiceID) == 0 {
				log.Infof("service[%s/%s/%s/%s] does not exist",
					provider.Environment, provider.AppId, provider.ServiceName, providerVersionRule)
				continue
			}
			if dr.provider.ServiceId != latestServiceID[0] {
				continue
			}

		} else {
			// 不符合版本限制
			if !VersionMatchRule(providerVersion, providerVersionRule) {
				continue
			}
		}

		log.Debugf("providerETCD is %s", providerVersionRuleArr)
		allConsumers = append(allConsumers, kv.Value.(*pb.MicroServiceDependency).Dependency...)
	}
	return allConsumers, nil
}

// provider的被依赖关系
func NewProviderDependencyRelation(ctx context.Context, domainProject string, provider *pb.MicroService) *DependencyRelation {
	return NewDependencyRelation(ctx, domainProject, nil, provider)
}

// consumer的依赖关系
func NewConsumerDependencyRelation(ctx context.Context, domainProject string, consumer *pb.MicroService) *DependencyRelation {
	return NewDependencyRelation(ctx, domainProject, consumer, nil)
}

// 依赖关系
func NewDependencyRelation(ctx context.Context, domainProject string, consumer *pb.MicroService, provider *pb.MicroService) *DependencyRelation {
	return &DependencyRelation{
		ctx:           ctx,
		domainProject: domainProject,
		consumer:      consumer,
		provider:      provider,
	}
}
