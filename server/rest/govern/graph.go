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

package govern

//Node 节点信息
type Node struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	AppID    string   `json:"appId"`
	Version  string   `json:"version"`
	Type     string   `json:"type"`
	Color    string   `json:"color"`
	Position string   `json:"position"`
	Visits   []string `json:"-"`
}

//Line 连接线信息
type Line struct {
	From        Node   `json:"from"`
	To          Node   `json:"to"`
	Type        string `json:"type"`
	Color       string `json:"color"`
	Description string `json:"descriptor"`
}

//Circle 环信息
type Circle struct {
	Nodes []Node `json:"nodes"`
}

//Graph 图全集信息
type Graph struct {
	Nodes   []Node   `json:"nodes"`
	Lines   []Line   `json:"lines"`
	Circles []Circle `json:"circles"`
	Visits  []string `json:"-"`
}
