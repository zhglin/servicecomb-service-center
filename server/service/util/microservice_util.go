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
	"encoding/json"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"strings"

	"github.com/apache/servicecomb-service-center/pkg/log"
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	apt "github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/plugin"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"github.com/apache/servicecomb-service-center/server/plugin/quota"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"

	"context"
)

/*
	get Service by service id
*/
func GetServiceWithRev(ctx context.Context, domain string, id string, rev int64) (*pb.MicroService, error) {
	key := apt.GenerateServiceKey(domain, id)
	serviceResp, err := backend.Store().Service().Search(ctx,
		registry.WithStrKey(key),
		registry.WithRev(rev))
	if err != nil {
		return nil, err
	}
	if len(serviceResp.Kvs) == 0 {
		return nil, nil
	}
	return serviceResp.Kvs[0].Value.(*pb.MicroService), nil
}

// 根据serviceId获取service信息
func GetService(ctx context.Context, domainProject string, serviceID string) (*pb.MicroService, error) {
	key := apt.GenerateServiceKey(domainProject, serviceID)
	opts := append(FromContext(ctx), registry.WithStrKey(key))
	serviceResp, err := backend.Store().Service().Search(ctx, opts...)
	if err != nil {
		return nil, err
	}
	if len(serviceResp.Kvs) == 0 {
		return nil, nil
	}
	return serviceResp.Kvs[0].Value.(*pb.MicroService), nil
}

// GetServiceFromCache gets service from cache
// 直从本地cache获取service
func GetServiceFromCache(domainProject string, serviceID string) *pb.MicroService {
	ctx := context.WithValue(context.WithValue(context.Background(),
		util.CtxCacheOnly, "1"),
		util.CtxGlobal, "1")
	svc, _ := GetService(ctx, domainProject, serviceID)
	return svc
}

