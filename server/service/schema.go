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

package service

import (
	"fmt"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"strings"

	"github.com/apache/servicecomb-service-center/pkg/log"
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	"github.com/apache/servicecomb-service-center/pkg/util"
	apt "github.com/apache/servicecomb-service-center/server/core"
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/plugin"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
	"github.com/apache/servicecomb-service-center/server/plugin/quota"
	"github.com/apache/servicecomb-service-center/server/plugin/registry"
	scerr "github.com/apache/servicecomb-service-center/server/scerror"
	serviceUtil "github.com/apache/servicecomb-service-center/server/service/util"

	"context"
)

// 获取指定schemaId
func (s *MicroServiceService) GetSchemaInfo(ctx context.Context, in *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	err := Validate(in)
	if err != nil {
		log.Errorf(nil, "get schema[%s/%s] failed", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrInvalidParams, err.Error()),
		}, nil
	}

	domainProject := util.ParseDomainProject(ctx)

	// serviceId校验
	if !serviceUtil.ServiceExist(ctx, domainProject, in.ServiceId) {
		log.Errorf(nil, "get schema[%s/%s] failed, service does not exist", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrServiceNotExists, "Service does not exist."),
		}, nil
	}

	// schemeId不存在
	key := apt.GenerateServiceSchemaKey(domainProject, in.ServiceId, in.SchemaId)
	opts := append(serviceUtil.FromContext(ctx), registry.WithStrKey(key))
	resp, errDo := backend.Store().Schema().Search(ctx, opts...)
	if errDo != nil {
		log.Errorf(errDo, "get schema[%s/%s] failed", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrUnavailableBackend, errDo.Error()),
		}, errDo
	}
	if resp.Count == 0 {
		log.Errorf(errDo, "get schema[%s/%s] failed, schema does not exists", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrSchemaNotExists, "Do not have this schema info."),
		}, nil
	}

	// summary
	schemaSummary, err := getSchemaSummary(ctx, domainProject, in.ServiceId, in.SchemaId)
	if err != nil {
		log.Errorf(err, "get schema[%s/%s] failed, get schema summary failed", in.ServiceId, in.SchemaId)
		return &pb.GetSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrInternal, err.Error()),
		}, err
	}

	return &pb.GetSchemaResponse{
		Response:      proto.CreateResponse(proto.Response_SUCCESS, "Get schema info successfully."),
		Schema:        util.BytesToStringWithNoCopy(resp.Kvs[0].Value.([]byte)),
		SchemaSummary: schemaSummary,
	}, nil
}

