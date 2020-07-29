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
package etcd

import (
	"context"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
)

type innerListWatch struct {
	// registry实例
	Client registry.Registry
	// etcd key前缀
	Prefix string
	// 最后的版本号 etcd全局数据的版本。当数据发生变更，包括创建、修改、删除，revision 对应的都会 +1
	rev int64
}

// 获取全量数据
func (lw *innerListWatch) List(op ListWatchConfig) (*registry.PluginResponse, error) {
	otCtx, cancel := context.WithTimeout(op.Context, op.Timeout)
	defer cancel()
	resp, err := lw.Client.Do(otCtx, registry.WatchPrefixOpOptions(lw.Prefix)...)
	if err != nil {
		log.Errorf(err, "list prefix %s failed, current rev: %d", lw.Prefix, lw.Revision())
		return nil, err
	}
	// 设置版本号
	lw.setRevision(resp.Revision)
	return resp, nil
}

func (lw *innerListWatch) Revision() int64 {
	return lw.rev
}

func (lw *innerListWatch) setRevision(rev int64) {
	lw.rev = rev
}

// 每次都创建新的InnerWatcher
func (lw *innerListWatch) Watch(op ListWatchConfig) Watcher {
	return newInnerWatcher(lw, op)
}

// watch操作
func (lw *innerListWatch) DoWatch(ctx context.Context, f func(*registry.PluginResponse)) error {
	rev := lw.Revision()
	opts := append(
		registry.WatchPrefixOpOptions(lw.Prefix),
		registry.WithRev(rev+1),
		registry.WithWatchCallback(
			func(message string, resp *registry.PluginResponse) error {
				if resp == nil || len(resp.Kvs) == 0 {
					return fmt.Errorf("unknown event %s, watch prefix %s", resp, lw.Prefix)
				}

				lw.setRevision(resp.Revision)

				f(resp)
				return nil
			}))

	err := lw.Client.Watch(ctx, opts...)
	if err != nil { // compact可能会导致watch失败 or message body size lager than 4MB
		log.Errorf(err, "watch prefix %s failed, start rev: %d+1->%d->0", lw.Prefix, rev, lw.Revision())
		// 异常设置vision为0，让上层全量拉取
		lw.setRevision(0)
		// watch异常写入nil标记
		f(nil)
	}
	return err
}
