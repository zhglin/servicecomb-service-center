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

package rest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/apache/servicecomb-service-center/pkg/chain"
	errorsEx "github.com/apache/servicecomb-service-center/pkg/errors"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/util"
)

// URLPattern defines an uri pattern
type URLPattern struct {
	Method string
	Path   string
}

// path对应的handler
type urlPatternHandler struct {
	Name         string // Route中Func名称
	Path         string // Route的Path
	http.Handler        // Route的Func
}

// Route is a http route
type Route struct {
	// Method is one of the following: GET,PUT,POST,DELETE
	Method string
	// Path contains a path pattern
	Path string
	// rest callback function for the specified Method and Path
	// func(w http.ResponseWriter, r *http.Request),handlerFunc类型,本身实现了ServeHTTP方法
	Func func(w http.ResponseWriter, r *http.Request)
}

// ROAServantService defines a group of Routes
// 能被注册的service类型
type ROAServantService interface {
	URLPatterns() []Route
}

// ROAServerHandler is a HTTP request multiplexer
// Attention:
//   1. not thread-safe, must be initialized completely before serve http request
//   2. redirect not supported
// 支持内部自定义匹配规则
type ROAServerHandler struct {
	handlers  map[string][]*urlPatternHandler
	chainName string //通过chainName关联chain中的handler
}

// NewROAServerHander news an ROAServerHandler
// http handler
func NewROAServerHander() *ROAServerHandler {
	return &ROAServerHandler{
		handlers: make(map[string][]*urlPatternHandler),
	}
}

// RegisterServant registers a ROAServantService
// servant must be an pointer to service object
// 以struct的形式进行注册
func (roa *ROAServerHandler) RegisterServant(servant interface{}) {
	val := reflect.ValueOf(servant)
	ind := reflect.Indirect(val)
	typ := ind.Type()
	name := util.FileLastName(typ.PkgPath() + "." + typ.Name())
	if val.Kind() != reflect.Ptr {
		log.Errorf(nil, "<rest.RegisterServant> cannot use non-ptr servant struct `%s`", name)
		return
	}

	// 是否存在URLPatterns函数
	urlPatternFunc := val.MethodByName("URLPatterns")
	if !urlPatternFunc.IsValid() {
		log.Errorf(nil, "<rest.RegisterServant> no 'URLPatterns' function in servant struct `%s`", name)
		return
	}

	// 调用URLPatterns函数
	vals := urlPatternFunc.Call([]reflect.Value{})
	if len(vals) <= 0 {
		log.Errorf(nil, "<rest.RegisterServant> call 'URLPatterns' function failed in servant struct `%s`", name)
		return
	}

	// URLPatterns只返回一个参数 所以只取vals[0]
	val0 := vals[0]
	if !val.CanInterface() { // 能否不panic的调用interface()
		log.Errorf(nil, "<rest.RegisterServant> result of 'URLPatterns' function not interface type in servant struct `%s`", name)
		return
	}

	// 转换成Interface类型之后进一步转换类型，返回的类型必须是[]Route类型
	if routes, ok := val0.Interface().([]Route); ok {
		log.Infof("register servant %s", name)
		// 添加到route
		for _, route := range routes {
			err := roa.addRoute(&route)
			if err != nil {
				log.Errorf(err, "register route failed.")
			}
		}
	} else {
		log.Errorf(nil, "<rest.RegisterServant> result of 'URLPatterns' function not []*Route type in servant struct `%s`", name)
	}
}

// 设置名字,关联name对应的chain
func (roa *ROAServerHandler) setChainName(name string) {
	roa.chainName = name
}

//为handler添加route
func (roa *ROAServerHandler) addRoute(route *Route) (err error) {
	method := strings.ToUpper(route.Method)
	// 不是合法的http method  || 不是以/开头  || Func为nil
	if !isValidMethod(method) || !strings.HasPrefix(route.Path, "/") || route.Func == nil {
		message := fmt.Sprintf("Invalid route parameters(method: %s, path: %s)", method, route.Path)
		log.Errorf(nil, message)
		return errors.New(message)
	}

	roa.handlers[method] = append(roa.handlers[method], &urlPatternHandler{
		util.FormatFuncName(util.FuncName(route.Func)), route.Path, http.HandlerFunc(route.Func)})
	log.Infof("register route %s(%s)", route.Path, method)

	return nil
}