// 获取所有的schema
func (s *MicroServiceService) GetAllSchemaInfo(ctx context.Context, in *pb.GetAllSchemaRequest) (*pb.GetAllSchemaResponse, error) {
	err := Validate(in)
	if err != nil {
		log.Errorf(nil, "get service[%s] all schemas failed", in.ServiceId)
		return &pb.GetAllSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrInvalidParams, err.Error()),
		}, nil
	}

	domainProject := util.ParseDomainProject(ctx)

	// serviceId校验
	service, err := serviceUtil.GetService(ctx, domainProject, in.ServiceId)
	if err != nil {
		log.Errorf(err, "get service[%s] all schemas failed, get service failed", in.ServiceId)
		return &pb.GetAllSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrInternal, err.Error()),
		}, err
	}
	if service == nil {
		log.Errorf(nil, "get service[%s] all schemas failed, service does not exist", in.ServiceId)
		return &pb.GetAllSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrServiceNotExists, "Service does not exist."),
		}, nil
	}

	// 校验service.Schemas
	schemasList := service.Schemas
	if len(schemasList) == 0 {
		return &pb.GetAllSchemaResponse{
			Response: proto.CreateResponse(proto.Response_SUCCESS, "Do not have this schema info."),
			Schemas:  []*pb.Schema{},
		}, nil
	}

	// 获取所有的summary
	key := apt.GenerateServiceSchemaSummaryKey(domainProject, in.ServiceId, "")
	opts := append(serviceUtil.FromContext(ctx), registry.WithStrKey(key), registry.WithPrefix())
	resp, errDo := backend.Store().SchemaSummary().Search(ctx, opts...)
	if errDo != nil {
		log.Errorf(errDo, "get service[%s] all schema summaries failed", in.ServiceId)
		return &pb.GetAllSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrUnavailableBackend, errDo.Error()),
		}, errDo
	}

	respWithSchema := &discovery.Response{}
	if in.WithSchema { // 是否获取schema
		key := apt.GenerateServiceSchemaKey(domainProject, in.ServiceId, "")
		opts := append(serviceUtil.FromContext(ctx), registry.WithStrKey(key), registry.WithPrefix())
		respWithSchema, errDo = backend.Store().Schema().Search(ctx, opts...)
		if errDo != nil {
			log.Errorf(errDo, "get service[%s] all schemas failed", in.ServiceId)
			return &pb.GetAllSchemaResponse{
				Response: proto.CreateResponse(scerr.ErrUnavailableBackend, errDo.Error()),
			}, errDo
		}
	}

	schemas := make([]*pb.Schema, 0, len(schemasList))
	// 根据service.Schema拼数据
	for _, schemaID := range schemasList {
		tempSchema := &pb.Schema{}
		tempSchema.SchemaId = schemaID
		for _, summarySchema := range resp.Kvs {
			// summary对应的schemaId
			_, _, schemaIDOfSummary := apt.GetInfoFromSchemaSummaryKV(summarySchema.Key)
			if schemaID == schemaIDOfSummary {
				tempSchema.Summary = summarySchema.Value.(string)
			}
		}

		for _, contentSchema := range respWithSchema.Kvs {
			// schema对应的schemaId
			_, _, schemaIDOfSchema := apt.GetInfoFromSchemaKV(contentSchema.Key)
			if schemaID == schemaIDOfSchema {
				tempSchema.Schema = util.BytesToStringWithNoCopy(contentSchema.Value.([]byte))
			}
		}
		schemas = append(schemas, tempSchema)
	}

	return &pb.GetAllSchemaResponse{
		Response: proto.CreateResponse(proto.Response_SUCCESS, "Get all schema info successfully."),
		Schemas:  schemas,
	}, nil

}

// 删除指定schemaId
func (s *MicroServiceService) DeleteSchema(ctx context.Context, in *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error) {
	remoteIP := util.GetIPFromContext(ctx)
	err := Validate(in)
	if err != nil {
		log.Errorf(err, "delete schema[%s/%s] failed, operator: %s", in.ServiceId, in.SchemaId, remoteIP)
		return &pb.DeleteSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrInvalidParams, err.Error()),
		}, nil
	}
	domainProject := util.ParseDomainProject(ctx)

	// serviceId校验
	if !serviceUtil.ServiceExist(ctx, domainProject, in.ServiceId) {
		log.Errorf(nil, "delete schema[%s/%s] failed, service does not exist, operator: %s",
			in.ServiceId, in.SchemaId, remoteIP)
		return &pb.DeleteSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrServiceNotExists, "Service does not exist."),
		}, nil
	}

	// 校验schemaId
	key := apt.GenerateServiceSchemaKey(domainProject, in.ServiceId, in.SchemaId)
	exist, err := serviceUtil.CheckSchemaInfoExist(ctx, key)
	if err != nil {
		log.Errorf(err, "delete schema[%s/%s] failed, operator: %s", in.ServiceId, in.SchemaId, remoteIP)
		return &pb.DeleteSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrInternal, err.Error()),
		}, err
	}
	if !exist {
		log.Errorf(nil, "delete schema[%s/%s] failed, schema does not exist, operator: %s",
			in.ServiceId, in.SchemaId, remoteIP)
		return &pb.DeleteSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrSchemaNotExists, "Schema info does not exist."),
		}, nil
	}

	// 删除
	epSummaryKey := apt.GenerateServiceSchemaSummaryKey(domainProject, in.ServiceId, in.SchemaId)
	opts := []registry.PluginOp{
		registry.OpDel(registry.WithStrKey(epSummaryKey)),
		registry.OpDel(registry.WithStrKey(key)),
	}

	// 确保service存在
	resp, errDo := backend.Registry().TxnWithCmp(ctx, opts,
		[]registry.CompareOp{registry.OpCmp(
			registry.CmpVer(util.StringToBytesWithNoCopy(apt.GenerateServiceKey(domainProject, in.ServiceId))),
			registry.CmpNotEqual, 0)},
		nil)
	if errDo != nil {
		log.Errorf(errDo, "delete schema[%s/%s] failed, operator: %s", in.ServiceId, in.SchemaId, remoteIP)
		return &pb.DeleteSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrUnavailableBackend, errDo.Error()),
		}, errDo
	}
	if !resp.Succeeded {
		log.Errorf(nil, "delete schema[%s/%s] failed, service does not exist, operator: %s",
			in.ServiceId, in.SchemaId, remoteIP)
		return &pb.DeleteSchemaResponse{
			Response: proto.CreateResponse(scerr.ErrServiceNotExists, "Service does not exist."),
		}, nil
	}

	log.Infof("delete schema[%s/%s] info successfully, operator: %s", in.ServiceId, in.SchemaId, remoteIP)
	return &pb.DeleteSchemaResponse{
		Response: proto.CreateResponse(proto.Response_SUCCESS, "Delete schema info successfully."),
	}, nil
}

