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

package registry

type MicroServiceInstance struct {
	InstanceId string `protobuf:"bytes,1,opt,name=instanceId" json:"instanceId,omitempty"`
	// 关联的service id
	ServiceId string `protobuf:"bytes,2,opt,name=serviceId" json:"serviceId,omitempty"`
	// api的ip:端口号信息
	Endpoints []string `protobuf:"bytes,3,rep,name=endpoints" json:"endpoints,omitempty"`
	// 机器的hostName
	HostName   string            `protobuf:"bytes,4,opt,name=hostName" json:"hostName,omitempty"`
	Status     string            `protobuf:"bytes,5,opt,name=status" json:"status,omitempty"`
	Properties map[string]string `protobuf:"bytes,6,rep,name=properties" json:"properties,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	// 续租信息
	HealthCheck *HealthCheck `protobuf:"bytes,7,opt,name=healthCheck" json:"healthCheck,omitempty"`
	// 创建时间
	Timestamp      string          `protobuf:"bytes,8,opt,name=timestamp" json:"timestamp,omitempty"`
	DataCenterInfo *DataCenterInfo `protobuf:"bytes,9,opt,name=dataCenterInfo" json:"dataCenterInfo,omitempty"`
	// 修改时间
	ModTimestamp string `protobuf:"bytes,10,opt,name=modTimestamp" json:"modTimestamp,omitempty"`
	// 版本号 service的
	Version string `protobuf:"bytes,11,opt,name=version" json:"version,omitempty"`
}
type RegisterInstanceRequest struct {
	Instance *MicroServiceInstance `protobuf:"bytes,1,opt,name=instance" json:"instance,omitempty"`
}
type FindInstancesRequest struct {
	ConsumerServiceId string   `protobuf:"bytes,1,opt,name=consumerServiceId" json:"consumerServiceId,omitempty"`
	AppId             string   `protobuf:"bytes,2,opt,name=appId" json:"appId,omitempty"`
	ServiceName       string   `protobuf:"bytes,3,opt,name=serviceName" json:"serviceName,omitempty"`
	VersionRule       string   `protobuf:"bytes,4,opt,name=versionRule" json:"versionRule,omitempty"`
	Tags              []string `protobuf:"bytes,5,rep,name=tags" json:"tags,omitempty"`
	Environment       string   `protobuf:"bytes,6,opt,name=environment" json:"environment,omitempty"`
}
type FindInstancesResponse struct {
	Response  *Response               `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
	Instances []*MicroServiceInstance `protobuf:"bytes,2,rep,name=instances" json:"instances,omitempty"`
}

type GetOneInstanceRequest struct {
	ConsumerServiceId  string   `protobuf:"bytes,1,opt,name=consumerServiceId" json:"consumerServiceId,omitempty"`
	ProviderServiceId  string   `protobuf:"bytes,2,opt,name=providerServiceId" json:"providerServiceId,omitempty"`
	ProviderInstanceId string   `protobuf:"bytes,3,opt,name=providerInstanceId" json:"providerInstanceId,omitempty"`
	Tags               []string `protobuf:"bytes,4,rep,name=tags" json:"tags,omitempty"`
}
type GetOneInstanceResponse struct {
	Response *Response             `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
	Instance *MicroServiceInstance `protobuf:"bytes,2,opt,name=instance" json:"instance,omitempty"`
}
type GetInstancesRequest struct {
	ConsumerServiceId string   `protobuf:"bytes,1,opt,name=consumerServiceId" json:"consumerServiceId,omitempty"`
	ProviderServiceId string   `protobuf:"bytes,2,opt,name=providerServiceId" json:"providerServiceId,omitempty"`
	Tags              []string `protobuf:"bytes,3,rep,name=tags" json:"tags,omitempty"`
}

type GetInstancesResponse struct {
	Response  *Response               `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
	Instances []*MicroServiceInstance `protobuf:"bytes,2,rep,name=instances" json:"instances,omitempty"`
}

type UpdateInstanceStatusRequest struct {
	ServiceId  string `protobuf:"bytes,1,opt,name=serviceId" json:"serviceId,omitempty"`
	InstanceId string `protobuf:"bytes,2,opt,name=instanceId" json:"instanceId,omitempty"`
	Status     string `protobuf:"bytes,3,opt,name=status" json:"status,omitempty"`
}

type RegisterInstanceResponse struct {
	Response   *Response `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
	InstanceId string    `protobuf:"bytes,2,opt,name=instanceId" json:"instanceId,omitempty"`
}

type UnregisterInstanceRequest struct {
	ServiceId  string `protobuf:"bytes,1,opt,name=serviceId" json:"serviceId,omitempty"`
	InstanceId string `protobuf:"bytes,2,opt,name=instanceId" json:"instanceId,omitempty"`
}

type UnregisterInstanceResponse struct {
	Response *Response `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
}

type HeartbeatRequest struct {
	ServiceId  string `protobuf:"bytes,1,opt,name=serviceId" json:"serviceId,omitempty"`
	InstanceId string `protobuf:"bytes,2,opt,name=instanceId" json:"instanceId,omitempty"`
}

type HeartbeatResponse struct {
	Response *Response `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
}

type UpdateInstanceStatusResponse struct {
	Response *Response `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
}

type UpdateInstancePropsRequest struct {
	ServiceId  string            `protobuf:"bytes,1,opt,name=serviceId" json:"serviceId,omitempty"`
	InstanceId string            `protobuf:"bytes,2,opt,name=instanceId" json:"instanceId,omitempty"`
	Properties map[string]string `protobuf:"bytes,3,rep,name=properties" json:"properties,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

type UpdateInstancePropsResponse struct {
	Response *Response `protobuf:"bytes,1,opt,name=response" json:"response,omitempty"`
}
