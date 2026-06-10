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
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// URNRegistration carries the information a consumer needs to reason about a resource that has been
// registered: its provider-assigned ID and its output property map. The URN itself isn't stored
// here because callers look the registration up by URN via [URNBroker.Get]. ID may be empty for
// resources that don't have provider IDs (e.g. components).
type URNRegistration struct {
	ID      resource.ID
	Outputs resource.PropertyMap
}

// URNBroker provides per-URN promises that fulfill when a resource has been registered. It lets
// multiple Sources running concurrently against the same engine update wait for one another's
// RegisterResource calls without knowing in advance who will produce a given URN.
//
// The intended pattern:
//
//   - Anyone that needs the outputs of a resource by URN calls Get(urn) to receive a promise, then
//     blocks on it via Result(ctx).
//   - The resource monitor calls Resolve(urn, id, outputs) after each successful registration. The
//     first Resolve for a URN wins; subsequent calls are no-ops.
//   - URNs that some still-running source intends to register are pre-declared via MarkExpected.
//     When the engine knows no more registrations are coming from the program (and only Expected
//     URNs might still arrive from snippets), it calls RejectUnresolved to surface "no source
//     registered this URN" failures for any pending entry that wasn't pre-declared.
type URNBroker struct {
	mu       sync.Mutex
	promises map[resource.URN]*promise.CompletionSource[URNRegistration]
	expected map[resource.URN]struct{}
	// closedErr, if non-nil, indicates RejectUnresolved has been called: any subsequent Get for a URN that
	// wasn't marked Expected returns an already-rejected promise. Without this flag a slow source that calls
	// Get *after* the sweep would create a fresh pending entry that no one will ever resolve.
	closedErr error
}

// NewURNBroker returns a broker with no pending or resolved entries.
func NewURNBroker() *URNBroker {
	return &URNBroker{
		promises: map[resource.URN]*promise.CompletionSource[URNRegistration]{},
		expected: map[resource.URN]struct{}{},
	}
}

// MarkExpected records that some still-running source intends to Resolve urn. Subsequent calls to
// RejectUnresolved will leave Expected URNs alone (they're still in flight, not dead). Repeated calls
// for the same URN are no-ops.
func (b *URNBroker) MarkExpected(urn resource.URN) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expected[urn] = struct{}{}
}

// RejectUnresolved rejects every currently-pending promise that has not been marked as Expected, and
// causes any subsequent [URNBroker.Get] for a non-Expected URN to return an already-rejected promise.
// Used once the engine knows no further registrations are coming from sources that didn't pre-declare
// their URNs (typically called after the main program's promise resolves). Repeated calls are no-ops:
// the first err is the one observed by Get callers.
func (b *URNBroker) RejectUnresolved(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closedErr != nil {
		return
	}
	b.closedErr = err
	for urn, cs := range b.promises {
		if _, ok := b.expected[urn]; ok {
			continue
		}
		cs.Reject(err)
	}
}

// entry returns the CompletionSource for urn, creating it if absent. Caller must hold b.mu.
func (b *URNBroker) entry(urn resource.URN) *promise.CompletionSource[URNRegistration] {
	cs, ok := b.promises[urn]
	if !ok {
		cs = &promise.CompletionSource[URNRegistration]{}
		b.promises[urn] = cs
	}
	return cs
}

// Get returns a promise that will be fulfilled with the registration (id + outputs) of the resource
// registered at the given URN. Subsequent Gets for the same URN return the same promise. Safe to
// call from any goroutine.
//
// If [URNBroker.RejectUnresolved] has already been called, a Get for a URN that has not been marked
// Expected returns a promise that is already rejected — this closes a race where a slow source might
// otherwise call Get after the sweep and sit forever on a freshly-created pending entry.
func (b *URNBroker) Get(urn resource.URN) *promise.Promise[URNRegistration] {
	b.mu.Lock()
	defer b.mu.Unlock()
	cs := b.entry(urn)
	if b.closedErr != nil {
		if _, ok := b.expected[urn]; !ok {
			cs.Reject(b.closedErr)
		}
	}
	return cs.Promise()
}

// Resolve fulfills the promise for the given URN with the supplied id and outputs. id may be empty
// for resources that don't have provider IDs (e.g. components). Repeated Resolve calls for the
// same URN are no-ops.
func (b *URNBroker) Resolve(urn resource.URN, id resource.ID, outputs resource.PropertyMap) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entry(urn).Fulfill(URNRegistration{ID: id, Outputs: outputs})
}

// Reject fails the promise for the given URN. Useful for shutdown or to signal that a referenced
// URN will never be registered. Repeated Rejects/Resolves for the same URN are no-ops.
func (b *URNBroker) Reject(urn resource.URN, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entry(urn).Reject(err)
}
