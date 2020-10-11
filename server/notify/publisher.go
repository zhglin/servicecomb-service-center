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

package notify

import (
	"context"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"sync"
	"time"
)

var publisher *Publisher

// 初始化并执行publisher
func init() {
	publisher = NewPublisher()
	publisher.Run()
}

type Publisher struct {
	wss       []*WebSocket  // 管理调度所有的webSocket
	lock      sync.Mutex
	goroutine *gopool.Pool
}

// 执行
func (wh *Publisher) Run() {
	gopool.Go(publisher.loop)
}

// 关闭
func (wh *Publisher) Stop() {
	wh.goroutine.Close(true)
}

// 协程执行
func (wh *Publisher) dispatch(ws *WebSocket, payload interface{}) {
	wh.goroutine.Do(func(ctx context.Context) {
		ws.HandleWatchWebSocketJob(payload)
	})
}

// 定时处理 webSocket
func (wh *Publisher) loop(ctx context.Context) {
	defer wh.Stop() // 退出关闭publisher
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			// server shutdown
			return
		case <-ticker.C: // 500毫秒
			var removes []int
			for i, ws := range wh.wss {
				if payload := ws.Pick(); payload != nil {
					if _, ok := payload.(error); ok { // 异常的 已关闭的会因为conn写入失败设置error
						removes = append(removes, i)
					}
					wh.dispatch(ws, payload) // 异步处理
				}
			}
			if len(removes) == 0 {
				continue
			}

			// 删除异常的webSocket
			wh.lock.Lock()
			var (
				news []*WebSocket
				s    int
			)
			for _, e := range removes {
				news = append(news, wh.wss[s:e]...)
				s = e + 1
			}
			if s < len(wh.wss) {
				news = append(news, wh.wss[s:]...)
			}
			wh.wss = news
			wh.lock.Unlock()
		}
	}
}

// 添加webSocket
func (wh *Publisher) Accept(ws *WebSocket) {
	wh.lock.Lock()
	wh.wss = append(wh.wss, ws)
	wh.lock.Unlock()
}

func NewPublisher() *Publisher {
	return &Publisher{
		goroutine: gopool.New(context.Background()),
	}
}
