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

package core

import (
	"github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
)

// 存放etcd中的具体路径
const (
	SPLIT                    = "/"
	RegistryRootKey          = "cse-sr"
	RegistrySysKey           = "sys" //service_center的全局最大版本号
	RegistryServiceKey       = "ms"
	RegistryInstanceKey      = "inst"
	RegistryFile             = "files"
	RegistryIndex            = "indexes"
	RegistryRuleKey          = "rules"
	RegistryRuleIndexKey     = "rule-indexes"
	RegistryDomainKey        = "domains"
	RegistryProjectKey       = "projects"
	RegistryAliasKey         = "alias"
	RegistryTagKey           = "tags"
	RegistrySchemaKey        = "schemas"
	RegistrySchemaSummaryKey = "schema-sum"
	RegistryLeaseKey         = "leases"
	RegistryDependencyKey    = "deps"
	RegistryDepsRuleKey      = "dep-rules"
	RegistryDepsQueueKey     = "dep-queue"
	RegistryMetricsKey       = "metrics"
	DepsQueueUUID            = "0"
	DepsConsumer             = "c"
	DepsProvider             = "p"
)

func GetRootKey() string {
	return SPLIT + RegistryRootKey
}

//Service	/cse-sr/ms/files/domin/project/
func GetServiceRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryFile,
		domainProject,
	}, SPLIT)
}

//ServiceIndex前缀	/cse-sr/ms/indexes/{domainProject}
func GetServiceIndexRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryIndex,
		domainProject,
	}, SPLIT)
}

//ServiceAlias	/cse-sr/ms/alias/domin/project/environment/appId/alias/version       => serviceId
func GetServiceAliasRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryAliasKey,
		domainProject,
	}, SPLIT)
}

func GetServiceAppKey(domainProject, env, appID string) string {
	return util.StringJoin([]string{
		GetServiceIndexRootKey(domainProject),
		env,
		appID,
	}, SPLIT)
}

// /cse-sr/ms/rules/domin/project/
func GetServiceRuleRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryRuleKey,
		domainProject,
	}, SPLIT)
}

// /cse-sr/ms/rule-indexes/domin/project/
func GetServiceRuleIndexRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryRuleIndexKey,
		domainProject,
	}, SPLIT)
}

// /cse-sr/ms/tags/domin/project/
func GetServiceTagRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryTagKey,
		domainProject,
	}, SPLIT)
}

func GetServiceSchemaRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistrySchemaKey,
		domainProject,
	}, SPLIT)
}

func GetInstanceRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryInstanceKey,
		RegistryFile,
		domainProject,
	}, SPLIT)
}

func GetInstanceLeaseRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryInstanceKey,
		RegistryLeaseKey,
		domainProject,
	}, SPLIT)
}

//Service	/cse-sr/ms/files/{domin/project}/{serviceId}  => date {pb.MicroService}
func GenerateServiceKey(domainProject string, serviceID string) string {
	return util.StringJoin([]string{
		GetServiceRootKey(domainProject),
		serviceID,
	}, SPLIT)
}

//RuleIndex		/cse-sr/ms/rule-indexes/{domin/project}/{serciveId}/{Attribute}/{Pattern} => ruleId
func GenerateRuleIndexKey(domainProject string, serviceID string, attr string, pattern string) string {
	return util.StringJoin([]string{
		GetServiceRuleIndexRootKey(domainProject),
		serviceID,
		attr,
		pattern,
	}, SPLIT)
}

//ServiceIndex /cse-sr/ms/indexes/{domin/project}/{environment}/{appId}/{serviceName}/{version} => serviceId
func GenerateServiceIndexKey(key *registry.MicroServiceKey) string {
	return util.StringJoin([]string{
		GetServiceIndexRootKey(key.Tenant),
		key.Environment,
		key.AppId,
		key.ServiceName,
		key.Version,
	}, SPLIT)
}

//ServiceAlias	/cse-sr/ms/alias/{domin/project}/{environment}/{appId}/{alias}/{version}	=> serviceId
func GenerateServiceAliasKey(key *registry.MicroServiceKey) string {
	return util.StringJoin([]string{
		GetServiceAliasRootKey(key.Tenant),
		key.Environment,
		key.AppId,
		key.Alias,
		key.Version,
	}, SPLIT)
}

//Rule	/cse-sr/ms/rules/{domin/project}/{serviceId}/{ruleId}  => data {pb.ServiceRule}
func GenerateServiceRuleKey(domainProject string, serviceID string, ruleID string) string {
	return util.StringJoin([]string{
		GetServiceRuleRootKey(domainProject),
		serviceID,
		ruleID,
	}, SPLIT)
}

//ServiceTag	/cse-sr/ms/tags/{domin/project}/{serviceId} => date {map}
func GenerateServiceTagKey(domainProject string, serviceID string) string {
	return util.StringJoin([]string{
		GetServiceTagRootKey(domainProject),
		serviceID,
	}, SPLIT)
}

//schemaKey /cse-sr/ms/schemas/{serviceId}/{schemaId} => pb.Schema{SchemaId:schemaID, Schema:schema}
func GenerateServiceSchemaKey(domainProject string, serviceID string, schemaID string) string {
	return util.StringJoin([]string{
		GetServiceSchemaRootKey(domainProject),
		serviceID,
		schemaID,
	}, SPLIT)
}

