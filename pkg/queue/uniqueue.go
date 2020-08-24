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

package queue

import (
	"context"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/log"
)
// 去重队列 不管put多少个只会存在一个未消费的item
type UniQueue struct {
	queue chan interface{}
}

func (uq *UniQueue) Get(ctx context.Context) interface{} {
	select {
	case <-ctx.Done():
		return nil
	case item := <-uq.queue:
		return item
	}
}

func (uq *UniQueue) Chan() <-chan interface{} {
	return uq.queue
}

// 加入一个item
func (uq *UniQueue) Put(value interface{}) (e error) {
	// 已经close的 再put
	defer func() {
		if r := recover(); r != nil {
			log.Panic(r)

			e = fmt.Errorf("%v", r)
		}
	}()
	// 如果chan中已存在 这里会抛弃掉 连续put只保留最后一个
	select {
	case _, ok := <-uq.queue:
		if !ok {
			return fmt.Errorf("channel is closed")
		}
	default:
	}
	// 写入
	select {
	case uq.queue <- value:
	default:
	}
	return
}

// 安全的关闭
func (uq *UniQueue) Close() {
	select {
	case _, ok := <-uq.queue:
		if !ok {
			return
		}
	default:
	}
	close(uq.queue)
}

// queue中只允许存在一个
func NewUniQueue() (uq *UniQueue) {
	return &UniQueue{
		queue: make(chan interface{}, 1),
	}
}
