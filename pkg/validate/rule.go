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
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/util"
	"reflect"
	"unicode/utf8"
)

// 字段校验规则
type Rule struct {
	Min    int	// 最小长度
	Max    int  // 最大程度
	Regexp Method // 字符串的匹配规则
	Hide   bool // if true, do not print the value when return invalid result 是否展示字段值
}

func (v *Rule) String() string {
	arr := [4]string{}
	s := arr[:0]
	if v.Min != 0 {
		s = append(s, fmt.Sprintf("Min: %d", v.Min))
	}
	if v.Max != 0 {
		s = append(s, fmt.Sprintf("Max: %d", v.Max))
	}
	if v.Regexp != nil {
		s = append(s, fmt.Sprintf("Regexp: %s", v.Regexp))
	}
	return "{" + util.StringJoin(s, ", ") + "}"
}

// 字段校验
func (v *Rule) Match(s interface{}) (ok bool, invalidValue interface{}) {
	invalidValue = s
	var invalid bool  // 是否校验失败
	sv := reflect.ValueOf(s)
	k := sv.Kind()
	if v.Min > 0 && !invalid {
		switch k {
		case reflect.String:
			invalid = len(sv.String()) < v.Min
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			invalid = sv.Int() < int64(v.Min)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			invalid = sv.Uint() < uint64(v.Min)
		case reflect.Float32, reflect.Float64:
			invalid = sv.Float() < float64(v.Min)
		case reflect.Slice, reflect.Map, reflect.Array:
			invalid = sv.Len() < v.Min
		case reflect.Ptr:
			invalid = sv.IsNil()
		default:
			invalid = false
		}
	}
	if v.Max > 0 && !invalid {
		switch k {
		case reflect.String:
			invalid = utf8.RuneCountInString(sv.String()) > v.Max
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			invalid = sv.Int() > int64(v.Max)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			invalid = sv.Uint() > uint64(v.Max)
		case reflect.Float32, reflect.Float64:
			invalid = sv.Float() > float64(v.Max)
		case reflect.Slice, reflect.Map, reflect.Array:
			invalid = sv.Len() > v.Max
		default:
			invalid = false
		}
	}
	if v.Regexp != nil && !invalid {
		switch k {
		case reflect.Map:
			itemV := Rule{
				Regexp: v.Regexp,
			}
			keys := sv.MapKeys() // map的值
			for _, key := range keys {
				// 递归
				if ok, v := itemV.Match(key.Interface()); !ok {
					invalid = true
					invalidValue = v
					break
				}
				// 校验map的键
				if ok, v := itemV.Match(sv.MapIndex(key).Interface()); !ok {
					invalid = true
					invalidValue = v
					break
				}
			}
		case reflect.Slice, reflect.Array:
			itemV := Rule{
				Regexp: v.Regexp,
			}
			// 校验数组的值
			for i, l := 0, sv.Len(); i < l; i++ {
				if ok, v := itemV.Match(sv.Index(i).Interface()); !ok {
					invalid = true
					invalidValue = v
					break
				}
			}
		default:
			// 只校验字符串
			str, ok := s.(string)
			if ok {
				invalid = !v.Regexp.MatchString(str)
			} else {
				invalid = false
			}
		}
	}
	ok = !invalid
	return
}
