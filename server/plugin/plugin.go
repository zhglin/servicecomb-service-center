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

package plugin

import (
	"sync"

	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/plugin"
	"github.com/apache/servicecomb-service-center/pkg/util"

	"github.com/astaxie/beego"
)

//初始化plugin管理器
var pluginMgr = &Manager{}

func init() {
	pluginMgr.Initialize()
}

//plugin实例
type wrapInstance struct {
	dynamic  bool
	instance Instance
	lock     sync.RWMutex
}

// Manager manages plugin instance generation.
// Manager keeps the plugin instance currently used by server
// for every plugin interface.
//plugin管理器
type Manager struct {
	//plugin配置 PluginName plugin名字 PluginImplName plugin子名称 一个plugin可以有多个实现
	plugins map[Name]map[ImplName]*Plugin
	//plugin实例
	instances map[Name]*wrapInstance
}

// Initialize initializes the struct
// 初始化plugin管理器
func (pm *Manager) Initialize() {
	pm.plugins = make(map[Name]map[ImplName]*Plugin, int(typeEnd))
	pm.instances = make(map[Name]*wrapInstance, int(typeEnd))
	//初始化每个plugin的实例
	for t := Name(0); t != typeEnd; t++ {
		pm.instances[t] = &wrapInstance{}
	}
}

// ReloadAll reloads all the plugin instances
// 重置所有的plugin实例
func (pm *Manager) ReloadAll() {
	for pn := range pm.instances {
		pm.Reload(pn)
	}
}

// Register registers a 'Plugin'
// unsafe
// 向plugin管理器注册插件
func (pm *Manager) Register(p Plugin) {
	m, ok := pm.plugins[p.PName]
	if !ok {
		m = make(map[ImplName]*Plugin, 5)
	}
	m[p.Name] = &p
	pm.plugins[p.PName] = m
	log.Infof("load '%s' plugin named '%s'", p.PName, p.Name)
}

// Get gets a 'Plugin'
// 获plugin信息
func (pm *Manager) Get(pn Name, name ImplName) *Plugin {
	m, ok := pm.plugins[pn]
	if !ok {
		return nil
	}
	return m[name]
}

// Instance gets an plugin instance.
// What plugin instance you get is depended on the supplied go plugin files
// (high priority) or the plugin config(low priority)
//
// The go plugin file should be {plugins_dir}/{Name}_plugin.so.
// ('plugins_dir' must be configured as a valid path in service-center config.)
// The plugin config in service-center config should be:
// {Name}_plugin = {ImplName}
//
// e.g. For registry plugin, you can set a config in app.conf:
// plugins_dir = /home, and supply a go plugin file: /home/registry_plugin.so;
// or if you want to use etcd as registry, you can set a config in app.conf:
// registry_plugin = etcd.
// 获取组件实例 不存在就创建
func (pm *Manager) Instance(pn Name) Instance {
	wi := pm.instances[pn]
	wi.lock.RLock()
	if wi.instance != nil {
		wi.lock.RUnlock()
		return wi.instance
	}
	wi.lock.RUnlock()

	wi.lock.Lock()
	if wi.instance != nil {
		wi.lock.Unlock()
		return wi.instance
	}
	//创建Plugin实例
	pm.New(pn)
	wi.lock.Unlock()

	return wi.instance
}

// New initializes and sets the instance of a plugin interface,
// but not returns it.
// Use 'Instance' if you want to get the plugin instance.
// We suggest you to use 'Instance' instead of 'New'.
func (pm *Manager) New(pn Name) {
	var (
		title = STATIC //非动态组件
		f     func() Instance
	)

	wi := pm.instances[pn]
	p := pm.existDynamicPlugin(pn)
	if p != nil {
		// Dynamic plugin has high priority.
		wi.dynamic = true
		title = DYNAMIC //静态组件
		f = p.New
	} else {
		wi.dynamic = false
		m, ok := pm.plugins[pn]
		if !ok {
			return
		}
		//根据plugin配置生成对应的组件实例
		name := beego.AppConfig.DefaultString(pn.String()+"_plugin", BUILDIN)
		p, ok = m[ImplName(name)]
		if !ok {
			return
		}

		f = p.New
		//把配置使用的plugin设置到 serviceInfo中
		pn.ActiveConfigs().Set(keyPluginName, name)
	}
	log.Infof("call %s '%s' plugin %s(), new a '%s' instance",
		title, p.PName, util.FuncName(f), p.Name)

	wi.instance = f()
}

// Reload reloads the instance of the specified plugin interface.
// 重置plugin实例
func (pm *Manager) Reload(pn Name) {
	wi := pm.instances[pn]
	wi.lock.Lock()
	wi.instance = nil
	//清理ServerInfo config中的plugins
	pn.ClearConfigs()
	wi.lock.Unlock()
}

//是否是动态组件
func (pm *Manager) existDynamicPlugin(pn Name) *Plugin {
	m, ok := pm.plugins[pn]
	if !ok {
		return nil
	}
	// 'buildin' implement of all plugins should call DynamicPluginFunc()
	if plugin.GetLoader().Exist(pn.String()) {
		return m[BUILDIN]
	}
	return nil
}

// Plugins returns the 'PluginManager'.
// 返回plugin管理器
func Plugins() *Manager {
	return pluginMgr
}

// RegisterPlugin registers a 'Plugin'.
// 注册一个plugin
func RegisterPlugin(p Plugin) {
	Plugins().Register(p)
}

// LoadPlugins loads and sets all the plugin interfaces's instance.
// 创建所有已注册的plugin
func LoadPlugins() {
	for t := Name(0); t != typeEnd; t++ {
		Plugins().Instance(t)
	}
}
