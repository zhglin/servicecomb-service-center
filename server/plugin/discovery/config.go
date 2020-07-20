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
package discovery

import (
	"fmt"
	pb "github.com/apache/servicecomb-service-center/server/core/proto"
	"time"
)
//addOn配置
type Config struct {
	// Key is the prefix to unique specify resource type
	// key前缀
	Key          string
	// 初始化cache大小
	InitSize     int
	// 全量从etcd拉取数据时 etcd链接超时时间
	Timeout      time.Duration
	Period       time.Duration
	// 事件处理函数
	DeferHandler DeferHandler
	// 事件处理链
	OnEvent      KvEventFunc
	// 从etcd获取的数据进行解析的函数
	Parser       pb.Parser
}

func (cfg *Config) String() string {
	return fmt.Sprintf("{key: %s, timeout: %s, period: %s}",
		cfg.Key, cfg.Timeout, cfg.Period)
}

func (cfg *Config) WithPrefix(key string) *Config {
	cfg.Key = key
	return cfg
}

func (cfg *Config) WithInitSize(size int) *Config {
	cfg.InitSize = size
	return cfg
}

func (cfg *Config) WithTimeout(ot time.Duration) *Config {
	cfg.Timeout = ot
	return cfg
}

func (cfg *Config) WithPeriod(ot time.Duration) *Config {
	cfg.Period = ot
	return cfg
}

func (cfg *Config) WithDeferHandler(h DeferHandler) *Config {
	cfg.DeferHandler = h
	return cfg
}
// 设置一个处理器 没啥用 AppendEventFunc支持此功能
func (cfg *Config) WithEventFunc(f KvEventFunc) *Config {
	cfg.OnEvent = f
	return cfg
}
// 把事件处理函数组装成处理链
func (cfg *Config) AppendEventFunc(f KvEventFunc) *Config {
	if prev := cfg.OnEvent; prev != nil {
		next := f
		f = func(evt KvEvent) {
			prev(evt)
			next(evt)
		}
	}
	cfg.OnEvent = f
	return cfg
}

func (cfg *Config) WithParser(parser pb.Parser) *Config {
	cfg.Parser = parser
	return cfg
}

func Configure() *Config {
	return &Config{
		Key:      "/",
		Timeout:  DEFAULT_TIMEOUT,
		Period:   time.Second,
		InitSize: DEFAULT_CACHE_INIT_SIZE,
		Parser:   pb.BytesParser,
	}
}
