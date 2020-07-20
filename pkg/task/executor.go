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

package task

import (
	"context"
	"errors"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/queue"
)

type Executor struct {
	pool       *gopool.Pool
	tasks      *queue.UniQueue  //队列
	latestTask Task //最后一次执行的task实例
}

// 添加task
func (s *Executor) AddTask(task Task) (err error) {
	if task == nil {
		return errors.New("invalid parameters")
	}
	// 已经close了的会报错
	err = s.tasks.Put(task)
	if err != nil {
		return
	}
	// 如果最后一次的执行有错 返回错误
	return s.latestTask.Err()
}

func (s *Executor) Execute() {
	select {
	case task, ok := <-s.tasks.Chan():
		// 已经关闭了
		if !ok {
			return
		}
		// 执行
		s.pool.Do(func(ctx context.Context) {
			at := task.(Task)
			err := at.Do(ctx)
			if err != nil {
				log.Error("", err)
			}
			s.latestTask = at
		})
	default:
	}
}
// 关闭executor
func (s *Executor) Close() {
	s.tasks.Close()
}

func NewExecutor(pool *gopool.Pool, task Task) *Executor {
	return &Executor{
		pool:       pool,
		tasks:      queue.NewUniQueue(),
		latestTask: task, //最后一次执行的task，里面保留着执行结果
	}
}
