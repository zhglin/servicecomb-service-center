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

import (
	"context"
	errorsEx "github.com/apache/servicecomb-service-center/pkg/errors"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/util"
)

// Next函数的参数 是否设置callbackFunc
type InvocationOption func(op InvocationOp) InvocationOp

type InvocationOp struct {
	Func  CallbackFunc  // 回调函数
	Async bool          // 是否异步执行
}

// 同步调用
func WithFunc(f func(r Result)) InvocationOption {
	return func(op InvocationOp) InvocationOp { op.Func = f; return op }
}

// 异步调用
func WithAsyncFunc(f func(r Result)) InvocationOption {
	return func(op InvocationOp) InvocationOp { op.Func = f; op.Async = true; return op }
}

// http请求的调用链，chain在http.Handler之前执行
// http.Handler在callback中的第一个执行，后面跟着chain中handle函数动态添加的callback
type Invocation struct {
	Callback
	context *util.StringContext
	chain   Chain
}

func (i *Invocation) Init(ctx context.Context, ch Chain) {
	i.context = util.NewStringContext(ctx)
	i.chain = ch
}

func (i *Invocation) Context() context.Context {
	return i.context
}

// 设置链路需要的数据
func (i *Invocation) WithContext(key string, val interface{}) *Invocation {
	i.context.SetKV(key, val)
	return i
}

// Next is the method to go next step in handler chain
// WithFunc and WithAsyncFunc options can add customize callbacks in chain
// and the callbacks seq like below
// i.Success/Fail() -> CB1 ---> CB3 ----------> END           goroutine 0
//                          \-> CB2(async) \                  goroutine 1
//                                          \-> CB4(async)    goroutine 1 or 2
// 执行chain中的下一个handle，支持添加回调同步或者异步调用，
// chain中的handle中进行主动调用，如果handle执行异常，不调用此方法就终止后续handle
func (i *Invocation) Next(opts ...InvocationOption) {
	var op InvocationOp
	for _, opt := range opts {
		op = opt(op)
	}
	// 设置callBack
	i.setCallback(op.Func, op.Async)
	i.chain.Next(i)
}

// 设置callbackFunc
func (i *Invocation) setCallback(f CallbackFunc, async bool) {
	if f == nil {
		return
	}

	if i.Func == nil {
		i.Func = f
		i.Async = async
		return
	}
	// 链接已有的i.Func，http.Handler在第一个，f依次跟在后面
	cb := i.Func
	// 这里只是构建i.Func并没有执行
	i.Func = func(r Result) {
		cb(r)
		callback(f, async, r)
	}
}

// 执行callback
func callback(f CallbackFunc, async bool, r Result) {
	c := Callback{Func: f, Async: async}
	c.Invoke(r)
}

// 匹配到路由后的http.Handler触发,设置http.Handler放到第一个callbackFunc中,执行第一个chain
func (i *Invocation) Invoke(f CallbackFunc) {
	defer func() { // 处理所有chain中handler的panic
		itf := recover()
		if itf == nil {
			return
		}
		log.Panic(itf)
		// Invoke会依次调用chain中的handle，如果handle有panic异常，把错误信息传递给callbackFunc
		i.Fail(errorsEx.RaiseError(itf))
	}()
	i.Func = f
	i.chain.Next(i)
}

// 关联并管理chain的调用 invocation是在http.Handler的方法中创建出来的
func NewInvocation(ctx context.Context, ch Chain) (inv Invocation) {
	inv.Init(ctx, ch)
	return inv
}