// ModifySchemas covers all the schemas of a service.
// To cover the old schemas, ModifySchemas adds new schemas into, delete and
// modify the old schemas.
// 1. When the service is in production environment and schema is not editable:
// If the request contains a new schemaID (the number of schemaIDs of
// the service is also required to be 0, or the request will be rejected),
// the new schemaID will be automatically added to the service information.
// Schema is only allowed to add.
// 2. Other cases:
// If the request contains a new schemaID,
// the new schemaID will be automatically added to the service information.
// Schema is allowed to add/delete/modify.
// 添加，修改，删除 批量
func (s *MicroServiceService) ModifySchemas(ctx context.Context, in *pb.ModifySchemasRequest) (*pb.ModifySchemasResponse, error) {
	remoteIP := util.GetIPFromContext(ctx)
	err := Validate(in)
	if err != nil {
		log.Errorf(err, "modify service[%s] schemas failed, operator: %s", in.ServiceId, remoteIP)
		return &pb.ModifySchemasResponse{
			Response: proto.CreateResponse(scerr.ErrInvalidParams, "Invalid request."),
		}, nil
	}
	serviceID := in.ServiceId

	domainProject := util.ParseDomainProject(ctx)

	service, err := serviceUtil.GetService(ctx, domainProject, serviceID)
	if err != nil {
		log.Errorf(err, "modify service[%s] schemas failed, get service failed, operator: %s", serviceID, remoteIP)
		return &pb.ModifySchemasResponse{
			Response: proto.CreateResponse(scerr.ErrInternal, err.Error()),
		}, err
	}
	if service == nil {
		log.Errorf(nil, "modify service[%s] schemas failed, service does not exist, operator: %s",
			serviceID, remoteIP)
		return &pb.ModifySchemasResponse{
			Response: proto.CreateResponse(scerr.ErrServiceNotExists, "Service does not exist."),
		}, nil
	}

	respErr := s.modifySchemas(ctx, domainProject, service, in.Schemas)
	if respErr != nil {
		log.Errorf(nil, "modify service[%s] schemas failed, operator: %s", serviceID, remoteIP)
		resp := &pb.ModifySchemasResponse{
			Response: proto.CreateResponseWithSCErr(respErr),
		}
		if respErr.InternalError() {
			return resp, respErr
		}
		return resp, nil
	}

	return &pb.ModifySchemasResponse{
		Response: proto.CreateResponse(proto.Response_SUCCESS, "modify schemas info successfully."),
	}, nil
}

