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

package cache

type Option struct {
	//需要处理的层级
	Level int
	//是否cache
	NoCache bool
}

// get node before level l
func (op Option) ToLevel(l int) Option {
	op.Level = l
	return op
}

// get node without cache
func (op Option) Temporary(b bool) Option {
	op.NoCache = b
	return op
}

func Options() (op Option) {
	return
}
