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

import "github.com/apache/servicecomb-service-center/pkg/util"
//节点的信息
type Node struct {
	// user data
	Cache *Cache				//存储value
	// tree will set the value below after the node added in.
	Name   string				//节点名称
	Tree   *Tree				//顶级树
	Childs *util.ConcurrentMap  //子级node
	Level  int					//层级数
}

func (n *Node) ChildNodes() (nodes []*Node) {
	n.Childs.ForEach(func(item util.MapItem) (next bool) {
		nodes = append(nodes, item.Value.(*Node))
		return true
	})
	return
}

func NewNode() *Node {
	return &Node{
		Cache:  NewCache(),
		Childs: util.NewConcurrentMap(0),
	}
}