// schemas最新的 schemasFromDb已存在的  schemaIDsInService service中的
// 过滤
func schemasAnalysis(schemas []*pb.Schema, schemasFromDb []*pb.Schema, schemaIDsInService []string) (
	[]*pb.Schema, []*pb.Schema, []*pb.Schema, []string) {
	needUpdateSchemas := make([]*pb.Schema, 0, len(schemas))
	needAddSchemas := make([]*pb.Schema, 0, len(schemas))
	needDeleteSchemas := make([]*pb.Schema, 0, len(schemasFromDb))
	nonExistSchemaIds := make([]string, 0, len(schemas))

	duplicate := make(map[string]struct{})
	for _, schema := range schemas {
		if _, ok := duplicate[schema.SchemaId]; ok {
			continue
		}
		duplicate[schema.SchemaId] = struct{}{}

		exist := false
		// db中已存在 修改
		for _, schemaFromDb := range schemasFromDb {
			if schema.SchemaId == schemaFromDb.SchemaId {
				needUpdateSchemas = append(needUpdateSchemas, schema)
				exist = true
				break
			}
		}

		// db中不存在 添加
		if !exist {
			needAddSchemas = append(needAddSchemas, schema)
		}

		exist = false
		for _, schemaID := range schemaIDsInService {
			if schema.SchemaId == schemaID {
				exist = true
			}
		}

		// service.schema中不存在
		if !exist {
			nonExistSchemaIds = append(nonExistSchemaIds, schema.SchemaId)
		}
	}

	for _, schemaFromDb := range schemasFromDb {
		exist := false
		for _, schema := range schemas {
			if schema.SchemaId == schemaFromDb.SchemaId {
				exist = true
				break
			}
		}
		if !exist {
			needDeleteSchemas = append(needDeleteSchemas, schemaFromDb)
		}
	}

	return needUpdateSchemas, needAddSchemas, needDeleteSchemas, nonExistSchemaIds
}

