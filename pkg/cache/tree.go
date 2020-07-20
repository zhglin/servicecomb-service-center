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
package cache

import (
	"context"
	"errors"
	"github.com/karlseguin/ccache"
	"sync"
)

var errNilNode = errors.New("nil node")

//层次级cache
type Tree struct {
	Config  *Config  		//cache配置
	roots   *ccache.Cache   //cache根节点
	filters []Filter		//层级关系
	lock    sync.RWMutex
}

//添加层级关系
func (t *Tree) AddFilter(fs ...Filter) *Tree {
	t.lock.Lock()
	for _, f := range fs {
		t.filters = append(t.filters, f)
	}
	t.lock.Unlock()
	return t
}

//获取cache
func (t *Tree) Get(ctx context.Context, ops ...Option) (node *Node, err error) {
	var op Option
	if len(ops) > 0 {
		op = ops[0]
	}

	var (
		parent *Node
		i      int
	)

	//是否进行缓存  默认缓存 写到tree.cache中 否则就是一次性的 每次Get重新生成
	if !op.NoCache {
		if parent, err = t.getOrCreateRoot(ctx); parent == nil {
			return
		}
		i++
		// parent may be a temp root in concurrent scene
	}

	for ; i < len(t.filters); i++ {
		if op.Level > 0 && op.Level == i {
			break
		}
		if parent, err = t.getOrCreateNode(ctx, i, parent); parent == nil {
			break
		}
	}
	node = parent
	return
}

//删除顶级cache
func (t *Tree) Remove(ctx context.Context) {
	if len(t.filters) == 0 {
		return
	}

	t.roots.Delete(t.filters[0].Name(ctx, nil))
}

//第一级加入到cache中
func (t *Tree) getOrCreateRoot(ctx context.Context) (node *Node, err error) {
	if len(t.filters) == 0 {
		return
	}

	filter := t.filters[0]
	name := filter.Name(ctx, nil)
	//查找 || 创建cache（有过期时间）
	item, err := t.roots.Fetch(name, t.Config.TTL(), func() (interface{}, error) {
		node, err := t.getOrCreateNode(ctx, 0, nil)
		if err != nil {
			return nil, err
		}
		if node == nil {
			return nil, errNilNode
		}
		return node, nil
	})
	switch err {
	case nil:
		node = item.Value().(*Node)
	case errNilNode:
		err = nil
	}
	return
}

//创建node节点
func (t *Tree) getOrCreateNode(ctx context.Context, idx int, parent *Node) (node *Node, err error) {
	filter := t.filters[idx]
	//当前节点的名称
	name := t.nodeFullName(filter.Name(ctx, parent), parent)
	//如果是顶级节点 直接创建返回
	if parent == nil {
		// new a temp node
		return t.createNode(ctx, idx, name, parent)
	}

	//子级节点  在父节点下查找子节点  找不到就创建
	item, err := parent.Childs.Fetch(name, func() (interface{}, error) {
		node, err := t.createNode(ctx, idx, name, parent)
		if err != nil {
			return nil, err
		}
		if node == nil {
			return nil, errNilNode
		}
		return node, nil
	})
	switch err {
	case nil:
		node = item.(*Node)
	case errNilNode:
		err = nil
	}
	return
}

//缓存key 父节点name+当前节点name
func (t *Tree) nodeFullName(name string, parent *Node) string {
	if parent != nil {
		name = parent.Name + "." + name
	}
	return name
}

//创建node
func (t *Tree) createNode(ctx context.Context, idx int, name string, parent *Node) (node *Node, err error) {
	node, err = t.filters[idx].Init(ctx, parent)
	if node == nil {
		return
	}
	node.Name = name
	node.Tree = t
	node.Level = idx
	return
}

func NewTree(cfg *Config) *Tree {
	return &Tree{
		Config: cfg,
		roots:  ccache.New(ccache.Configure().MaxSize(cfg.MaxSize())),
	}
}
