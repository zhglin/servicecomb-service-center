// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"github.com/apache/servicecomb-service-center/server/core"
	pb "github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"time"
)

const (
	leaseProfTimeFmt           = "15:04:05.000"
	eventBlockSize             = 1000
	deferCheckWindow           = 2 * time.Second // instance DELETE event will be delay. 处理被保护事件的事件间隔
	selfPreservationPercentage = 0.8 // 触发自我保护的比例
	selfPreservationMaxTTL     = 10 * 60 // 10min //最大保护时间
	selfPreservationInitCount  = 5 //开启自我保护的最少数量
)
// discovery 管理的数据类型
// 要先加载discovery 初始化好各个类型供其他子系统设置EventHandler
var (
	DOMAIN          discovery.Type
	PROJECT         discovery.Type
	SERVICE         discovery.Type
	ServiceIndex    discovery.Type
	ServiceAlias    discovery.Type
	ServiceTag      discovery.Type
	RULE            discovery.Type
	RuleIndex       discovery.Type
	DependencyRule  discovery.Type
	DependencyQueue discovery.Type
	SCHEMA          discovery.Type
	SchemaSummary   discovery.Type
	INSTANCE        discovery.Type
	LEASE           discovery.Type
)
/*
Service                   /cse-sr/ms/files/domin/project/serviceId  => date {pb.MicroService}
ServiceIndex              /cse-sr/ms/indexes/domin/project/environment/appId/serviceName/version => serviceId
ServiceAlias              /cse-sr/ms/alias/domin/project/environment/appId/alias/version       => serviceId
ServiceTag                /cse-sr/ms/tags/domin/project/serviceId => date {map}
Rule                      /cse-sr/ms/rules/domin/project/serviceId/ruleId  => data {pb.ServiceRule}
RuleIndex                 /cse-sr/ms/rule-indexes/domin/project/serciveId/Attribute/Pattern  => ruleId
SchemaSummary
Instance                  /cse-sr/inst/files/domin/project/serviceId/instanceId => date {MicroServiceInstance} //etcd设置ttl
Lease                     /cse-sr/inst/leases/domin/project/serviceId/instanceId  => leaseId  //etcd设置ttl
Schema
DependencyRule            /cse-sr/ms/dep-rules/domin/project/serviceType(c,p)/environment/AppId/ServiceName/Version
DependencyQueue           /cse-sr/ms/dep-queue/domin/project/consumerId(serviceId)/id(provider.AppId_provider.ServiceName)   //ConsumerDependency
Domain                    /cse-sr/domains/domin  => ''
Project                   /cse-sr/projects/domin/project => ''
*/
func registerInnerTypes() {
	SERVICE = Store().MustInstall(NewAddOn("SERVICE",
		discovery.Configure().WithPrefix(core.GetServiceRootKey("")).
			WithInitSize(500).WithParser(pb.ServiceParser)))
	INSTANCE = Store().MustInstall(NewAddOn("INSTANCE",
		discovery.Configure().WithPrefix(core.GetInstanceRootKey("")).
			WithInitSize(1000).WithParser(pb.InstanceParser).
			WithDeferHandler(NewInstanceEventDeferHandler())))
	DOMAIN = Store().MustInstall(NewAddOn("DOMAIN",
		discovery.Configure().WithPrefix(core.GetDomainRootKey()+core.SPLIT).
			WithInitSize(100).WithParser(pb.StringParser)))
	SCHEMA = Store().MustInstall(NewAddOn("SCHEMA",
		discovery.Configure().WithPrefix(core.GetServiceSchemaRootKey("")).
			WithInitSize(0)))
	SchemaSummary = Store().MustInstall(NewAddOn("SCHEMA_SUMMARY",
		discovery.Configure().WithPrefix(core.GetServiceSchemaSummaryRootKey("")).
			WithInitSize(100).WithParser(pb.StringParser)))
	RULE = Store().MustInstall(NewAddOn("RULE",
		discovery.Configure().WithPrefix(core.GetServiceRuleRootKey("")).
			WithInitSize(100).WithParser(pb.RuleParser)))
	LEASE = Store().MustInstall(NewAddOn("LEASE",
		discovery.Configure().WithPrefix(core.GetInstanceLeaseRootKey("")).
			WithInitSize(1000).WithParser(pb.StringParser)))
	ServiceIndex = Store().MustInstall(NewAddOn("SERVICE_INDEX",
		discovery.Configure().WithPrefix(core.GetServiceIndexRootKey("")).
			WithInitSize(500).WithParser(pb.StringParser)))
	ServiceAlias = Store().MustInstall(NewAddOn("SERVICE_ALIAS",
		discovery.Configure().WithPrefix(core.GetServiceAliasRootKey("")).
			WithInitSize(100).WithParser(pb.StringParser)))
	ServiceTag = Store().MustInstall(NewAddOn("SERVICE_TAG",
		discovery.Configure().WithPrefix(core.GetServiceTagRootKey("")).
			WithInitSize(100).WithParser(pb.MapParser)))
	RuleIndex = Store().MustInstall(NewAddOn("RULE_INDEX",
		discovery.Configure().WithPrefix(core.GetServiceRuleIndexRootKey("")).
			WithInitSize(100).WithParser(pb.StringParser)))
	DependencyRule = Store().MustInstall(NewAddOn("DEPENDENCY_RULE",
		discovery.Configure().WithPrefix(core.GetServiceDependencyRuleRootKey("")).
			WithInitSize(100).WithParser(pb.DependencyRuleParser)))
	DependencyQueue = Store().MustInstall(NewAddOn("DEPENDENCY_QUEUE",
		discovery.Configure().WithPrefix(core.GetServiceDependencyQueueRootKey("")).
			WithInitSize(100).WithParser(pb.DependencyQueueParser)))
	PROJECT = Store().MustInstall(NewAddOn("PROJECT",
		discovery.Configure().WithPrefix(core.GetProjectRootKey("")).
			WithInitSize(100).WithParser(pb.StringParser)))
}