//SummaryKey /cse-sr/ms/schema-sum/{domainProject}/{serviceId}/{schemaId} => Summary
func GenerateServiceSchemaSummaryKey(domainProject string, serviceID string, schemaID string) string {
	return util.StringJoin([]string{
		GetServiceSchemaSummaryRootKey(domainProject),
		serviceID,
		schemaID,
	}, SPLIT)
}

func GetServiceSchemaSummaryRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistrySchemaSummaryKey,
		domainProject,
	}, SPLIT)
}

// Instance	/cse-sr/inst/files/domin/project/serviceId/instanceId => date {MicroServiceInstance}
// 绑定lease
func GenerateInstanceKey(domainProject string, serviceID string, instanceID string) string {
	return util.StringJoin([]string{
		GetInstanceRootKey(domainProject),
		serviceID,
		instanceID,
	}, SPLIT)
}

// Lease  /cse-sr/inst/leases/domin/project/serviceId/instanceId  => leaseId  租约
// 绑定lease
func GenerateInstanceLeaseKey(domainProject string, serviceID string, instanceID string) string {
	return util.StringJoin([]string{
		GetInstanceLeaseRootKey(domainProject),
		serviceID,
		instanceID,
	}, SPLIT)
}

// service依赖key  provider consumer
func GenerateServiceDependencyRuleKey(serviceType string, domainProject string, in *registry.MicroServiceKey) string {
	if in == nil {
		return util.StringJoin([]string{
			GetServiceDependencyRuleRootKey(domainProject),
			serviceType,
		}, SPLIT)
	}
	if in.ServiceName == "*" {
		return util.StringJoin([]string{
			GetServiceDependencyRuleRootKey(domainProject),
			serviceType,
			in.Environment,
			in.ServiceName,
		}, SPLIT)
	}
	return util.StringJoin([]string{
		GetServiceDependencyRuleRootKey(domainProject),
		serviceType,
		in.Environment,
		in.AppId,
		in.ServiceName,
		in.Version,
	}, SPLIT)
}

// consumer /cse-sr/ms/dep-rules/{domainProject}/c/{Environment}/{AppId}/{ServiceName}/{Version}
// value => []*MicroServiceKey (consumer对应的多个provider)
func GenerateConsumerDependencyRuleKey(domainProject string, in *registry.MicroServiceKey) string {
	return GenerateServiceDependencyRuleKey(DepsConsumer, domainProject, in)
}

// provider /cse-sr/ms/dep-rules/{domainProject}/p/{Environment}/{AppId}/{ServiceName}/{Version}
// provider /cse-sr/ms/dep-rules/{domainProject}/p/{Environment}/{ServiceName==*}
// value => []*MicroServiceKey (provider对应的多个consumer)
func GenerateProviderDependencyRuleKey(domainProject string, in *registry.MicroServiceKey) string {
	return GenerateServiceDependencyRuleKey(DepsProvider, domainProject, in)
}

// DependencyRule前缀    /cse-sr/ms/dep-rules/{domainProject}
func GetServiceDependencyRuleRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryDepsRuleKey,
		domainProject,
	}, SPLIT)
}

// /cse-sr/ms/dep-queue/{domainProject}
func GetServiceDependencyQueueRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryDepsQueueKey,
		domainProject,
	}, SPLIT)
}

// ConsumerDependency /cse-sr/ms/dep-queue/{domainProject}/{consumerId(服务id)}/{uuid} => pb.ConsumerDependency
func GenerateConsumerDependencyQueueKey(domainProject, consumerID, uuid string) string {
	return util.StringJoin([]string{
		GetServiceDependencyQueueRootKey(domainProject),
		consumerID,
		uuid,
	}, SPLIT)
}

func GetServiceDependencyRootKey(domainProject string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryServiceKey,
		RegistryDependencyKey,
		domainProject,
	}, SPLIT)
}

func GetDomainRootKey() string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryDomainKey,
	}, SPLIT)
}

// domin /cse-sr/domains/domain
func GenerateDomainKey(domain string) string {
	return util.StringJoin([]string{
		GetDomainRootKey(),
		domain,
	}, SPLIT)
}
func GenerateAccountKey(name string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		"accounts",
		name,
	}, SPLIT)
}
func GenerateRBACSecretKey() string {
	return util.StringJoin([]string{
		GetRootKey(),
		"rbac/secret",
	}, SPLIT)
}
func GetServerInfoKey() string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistrySysKey,
	}, SPLIT)
}

func GetMetricsRootKey() string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryMetricsKey,
	}, SPLIT)
}

func GenerateMetricsKey(name, utc, domain string) string {
	return util.StringJoin([]string{
		GetMetricsRootKey(),
		name,
		utc,
		domain,
	}, SPLIT)
}

// /cse-sr/projects/{domain}
func GetProjectRootKey(domain string) string {
	return util.StringJoin([]string{
		GetRootKey(),
		RegistryProjectKey,
		domain,
	}, SPLIT)
}

// project /cse-sr/projects/{domain}/{project}
func GenerateProjectKey(domain, project string) string {
	return util.StringJoin([]string{
		GetProjectRootKey(domain),
		project,
	}, SPLIT)
}
