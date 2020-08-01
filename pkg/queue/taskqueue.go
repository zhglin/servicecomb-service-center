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

package queue

import (
	"context"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
)

const (
	eventQueueSize = 1000
)

// 事件处理的接口
type Worker interface {
	Handle(ctx context.Context, obj interface{})
}

// 具体的任务
type Task struct {
	Object interface{}
	// Async can let workers handle this task concurrently, but
	// it will make this task unordered
	Async bool //是否异步执行，会导致事件的执行无序
}

// notify task队列 事件的读写
type TaskQueue struct {
	Workers []Worker //事件处理

	taskCh    chan Task //事件队列，缓冲大小是notify.type的queue大小
	goroutine *gopool.Pool
}

// AddWorker is the method to add Worker
// 添加事件处理的worker
func (q *TaskQueue) AddWorker(w Worker) {
	q.Workers = append(q.Workers, w)
}

// Add is the method to add task in queue, one task will be handled by all workers
// 写入队列 notify.type同类型的事件
func (q *TaskQueue) Add(t Task) {
	q.taskCh <- t
}

// 任务的分发 Do函数调用
func (q *TaskQueue) dispatch(ctx context.Context, w Worker, obj interface{}) {
	w.Handle(ctx, obj)
}

// Do is the method to trigger workers handle the task immediately
// 具体触发task执行的函数
func (q *TaskQueue) Do(ctx context.Context, task Task) {
	if task.Async {
		// 异步启动协程，run函数统一新建协程，如果不需要单独调用Do执行，不需要设置Async
		for _, w := range q.Workers {
			q.goroutine.Do(func(ctx context.Context) {
				q.dispatch(ctx, w, task.Object)
			})
		}
		return
	}

	// 同步执行
	for _, w := range q.Workers {
		q.dispatch(ctx, w, task.Object)
	}
}

// Run is the method to start a goroutine to pull and handle tasks from queue
// processor的执行函数，添加processor的时候调用，从taskCh队列中读取事件，对外通知
func (q *TaskQueue) Run() {
	q.goroutine.Do(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-q.taskCh:
				q.Do(ctx, task) // 每个task启动一个协程执行，防止task处理函数阻塞
			}
		}
	})
}

// Stop is the method to stop the workers gracefully
// 终止协程
func (q *TaskQueue) Stop() {
	q.goroutine.Close(true)
}

func NewTaskQueue(size int) *TaskQueue {
	if size <= 0 {
		size = eventQueueSize
	}
	return &TaskQueue{
		taskCh:    make(chan Task, size),
		goroutine: gopool.New(context.Background()),
	}
}
