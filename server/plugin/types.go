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

package plugin

import (
	"strconv"

	"github.com/apache/servicecomb-service-center/pkg/util"
	"github.com/apache/servicecomb-service-center/server/core"
)

// PluginName is an alias, it represents a plugin interface.
// plugin名称
type Name int

// ImplName is an alias，it represents a plugin interface implementation.
type ImplName string


// Instance is an instance of a plugin interface which is represented by
// Name.
//plugin 接口
type Instance interface{}

// String implements fmt.Stringer.
// plugin对应的名称
func (pn Name) String() string {
	if name, ok := pluginNames[pn]; ok {
		return name
	}
	return "PLUGIN" + strconv.Itoa(int(pn))
}

// ActiveConfigs returns all the server's plugin config
// 获取serviceInfo中使用的plugins名称
func (pn Name) ActiveConfigs() util.JSONObject {
	return core.ServerInfo.Config.Plugins.Object(pn.String())
}

// ClearConfigs clears the server's plugin config
func (pn Name) ClearConfigs() {
	core.ServerInfo.Config.Plugins.Set(pn.String(), nil)
}

// Plugin generates a plugin instance
// Plugin holds the 'Name' and 'ImplName'
// to manage the plugin instance generation.
// 标识一个plugin
type Plugin struct {
	// plugin的标识号
	PName Name
	// 同一个plugin的不同实现的标识名称
	Name  ImplName
	// New news an instance of 'PName' represented plugin interface
	// 创建plugin的实例的函数
	New func() Instance
}