// 操作schemas 每次按全部schema处理
func (s *MicroServiceService) modifySchemas(ctx context.Context, domainProject string, service *pb.MicroService, schemas []*pb.Schema) *scerr.Error {
	remoteIP := util.GetIPFromContext(ctx)
	serviceID := service.ServiceId
	// 获取所有的
	schemasFromDatabase, err := GetSchemasFromDatabase(ctx, domainProject, serviceID)
	if err != nil {
		log.Errorf(nil, "modify service[%s] schemas failed, get schemas failed, operator: %s",
			serviceID, remoteIP)
		return scerr.NewError(scerr.ErrUnavailableBackend, err.Error())
	}

	// 修改的， 添加的，删除的，不存在的
	needUpdateSchemas, needAddSchemas, needDeleteSchemas, nonExistSchemaIds := schemasAnalysis(schemas, schemasFromDatabase, service.Schemas)

	pluginOps := make([]registry.PluginOp, 0)
	if !s.isSchemaEditable(service) {
		// service的schemas不存在
		if len(service.Schemas) == 0 {
			res := quota.NewApplyQuotaResource(quota.SchemaQuotaType, domainProject, serviceID, int64(len(nonExistSchemaIds)))
			rst := plugin.Plugins().Quota().Apply4Quotas(ctx, res)
			errQuota := rst.Err
			if errQuota != nil {
				log.Errorf(errQuota, "modify service[%s] schemas failed, operator: %s", serviceID, remoteIP)
				return errQuota
			}

			// 不支持schemas的修改，会把不存在的schemaId写到service的schemas中
			service.Schemas = nonExistSchemaIds
			opt, err := serviceUtil.UpdateService(domainProject, serviceID, service)
			if err != nil {
				log.Errorf(err, "modify service[%s] schemas failed, update service.Schemas failed, operator: %s",
					serviceID, remoteIP)
				return scerr.NewError(scerr.ErrInternal, err.Error())
			}
			pluginOps = append(pluginOps, opt)
		} else {
			// 存在service.schemaIds中不存在的id报错  支持已存在的schemaId添加 不支持修改
			if len(nonExistSchemaIds) != 0 {
				errInfo := fmt.Errorf("Non-existent schemaIDs %v", nonExistSchemaIds)
				log.Errorf(errInfo, "modify service[%s] schemas failed, operator: %s", serviceID, remoteIP)
				return scerr.NewError(scerr.ErrUndefinedSchemaID, errInfo.Error())
			}
			// service.SchemaId中有的不存在的schema可以添加
			for _, needUpdateSchema := range needUpdateSchemas {
				exist, err := isExistSchemaSummary(ctx, domainProject, serviceID, needUpdateSchema.SchemaId)
				if err != nil {
					return scerr.NewError(scerr.ErrInternal, err.Error())
				}
				if !exist {
					// 不存在支持添加
					opts := schemaWithDatabaseOpera(registry.OpPut, domainProject, serviceID, needUpdateSchema)
					pluginOps = append(pluginOps, opts...)
				} else {
					// 已存在报错
					log.Warnf("schema[%s/%s] and it's summary already exist, skip to update, operator: %s",
						serviceID, needUpdateSchema.SchemaId, remoteIP)
				}
			}
		}

		for _, schema := range needAddSchemas {
			log.Infof("add new schema[%s/%s], operator: %s", serviceID, schema.SchemaId, remoteIP)
			opts := schemaWithDatabaseOpera(registry.OpPut, domainProject, service.ServiceId, schema)
			pluginOps = append(pluginOps, opts...)
		}
	} else {
		quotaSize := len(needAddSchemas) - len(needDeleteSchemas)
		if quotaSize > 0 {
			res := quota.NewApplyQuotaResource(quota.SchemaQuotaType, domainProject, serviceID, int64(quotaSize))
			rst := plugin.Plugins().Quota().Apply4Quotas(ctx, res)
			err := rst.Err
			if err != nil {
				log.Errorf(err, "modify service[%s] schemas failed, operator: %s", serviceID, remoteIP)
				return err
			}
		}

		var schemaIDs []string
		// 添加
		for _, schema := range needAddSchemas {
			log.Infof("add new schema[%s/%s], operator: %s", serviceID, schema.SchemaId, remoteIP)
			opts := schemaWithDatabaseOpera(registry.OpPut, domainProject, service.ServiceId, schema)
			pluginOps = append(pluginOps, opts...)
			schemaIDs = append(schemaIDs, schema.SchemaId)
		}

		// 修改
		for _, schema := range needUpdateSchemas {
			log.Infof("update schema[%s/%s], operator: %s", serviceID, schema.SchemaId, remoteIP)
			opts := schemaWithDatabaseOpera(registry.OpPut, domainProject, serviceID, schema)
			pluginOps = append(pluginOps, opts...)
			schemaIDs = append(schemaIDs, schema.SchemaId)
		}

		// 删除
		for _, schema := range needDeleteSchemas {
			log.Infof("delete non-existent schema[%s/%s], operator: %s", serviceID, schema.SchemaId, remoteIP)
			opts := schemaWithDatabaseOpera(registry.OpDel, domainProject, serviceID, schema)
			pluginOps = append(pluginOps, opts...)
		}

		// 设置service.schemas
		service.Schemas = schemaIDs
		opt, err := serviceUtil.UpdateService(domainProject, serviceID, service)
		if err != nil {
			log.Errorf(err, "modify service[%s] schemas failed, update service.Schemas failed, operator: %s",
				serviceID, remoteIP)
			return scerr.NewError(scerr.ErrInternal, err.Error())
		}
		pluginOps = append(pluginOps, opt)
	}

	// 写入etcd 确保service已存在
	if len(pluginOps) != 0 {
		resp, err := backend.BatchCommitWithCmp(ctx, pluginOps,
			[]registry.CompareOp{registry.OpCmp(
				registry.CmpVer(util.StringToBytesWithNoCopy(apt.GenerateServiceKey(domainProject, serviceID))),
				registry.CmpNotEqual, 0)},
			nil)
		if err != nil {
			return scerr.NewError(scerr.ErrUnavailableBackend, err.Error())
		}
		if !resp.Succeeded {
			return scerr.NewError(scerr.ErrServiceNotExists, "Service does not exist.")
		}
	}
	return nil
}

