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

import "time"

//cache的配置
type Config struct {
	ttl time.Duration //过期时间
	max int64         //最大数目
}

func (c *Config) TTL() time.Duration {
	return c.ttl
}

func (c *Config) WithTTL(ttl time.Duration) *Config {
	c.ttl = ttl
	return c
}

func (c *Config) MaxSize() int64 {
	return c.max
}

func (c *Config) WithMaxSize(s int64) *Config {
	c.max = s
	return c
}

func Configure() *Config {
	return &Config{
		ttl: 5 * time.Minute,
		max: 5000,
	}
}
