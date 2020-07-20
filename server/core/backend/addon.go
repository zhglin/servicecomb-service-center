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
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
)
//addOn 插件接口
type AddOn interface {
	Name() string
	Config() *discovery.Config
}
//插件实例
type addOn struct {
	name string	//名称
	cfg  *discovery.Config //配置
}
//获取名称
func (e *addOn) Name() string {
	return e.name
}
//获取配置
func (e *addOn) Config() *discovery.Config {
	return e.cfg
}
//创建AddOn
func NewAddOn(name string, cfg *discovery.Config) AddOn {
	return &addOn{
		name: name,
		cfg:  cfg,
	}
}