// 是否允许编辑schema 非生产环境 || service开启编辑功能
func (s *MicroServiceService) isSchemaEditable(service *pb.MicroService) bool {
	return (len(service.Environment) != 0 && service.Environment != pb.ENV_PROD) || s.schemaEditable
}

// schemasId是否在service.schemas中
func isExistSchemaID(service *pb.MicroService, schemas []*pb.Schema) bool {
	serviceSchemaIds := service.Schemas
	for _, schema := range schemas {
		if !containsValueInSlice(serviceSchemaIds, schema.SchemaId) {
			log.Errorf(nil, "schema[%s/%s] does not exist schemaID", service.ServiceId, schema.SchemaId)
			return false
		}
	}
	return true
}

// 添加schema
func schemaWithDatabaseOpera(invoke registry.Operation, domainProject string, serviceID string, schema *pb.Schema) []registry.PluginOp {
	pluginOps := make([]registry.PluginOp, 0)
	// schema value
	key := apt.GenerateServiceSchemaKey(domainProject, serviceID, schema.SchemaId)
	opt := invoke(registry.WithStrKey(key), registry.WithStrValue(schema.Schema))
	pluginOps = append(pluginOps, opt)
	// schema key
	keySummary := apt.GenerateServiceSchemaSummaryKey(domainProject, serviceID, schema.SchemaId)
	opt = invoke(registry.WithStrKey(keySummary), registry.WithStrValue(schema.Summary))
	pluginOps = append(pluginOps, opt)
	return pluginOps
}

// 获取所有schema
func GetSchemasFromDatabase(ctx context.Context, domainProject string, serviceID string) ([]*pb.Schema, error) {
	key := apt.GenerateServiceSchemaKey(domainProject, serviceID, "")
	resp, err := backend.Store().Schema().Search(ctx,
		registry.WithPrefix(),
		registry.WithStrKey(key))
	if err != nil {
		log.Errorf(err, "get service[%s]'s schema failed", serviceID)
		return nil, err
	}
	schemas := make([]*pb.Schema, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := util.BytesToStringWithNoCopy(kv.Key)
		tmp := strings.Split(key, "/")
		schemaID := tmp[len(tmp)-1]
		schema := util.BytesToStringWithNoCopy(kv.Value.([]byte))
		schemaStruct := &pb.Schema{
			SchemaId: schemaID,
			Schema:   schema,
		}
		schemas = append(schemas, schemaStruct)
	}
	return schemas, nil
}

// ModifySchema modifies a specific schema
// 1. When the service is in production environment and schema is not editable:
// If the request contains a new schemaID (the number of schemaIDs of
// the service is also required to be 0, or the request will be rejected),
// the new schemaID will be automatically added to the service information.
// Schema is only allowed to add.
// 2. Other cases:
// If the request contains a new schemaID,
// the new schemaID will be automatically added to the service information.
// Schema is allowed to add/modify.
// 修改指定schema
func (s *MicroServiceService) ModifySchema(ctx context.Context, request *pb.ModifySchemaRequest) (*pb.ModifySchemaResponse, error) {
	remoteIP := util.GetIPFromContext(ctx)
	domainProject := util.ParseDomainProject(ctx)
	// 能否修改
	respErr := s.canModifySchema(ctx, domainProject, request)
	if respErr != nil {
		resp := &pb.ModifySchemaResponse{
			Response: proto.CreateResponseWithSCErr(respErr),
		}
		if respErr.InternalError() {
			return resp, respErr
		}
		return resp, nil
	}

	serviceID := request.ServiceId
	schemaID := request.SchemaId

	schema := pb.Schema{
		SchemaId: schemaID,
		Summary:  request.Summary,
		Schema:   request.Schema,
	}
	err := s.modifySchema(ctx, serviceID, &schema)
	if err != nil {
		log.Errorf(err, "modify schema[%s/%s] failed, operator: %s", serviceID, schemaID, remoteIP)
		resp := &pb.ModifySchemaResponse{
			Response: proto.CreateResponseWithSCErr(err),
		}
		if err.InternalError() {
			return resp, err
		}
		return resp, nil
	}

	log.Infof("modify schema[%s/%s] successfully, operator: %s", serviceID, schemaID, remoteIP)
	return &pb.ModifySchemaResponse{
		Response: proto.CreateResponse(proto.Response_SUCCESS, "modify schema info success"),
	}, nil
}

