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

package broker

import (
	"github.com/apache/servicecomb-service-center/server/core/backend"
	"github.com/apache/servicecomb-service-center/server/plugin/discovery"
)

var (
	PARTICIPANT  discovery.Type
	VERSION      discovery.Type
	PACT         discovery.Type
	PactVersion  discovery.Type
	PactTag      discovery.Type
	VERIFICATION discovery.Type
	PactLatest   discovery.Type
)

var brokerKvStore = &BKvStore{}

func init() {
	PARTICIPANT = backend.Store().MustInstall(backend.NewAddOn("PARTICIPANT",
		discovery.Configure().WithPrefix(GetBrokerParticipantKey(""))))
	VERSION = backend.Store().MustInstall(backend.NewAddOn("VERSION",
		discovery.Configure().WithPrefix(GetBrokerVersionKey(""))))
	PACT = backend.Store().MustInstall(backend.NewAddOn("PACT",
		discovery.Configure().WithPrefix(GetBrokerPactKey(""))))
	PactVersion = backend.Store().MustInstall(backend.NewAddOn("PACT_VERSION",
		discovery.Configure().WithPrefix(GetBrokerPactVersionKey(""))))
	PactTag = backend.Store().MustInstall(backend.NewAddOn("PACT_TAG",
		discovery.Configure().WithPrefix(GetBrokerTagKey(""))))
	VERIFICATION = backend.Store().MustInstall(backend.NewAddOn("VERIFICATION",
		discovery.Configure().WithPrefix(GetBrokerVerificationKey(""))))
	PactLatest = backend.Store().MustInstall(backend.NewAddOn("PACT_LATEST",
		discovery.Configure().WithPrefix(GetBrokerLatestKey(""))))
}

type BKvStore struct {
}

func (s *BKvStore) Participant() discovery.Indexer {
	return backend.Store().Adaptors(PARTICIPANT)
}

func (s *BKvStore) Version() discovery.Indexer {
	return backend.Store().Adaptors(VERSION)
}

func (s *BKvStore) Pact() discovery.Indexer {
	return backend.Store().Adaptors(PACT)
}

func (s *BKvStore) PactVersion() discovery.Indexer {
	return backend.Store().Adaptors(PactVersion)
}

func (s *BKvStore) PactTag() discovery.Indexer {
	return backend.Store().Adaptors(PactTag)
}

func (s *BKvStore) Verification() discovery.Indexer {
	return backend.Store().Adaptors(VERIFICATION)
}

func (s *BKvStore) PactLatest() discovery.Indexer {
	return backend.Store().Adaptors(PactLatest)
}

func Store() *BKvStore {
	return brokerKvStore
}
