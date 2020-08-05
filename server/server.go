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

package server

import (
	//plugin
	_ "github.com/apache/servicecomb-service-center/server/service/event"
	"github.com/apache/servicecomb-service-center/server/service/rbac"
)
import (
	"fmt"
	"os"
	"time"

	"context"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	nf "github.com/apache/servicecomb-service-center/pkg/notify"
	"github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/mux"
	"github.com/apache/servicecomb-service-center/server/notify"
	"github.com/apache/servicecomb-service-center/server/plugin"
	serviceUtil "github.com/apache/servicecomb-service-center/server/service/util"
	"github.com/apache/servicecomb-service-center/server/task"
	"github.com/apache/servicecomb-service-center/version"
	"github.com/astaxie/beego"
)

const buildin = "buildin"

var server ServiceCenterServer

// 启动服务
func Run() {
	server.Run()
}

type ServiceCenterServer struct {
	apiService    *APIServer
	notifyService *nf.Service
	// discovery
	cacheService  *backend.KvStore
	goroutine     *gopool.Pool
}

func (s *ServiceCenterServer) Run() {
	s.initialize()

	s.startServices()

	s.waitForQuit()
}

func (s *ServiceCenterServer) waitForQuit() {
	err := <-s.apiService.Err()
	if err != nil {
		log.Errorf(err, "service center catch errors")
	}

	s.Stop()
}

// 是否需要升级etcd中的版本号
func (s *ServiceCenterServer) needUpgrade() bool {
	err := LoadServerVersion()
	if err != nil {
		log.Errorf(err, "check version failed, can not load the system config")
		return false
	}

	// etcd中的version 是否比 version.Ver().Version 小
	update := !serviceUtil.VersionMatchRule(core.ServerInfo.Version,
		fmt.Sprintf("%s+", version.Ver().Version))
	if !update && version.Ver().Version != core.ServerInfo.Version {
		log.Warnf(
			"there is a higher version '%s' in cluster, now running '%s' version may be incompatible",
			core.ServerInfo.Version, version.Ver().Version)
	}

	return update
}

// 同步etcd中记录的service_center节点的最大版本号
func (s *ServiceCenterServer) loadOrUpgradeServerVersion() {
	lock, err := mux.Lock(mux.GlobalLock)

	if err != nil {
		log.Errorf(err, "wait for server ready failed")
		os.Exit(1)
	}

	//etcd中的全局版本是否小于此节点的version
	if s.needUpgrade() {
		core.ServerInfo.Version = version.Ver().Version

		// 更新etcd中的最大版本号
		if err := UpgradeServerVersion(); err != nil {
			log.Errorf(err, "upgrade server version failed")
			os.Exit(1)
		}
	}

	err = lock.Unlock()
	if err != nil {
		log.Error("", err)
	}
}

// etcd压缩
func (s *ServiceCenterServer) compactBackendService() {
	delta := core.ServerInfo.Config.CompactIndexDelta
	if delta <= 0 || len(core.ServerInfo.Config.CompactInterval) == 0 {
		return
	}
	interval, err := time.ParseDuration(core.ServerInfo.Config.CompactInterval)
	if err != nil {
		log.Errorf(err, "invalid compact interval %s, reset to default interval 12h", core.ServerInfo.Config.CompactInterval)
		interval = 12 * time.Hour
	}
	s.goroutine.Do(func(ctx context.Context) {
		log.Infof("enabled the automatic compact mechanism, compact once every %s, reserve %d",
			core.ServerInfo.Config.CompactInterval, delta)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				lock, err := mux.Try(mux.GlobalLock)
				if err != nil {
					log.Errorf(err, "can not compact backend by this service center instance now")
					continue
				}

					err = backend.Registry().Compact(ctx, delta)
				if err != nil {
					log.Error("", err)
				}

				if err := lock.Unlock(); err != nil {
					log.Error("", err)
				}
			}
		}
	})
}

// clear services who have no instance
// 清理无instance的service
func (s *ServiceCenterServer) clearNoInstanceServices() {
	if !core.ServerInfo.Config.ServiceClearEnabled {
		return
	}
	log.Infof("service clear enabled, interval: %s, service TTL: %s",
		core.ServerInfo.Config.ServiceClearInterval,
		core.ServerInfo.Config.ServiceTTL)

	s.goroutine.Do(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(core.ServerInfo.Config.ServiceClearInterval):
				lock, err := mux.Try(mux.ServiceClearLock)
				if err != nil {
					log.Errorf(err, "can not clear no instance services by this service center instance now")
					continue
				}
				err = task.ClearNoInstanceServices(core.ServerInfo.Config.ServiceTTL)
				if err := lock.Unlock(); err != nil {
					log.Error("", err)
				}
				if err != nil {
					log.Errorf(err, "no-instance services cleanup failed")
					continue
				}
				log.Info("no-instance services cleanup succeed")
			}
		}
	})
}

// center server初始化
func (s *ServiceCenterServer) initialize() {
	s.cacheService = backend.Store()
	s.apiService = GetAPIServer()
	s.notifyService = notify.GetNotifyCenter()
	s.goroutine = gopool.New(context.Background())
}

func (s *ServiceCenterServer) startServices() {
	// notifications
	s.notifyService.Start()

	// load server plugins  创建各个plugin实例
	plugin.LoadPlugins()
	rbac.Init()
	// check version 保持etcd中记录的是service_center节点的最大版本号
	if core.ServerInfo.Config.SelfRegister {
		s.loadOrUpgradeServerVersion()
	}

	// cache mechanism
	// 启动discovery管理器
	s.cacheService.Run()
	<-s.cacheService.Ready()

	// 支持registry
	if buildin != beego.AppConfig.DefaultString("registry_plugin", buildin) {
		// compact backend automatically 压缩etcd历史版本
		s.compactBackendService()
		// clean no-instance services automatically  service清理
		s.clearNoInstanceServices()
	}

	// api service
	s.startAPIService()
}

func (s *ServiceCenterServer) startAPIService() {
	restIP := beego.AppConfig.String("httpaddr")
	restPort := beego.AppConfig.String("httpport")
	rpcIP := beego.AppConfig.DefaultString("rpcaddr", "")
	rpcPort := beego.AppConfig.DefaultString("rpcport", "")

	host, err := os.Hostname()
	if err != nil {
		host = restIP
		log.Errorf(err, "parse hostname failed")
	}
	core.Instance.HostName = host
	s.apiService.AddListener(REST, restIP, restPort)
	s.apiService.AddListener(RPC, rpcIP, rpcPort)
	s.apiService.Start()
}

func (s *ServiceCenterServer) Stop() {
	if s.apiService != nil {
		s.apiService.Stop()
	}

	if s.notifyService != nil {
		s.notifyService.Stop()
	}

	if s.cacheService != nil {
		s.cacheService.Stop()
	}

	s.goroutine.Close(true)

	gopool.CloseAndWait()

	backend.Registry().Close()

	log.Warnf("service center stopped")
	log.Sync()
}
