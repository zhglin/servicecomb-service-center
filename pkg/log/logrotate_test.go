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
package log

import (
	"os"
	"testing"
)

func TestLogRotate(t *testing.T) {
	s := pathReplacer.Replace("./sc.log")
	if s != "./sc.log" {
		t.Fatal("pathReplacer failed", s)
	}

	Rotate("../../etc", 1, 1)

	RotateFile("../../etc/conf/app.conf", 1, 1)

	if err := compressFile(
		"../../etc/conf/app.conf",
		"app",
		false); err != nil {
		t.Fatal("TestLogRotate failed", err)
	}

	os.Chmod("../../etc/conf/app.conf.zip", 0640)
	if err := removeFile("../../etc/conf/app.conf.zip"); err != nil {
		t.Fatal("TestLogRotate failed", err)
	}
}
