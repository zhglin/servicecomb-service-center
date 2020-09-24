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
package event

import (
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
)

func init() {
	discovery.AddEventHandler(NewDomainEventHandler())         // domain类型 监控
	discovery.AddEventHandler(NewServiceEventHandler())        // service类型,监控,创建domain,project,删除本地cache
	discovery.AddEventHandler(NewInstanceEventHandler())       // instance类型 通知
	discovery.AddEventHandler(NewRuleEventHandler())           // rule类型 通知
	discovery.AddEventHandler(NewTagEventHandler())            // tag类型 通知
	discovery.AddEventHandler(NewDependencyEventHandler())     // dependency 生成dependencyRule
	discovery.AddEventHandler(NewDependencyRuleEventHandler()) // dependencyRule  删除cache
	discovery.AddEventHandler(NewSchemaSummaryEventHandler())  // summary类型 监控
}
