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
package core_test

import (
	"runtime"
	"sync"

	"..//lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WriterSink", func() {
	const MaxThreads = 100

	var sink lager.Sink
	var writer *copyWriter

	BeforeSuite(func() {
		runtime.GOMAXPROCS(MaxThreads)
	})

	BeforeEach(func() {
		writer = NewCopyWriter()
		sink = lager.NewWriterSink(writer, lager.INFO)
	})

	Context("when logging above the minimum log level", func() {
		BeforeEach(func() {
			sink.Log(lager.INFO, []byte("hello world"))
		})

		It("writes to the given writer", func() {
			Ω(writer.Copy()).Should(Equal([]byte("hello world\n")))
		})
	})

	Context("when logging below the minimum log level", func() {
		BeforeEach(func() {
			sink.Log(lager.DEBUG, []byte("hello world"))
		})

		It("does not write to the given writer", func() {
			Ω(writer.Copy()).Should(Equal([]byte{}))
		})
	})

	Context("when logging from multiple threads", func() {
		var content = "abcdefg "

		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			for i := 0; i < MaxThreads; i++ {
				wg.Add(1)
				go func() {
					sink.Log(lager.INFO, []byte(content))
					wg.Done()
				}()
			}
			wg.Wait()
		})

		It("writes to the given writer", func() {
			expectedBytes := []byte{}
			for i := 0; i < MaxThreads; i++ {
				expectedBytes = append(expectedBytes, []byte(content)...)
				expectedBytes = append(expectedBytes, []byte("\n")...)
			}
			Ω(writer.Copy()).Should(Equal(expectedBytes))
		})
	})
})

// copyWriter is an INTENTIONALLY UNSAFE writer. Use it to test code that
// should be handling thread safety.
type copyWriter struct {
	contents []byte
	lock     *sync.RWMutex
}

func NewCopyWriter() *copyWriter {
	return &copyWriter{
		contents: []byte{},
		lock:     new(sync.RWMutex),
	}
}

// no, we really mean RLock on write.
func (writer *copyWriter) Write(p []byte) (n int, err error) {
	writer.lock.RLock()
	defer writer.lock.RUnlock()

	writer.contents = append(writer.contents, p...)
	return len(p), nil
}

func (writer *copyWriter) Copy() []byte {
	writer.lock.Lock()
	defer writer.lock.Unlock()

	contents := make([]byte, len(writer.contents))
	copy(contents, writer.contents)
	return contents
}
