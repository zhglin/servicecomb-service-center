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
package util

import (
	"context"
	"fmt"
	"github.com/apache/servicecomb-service-center/pkg/log"
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	apt "github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	scerr "github.com/apache/servicecomb-service-center/server/scerror"
	"reflect"
	"regexp"
	"strings"
)

type RuleFilter struct {
	DomainProject string
	ProviderRules []*pb.ServiceRule
}

// consumerId 是否符合rule
func (rf *RuleFilter) Filter(ctx context.Context, consumerID string) (bool, error) {
	copyCtx := util.SetContext(util.CloneContext(ctx), util.CtxCacheOnly, "1")
	consumer, err := GetService(copyCtx, rf.DomainProject, consumerID)
	if consumer == nil {
		return false, err
	}

	if len(rf.ProviderRules) == 0 {
		return true, nil
	}

	tags, err := GetTagsUtils(copyCtx, rf.DomainProject, consumerID)
	if err != nil {
		return false, err
	}
	matchErr := MatchRules(rf.ProviderRules, consumer, tags)
	if matchErr != nil {
		if matchErr.Code == scerr.ErrPermissionDeny {
			return false, nil
		}
		return false, matchErr
	}
	return true, nil
}

// 根据黑白名单过滤consumer
func (rf *RuleFilter) FilterAll(ctx context.Context, consumerIDs []string) (allow []string, deny []string, err error) {
	l := len(consumerIDs)
	if l == 0 || len(rf.ProviderRules) == 0 {
		return consumerIDs, nil, nil
	}

	allowIdx, denyIdx := 0, l
	consumers := make([]string, l)
	for _, consumerID := range consumerIDs {
		ok, err := rf.Filter(ctx, consumerID)
		if err != nil {
			return nil, nil, err
		}
		if ok { // 白名单
			consumers[allowIdx] = consumerID
			allowIdx++
		} else { // 黑名单
			denyIdx--
			consumers[denyIdx] = consumerID
		}
	}
	return consumers[:allowIdx], consumers[denyIdx:], nil
}

// 获取指定service的权限设置
func GetRulesUtil(ctx context.Context, domainProject string, serviceID string) ([]*pb.ServiceRule, error) {
	// /cse-sr/ms/rules/{domin/project}/{serviceId}
	key := util.StringJoin([]string{
		apt.GetServiceRuleRootKey(domainProject),
		serviceID,
		"",
	}, "/")

	//
	opts := append(FromContext(ctx), registry.WithStrKey(key), registry.WithPrefix())
	resp, err := backend.Store().Rule().Search(ctx, opts...)
	if err != nil {
		return nil, err
	}

	rules := []*pb.ServiceRule{}
	for _, kv := range resp.Kvs {
		rules = append(rules, kv.Value.(*pb.ServiceRule))
	}
	return rules, nil
}

// rule 是否已存在
func RuleExist(ctx context.Context, domainProject string, serviceID string, attr string, pattern string) bool {
	opts := append(FromContext(ctx),
		registry.WithStrKey(apt.GenerateRuleIndexKey(domainProject, serviceID, attr, pattern)),
		registry.WithCountOnly())
	resp, err := backend.Store().RuleIndex().Search(ctx, opts...)
	if err != nil || resp.Count == 0 {
		return false
	}
	return true
}

// 第一个的ruleType作为service的ruleType
func GetServiceRuleType(ctx context.Context, domainProject string, serviceID string) (string, int, error) {
	key := apt.GenerateServiceRuleKey(domainProject, serviceID, "")
	opts := append(FromContext(ctx),
		registry.WithStrKey(key),
		registry.WithPrefix())
	resp, err := backend.Store().Rule().Search(ctx, opts...)
	if err != nil {
		log.Errorf(err, "get service[%s] rule failed", serviceID)
		return "", 0, err
	}
	if len(resp.Kvs) == 0 {
		return "", 0, nil
	}
	return resp.Kvs[0].Value.(*pb.ServiceRule).RuleType, len(resp.Kvs), nil
}

func GetOneRule(ctx context.Context, domainProject, serviceID, ruleID string) (*pb.ServiceRule, error) {
	opts := append(FromContext(ctx),
		registry.WithStrKey(apt.GenerateServiceRuleKey(domainProject, serviceID, ruleID)))
	resp, err := backend.Store().Rule().Search(ctx, opts...)
	if err != nil {
		log.Errorf(err, "get service rule[%s/%s]", serviceID, ruleID)
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		log.Errorf(nil, "get service rule[%s/%s] failed", serviceID, ruleID)
		return nil, nil
	}
	return resp.Kvs[0].Value.(*pb.ServiceRule), nil
}

// provider能否被依赖 microService中的权限，environment
func AllowAcrossDimension(ctx context.Context, providerService *pb.MicroService, consumerService *pb.MicroService) error {
	// appId不同
	if providerService.AppId != consumerService.AppId {
		if len(providerService.Properties) == 0 {
			return fmt.Errorf("not allow across app access")
		}
		// 必须设置properties的PROP_ALLOW_CROSS_APP属性为true,才允许被依赖
		if allowCrossApp, ok := providerService.Properties[proto.PROP_ALLOW_CROSS_APP]; !ok || strings.ToLower(allowCrossApp) != "true" {
			return fmt.Errorf("not allow across app access")
		}
	}

	// 非共享的
	if !apt.IsShared(proto.MicroServiceToKey(util.ParseTargetDomainProject(ctx), providerService)) &&
		providerService.Environment != consumerService.Environment {
		return fmt.Errorf("not allow across environment access")
	}

	return nil
}

