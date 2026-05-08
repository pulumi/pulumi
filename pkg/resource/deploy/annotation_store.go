// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deploy

import (
	"regexp"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// userAPIAnnotationKey matches annotation keys of the form "user:api/{kind}" where kind is
// a non-empty identifier. Policy writes are restricted to user:api source per the V2 design.
var userAPIAnnotationKey = regexp.MustCompile(`^user:api/[a-z0-9._-]{1,128}$`)

// AnnotationStore is a thread-safe in-memory cache of resource annotations for the duration
// of a deployment. It is seeded from the service at the start of an update and accumulates
// policy-written annotations to be flushed at end of run. Inner key is "{source}/{kind}".
type AnnotationStore struct {
	mu sync.RWMutex
	// outer key: URN; inner key: "{source}/{kind}".
	annotations map[resource.URN]map[string]resource.PropertyMap
	pending     []plugin.AnalyzeAnnotationChange
}

func NewAnnotationStore() *AnnotationStore {
	return &AnnotationStore{
		annotations: make(map[resource.URN]map[string]resource.PropertyMap),
	}
}

// SeedAll bulk-loads annotations grouped by URN, typically from a pre-run fetch.
func (s *AnnotationStore) SeedAll(entries map[resource.URN]map[string]resource.PropertyMap) {
	if len(entries) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for urn, byKey := range entries {
		if len(byKey) == 0 {
			continue
		}
		dst, ok := s.annotations[urn]
		if !ok {
			dst = make(map[string]resource.PropertyMap, len(byKey))
			s.annotations[urn] = dst
		}
		for key, data := range byKey {
			dst[key] = data
		}
	}
}

// Get returns a copy of all annotations for the given URN, keyed by "{source}/{kind}".
func (s *AnnotationStore) Get(urn resource.URN) map[string]resource.PropertyMap {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.annotations[urn]
	if len(src) == 0 {
		return nil
	}
	result := make(map[string]resource.PropertyMap, len(src))
	for k, v := range src {
		result[k] = v
	}
	return result
}

// ApplyPolicyWrites applies annotation writes from a policy response to the cache and
// appends them to the pending list for post-run flush. Writes whose key does not match
// "user:api/{kind}" are dropped with a warning.
func (s *AnnotationStore) ApplyPolicyWrites(writes []plugin.AnalyzeAnnotationChange) {
	if len(writes) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range writes {
		if !userAPIAnnotationKey.MatchString(w.Key) {
			logging.Warningf("dropping policy annotation write with invalid key %q on resource %s "+
				"(must match user:api/<kind>)", w.Key, w.URN)
			continue
		}
		logging.V(9).Infof("AnnotationStore.ApplyPolicyWrite: urn=%s key=%s", w.URN, w.Key)
		dst, ok := s.annotations[w.URN]
		if !ok {
			dst = make(map[string]resource.PropertyMap)
			s.annotations[w.URN] = dst
		}
		dst[w.Key] = w.Data
		s.pending = append(s.pending, w)
	}
}

// PendingWrites returns a copy of the pending annotation writes.
func (s *AnnotationStore) PendingWrites() []plugin.AnalyzeAnnotationChange {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.pending) == 0 {
		return nil
	}
	result := make([]plugin.AnalyzeAnnotationChange, len(s.pending))
	copy(result, s.pending)
	return result
}
