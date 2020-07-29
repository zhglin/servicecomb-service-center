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
	"github.com/apache/servicecomb-service-center/pkg/util"
	"strings"
	"sync"
)

// KvCache implements Cache.
// KvCache is dedicated to stores service discovery data,
// e.g. service, instance, lease.
// cache存储器 提供cache的读写
type KvCache struct {
	Cfg   *Config
	// discovery.Type 对应的名字
	name  string
	// 存储器 两层 为了模拟etcd的目录层次结构 实际使用中是用不到的，存储到etcd中的key都是固定的
	// 一级map是prefix  二级全路径
	store map[string]map[string]*KeyValue
	rwMux sync.RWMutex
	// 是否已经脏了
	dirty bool
}

func (c *KvCache) Name() string {
	return c.name
}

// 缓存长度
func (c *KvCache) Size() (l int) {
	c.rwMux.RLock()
	l = int(util.Sizeof(c.store))
	c.rwMux.RUnlock()
	return
}

// 获取
func (c *KvCache) Get(key string) (v *KeyValue) {
	c.rwMux.RLock()
	prefix := c.prefix(key)
	if p, ok := c.store[prefix]; ok {
		v = p[key]
	}
	c.rwMux.RUnlock()
	return
}

// 获取当前config的所有数量以及keyValue
func (c *KvCache) GetAll(arr *[]*KeyValue) (count int) {
	c.rwMux.RLock()
	count = c.getPrefixKey(arr, c.Cfg.Key)
	c.rwMux.RUnlock()
	return
}

// 获取指定前缀的KeyValue 以及 数量
func (c *KvCache) GetPrefix(prefix string, arr *[]*KeyValue) (count int) {
	c.rwMux.RLock()
	count = c.getPrefixKey(arr, prefix)
	c.rwMux.RUnlock()
	return
}

// 设置keyValue
func (c *KvCache) Put(key string, v *KeyValue) {
	c.rwMux.Lock()
	c.addPrefixKey(key, v)
	c.rwMux.Unlock()
}

// 删除key
func (c *KvCache) Remove(key string) {
	c.rwMux.Lock()
	c.deletePrefixKey(key)
	c.rwMux.Unlock()
}

// 把cache设置成脏的
func (c *KvCache) MarkDirty() {
	c.dirty = true
}

// 返回cache是否已经脏了
func (c *KvCache) Dirty() bool { return c.dirty }

// 清理cache
func (c *KvCache) Clear() {
	c.rwMux.Lock()
	c.dirty = false
	// 重置store
	c.store = make(map[string]map[string]*KeyValue)
	c.rwMux.Unlock()
}

// 根据iter 过滤
func (c *KvCache) ForEach(iter func(k string, v *KeyValue) (next bool)) {
	c.rwMux.RLock()
loopParent:
	for _, p := range c.store {
		for k, v := range p {
			if v == nil {
				continue loopParent  // 跳转到loopParent重新执行 p剩下的数据不在处理
			}
			if !iter(k, v) {
				break loopParent	// 跳转到loopParent 不在执行下面的for循环
			}
		}
	}
	c.rwMux.RUnlock()
}

// 获取key前缀 key中最后一个/之前的字符串作为前缀 这也是存储在etcd中数据的key的规则
func (c *KvCache) prefix(key string) string {
	if len(key) == 0 {
		return ""
	}
	return key[:strings.LastIndex(key[:len(key)-1], "/")+1]
}

// 获取全部prefix前缀keyValue
func (c *KvCache) getPrefixKey(arr *[]*KeyValue, prefix string) (count int) {
	keysRef, ok := c.store[prefix]
	if !ok {
		return 0
	}

	// TODO support sort option
	// 只获取总数
	if arr == nil {
		for key := range keysRef {
			// 因为是目录层次的 所以是递归获取
			if n := c.getPrefixKey(nil, key); n > 0 {
				count += n
				continue
			}
			count++
		}
		return
	}
	// 获取keyValue以及count
	for key, val := range keysRef {
		if n := c.getPrefixKey(arr, key); n > 0 {
			count += n
			continue
		}
		*arr = append(*arr, val)
		count++
	}
	return
}

// 添加 覆盖旧值keyValue
func (c *KvCache) addPrefixKey(key string, val *KeyValue) {
	//c.cfg.key只是etcd中的前缀 长度一定要小于key
	if len(c.Cfg.Key) >= len(key) {
		return
	}
	prefix := c.prefix(key)
	if len(prefix) == 0 {
		return
	}
	keys, ok := c.store[prefix]
	if !ok {
		// build parent index key and new child nodes
		keys = make(map[string]*KeyValue)
		c.store[prefix] = keys
	} else if _, ok := keys[key]; ok { // 已存在
		if val != nil {
			// override the value 覆盖旧值
			keys[key] = val
		}
		return
	}

	// 不存在直接设置
	keys[key], key = val, prefix
	// 只是创建上层map
	c.addPrefixKey(key, nil)
}

// 删除key
func (c *KvCache) deletePrefixKey(key string) {
	prefix := c.prefix(key)
	m, ok := c.store[prefix]
	if !ok {
		return
	}
	delete(m, key)

	// remove parent which has no child
	// 如果第二层map没有元素了 就删掉第一层的map 并递归删上一级
	if len(m) == 0 {
		delete(c.store, prefix)
		c.deletePrefixKey(prefix)
	}
}

// 创建kvCache  name是对应的discovery.Type
func NewKvCache(name string, cfg *Config) *KvCache {
	return &KvCache{
		Cfg:   cfg,
		name:  name,
		store: make(map[string]map[string]*KeyValue),
	}
}
