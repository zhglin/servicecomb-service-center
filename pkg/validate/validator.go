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

package validate

import (
	"errors"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"reflect"
	"sync"
)

type Validator struct {
	rules map[string]*Rule      // 字段对应的rule规则
	subs  map[string]*Validator // 子结构对应的validator
	once  sync.Once
}

func (v *Validator) Init(f func(*Validator)) *Validator {
	v.once.Do(func() {
		f(v)
	})
	return v
}

func (v *Validator) GetRule(name string) *Rule {
	if v.rules == nil {
		return nil
	}
	return v.rules[name]
}

// 添加校验规则 字段名
func (v *Validator) AddRule(name string, rule *Rule) {
	if v.rules == nil {
		v.rules = make(map[string](*Rule))
	}
	v.rules[name] = rule
}

func (v *Validator) GetRules() map[string](*Rule) {
	return v.rules
}

func (v *Validator) AddRules(in map[string](*Rule)) {
	if len(in) == 0 {
		return
	}
	for key, value := range in {
		v.AddRule(key, value)
	}
}

func (v *Validator) GetSub(name string) *Validator {
	if v.subs == nil {
		return nil
	}
	return v.subs[name]
}

// 添加子结构校验规则
func (v *Validator) AddSub(name string, s *Validator) {
	if v.subs == nil {
		v.subs = make(map[string](*Validator))
	}
	v.subs[name] = s
}

func (v *Validator) GetSubs() map[string](*Validator) {
	return v.subs
}

// 批量添加子结构
func (v *Validator) AddSubs(in map[string](*Validator)) {
	if len(in) == 0 {
		return
	}
	for key, value := range in {
		v.AddSub(key, value)
	}
}

// 校验
func (v *Validator) Validate(s interface{}) error {
	sv := reflect.ValueOf(s)
	k := sv.Kind()
	switch k {
	case reflect.Ptr: // 解引用
		if !sv.IsNil() {
			return v.Validate(sv.Elem().Interface())
		}
		return errors.New("invalid nil pointer")
	case reflect.Struct:
	default: // 非struct类型
		return fmt.Errorf("not support validate type '%s'", k)
	}

	st := util.Reflect(s)
	for i, l := 0, sv.NumField(); i < l; i++ {
		field := sv.Field(i)
		fieldName := st.Fields[i].Name
		rule, ok := v.rules[fieldName] // 字段的规则
		subV, sub := v.subs[fieldName] // 字段的子结构

		// 字段是指针 解引用
		fi := field.Interface()
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			fi = field.Elem().Interface()
			field = reflect.ValueOf(fi)
		}

		// 存在对应的字段校验规则
		if ok {
			// check current rule
			// if pointer, check it's a nil pointer or not
			// if array, slice and map, check the length and regex
			// if sub type is not a string when do regex check, return OK
			ok, invalidValue := rule.Match(fi)
			if !ok {
				if rule.Hide {
					return fmt.Errorf("field '%s.%s' does not match rule: %s", st.Type.Name(), fieldName, rule)
				}
				return fmt.Errorf("field '%s.%s' invalid value '%v' does not match rule: %s", st.Type.Name(), fieldName, invalidValue, rule)
			}
		}

		// 存在子结构
		if sub {
			// check sub rule
			// do not support sub type is not pointer or struct
			switch field.Kind() {
			case reflect.Struct:
				if !sub {
					continue
				}
				if err := subV.Validate(fi); err != nil {
					return err
				}
			case reflect.Array, reflect.Slice:
				if !sub {
					break
				}
				for i, l := 0, field.Len(); i < l; i++ {
					if err := subV.Validate(field.Index(i).Interface()); err != nil {
						return err
					}
				}
			case reflect.Map:
				if !sub {
					break
				}
				keys := field.MapKeys()
				for _, key := range keys {
					// TODO how to validate non-base type key
					if err := subV.Validate(field.MapIndex(key).Interface()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func NewValidator() *Validator {
	return &Validator{}
}
