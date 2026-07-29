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

package resourcetracker

import (
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

const (
	comp  = resource.URN("urn:pulumi:dev::proj::pkgA:m:typComponent::comp")
	child = resource.URN("urn:pulumi:dev::proj::pkgA:m:typComponent$pkgA:m:typChild::child")
	inner = resource.URN("urn:pulumi:dev::proj::pkgA:m:typComponent$pkgA:m:typInner::inner")
)

func urnSet(urns ...resource.URN) mapset.Set[resource.URN] {
	return mapset.NewThreadUnsafeSet(urns...)
}

func TestHasUnresolved(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		track      func(t *Tracker)
		roots      mapset.Set[resource.URN]
		unresolved bool
	}{
		{
			name:       "unknown root is skipped",
			track:      func(*Tracker) {},
			roots:      urnSet(comp),
			unresolved: false,
		},
		{
			name: "created custom root",
			track: func(tr *Tracker) {
				tr.Track(child, comp, true, true)
			},
			roots:      urnSet(child),
			unresolved: false,
		},
		{
			name: "unresolved custom root",
			track: func(tr *Tracker) {
				tr.Track(child, comp, true, false)
			},
			roots:      urnSet(child),
			unresolved: true,
		},
		{
			name: "component aggregates an unresolved child",
			track: func(tr *Tracker) {
				tr.Track(child, comp, true, false)
				tr.Track(comp, "", false, false)
			},
			roots:      urnSet(comp),
			unresolved: true,
		},
		{
			name: "component aggregates a created child",
			track: func(tr *Tracker) {
				tr.Track(child, comp, true, true)
				tr.Track(comp, "", false, false)
			},
			roots:      urnSet(comp),
			unresolved: false,
		},
		{
			name: "expansion descends through nested components",
			track: func(tr *Tracker) {
				tr.Track(child, inner, true, false)
				tr.Track(inner, comp, false, false)
				tr.Track(comp, "", false, false)
			},
			roots:      urnSet(comp),
			unresolved: true,
		},
		{
			name: "expansion stops at a custom resource",
			track: func(tr *Tracker) {
				// A custom resource contributes only itself, so an uncreated resource parented to one is
				// not part of the root's expansion.
				tr.Track(child, inner, true, false)
				tr.Track(inner, comp, true, true)
				tr.Track(comp, "", false, false)
			},
			roots:      urnSet(comp),
			unresolved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var tracker Tracker
			tt.track(&tracker)
			assert.Equal(t, tt.unresolved, tracker.HasUnresolved(tt.roots))
		})
	}
}

// TestHasUnresolvedAwaitsRegistration covers a component provider that returns from Construct without awaiting the
// children it registered. Those registrations are still in flight and so have produced nothing, which a tracker that
// only read what it had been told about would report as "no such resource" -- the very ordering bug callers use this
// package to avoid. Nothing in the protocol requires a provider to await its children, so the tracker waits for the
// registrations instead of assuming it.
func TestHasUnresolvedAwaitsRegistration(t *testing.T) {
	t.Parallel()

	var tracker Tracker
	tracker.Track(comp, "", false, false)
	registered := tracker.MarkInFlight(comp)

	gated := make(chan bool, 1)
	go func() { gated <- tracker.HasUnresolved(urnSet(comp)) }()

	select {
	case <-gated:
		t.Fatal("the tracker answered while a registration beneath the root was still in flight")
	case <-time.After(100 * time.Millisecond):
	}

	// Completing the registration publishes the child with no id, as a create does during a preview.
	tracker.Track(child, comp, true, false)
	registered()

	assert.True(t, <-gated, "the child was not created, so the root has an unresolved descendant")
}

// TestHasUnresolvedIgnoresUnrelatedRegistrations pins that the wait is scoped to the expansion: a registration in
// flight somewhere else in the tree must not hold the caller up.
func TestHasUnresolvedIgnoresUnrelatedRegistrations(t *testing.T) {
	t.Parallel()

	other := resource.URN("urn:pulumi:dev::proj::pkgA:m:typOther::other")

	var tracker Tracker
	tracker.Track(comp, "", false, false)
	tracker.Track(other, "", false, false)
	defer tracker.MarkInFlight(other)()

	assert.False(t, tracker.HasUnresolved(urnSet(comp)))
}
