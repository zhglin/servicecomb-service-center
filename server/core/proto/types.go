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

package proto

import (
	"time"

	"github.com/apache/servicecomb-service-center/pkg/util"
)

type ServerConfig struct {
	MaxHeaderBytes int64 `json:"maxHeaderBytes"` // rest 请求的头域最大长度
	MaxBodyBytes   int64 `json:"maxBodyBytes"`   // rest 读取的最大body 限制接收到的请求的Body的大小

	ReadHeaderTimeout string `json:"readHeaderTimeout"` // rest server配置 允许读取请求头的时间
	ReadTimeout       string `json:"readTimeout"`       // rest server配置 读取整个文件的最大持续时间,包括正文。
	IdleTimeout       string `json:"idleTimeout"`       // rest server配置 等待的最大时间 keep-live
	WriteTimeout      string `json:"writeTimeout"`      // rest server配置 写入response的超时时间

	LimitTTLUnit     string `json:"limitTTLUnit"`
	LimitConnections int64  `json:"limitConnections"`
	LimitIPLookup    string `json:"limitIPLookup"`

	SslEnabled    bool   `json:"sslEnabled,string"` // rest 是否开启ssl
	SslMinVersion string `json:"sslMinVersion"`
	SslVerifyPeer bool   `json:"sslVerifyPeer,string"`
	SslCiphers    string `json:"sslCiphers"`

	AutoSyncInterval string `json:"-"`
	// etcd压缩 保留最近多少个版本 最新版本-CompactIndexDelta
	CompactIndexDelta int64 `json:"-"`
	// etcd压缩的间隔时间
	CompactInterval string `json:"-"`
	// 是否开启性能监控
	EnablePProf bool `json:"enablePProf"`
	// discovery 是否开启缓存
	EnableCache bool `json:"enableCache"`

	LogRotateSize  int64  `json:"-"`
	LogBackupCount int64  `json:"-"`
	LogFilePath    string `json:"-"`
	LogLevel       string `json:"-"`
	LogFormat      string `json:"-"`
	LogSys         bool   `json:"-"`
	// 是否记录请求成功日志
	EnableAccessLog bool   `json:"-"`
	AccessLogFile   string `json:"-"`

	PluginsDir string `json:"-"`
	// 这里会记录各个plugins的信息
	Plugins util.JSONObject `json:"plugins"`
	// 是否自注册
	SelfRegister bool `json:"selfRegister"`

	//clear no-instance services
	// 是否清理没有instance的service
	ServiceClearEnabled bool `json:"serviceClearEnabled"`
	// 清理service的间隔时间(定时)
	ServiceClearInterval time.Duration `json:"serviceClearInterval"`
	//if a service's existence time reaches this value, it can be cleared
	// 清理的service的创建时间距离当前时间的间隔
	ServiceTTL time.Duration `json:"serviceTTL"`
	//CacheTTL is the ttl of cache discovery
	//缓存清理间隔，未设置就永远不清理
	CacheTTL time.Duration `json:"cacheTTL"`

	// if want disable Test Schema, SchemaDisable set true
	// 是否禁用修改schema功能
	SchemaDisable bool `json:"schemaDisable"`
}

// version 会读取etcd中的全局版本号进行更新 默认是0
type ServerInformation struct {
	Version string       `json:"version"`
	Config  ServerConfig `json:"-"`
}

func NewServerInformation() *ServerInformation {
	return &ServerInformation{Config: ServerConfig{Plugins: make(util.JSONObject)}}
}