// ServeHTTP implements http.Handler
// 匹配到路径之后的回调函数 进行内部的路由匹配
func (roa *ROAServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, ph := range roa.handlers[r.Method] {
		// 能匹配到当前roa
		if params, ok := ph.try(r.URL.Path); ok {
			// 如果有动态参数,重新进行get参数拼接
			if len(params) > 0 {
				r.URL.RawQuery = params + r.URL.RawQuery
			}

			roa.serve(ph, w, r)
			return
		}
	}

	// 对应的method都没匹配到
	allowed := make([]string, 0, len(roa.handlers))
	for method, handlers := range roa.handlers {
		if method == r.Method {
			continue
		}

		// 能不能匹配到别的method的地址
		for _, ph := range handlers {
			if _, ok := ph.try(r.URL.Path); ok {
				allowed = append(allowed, method)
			}
		}
	}

	// path都匹配不上
	if len(allowed) == 0 {
		http.NotFound(w, r)
		return
	}

	// 把能匹配的method写入allow头
	w.Header().Add(HeaderAllow, util.StringJoin(allowed, ", "))
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func (roa *ROAServerHandler) serve(ph *urlPatternHandler, w http.ResponseWriter, r *http.Request) {
	ctx := util.NewStringContext(r.Context())
	// 替换context
	if ctx != r.Context() {
		nr := r.WithContext(ctx)
		*r = *nr
	}

	// 创建invocation，从handlersMap中获取chainName对应的handler，丢给invocation
	inv := chain.NewInvocation(ctx, chain.NewChain(roa.chainName, chain.Handlers(roa.chainName)))
	inv.WithContext(CtxResponse, w).
		WithContext(CtxRequest, r).
		WithContext(CtxMatchPattern, ph.Path).
		WithContext(CtxMatchFunc, ph.Name).
		Invoke(
			func(ret chain.Result) {
				defer func() { // 这里处理的只是ServeHTTP的painc
					err := ret.Err
					itf := recover()
					if itf != nil {
						log.Panic(itf)

						err = errorsEx.RaiseError(itf)
					}
					if _, ok := err.(errorsEx.InternalError); ok {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					if err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
				}()
				if ret.OK { // 只有chain中的handler执行成功才会执行
					ph.ServeHTTP(w, r)
				}
			})
}

// 根据path判断能否匹配上当前的urlPatternHandler
func (roa *urlPatternHandler) try(path string) (p string, _ bool) {
	// j标识roa.path的匹配位置  	l标识roa.path长度
	// i标识path的匹配位置		sl标识path长度
	var i, j int
	l, sl := len(roa.Path), len(path)
	for i < sl {
		switch {
		case j >= l:
			// 长度不同,path>roa.path, roa.Path是否是/结尾的子树
			if roa.Path != "/" && l > 0 && roa.Path[l-1] == '/' {
				return p, true
			}
			return "", false
		case roa.Path[j] == ':': // 处理动态参数
			var val string
			var nextc byte
			o := j
			// ASCII字符表 0代表空字符
			// 匹配出来:到/的 参数名 调整j的值
			_, nextc, j = match(roa.Path, isAlnum, 0, j+1)
			// 匹配参数名对应的val 调整i
			val, _, i = match(path, matchParticial, nextc, i)
			// 构建k=v形式的get参数
			p += url.QueryEscape(roa.Path[o:j]) + "=" + url.QueryEscape(val) + "&"
		case path[i] == roa.Path[j]:
			i++
			j++
		default:
			return "", false
		}
	}
	// roa.path > path 未匹配上
	if j != l {
		return "", false
	}
	return p, true
}

func match(s string, f func(c byte) bool, exclude byte, i int) (matched string, next byte, j int) {
	j = i
	for j < len(s) && f(s[j]) && s[j] != exclude {
		j++
	}

	if j < len(s) {
		next = s[j]
	}
	return s[i:j], next, j
}

func matchParticial(c byte) bool {
	return c != '/'
}

func isAlpha(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// 是否是合法的字符 不包括/
func isAlnum(ch byte) bool {
	return isAlpha(ch) || isDigit(ch)
}
