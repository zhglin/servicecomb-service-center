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

package backend

import (
	"context"
	"errors"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/backoff"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/apache/servicecomb-service-center/server/plugin"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	"sync"
	"time"
)

var (
	engineInstance *RegistryEngine
	singletonLock  sync.Mutex
)

const (
	// the same as v3rpc.MaxOpsPerTxn = 128
	MaxTxnNumberOneTime = 128 // etcd一次事务操作的最大数量
)

// 创建register组件
func NewEngine() (*RegistryEngine, error) {
	instance := plugin.Plugins().Registry()
	if instance == nil {
		return nil, errors.New("register center client plugin does not exist")
	}
	select {
	case err := <-instance.Err():
		plugin.Plugins().Reload(plugin.REGISTRY)
		return nil, err
	case <-instance.Ready():
	}
	return &RegistryEngine{
		Registry:  instance,
		goroutine: gopool.New(context.Background()),
	}, nil
}

func Registry() registry.Registry {
	return GetRegistryEngine()
}

// 获取register组件
func GetRegistryEngine() *RegistryEngine {
	if engineInstance == nil {
		singletonLock.Lock()
		for i := 0; engineInstance == nil; i++ {
			inst, err := NewEngine()
			if err != nil {
				log.Errorf(err, "get register center client failed")
			}
			engineInstance = inst

			if engineInstance != nil {
				singletonLock.Unlock()
				return engineInstance
			}

			t := backoff.GetBackoff().Delay(i)
			log.Errorf(nil, "initialize service center failed, retry after %s", t)
			<-time.After(t)
		}
		singletonLock.Unlock()
	}
	return engineInstance
}

// BatchCommitWithCmp 代理
func BatchCommit(ctx context.Context, opts []registry.PluginOp) error {
	_, err := BatchCommitWithCmp(ctx, opts, nil, nil)
	return err
}

// 分批执行etcd事务
func BatchCommitWithCmp(ctx context.Context, opts []registry.PluginOp,
	cmp []registry.CompareOp, fail []registry.PluginOp) (resp *registry.PluginResponse, err error) {
	lenOpts := len(opts)
	tmpLen := lenOpts
	var tmpOpts []registry.PluginOp
	for i := 0; tmpLen > 0; i++ {
		tmpLen = lenOpts - (i+1)*MaxTxnNumberOneTime
		if tmpLen > 0 {
			tmpOpts = opts[i*MaxTxnNumberOneTime : (i+1)*MaxTxnNumberOneTime]
		} else {
			tmpOpts = opts[i*MaxTxnNumberOneTime : lenOpts]
		}
		resp, err = Registry().TxnWithCmp(ctx, tmpOpts, cmp, fail)
		if err != nil || !resp.Succeeded {
			return
		}
	}
	return
}

// register plug
type RegistryEngine struct {
	registry.Registry
	goroutine *gopool.Pool
}

// 启动当前service_center节点
func (s *RegistryEngine) Start() error {
	err := s.selfRegister(context.Background())
	if err != nil {
		return err
	}

	// 续约
	s.heartBeatService()

	// 上报监控
	ReportScInstance()
	return nil
}

// service_center节点关闭
func (s *RegistryEngine) Stop() {
	s.goroutine.Close(true)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.unregisterInstance(ctx); err != nil {
		log.Error("stop registry engine failed", err)
	}
}

// 自注册
func (s *RegistryEngine) selfRegister(ctx context.Context) error {
	err := s.registryService(context.Background())
	if err != nil {
		return err
	}
	// 实例信息
	return s.registryInstance(context.Background())
}

