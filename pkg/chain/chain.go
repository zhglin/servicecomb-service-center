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

package chain

// http调用链  chain中的handler都是依次同步调用
type Chain struct {
	// 标识不同路由子树，不同的子树可以设置不同的chain
	name         string
	// handler
	handlers     []Handler
	// 已执行到第几个handler
	currentIndex int
}

func (c *Chain) Init(chainName string, hs []Handler) {
	c.name = chainName
	c.currentIndex = -1
	c.handlers = hs
}

func (c *Chain) Name() string {
	return c.name
}

func (c *Chain) syncNext(i *Invocation) {
	// 当前chain中的handlers已全部执行完，触发callbackFunc的执行
	if c.currentIndex >= len(c.handlers)-1 {
		i.Success()
		return
	}

	// 执行下一个currentIndex指定的handle
	c.currentIndex++
	c.handlers[c.currentIndex].Handle(i)
}

// 执行chain中的下一个Handler
func (c *Chain) Next(i *Invocation) {
	c.syncNext(i)
}

// 创建不同name的chain
func NewChain(name string, handlers []Handler) (ch Chain) {
	ch.Init(name, handlers)
	return ch
}