// consumer的tags能否匹配provider的rule
func MatchRules(rulesOfProvider []*pb.ServiceRule, consumer *pb.MicroService, tagsOfConsumer map[string]string) *scerr.Error {
	if consumer == nil {
		return scerr.NewError(scerr.ErrInvalidParams, "consumer is nil")
	}

	if len(rulesOfProvider) <= 0 {
		return nil
	}

	// 白名单 匹配成功一条rule就算成功
	if rulesOfProvider[0].RuleType == "WHITE" {
		return patternWhiteList(rulesOfProvider, tagsOfConsumer, consumer)
	}
	// 黑名单
	return patternBlackList(rulesOfProvider, tagsOfConsumer, consumer)
}

// 匹配白名单,能匹配上说明有权限,一个service有多条rule,有一条匹配成功就算成功
func patternWhiteList(rulesOfProvider []*pb.ServiceRule, tagsOfConsumer map[string]string, consumer *pb.MicroService) *scerr.Error {
	v := reflect.Indirect(reflect.ValueOf(consumer))
	consumerID := consumer.ServiceId
	for _, rule := range rulesOfProvider {
		// 获取rule中用来权限校验的值 没值标识没权限
		value, err := parsePattern(v, rule, tagsOfConsumer, consumerID)
		if err != nil {
			return err
		}
		if len(value) == 0 {
			continue
		}

		// 正则匹配成功
		match, _ := regexp.MatchString(rule.Pattern, value)
		if match {
			log.Infof("consumer[%s][%s/%s/%s/%s] match white list, rule.Pattern is %s, value is %s",
				consumerID, consumer.Environment, consumer.AppId, consumer.ServiceName, consumer.Version,
				rule.Pattern, value)
			return nil
		}
	}
	return scerr.NewError(scerr.ErrPermissionDeny, "Not found in white list")
}

// 获取权限校验的值 哪个值用来做校验
func parsePattern(v reflect.Value, rule *pb.ServiceRule, tagsOfConsumer map[string]string, consumerID string) (string, *scerr.Error) {
	// 校验service的tags
	if strings.HasPrefix(rule.Attribute, "tag_") {
		key := rule.Attribute[4:]
		value := tagsOfConsumer[key]
		if len(value) == 0 {
			log.Infof("can not find service[%s] tag[%s]", consumerID, key)
		}
		return value, nil
	}

	// 校验microService的属性值
	key := v.FieldByName(rule.Attribute)
	if !key.IsValid() {
		log.Errorf(nil, "can not find service[%] field[%s], ruleID is %s",
			consumerID, rule.Attribute, rule.RuleId)
		return "", scerr.NewErrorf(scerr.ErrInternal, "Can not find field '%s'", rule.Attribute)
	}
	return key.String(), nil

}

// 黑名单匹配 同白名单一样 匹配一条就返回
func patternBlackList(rulesOfProvider []*pb.ServiceRule, tagsOfConsumer map[string]string, consumer *pb.MicroService) *scerr.Error {
	v := reflect.Indirect(reflect.ValueOf(consumer))
	consumerID := consumer.ServiceId
	for _, rule := range rulesOfProvider {
		var value string
		value, err := parsePattern(v, rule, tagsOfConsumer, consumerID)
		if err != nil {
			return err
		}
		if len(value) == 0 {
			continue
		}

		match, _ := regexp.MatchString(rule.Pattern, value)
		if match {
			log.Warnf("no permission to access, consumer[%s][%s/%s/%s/%s] match black list, rule.Pattern is %s, value is %s",
				consumerID, consumer.Environment, consumer.AppId, consumer.ServiceName, consumer.Version,
				rule.Pattern, value)
			return scerr.NewError(scerr.ErrPermissionDeny, "Found in black list")
		}
	}
	return nil
}

// 指定的consumerId是否允许依赖providerId
func Accessible(ctx context.Context, consumerID string, providerID string) *scerr.Error {
	if len(consumerID) == 0 {
		return nil
	}

	domainProject := util.ParseDomainProject(ctx)
	targetDomainProject := util.ParseTargetDomainProject(ctx)

	// consumer的Service的信息
	consumerService, err := GetService(ctx, domainProject, consumerID)
	if err != nil {
		return scerr.NewErrorf(scerr.ErrInternal, "An error occurred in query consumer(%s)", err.Error())
	}
	if consumerService == nil {
		return scerr.NewError(scerr.ErrServiceNotExists, "consumer serviceID is invalid")
	}

	// 跨应用权限  provider的service信息
	providerService, err := GetService(ctx, targetDomainProject, providerID)
	if err != nil {
		return scerr.NewErrorf(scerr.ErrInternal, "An error occurred in query provider(%s)", err.Error())
	}
	if providerService == nil {
		return scerr.NewError(scerr.ErrServiceNotExists, "provider serviceID is invalid")
	}

	// 是否允许provider被依赖
	err = AllowAcrossDimension(ctx, providerService, consumerService)
	if err != nil {
		return scerr.NewError(scerr.ErrPermissionDeny, err.Error())
	}

	ctx = util.SetContext(util.CloneContext(ctx), util.CtxCacheOnly, "1")

	// provider的黑白名单
	rules, err := GetRulesUtil(ctx, targetDomainProject, providerID)
	if err != nil {
		return scerr.NewErrorf(scerr.ErrInternal, "An error occurred in query provider rules(%s)", err.Error())
	}

	if len(rules) == 0 {
		return nil
	}

	// consumer的tags
	validateTags, err := GetTagsUtils(ctx, domainProject, consumerService.ServiceId)
	if err != nil {
		return scerr.NewErrorf(scerr.ErrInternal, "An error occurred in query consumer tags(%s)", err.Error())
	}

	return MatchRules(rules, consumerService, validateTags)
}
