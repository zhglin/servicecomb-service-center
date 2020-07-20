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
package notify

import (
	"testing"
	"time"
)

type mockSubscriberChan struct {
	Subscriber
	job chan Event
}

func (s *mockSubscriberChan) OnMessage(job Event) {
	s.job <- job
}

func TestProcessor_Do(t *testing.T) {
	INSTANCE := RegisterType("INSTANCE", 1)
	delay := 50 * time.Millisecond
	mock1 := &mockSubscriberChan{Subscriber: NewSubscriber(INSTANCE, "s1", "g1"),
		job: make(chan Event, 1)}
	mock2 := &mockSubscriberChan{Subscriber: NewSubscriber(INSTANCE, "s1", "g2"),
		job: make(chan Event, 1)}
	p := NewProcessor("p1", 0)
	if p.Name() != "p1" {
		t.Fatalf("TestProcessor_Do")
	}
	if p.Subjects(mock1.Subject()) != nil {
		t.Fatalf("TestProcessor_Do")
	}
	p.AddSubscriber(mock1)
	if p.Subjects(mock1.Subject()).Groups(mock1.Group()).Subscribers(mock1.ID()) != mock1 {
		t.Fatalf("TestProcessor_Do")
	}
	p.Remove(NewSubscriber(INSTANCE, "s2", "g1"))
	p.Remove(NewSubscriber(INSTANCE, "s1", "g2"))
	p.Remove(mock1)
	if p.Subjects(mock1.Subject()) != nil {
		t.Fatalf("TestProcessor_Do")
	}
	p.AddSubscriber(mock1)
	p.Clear()
	if p.Subjects(mock1.Subject()) != nil {
		t.Fatalf("TestProcessor_Do")
	}
	p.AddSubscriber(mock1)
	p.AddSubscriber(mock2)
	job := &baseEvent{group: "g1"}
	p.Handle(nil, job)
	select {
	case <-mock1.job:
		t.Fatalf("TestProcessor_Do")
	case <-time.After(delay):
	}
	job.subject = "s1"
	job.group = "g3"
	p.Handle(nil, job)
	select {
	case <-mock1.job:
		t.Fatalf("TestProcessor_Do")
	case <-time.After(delay):
	}
	job.subject = "s1"
	job.group = "g1"
	p.Handle(nil, job)
	select {
	case j := <-mock1.job:
		if j != job {
			t.Fatalf("TestProcessor_Do")
		}
	case <-time.After(delay):
		t.Fatalf("TestProcessor_Do")
	}
	select {
	case <-mock2.job:
		t.Fatalf("TestProcessor_Do")
	case <-time.After(delay):
	}
	job.subject = "s1"
	job.group = ""
	p.Handle(nil, job)
	select {
	case j := <-mock1.job:
		if j != job {
			t.Fatalf("TestProcessor_Do")
		}
	case <-time.After(delay):
		t.Fatalf("TestProcessor_Do")
	}
	select {
	case j := <-mock2.job:
		if j != job {
			t.Fatalf("TestProcessor_Do")
		}
	case <-time.After(delay):
		t.Fatalf("TestProcessor_Do")
	}
}
