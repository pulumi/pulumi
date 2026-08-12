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

// Package resourcetracker records the resources an operation produces, as it produces them, so that a resource monitor
// can answer "has everything beneath these URNs been created?" without scanning every resource the operation has
// touched.
package resourcetracker

import (
	"sync"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type entry struct {
	custom bool // the tracked resource is custom
	absent bool // if a custom resource is not available
}

// Tracker answers whether everything beneath a set of URNs has been created.
//
// The zero value is ready to use.
type Tracker struct {
	mu      sync.Mutex
	tracked *sync.Cond

	byURN    map[resource.URN]entry
	children map[resource.URN][]resource.URN // parent URN to the URNs registered beneath it
	inFlight map[resource.URN]int            // parent URN to the number of registrations in flight beneath it
}

// init prepares the tracker's maps. Called under mu, so that the zero value is usable.
func (t *Tracker) init() {
	if t.byURN == nil {
		t.byURN = map[resource.URN]entry{}
		t.children = map[resource.URN][]resource.URN{}
		t.inFlight = map[resource.URN]int{}
		t.tracked = sync.NewCond(&t.mu)
	}
}

// MarkInFlight records a registration in flight beneath parent, returning the function that records its completion.
//
// Failing to call the returned function may deadlock the engine.
func (t *Tracker) MarkInFlight(parent resource.URN) func() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.init()
	t.inFlight[parent]++

	return func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.inFlight[parent]--; t.inFlight[parent] == 0 {
			delete(t.inFlight, parent)
		}
		t.tracked.Broadcast()
	}
}

// Track records a resource an operation has produced. created distinguishes a resource that exists from one whose
// creation is still outstanding; it is only consulted for a custom resource, since a component has nothing to create.
func (t *Tracker) Track(urn, parent resource.URN, custom, created bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.init()

	if _, retracked := t.byURN[urn]; !retracked {
		t.children[parent] = append(t.children[parent], urn)
	}
	t.byURN[urn] = entry{custom: custom, absent: custom && !created}
	t.tracked.Broadcast()
}

// HasUnresolved reports whether any custom resource in the expansion of roots has not been created.
//
// The expansion mirrors the rule the SDKs apply client-side: a custom resource contributes only itself, and a component
// aggregates every descendant reachable through component ancestors, stopping at the first custom resource on each
// branch.
//
// Registrations in flight beneath a component in that expansion are waited for.
func (t *Tracker) HasUnresolved(roots mapset.Set[resource.URN]) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.init()

	for {
		unresolved, components := t.expand(roots)
		if unresolved {
			return true
		}
		// The expansion is a snapshot: a registration that completes during the wait can extend it, so recompute
		// rather than waiting on the components reached this time around.
		waitingOn, waiting := t.inFlightUnder(components)
		if !waiting {
			return false
		}
		logging.V(5).Infof("resourcetracker: waiting on a registration beneath %v", waitingOn)
		t.tracked.Wait()
	}
}

// expand walks down from roots, reporting whether it reached a custom resource that has not been created, and
// returning the components it reached. Untracked URNs are skipped: they are not resources this operation produced, so
// the caller could not see them either. Called under mu.
func (t *Tracker) expand(roots mapset.Set[resource.URN]) (bool, mapset.Set[resource.URN]) {
	components := mapset.NewThreadUnsafeSet[resource.URN]()

	frontier := roots.ToSlice()
	for len(frontier) > 0 {
		urn := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]

		res, ok := t.byURN[urn]
		if !ok {
			logging.V(5).Infof("resourcetracker: ignoring unknown URN %v", urn)
			continue
		}
		if res.custom {
			if res.absent {
				return true, nil
			}
			continue
		}
		// Add reports whether the URN was new, which keeps a cycle in the parent graph from looping forever.
		if components.Add(urn) {
			frontier = append(frontier, t.children[urn]...)
		}
	}
	return false, components
}

// inFlightUnder returns a component with a registration in flight beneath it, if there is one. Called under mu.
func (t *Tracker) inFlightUnder(components mapset.Set[resource.URN]) (resource.URN, bool) {
	for parent := range t.inFlight {
		if components.Contains(parent) {
			return parent, true
		}
	}
	return "", false
}
