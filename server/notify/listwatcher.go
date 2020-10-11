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
	"context"
	"github.com/apache/servicecomb-service-center/pkg/gopool"
	"github.com/apache/servicecomb-service-center/pkg/log"
	"github.com/apache/servicecomb-service-center/pkg/notify"
	pb "github.com/apache/servicecomb-service-center/pkg/registry"
	simple "github.com/apache/servicecomb-service-center/pkg/time"
	"time"
)

// 状态变化推送
type InstanceEvent struct {
	notify.Event
	Revision int64
	Response *pb.WatchInstanceResponse
}

// Socked订阅者
type InstanceEventListWatcher struct {
	notify.Subscriber
	Job          chan *InstanceEvent  // 有缓冲的chain
	ListRevision int64
	ListFunc     func() (results []*pb.WatchInstanceResponse, rev int64)
	listCh       chan struct{}  // 只是等待OnAccept()执行完成
}

// 设置异常并关闭subscriber  conn写入异常
func (w *InstanceEventListWatcher) SetError(err error) {
	w.Subscriber.SetError(err)
	// 触发清理job
	e := w.Service().Publish(notify.NewNotifyServiceHealthCheckJob(w))
	if e != nil {
		log.Error("", e)
	}
}

// 添加到notifyService时的调用
func (w *InstanceEventListWatcher) OnAccept() {
	if w.Err() != nil {
		return
	}
	log.Debugf("accepted by notify service, %s watcher %s %s", w.Type(), w.Group(), w.Subject())
	gopool.Go(w.listAndPublishJobs)
}

// 异步发送所有ListFunc数据
func (w *InstanceEventListWatcher) listAndPublishJobs(_ context.Context) {
	defer close(w.listCh)
	if w.ListFunc == nil {
		return
	}
	results, rev := w.ListFunc()
	w.ListRevision = rev
	for _, response := range results {
		w.sendMessage(NewInstanceEvent(w.Group(), w.Subject(), w.ListRevision, response))
	}
}

//被通知 notifyService的调用
func (w *InstanceEventListWatcher) OnMessage(job notify.Event) {
	if w.Err() != nil {
		return
	}

	wJob, ok := job.(*InstanceEvent)
	if !ok {
		return
	}

	select {
	case <-w.listCh: // 等待listAndPublishJobs()的close
	default:
		timer := time.NewTimer(w.Timeout())
		select {
		case <-w.listCh:
			timer.Stop()
		case <-timer.C:	// 未准备好
			log.Errorf(nil,
				"the %s listwatcher %s %s is not ready[over %s], send the event %v",
				w.Type(), w.Group(), w.Subject(), w.Timeout(), job)
		}
	}

	if wJob.Revision <= w.ListRevision {
		log.Warnf("unexpected notify %s job is coming in, watcher %s %s, job is %v, current revision is %v",
			w.Type(), w.Group(), w.Subject(), job, w.ListRevision)
		return
	}
	w.sendMessage(wJob) // 统一收集job
}

// 统一存放job  超时丢弃
func (w *InstanceEventListWatcher) sendMessage(job *InstanceEvent) {
	defer log.Recover()
	select {
	case w.Job <- job:
	default:
		timer := time.NewTimer(w.Timeout())
		select {
		case w.Job <- job:
			timer.Stop()
		case <-timer.C:
			log.Errorf(nil,
				"the %s watcher %s %s event queue is full[over %s], drop the event %v",
				w.Type(), w.Group(), w.Subject(), w.Timeout(), job)
		}
	}
}

// 添加事件的超时时间
func (w *InstanceEventListWatcher) Timeout() time.Duration {
	return AddJobTimeout
}

func (w *InstanceEventListWatcher) Close() {
	close(w.Job)
}

func NewInstanceEvent(serviceID, domainProject string, rev int64, response *pb.WatchInstanceResponse) *InstanceEvent {
	return &InstanceEvent{
		Event:    notify.NewEvent(INSTANCE, domainProject, serviceID),
		Revision: rev,
		Response: response,
	}
}

func NewInstanceEventWithTime(serviceID, domainProject string, rev int64, createAt simple.Time, response *pb.WatchInstanceResponse) *InstanceEvent {
	return &InstanceEvent{
		Event:    notify.NewEventWithTime(INSTANCE, domainProject, serviceID, createAt),
		Revision: rev,
		Response: response,
	}
}

func NewInstanceEventListWatcher(serviceID, domainProject string,
	listFunc func() (results []*pb.WatchInstanceResponse, rev int64)) *InstanceEventListWatcher {
	watcher := &InstanceEventListWatcher{
		Subscriber: notify.NewSubscriber(INSTANCE, domainProject, serviceID),
		Job:        make(chan *InstanceEvent, INSTANCE.QueueSize()),
		ListFunc:   listFunc,
		listCh:     make(chan struct{}),
	}
	return watcher
}
