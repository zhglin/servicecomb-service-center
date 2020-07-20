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
package plugins

import (
	pg "plugin"
	"sync"

	"github.com/apache/servicecomb-service-center/pkg/plugin"
)

var (
	pluginMgr    Manager
	pluginConfig map[string]string
)

type Manager map[PluginType]*pluginMap

type PluginInstance interface{}

type pluginMap struct {
	plugins  map[string]*Plugin
	dynamic  bool
	instance PluginInstance
	lock     sync.RWMutex
}

type Plugin struct {
	Kind PluginType
	Name string
	New  func() PluginInstance
}

func init() {
	pluginMgr = make(Manager, pluginTotal)
	for t := PluginType(0); t != pluginTotal; t++ {
		pluginMgr[t] = &pluginMap{plugins: map[string]*Plugin{}}
	}

	pluginConfig = make(map[string]string, pluginTotal)
}

func (m Manager) Register(p *Plugin) {
	pm, ok := m[p.Kind]
	if !ok {
		pm = &pluginMap{plugins: map[string]*Plugin{}}
	}
	pm.plugins[p.Name] = p
	m[p.Kind] = pm
}

func (m Manager) Get(pn PluginType, name string) *Plugin {
	pm, ok := m[pn]
	if !ok {
		return nil
	}
	return pm.plugins[name]
}

func (m Manager) Instance(pn PluginType) PluginInstance {
	pm := m[pn]
	pm.lock.RLock()
	if pm.instance != nil {
		pm.lock.RUnlock()
		return pm.instance
	}
	pm.lock.RUnlock()

	pm.lock.Lock()
	if pm.instance != nil {
		pm.lock.Unlock()
		return pm.instance
	}
	m.New(pn)
	pm.lock.Unlock()
	return pm.instance
}

func (m Manager) New(pn PluginType) {
	var (
		//title = STATIC
		f func() PluginInstance
	)

	wi := m[pn]
	p := m.existDynamicPlugin(pn)
	if p != nil {
		wi.dynamic = true
		//title = DYNAMIC
		f = p.New
	} else {
		wi.dynamic = false
		pm, ok := m[pn]
		if !ok {
			return
		}

		name, ok := pluginConfig[pn.String()]
		if !ok {
			name = BUILDIN
		}

		p, ok = pm.plugins[name]
		if !ok {
			return
		}

		f = p.New
	}
	wi.instance = f()
}

func (m Manager) existDynamicPlugin(pn PluginType) *Plugin {
	pm, ok := m[pn]
	if !ok {
		return nil
	}
	// 'buildin' implement of all plugins should call DynamicPluginFunc()
	if plugin.GetLoader().Exist(pn.String()) {
		return pm.plugins[BUILDIN]
	}
	return nil
}

func DynamicPluginFunc(pn PluginType, funcName string) pg.Symbol {
	if wi, ok := pluginMgr[pn]; ok && !wi.dynamic {
		return nil
	}

	f, err := plugin.FindFunc(pn.String(), funcName)
	if err != nil {
		return nil
	}
	return f
}
func Plugins() Manager {
	return pluginMgr
}

func RegisterPlugin(p *Plugin) {
	pluginMgr.Register(p)
}

func LoadPlugins() {
	for t := PluginType(0); t != pluginTotal; t++ {
		pluginMgr.Instance(t)
	}
}

func SetPluginConfig(key, val string) {
	pluginConfig[key] = val
}