// 能否能修改schema
func (s *MicroServiceService) canModifySchema(ctx context.Context, domainProject string, in *pb.ModifySchemaRequest) *scerr.Error {
	remoteIP := util.GetIPFromContext(ctx)
	serviceID := in.ServiceId
	schemaID := in.SchemaId
	if len(schemaID) == 0 || len(serviceID) == 0 {
		log.Errorf(nil, "update schema[%s/%s] failed, invalid params, operator: %s",
			serviceID, schemaID, remoteIP)
		return scerr.NewError(scerr.ErrInvalidParams, "serviceID or schemaID is nil")
	}
	err := Validate(in)
	if err != nil {
		log.Errorf(err, "update schema[%s/%s] failed, operator: %s", serviceID, schemaID, remoteIP)
		return scerr.NewError(scerr.ErrInvalidParams, err.Error())
	}

	//
	res := quota.NewApplyQuotaResource(quota.SchemaQuotaType, domainProject, serviceID, 1)
	rst := plugin.Plugins().Quota().Apply4Quotas(ctx, res)
	errQuota := rst.Err
	if errQuota != nil {
		log.Errorf(errQuota, "update schema[%s/%s] failed, operator: %s", serviceID, schemaID, remoteIP)
		return errQuota
	}

	//
	if len(in.Summary) == 0 {
		log.Warnf("schema[%s/%s]'s summary is empty, operator: %s", serviceID, schemaID, remoteIP)
	}
	return nil
}

