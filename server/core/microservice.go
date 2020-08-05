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
	"context"
	"github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/apache/servicecomb-service-center/version"
	"github.com/astaxie/beego"
	"os"
	"strings"
)

var (
	ServiceAPI         proto.ServiceCtrlServer
	InstanceAPI        proto.ServiceInstanceCtrlServerEx
	Service            *registry.MicroService
	Instance           *registry.MicroServiceInstance
	sharedServiceNames map[string]struct{}
)

const (
	RegistryDomain        = "default"
	RegistryProject       = "default"
	RegistryDomainProject = "default/default"

	RegistryAppID        = "default"
	RegistryServiceName  = "SERVICECENTER"
	RegistryServiceAlias = "SERVICECENTER"

	RegistryDefaultLeaseRenewalinterval int32 = 30  // 默认续约时间
	RegistryDefaultLeaseRetrytimes      int32 = 3	// etcd租约时间 30 * 3

	CtxScSelf     = "_sc_self"
	CtxScRegistry = "_registryOnly"
)

func init() {
	prepareSelfRegistration()
	//设置共享Environment
	SetSharedMode()
}

func prepareSelfRegistration() {
	Service = &registry.MicroService{
		Environment: registry.ENV_PROD,
		AppId:       RegistryAppID,
		ServiceName: RegistryServiceName,
		Alias:       RegistryServiceAlias,
		Version:     version.Ver().Version,
		Status:      registry.MS_UP,
		Level:       "BACK",
		Schemas: []string{
			"servicecenter.grpc.api.ServiceCtrl",
			"servicecenter.grpc.api.ServiceInstanceCtrl",
		},
		Properties: map[string]string{
			proto.PROP_ALLOW_CROSS_APP: "true",
		},
	}
	if beego.BConfig.RunMode == "dev" {
		Service.Environment = registry.ENV_DEV
	}

	Instance = &registry.MicroServiceInstance{
		Status: registry.MSI_UP,
		HealthCheck: &registry.HealthCheck{
			Mode:     registry.CHECK_BY_HEARTBEAT,
			Interval: RegistryDefaultLeaseRenewalinterval,
			Times:    RegistryDefaultLeaseRetrytimes,
		},
	}
}

// 注册中心节点的默认context
func AddDefaultContextValue(ctx context.Context) context.Context {
	return util.SetContext(util.SetContext(util.SetDomainProject(ctx,
		RegistryDomain, RegistryProject),
		CtxScSelf, true),
		CtxScRegistry, "1")
}

func IsDefaultDomainProject(domainProject string) bool {
	return domainProject == RegistryDomainProject
}
//读取配置
func SetSharedMode() {
	sharedServiceNames = make(map[string]struct{})
	for _, s := range strings.Split(os.Getenv("CSE_SHARED_SERVICES"), ",") {
		if len(s) > 0 {
			sharedServiceNames[s] = struct{}{}
		}
	}
	//注册中心节点默认共享 （注册中心的domain，project，appId都为default）
	sharedServiceNames[Service.ServiceName] = struct{}{}
}

//服务是否是不分Environment
func IsShared(key *registry.MicroServiceKey) bool {
	//domain,project 必须都为 default
	if !IsDefaultDomainProject(key.Tenant) {
		return false
	}
		//AppId必须==default
		if key.AppId != RegistryAppID {
		return false
	}
	_, ok := sharedServiceNames[key.ServiceName]
	if !ok {
		_, ok = sharedServiceNames[key.Alias]
	}
	return ok
}

// 是否是service_center节点
func IsSCInstance(ctx context.Context) bool {
	b, _ := ctx.Value(CtxScSelf).(bool)
	return b
}

// 服务是否存在的请求参数
func GetExistenceRequest() *registry.GetExistenceRequest {
	return &registry.GetExistenceRequest{
		Type:        proto.EXISTENCE_MS,
		Environment: Service.Environment,
		AppId:       Service.AppId,
		ServiceName: Service.ServiceName,
		Version:     Service.Version,
	}
}

func GetServiceRequest(serviceID string) *registry.GetServiceRequest {
	return &registry.GetServiceRequest{
		ServiceId: serviceID,
	}
}

func CreateServiceRequest() *registry.CreateServiceRequest {
	return &registry.CreateServiceRequest{
		Service: Service,
	}
}

func RegisterInstanceRequest() *registry.RegisterInstanceRequest {
	return &registry.RegisterInstanceRequest{
		Instance: Instance,
	}
}

func UnregisterInstanceRequest() *registry.UnregisterInstanceRequest {
	return &registry.UnregisterInstanceRequest{
		ServiceId:  Instance.ServiceId,
		InstanceId: Instance.InstanceId,
	}
}

func HeartbeatRequest() *registry.HeartbeatRequest {
	return &registry.HeartbeatRequest{
		ServiceId:  Instance.ServiceId,
		InstanceId: Instance.InstanceId,
	}
}
