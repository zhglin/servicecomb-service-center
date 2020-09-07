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

package gopool

import (
	"context"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"sync"
	"time"
)

// 协程池
var GlobalConfig = Configure()

// 全局的协程池
var defaultGo *Pool

func init() {
	defaultGo = New(context.Background())
}

// 协程池配置
type Config struct {
	Concurrent  int           //最大协程数
	IdleTimeout time.Duration //每个协程的最大空闲时间
}

func (c *Config) Workers(max int) *Config {
	c.Concurrent = max
	return c
}

func (c *Config) Idle(time time.Duration) *Config {
	c.IdleTimeout = time
	return c
}

func Configure() *Config {
	return &Config{
		Concurrent:  1000,
		IdleTimeout: 60 * time.Second,
	}
}

type Pool struct {
	// 配置
	Cfg *Config

	// job context
	ctx    context.Context
	cancel context.CancelFunc
	// pending is the chan to block Pool.Do() when go pool is full
	// workers满了 写入pending
	pending chan func(ctx context.Context)
	// workers is the counter of the worker
	// 最大config.Concurrent 如果满了就写入pending
	workers chan struct{}
	// 用来更新closed
	mux    sync.RWMutex
	wg     sync.WaitGroup
	closed bool
}

// 执行f
func (g *Pool) execute(f func(ctx context.Context)) {
	defer log.Recover()
	// g.ctx控制协程退出
	f(g.ctx)
}

// Do pick one idle goroutine to do the f once
// 添加并执行
func (g *Pool) Do(f func(context.Context)) *Pool {
	// 没有判断是否已close 已经close的会panic
	// 判断close需要加锁
	defer log.Recover()
	select {
	// 没有空闲workers 写入pending
	case g.pending <- f: // block if workers are busy
	// workers有空闲
	case g.workers <- struct{}{}:
		g.wg.Add(1)
		// 创建个协程
		go g.loop(f)
	}
	return g
}

// 循环执行f
func (g *Pool) loop(f func(context.Context)) {
	defer g.wg.Done()
	// 减少workers数量
	defer func() { <-g.workers }()
	// 协程退出时间
	timer := time.NewTimer(g.Cfg.IdleTimeout)
	defer timer.Stop()
	for {
		// 执行f
		g.execute(f)

		select {
		// 到时退出
		case <-timer.C:
			return
		// 从pending读取
		case f = <-g.pending:
			if f == nil {
				return
			}
			// 取出f 重置timer
			util.ResetTimer(timer, g.Cfg.IdleTimeout)
		}
	}
}

// Close will call context.Cancel(), so all goroutines maybe exit when job does not complete
// 关闭 会强制取消携程池中的协程
// 有些协程内部是一直for循环 协程内部要处理content.Done() close才有效果
func (g *Pool) Close(grace bool) {
	g.mux.Lock()
	if g.closed {
		g.mux.Unlock()
		return
	}
	g.closed = true
	g.mux.Unlock()

	close(g.pending)
	close(g.workers)
	// 强制取消所有协程
	g.cancel()
	// 是否阻塞到所有协程退出
	if grace {
		g.wg.Wait()
	}
}

// Done will wait for all goroutines complete the jobs and then close the pool
// 关闭并阻塞到所有协程退出
// 协程内部一直for循环的 不能用 会一直阻塞
func (g *Pool) Done() {
	g.mux.Lock()
	if g.closed {
		g.mux.Unlock()
		return
	}
	g.closed = true
	g.mux.Unlock()

	close(g.pending)
	close(g.workers)
	// 阻塞到所有协程退出
	g.wg.Wait()
}

// 创建协程池
func New(ctx context.Context, cfgs ...*Config) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	if len(cfgs) == 0 {
		cfgs = append(cfgs, GlobalConfig)
	}
	cfg := cfgs[0]
	gr := &Pool{
		Cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
		pending: make(chan func(context.Context)),
		workers: make(chan struct{}, cfg.Concurrent),
	}
	return gr
}

//添加到全局的goPool
func Go(f func(context.Context)) {
	defaultGo.Do(f)
}

// 关闭全局的goPool
func CloseAndWait() {
	defaultGo.Close(true)
	log.Debugf("all goroutines exited")
}