// domainProject下所有service
func getServicesRawData(ctx context.Context, domainProject string) ([]*discovery.KeyValue, error) {
	key := apt.GenerateServiceKey(domainProject, "")
	opts := append(FromContext(ctx),
		registry.WithStrKey(key),
		registry.WithPrefix())
	resp, err := backend.Store().Service().Search(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return resp.Kvs, err
}

//GetAllServicesAcrossDomainProject get services of all domains, projects
//the map's key is domainProject
// 获取所有的service
func GetAllServicesAcrossDomainProject(ctx context.Context) (map[string][]*pb.MicroService, error) {
	key := apt.GetServiceRootKey("")
	opts := append(FromContext(ctx),
		registry.WithStrKey(key),
		registry.WithPrefix())

	//获取全量的service
	serviceResp, err := backend.Store().Service().Search(ctx, opts...)
	if err != nil {
		return nil, err
	}

	//不存在service
	services := make(map[string][]*pb.MicroService)
	if len(serviceResp.Kvs) == 0 {
		return services, nil
	}

	for _, value := range serviceResp.Kvs {
		prefix := util.BytesToStringWithNoCopy(value.Key)
		parts := strings.Split(prefix, apt.SPLIT)
		// prefix=/cse-sr/ms/files/domin/project/serviceId
		if len(parts) != 7 {
			continue
		}
		domainProject := parts[4] + apt.SPLIT + parts[5]
		microService, ok := value.Value.(*pb.MicroService)
		if !ok {
			log.Errorf(nil, "backend key[%s]'s value is not type *pb.MicroService", prefix)
			continue
		}
		services[domainProject] = append(services[domainProject], microService)
	}
	return services, nil
}

// 获取domainProject下所有service
func GetServicesByDomainProject(ctx context.Context, domainProject string) ([]*pb.MicroService, error) {
	kvs, err := getServicesRawData(ctx, domainProject)
	if err != nil {
		return nil, err
	}
	services := []*pb.MicroService{}
	for _, kv := range kvs {
		services = append(services, kv.Value.(*pb.MicroService))
	}
	return services, nil
}

// 根据service信息获取service的id
func GetServiceID(ctx context.Context, key *pb.MicroServiceKey) (serviceID string, err error) {
	serviceID, err = searchServiceID(ctx, key)
	if err != nil {
		return
	}
	if len(serviceID) == 0 {
		// 别名查询
		log.Debugf("could not search microservice[%s/%s/%s/%s] id by 'serviceName', now try 'alias'",
			key.Environment, key.AppId, key.ServiceName, key.Version)
		return searchServiceIDFromAlias(ctx, key)
	}
	return
}

// 从discovery获取serviceId
func searchServiceID(ctx context.Context, key *pb.MicroServiceKey) (string, error) {
	opts := append(FromContext(ctx), registry.WithStrKey(apt.GenerateServiceIndexKey(key)))
	resp, err := backend.Store().ServiceIndex().Search(ctx, opts...)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return resp.Kvs[0].Value.(string), nil
}

// 从discovery的alias获取serviceId
func searchServiceIDFromAlias(ctx context.Context, key *pb.MicroServiceKey) (string, error) {
	opts := append(FromContext(ctx), registry.WithStrKey(apt.GenerateServiceAliasKey(key)))
	resp, err := backend.Store().ServiceAlias().Search(ctx, opts...)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return resp.Kvs[0].Value.(string), nil
}

func GetServiceAllVersions(ctx context.Context, key *pb.MicroServiceKey, alias bool) (*discovery.Response, error) {
	copy := *key
	copy.Version = ""
	var (
		prefix  string
		indexer discovery.Indexer
	)
	if alias {
		prefix = apt.GenerateServiceAliasKey(&copy)
		indexer = backend.Store().ServiceAlias()
	} else {
		prefix = apt.GenerateServiceIndexKey(&copy)
		indexer = backend.Store().ServiceIndex()
	}
	opts := append(FromContext(ctx),
		registry.WithStrKey(prefix),
		registry.WithPrefix(),
		registry.WithDescendOrder())
	resp, err := indexer.Search(ctx, opts...)
	return resp, err
}

// 根据版本规则查找serviceIds
func FindServiceIds(ctx context.Context, versionRule string, key *pb.MicroServiceKey) ([]string, bool, error) {
	// 版本规则
	match := ParseVersionRule(versionRule)
	if match == nil {
		copy := *key
		copy.Version = versionRule
		serviceID, err := GetServiceID(ctx, &copy)
		if err != nil {
			return nil, false, err
		}
		if len(serviceID) > 0 {
			return []string{serviceID}, true, nil
		}
		return nil, false, nil
	}

	searchAlias := false
	alsoFindAlias := len(key.Alias) > 0

FIND_RULE:
	resp, err := GetServiceAllVersions(ctx, key, searchAlias)
	if err != nil {
		return nil, false, err
	}
	if len(resp.Kvs) == 0 {
		if !alsoFindAlias {
			return nil, false, nil
		}
		searchAlias = true
		alsoFindAlias = false
		goto FIND_RULE
	}
	return match(resp.Kvs), true, nil
}

// 校验serviceId是否存在
func ServiceExist(ctx context.Context, domainProject string, serviceID string) bool {
	opts := append(FromContext(ctx),
		registry.WithStrKey(apt.GenerateServiceKey(domainProject, serviceID)),
		registry.WithCountOnly())
	resp, err := backend.Store().Service().Search(ctx, opts...)
	if err != nil || resp.Count == 0 {
		return false
	}
	return true
}

// 获取指定domainProject下的service
func GetAllServiceUtil(ctx context.Context) ([]*pb.MicroService, error) {
	domainProject := util.ParseDomainProject(ctx)
	services, err := GetServicesByDomainProject(ctx, domainProject)
	if err != nil {
		return nil, err
	}
	return services, nil
}

func RemandServiceQuota(ctx context.Context) {
	plugin.Plugins().Quota().RemandQuotas(ctx, quota.MicroServiceQuotaType)
}

func RemandInstanceQuota(ctx context.Context) {
	plugin.Plugins().Quota().RemandQuotas(ctx, quota.MicroServiceInstanceQuotaType)
}

// 修改service
func UpdateService(domainProject string, serviceID string, service *pb.MicroService) (opt registry.PluginOp, err error) {
	opt = registry.PluginOp{}
	key := apt.GenerateServiceKey(domainProject, serviceID)
	data, err := json.Marshal(service)
	if err != nil {
		log.Errorf(err, "marshal service file failed")
		return
	}
	opt = registry.OpPut(registry.WithStrKey(key), registry.WithValue(data))
	return
}

func GetOneDomainProjectServiceCount(ctx context.Context, domainProject string) (int64, error) {
	key := apt.GenerateServiceKey(domainProject, "")
	opts := append(FromContext(ctx),
		registry.WithStrKey(key),
		registry.WithCountOnly(),
		registry.WithPrefix())
	resp, err := backend.Store().Service().Search(ctx, opts...)
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func GetOneDomainProjectInstanceCount(ctx context.Context, domainProject string) (int64, error) {
	key := apt.GetInstanceRootKey(domainProject) + "/"
	opts := append(FromContext(ctx),
		registry.WithStrKey(key),
		registry.WithCountOnly(),
		registry.WithPrefix())
	resp, err := backend.Store().Instance().Search(ctx, opts...)
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

// 设置service的默认数据
func SetServiceDefaultValue(service *pb.MicroService) {
	if len(service.AppId) == 0 {
		service.AppId = proto.APP_ID
	}
	if len(service.Version) == 0 {
		service.Version = proto.VERSION
	}
	if len(service.Level) == 0 {
		service.Level = "BACK"
	}
	if len(service.Status) == 0 {
		service.Status = pb.MS_UP
	}
}
