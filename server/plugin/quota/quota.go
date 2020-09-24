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

package quota

import (
	"context"
	"github.com/apache/servicecomb-service-center/pkg/util"
	scerr "github.com/apache/servicecomb-service-center/server/scerror"
	"strconv"
)

const (
	defaultServiceLimit  = 50000
	defaultInstanceLimit = 150000
	defaultSchemaLimit   = 100
	defaultRuleLimit     = 100
	defaultTagLimit      = 100
)

const (
	RuleQuotaType ResourceType = iota
	SchemaQuotaType
	TagQuotaType
	MicroServiceQuotaType
	MicroServiceInstanceQuotaType
)

// 各种资源类型对应的配额数
var (
	DefaultServiceQuota  = util.GetEnvInt("QUOTA_SERVICE", defaultServiceLimit)
	DefaultInstanceQuota = util.GetEnvInt("QUOTA_INSTANCE", defaultInstanceLimit)
	DefaultSchemaQuota   = util.GetEnvInt("QUOTA_SCHEMA", defaultSchemaLimit)
	DefaultTagQuota      = util.GetEnvInt("QUOTA_TAG", defaultTagLimit)
	DefaultRuleQuota     = util.GetEnvInt("QUOTA_RULE", defaultRuleLimit)
)

type ApplyQuotaResult struct {
	Err *scerr.Error

	reporter Reporter
}

func (r *ApplyQuotaResult) ReportUsedQuota(ctx context.Context) error {
	if r == nil || r.reporter == nil {
		return nil
	}
	return r.reporter.ReportUsedQuota(ctx)
}

func (r *ApplyQuotaResult) Close(ctx context.Context) {
	if r == nil || r.reporter == nil {
		return
	}
	r.reporter.Close(ctx)
}

func NewApplyQuotaResult(reporter Reporter, err *scerr.Error) *ApplyQuotaResult {
	return &ApplyQuotaResult{
		reporter: reporter,
		Err:      err,
	}
}

// 资源申请参数
type ApplyQuotaResource struct {
	QuotaType     ResourceType //资源类型
	DomainProject string
	ServiceID     string
	QuotaSize     int64 // 申请数量
}

func NewApplyQuotaResource(quotaType ResourceType, domainProject, serviceID string, quotaSize int64) *ApplyQuotaResource {
	return &ApplyQuotaResource{
		quotaType,
		domainProject,
		serviceID,
		quotaSize,
	}
}

// 资源申请接口
type Manager interface {
	Apply4Quotas(ctx context.Context, res *ApplyQuotaResource) *ApplyQuotaResult // 申请
	RemandQuotas(ctx context.Context, quotaType ResourceType)
}

// 申请到的操作 上报使用
type Reporter interface {
	ReportUsedQuota(ctx context.Context) error
	Close(ctx context.Context)
}

type ResourceType int

func (r ResourceType) String() string {
	switch r {
	case RuleQuotaType:
		return "RULE"
	case SchemaQuotaType:
		return "SCHEMA"
	case TagQuotaType:
		return "TAG"
	case MicroServiceQuotaType:
		return "SERVICE"
	case MicroServiceInstanceQuotaType:
		return "INSTANCE"
	default:
		return "RESOURCE" + strconv.Itoa(int(r))
	}
}
