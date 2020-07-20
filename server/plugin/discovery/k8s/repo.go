// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !nok8s

package k8s

import (
	mgr "github.com/apache/servicecomb-service-center/server/plugin"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery/k8s/adaptor"
)

func init() {
	mgr.RegisterPlugin(mgr.Plugin{PName: mgr.DISCOVERY, Name: "k8s", New: NewRepository})
}

type Repository struct {
}

func (r *Repository) New(t discovery.Type, cfg *discovery.Config) discovery.Adaptor {
	return adaptor.NewK8sAdaptor(t, cfg)
}

func NewRepository() mgr.Instance {
	return &Repository{}
}
