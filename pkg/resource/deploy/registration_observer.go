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
	"errors"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// URNRegistration carries the information a consumer needs to reason about a resource that has been
// registered: its provider-assigned ID and its output property map. The URN itself isn't stored
// here because callers wait for the registration by URN. ID may be empty for resources that don't
// have provider IDs (e.g. components).
type URNRegistration struct {
	ID      resource.ID
	Outputs resource.PropertyMap
}

// RegistrationObserver provides per-URN promises that fulfill when a resource has been registered.
// It lets multiple Sources running concurrently against the same engine update wait for one
// another's RegisterResource calls without knowing in advance who will produce a given URN.
//
// The intended pattern:
//
//   - Sources that need the outputs of a resource by URN call Wait(urn) to receive a promise, then
//     block on it via Result(ctx).
//   - The resource monitor calls Resolve(urn, id, outputs) after each successful registration. The
//     first Resolve for a URN wins; subsequent calls are no-ops.
//   - Each source registers itself with NewSource before execution and calls Done when it exits.
//   - If every remaining source is waiting, no source can make progress and unresolved references
//     are rejected. This detects missing references and dependency cycles.
type RegistrationObserver struct {
	mu           sync.Mutex
	promises     map[resource.URN]*promise.CompletionSource[URNRegistration]
	sources      map[*RegistrationObserverSource]resource.URN
	sourcesReady bool
	closedErr    error
}

// NewRegistrationObserver returns an observer with no pending or resolved entries.
func NewRegistrationObserver() *RegistrationObserver {
	return &RegistrationObserver{
		promises: map[resource.URN]*promise.CompletionSource[URNRegistration]{},
		sources:  map[*RegistrationObserverSource]resource.URN{},
	}
}

// RegistrationObserverSource tracks whether one source can still register resources or is blocked
// waiting for another source to register a referenced resource.
type RegistrationObserverSource struct {
	observer *RegistrationObserver
	once     sync.Once
}

// NewSource registers a source with the observer. All sources must be registered before SourcesReady
// is called and execution begins.
func (o *RegistrationObserver) NewSource() *RegistrationObserverSource {
	o.mu.Lock()
	defer o.mu.Unlock()
	source := &RegistrationObserverSource{observer: o}
	o.sources[source] = ""
	return source
}

// SourcesReady enables quiescence detection after all sources have been registered.
func (o *RegistrationObserver) SourcesReady() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sourcesReady = true
	o.rejectIfQuiescent()
}

// Wait returns a promise for urn and marks this source as blocked until that URN is resolved or
// rejected. Settling the URN makes the source runnable while holding the observer lock, before any
// other source can finish and trigger quiescence detection.
func (s *RegistrationObserverSource) Wait(urn resource.URN) *promise.Promise[URNRegistration] {
	s.observer.mu.Lock()
	defer s.observer.mu.Unlock()
	cs := s.observer.entry(urn)
	if s.observer.closedErr != nil {
		cs.Reject(s.observer.closedErr)
		return cs.Promise()
	}
	if _, _, settled := cs.Promise().TryResult(); !settled {
		if _, ok := s.observer.sources[s]; ok {
			s.observer.sources[s] = urn
			s.observer.rejectIfQuiescent()
		}
	}
	return cs.Promise()
}

// Done removes this source from the active source set.
func (s *RegistrationObserverSource) Done() {
	s.once.Do(func() {
		s.observer.mu.Lock()
		defer s.observer.mu.Unlock()
		delete(s.observer.sources, s)
		s.observer.rejectIfQuiescent()
	})
}

// rejectIfQuiescent rejects unresolved references when no source can make progress.
// Caller must hold o.mu.
func (o *RegistrationObserver) rejectIfQuiescent() {
	if !o.sourcesReady || o.closedErr != nil {
		return
	}
	for _, waitingFor := range o.sources {
		if waitingFor == "" {
			return
		}
	}
	o.closedErr = errors.New("no source registered this URN")
	for _, cs := range o.promises {
		cs.Reject(o.closedErr)
	}
}

// entry returns the CompletionSource for urn, creating it if absent. Caller must hold o.mu.
func (o *RegistrationObserver) entry(urn resource.URN) *promise.CompletionSource[URNRegistration] {
	cs, ok := o.promises[urn]
	if !ok {
		cs = &promise.CompletionSource[URNRegistration]{}
		o.promises[urn] = cs
	}
	return cs
}

// Resolve fulfills the promise for the given URN with the supplied id and outputs. id may be empty
// for resources that don't have provider IDs (e.g. components). Repeated Resolve calls for the
// same URN are no-ops.
func (o *RegistrationObserver) Resolve(urn resource.URN, id resource.ID, outputs resource.PropertyMap) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.makeWaitersRunnable(urn)
	o.entry(urn).Fulfill(URNRegistration{ID: id, Outputs: outputs})
}

// Reject fails the promise for the given URN. Useful for shutdown or to signal that a referenced
// URN will never be registered. Repeated Rejects/Resolves for the same URN are no-ops.
func (o *RegistrationObserver) Reject(urn resource.URN, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.makeWaitersRunnable(urn)
	o.entry(urn).Reject(err)
}

// makeWaitersRunnable marks sources waiting for urn as able to register resources again.
// Caller must hold o.mu.
func (o *RegistrationObserver) makeWaitersRunnable(urn resource.URN) {
	for source, waitingFor := range o.sources {
		if waitingFor == urn {
			o.sources[source] = ""
		}
	}
}
