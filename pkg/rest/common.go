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

package rest

import (
	"net/http"
)

const (
	HTTPMethodGet    = http.MethodGet
	HTTPMethodPut    = http.MethodPut
	HTTPMethodPost   = http.MethodPost
	HTTPMethodDelete = http.MethodDelete

	CtxResponse     = "_server_response"      // response
	CtxRequest      = "_server_request"       // request
	CtxMatchPattern = "_server_match_pattern" // 注册时的uri
	CtxMatchFunc    = "_server_match_func"    // 注册的httpHandler函数名

	ServerChainName = "_server_chain"

	HeaderResponseStatus = "X-Response-Status"

	HeaderAllow           = "Allow"
	HeaderHost            = "Host"
	HeaderServer          = "Server" //标记server center名称+版本号
	HeaderContentType     = "Content-Type"
	HeaderContentEncoding = "Content-Encoding"
	HeaderAccept          = "Accept"
	HeaderAcceptEncoding  = "Accept-Encoding"

	AcceptAny = "*/*"

	ContentTypeJSON = "application/json; charset=UTF-8"
	ContentTypeText = "text/plain; charset=UTF-8"

	DefaultConnPoolPerHostSize = 5
)

// 是否是合法的http method
func isValidMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete:
		return true
	default:
		return false
	}
}