// 修改一个schema
func (s *MicroServiceService) modifySchema(ctx context.Context, serviceID string, schema *pb.Schema) *scerr.Error {
	remoteIP := util.GetIPFromContext(ctx)
	domainProject := util.ParseDomainProject(ctx)
	schemaID := schema.SchemaId

	// service校验
	service, err := serviceUtil.GetService(ctx, domainProject, serviceID)
	if err != nil {
		log.Errorf(err, "modify schema[%s/%s] failed, get service failed, operator: %s",
			serviceID, schemaID, remoteIP)
		return scerr.NewError(scerr.ErrInternal, err.Error())
	}
	if service == nil {
		log.Errorf(nil, "modify schema[%s/%s] failed, service does not exist, operator: %s",
			serviceID, schemaID, remoteIP)
		return scerr.NewError(scerr.ErrServiceNotExists, "Service does not exist")
	}

	var pluginOps []registry.PluginOp
	// 是否存在service.schemas中
	isExist := isExistSchemaID(service, []*pb.Schema{schema})

	// 不支持修改 只允许添加
	if !s.isSchemaEditable(service) {
		// 不在service.Schemas中
		if len(service.Schemas) != 0 && !isExist {
			return scerr.NewError(scerr.ErrUndefinedSchemaID, "Non-existent schemaID can't be added in "+pb.ENV_PROD)
		}

		// schemaId是否已存在
		key := apt.GenerateServiceSchemaKey(domainProject, serviceID, schemaID)
		respSchema, err := backend.Store().Schema().Search(ctx, registry.WithStrKey(key), registry.WithCountOnly())
		if err != nil {
			log.Errorf(err, "modify schema[%s/%s] failed, get schema summary failed, operator: %s",
				serviceID, schemaID, remoteIP)
			return scerr.NewError(scerr.ErrUnavailableBackend, err.Error())
		}

		// 已存在
		if respSchema.Count != 0 {
			// 校验schema对应的summary
			if len(schema.Summary) == 0 {
				log.Errorf(err, "%s mode, schema[%s/%s] already exists, can not be changed, operator: %s",
					pb.ENV_PROD, serviceID, schemaID, remoteIP)
				return scerr.NewError(scerr.ErrModifySchemaNotAllow, "schema already exist, can not be changed in "+pb.ENV_PROD)
			}

			exist, err := isExistSchemaSummary(ctx, domainProject, serviceID, schemaID)
			if err != nil {
				log.Errorf(err, "check schema[%s/%s] summary existence failed, operator: %s",
					serviceID, schemaID, remoteIP)
				return scerr.NewError(scerr.ErrInternal, err.Error())
			}
			if exist {
				log.Errorf(err, "%s mode, schema[%s/%s] already exist, can not be changed, operator: %s",
					pb.ENV_PROD, serviceID, schemaID, remoteIP)
				return scerr.NewError(scerr.ErrModifySchemaNotAllow, "schema already exist, can not be changed in "+pb.ENV_PROD)
			}
		}

		//
		if len(service.Schemas) == 0 {
			service.Schemas = append(service.Schemas, schemaID)
			opt, err := serviceUtil.UpdateService(domainProject, serviceID, service)
			if err != nil {
				log.Errorf(err, "modify schema[%s/%s] failed, update service.Schemas failed, operator: %s",
					serviceID, schemaID, remoteIP)
				return scerr.NewError(scerr.ErrInternal, err.Error())
			}
			pluginOps = append(pluginOps, opt)
		}
	} else {
		if !isExist {
			service.Schemas = append(service.Schemas, schemaID)
			opt, err := serviceUtil.UpdateService(domainProject, serviceID, service)
			if err != nil {
				log.Errorf(err, "modify schema[%s/%s] failed, update service.Schemas failed, operator: %s",
					serviceID, schemaID, remoteIP)
				return scerr.NewError(scerr.ErrInternal, err.Error())
			}
			pluginOps = append(pluginOps, opt)
		}
	}

	opts := CommitSchemaInfo(domainProject, serviceID, schema)
	pluginOps = append(pluginOps, opts...)

	resp, err := backend.Registry().TxnWithCmp(ctx, pluginOps,
		[]registry.CompareOp{registry.OpCmp(
			registry.CmpVer(util.StringToBytesWithNoCopy(apt.GenerateServiceKey(domainProject, serviceID))),
			registry.CmpNotEqual, 0)},
		nil)
	if err != nil {
		return scerr.NewError(scerr.ErrUnavailableBackend, err.Error())
	}
	if !resp.Succeeded {
		return scerr.NewError(scerr.ErrServiceNotExists, "Service does not exist.")
	}
	return nil
}

// schemaId是否存在 summary
func isExistSchemaSummary(ctx context.Context, domainProject, serviceID, schemaID string) (bool, error) {
	key := apt.GenerateServiceSchemaSummaryKey(domainProject, serviceID, schemaID)
	resp, err := backend.Store().SchemaSummary().Search(ctx, registry.WithStrKey(key), registry.WithCountOnly())
	if err != nil {
		return true, err
	}
	if resp.Count == 0 {
		return false, nil
	}
	return true, nil
}

func CommitSchemaInfo(domainProject string, serviceID string, schema *pb.Schema) []registry.PluginOp {
	if len(schema.Summary) != 0 {
		return schemaWithDatabaseOpera(registry.OpPut, domainProject, serviceID, schema)
	}
	key := apt.GenerateServiceSchemaKey(domainProject, serviceID, schema.SchemaId)
	opt := registry.OpPut(registry.WithStrKey(key), registry.WithStrValue(schema.Schema))
	return []registry.PluginOp{opt}
}

func containsValueInSlice(in []string, value string) bool {
	if in == nil || len(value) == 0 {
		return false
	}
	for _, i := range in {
		if i == value {
			return true
		}
	}
	return false
}

func getSchemaSummary(ctx context.Context, domainProject string, serviceID string, schemaID string) (string, error) {
	key := apt.GenerateServiceSchemaSummaryKey(domainProject, serviceID, schemaID)
	resp, err := backend.Store().SchemaSummary().Search(ctx,
		registry.WithStrKey(key),
	)
	if err != nil {
		log.Errorf(err, "get schema[%s/%s] summary failed", serviceID, schemaID)
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return resp.Kvs[0].Value.(string), nil
}