// 注册当前service_center
func (s *RegistryEngine) registryService(pCtx context.Context) error {
	ctx := core.AddDefaultContextValue(pCtx)
	// 校验服务存不存在
	respE, err := core.ServiceAPI.Exist(ctx, core.GetExistenceRequest())
	if err != nil {
		log.Error("query service center existence failed", err)
		return err
	}
	// ServiceIndex存在
	if respE.Response.GetCode() == proto.Response_SUCCESS {
		log.Warnf("service center service[%s] already registered", respE.ServiceId)
		// Service不存在，不会进行重新注册
		respG, err := core.ServiceAPI.GetOne(ctx, core.GetServiceRequest(respE.ServiceId))
		if respG.Response.GetCode() != proto.Response_SUCCESS {
			log.Errorf(err, "query service center service[%s] info failed", respE.ServiceId)
			return fmt.Errorf("service center service file lost")
		}
		core.Service = respG.Service
		return nil
	}

	// 新建服务
	respS, err := core.ServiceAPI.Create(ctx, core.CreateServiceRequest())
	if err != nil {
		log.Error("register service center failed", err)
		return err
	}
	core.Service.ServiceId = respS.ServiceId
	log.Infof("register service center service[%s]", respS.ServiceId)
	return nil
}

// 注册当前实例
func (s *RegistryEngine) registryInstance(pCtx context.Context) error {
	core.Instance.InstanceId = ""
	core.Instance.ServiceId = core.Service.ServiceId

	ctx := core.AddDefaultContextValue(pCtx)

	// 注册
	respI, err := core.InstanceAPI.Register(ctx, core.RegisterInstanceRequest())
	if err != nil {
		log.Error("register failed", err)
		return err
	}
	if respI.Response.GetCode() != proto.Response_SUCCESS {
		err = fmt.Errorf("register service center[%s] instance failed, %s",
			core.Instance.ServiceId, respI.Response.GetMessage())
		log.Error(err.Error(), nil)
		return err
	}
	core.Instance.InstanceId = respI.InstanceId
	log.Infof("register service center instance[%s/%s], endpoints is %s",
		core.Service.ServiceId, respI.InstanceId, core.Instance.Endpoints)
	return nil
}

// 取消instance实例  不删除service
func (s *RegistryEngine) unregisterInstance(pCtx context.Context) error {
	if len(core.Instance.InstanceId) == 0 {
		return nil
	}
	ctx := core.AddDefaultContextValue(pCtx)
	respI, err := core.InstanceAPI.Unregister(ctx, core.UnregisterInstanceRequest())
	if err != nil {
		log.Error("unregister failed", err)
		return err
	}
	if respI.Response.GetCode() != proto.Response_SUCCESS {
		err = fmt.Errorf("unregister service center instance[%s/%s] failed, %s",
			core.Instance.ServiceId, core.Instance.InstanceId, respI.Response.GetMessage())
		log.Error(err.Error(), nil)
		return err
	}
	log.Warnf("unregister service center instance[%s/%s]",
		core.Service.ServiceId, core.Instance.InstanceId)
	return nil
}

//续约
func (s *RegistryEngine) sendHeartBeat(pCtx context.Context) {
	ctx := core.AddDefaultContextValue(pCtx)
	respI, err := core.InstanceAPI.Heartbeat(ctx, core.HeartbeatRequest())
	if err != nil {
		log.Error("sen heartbeat failed", err)
		return
	}
	if respI.Response.GetCode() == proto.Response_SUCCESS {
		log.Debugf("update service center instance[%s/%s] heartbeat",
			core.Instance.ServiceId, core.Instance.InstanceId)
		return
	}
	log.Errorf(err, "update service center instance[%s/%s] heartbeat failed",
		core.Instance.ServiceId, core.Instance.InstanceId)

	//服务不存在，创建服务
	err = s.selfRegister(pCtx)
	if err != nil {
		log.Errorf(err, "retry to register[%s/%s/%s/%s] failed",
			core.Service.Environment, core.Service.AppId, core.Service.ServiceName, core.Service.Version)
	}
}

// 定时续约
func (s *RegistryEngine) heartBeatService() {
	s.goroutine.Do(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
				// 续约时间间隔
			case <-time.After(time.Duration(core.Instance.HealthCheck.Interval) * time.Second):
				s.sendHeartBeat(ctx)
			}
		}
	})
}
