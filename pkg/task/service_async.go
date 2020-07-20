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
	"github.com/apache/servicecomb-service-center/pkg/util"
	"sync"
	"sync/atomic"
	"time"
)

const (
	initExecutorCount      = 1000
	removeExecutorInterval = 30 * time.Second
	initExecutorTTL        = 4
	executeInterval        = 1 * time.Second
	compactTimes           = 2
)

type executorWithTTL struct {
	*Executor	//执行器
	TTL int64  //执行周期次数 每removeExecutorInterval时间间隔-1
}
// 异步任务的Executor管理器 add一次执行一次
type AsyncTaskService struct {
	//每个任务的executor 以task的key标识
	executors map[string]*executorWithTTL
	goroutine *gopool.Pool
	lock      sync.RWMutex
	ready     chan struct{}
	isClose   bool
}
// 获取 || 创建 任务执行器
func (lat *AsyncTaskService) getOrNewExecutor(task Task) (s *Executor, isNew bool) {
	var (
		ok  bool
		key = task.Key()
	)

	lat.lock.RLock()
	se, ok := lat.executors[key]
	lat.lock.RUnlock()
	if !ok {
		lat.lock.Lock()
		se, ok = lat.executors[key]
		if !ok {
			//创建执行器
			isNew = true
			se = &executorWithTTL{
				Executor: NewExecutor(lat.goroutine, task),
				TTL:      initExecutorTTL,
			}
			lat.executors[key] = se
		}
		lat.lock.Unlock()
	}
	atomic.StoreInt64(&se.TTL, initExecutorTTL)
	return se.Executor, isNew
}

//添加任务
func (lat *AsyncTaskService) Add(ctx context.Context, task Task) error {
	if task == nil || ctx == nil {
		return errors.New("invalid parameters")
	}
	// 获取执行器
	s, isNew := lat.getOrNewExecutor(task)
	// 如果是首次添加就直接执行
	if isNew {
		// do immediately at first time
		return task.Do(ctx)
	}
	// 非首次的添加到executor中 延迟执行
	return s.AddTask(task)
}
// 删除任务
func (lat *AsyncTaskService) removeExecutor(key string) {
	if s, ok := lat.executors[key]; ok {
		s.Close()
		delete(lat.executors, key)
	}
}
// key对应的最后一次执行的task
func (lat *AsyncTaskService) LatestHandled(key string) (Task, error) {
	lat.lock.RLock()
	s, ok := lat.executors[key]
	lat.lock.RUnlock()
	if !ok {
		return nil, errors.New("expired behavior")
	}
	return s.latestTask, nil
}

func (lat *AsyncTaskService) daemon(ctx context.Context) {
	util.SafeCloseChan(lat.ready)
	ticker := time.NewTicker(removeExecutorInterval)
	// lat.executors的最大长度 只加不减
	max := 0
	//执行间隔时间
	timer := time.NewTimer(executeInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Debugf("daemon thread exited for AsyncTaskService stopped")
			return
		case <-timer.C:  //定时执行
			// 加锁读出来所有任务 然后依次执行
			lat.lock.RLock()
			l := len(lat.executors)
			slice := make([]*executorWithTTL, 0, l)
			for _, s := range lat.executors {
				slice = append(slice, s)
			}
			lat.lock.RUnlock()

			for _, s := range slice {
				s.Execute() // non-blocked
			}

			timer.Reset(executeInterval)
		case <-ticker.C:	//定时清理
			util.ResetTimer(timer, executeInterval)

			lat.lock.RLock()
			l := len(lat.executors)
			if l > max {
				max = l
			}

			removes := make([]string, 0, l)
			// 找出来需要清理的executors
			for key, se := range lat.executors {
				if atomic.AddInt64(&se.TTL, -1) == 0 {
					removes = append(removes, key)
				}
			}
			lat.lock.RUnlock()

			if len(removes) == 0 {
				continue
			}

			lat.lock.Lock()
			for _, key := range removes {
				lat.removeExecutor(key)
			}

			l = len(lat.executors)
			// 是否需要缩小lat.executor
			if max > initExecutorCount && max > l*compactTimes {
				lat.renew()
				max = l
			}
			lat.lock.Unlock()

			log.Debugf("daemon thread completed, %d executor(s) removed", len(removes))
		}
	}
}
// 执行taskService
func (lat *AsyncTaskService) Run() {
	lat.lock.Lock()
	if !lat.isClose {
		lat.lock.Unlock()
		return
	}
	lat.isClose = false
	lat.lock.Unlock()
	// 主要执行lat.daemon
	lat.goroutine.Do(lat.daemon)
}
// 关闭taskService  stop之后可以run
func (lat *AsyncTaskService) Stop() {
	lat.lock.Lock()
	if lat.isClose {
		lat.lock.Unlock()
		return
	}
	lat.isClose = true
	//删除所有executors
	for key := range lat.executors {
		lat.removeExecutor(key)
	}

	lat.lock.Unlock()

	lat.goroutine.Close(true)

	util.SafeCloseChan(lat.ready)
}
// 是否准备好
func (lat *AsyncTaskService) Ready() <-chan struct{} {
	return lat.ready
}
// 重置lat.executors  map缩容
func (lat *AsyncTaskService) renew() {
	newExecutor := make(map[string]*executorWithTTL)
	for k, e := range lat.executors {
		newExecutor[k] = e
	}
	lat.executors = newExecutor
}
// 创建taskService
func NewTaskService() TaskService {
	lat := &AsyncTaskService{
		goroutine: gopool.New(context.Background()),
		ready:     make(chan struct{}),
		isClose:   true,
	}
	lat.renew()
	return lat
}
