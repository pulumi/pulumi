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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestRegistrationObserver_ResolveBeforeGet(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	b.Resolve(urn, "", outputs)

	got, err := b.Get(urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}

func TestRegistrationObserver_GetBeforeResolve(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	p := b.Get(urn)

	// Resolve from another goroutine; Result should unblock.
	go b.Resolve(urn, "", outputs)

	got, err := p.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}

// TestRegistrationObserver_ResolvePropagatesID pins the contract that the provider-assigned ID supplied to
// Resolve round-trips through Get's URNRegistration. The other tests use "" for ID since they're
// about other behavior (sticky rejection, expected URNs, etc.).
func TestRegistrationObserver_ResolvePropagatesID(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	id := resource.ID("res-id-42")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	b.Resolve(urn, id, outputs)

	got, err := b.Get(urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
	require.Equal(t, outputs, got.Outputs)
}

func TestRegistrationObserver_GetReturnsSamePromise(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	p1 := b.Get(urn)
	p2 := b.Get(urn)
	require.Same(t, p1, p2, "repeated Gets for the same URN should return the same promise")
}

func TestRegistrationObserver_FirstResolveWins(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	first := resource.PropertyMap{"k": resource.NewProperty("first")}
	second := resource.PropertyMap{"k": resource.NewProperty("second")}

	b.Resolve(urn, "", first)
	b.Resolve(urn, "", second)

	got, err := b.Get(urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, first, got.Outputs, "the first Resolve should win; later ones are no-ops")
}

func TestRegistrationObserver_Reject(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	rejErr := errors.New("nope")

	p := b.Get(urn)
	b.Reject(urn, rejErr)

	_, err := p.Result(t.Context())
	require.ErrorIs(t, err, rejErr)
}

// TestRegistrationObserver_ConcurrentGetsAndResolves stresses the broker with many goroutines all racing to
// Get and Resolve the same set of URNs, asserting every Get observes the resolved value.
func TestRegistrationObserver_ConcurrentGetsAndResolves(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	const urns = 50
	const gettersPerURN = 10

	all := make([]resource.URN, urns)
	want := make(map[resource.URN]resource.PropertyMap, urns)
	for i := range urns {
		urn := resource.URN("urn:pulumi:s::p::pkg:typ::" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		all[i] = urn
		want[urn] = resource.PropertyMap{"i": resource.NewProperty(float64(i))}
	}

	var wg sync.WaitGroup
	ctx := t.Context()

	for _, urn := range all {
		for range gettersPerURN {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := b.Get(urn).Result(ctx)
				require.NoError(t, err)
				require.Equal(t, want[urn], got.Outputs)
			}()
		}
	}

	// Resolve every URN from a separate goroutine.
	for _, urn := range all {
		go b.Resolve(urn, "", want[urn])
	}

	wg.Wait()
}

func TestRegistrationObserver_RejectUnresolvedRejectsPendingNonExpected(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	pending := resource.URN("urn:pulumi:s::p::pkg:typ::pending")
	expected := resource.URN("urn:pulumi:s::p::pkg:typ::expected")
	resolved := resource.URN("urn:pulumi:s::p::pkg:typ::resolved")

	pendingP := b.Get(pending)
	expectedP := b.Get(expected)
	b.MarkExpected(expected)

	resolvedOutputs := resource.PropertyMap{"k": resource.NewProperty("v")}
	b.Resolve(resolved, "", resolvedOutputs)
	resolvedP := b.Get(resolved)

	rejErr := errors.New("nobody registered this")
	b.RejectUnresolved(rejErr)

	// Pending non-Expected: rejected.
	_, err := pendingP.Result(t.Context())
	require.ErrorIs(t, err, rejErr)

	// Expected: untouched.
	if _, _, ok := expectedP.TryResult(); ok {
		t.Fatal("Expected URN should not have been resolved or rejected by RejectUnresolved")
	}

	// Already resolved: untouched.
	got, err := resolvedP.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, resolvedOutputs, got.Outputs)
}

func TestRegistrationObserver_RejectUnresolvedIsIdempotent(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	p := b.Get(urn)

	first := errors.New("first rejection")
	second := errors.New("second rejection")

	b.RejectUnresolved(first)
	b.RejectUnresolved(second)

	_, err := p.Result(t.Context())
	require.ErrorIs(t, err, first, "first rejection should win; second is a no-op")
}

func TestRegistrationObserver_MarkExpectedAfterGetStillProtects(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	p := b.Get(urn)
	b.MarkExpected(urn)

	b.RejectUnresolved(errors.New("sweep"))

	// Still pending — MarkExpected after Get still protected the entry.
	if _, _, ok := p.TryResult(); ok {
		t.Fatal("Expected URN should not have been rejected")
	}

	// And a later Resolve should still work.
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}
	b.Resolve(urn, "", outputs)
	got, err := p.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}

// TestRegistrationObserver_GetAfterRejectUnresolvedIsStickyRejected guards a race where a slow source calls Get
// after the sweep. Without stickiness it would create a fresh pending entry and hang.
func TestRegistrationObserver_GetAfterRejectUnresolvedIsStickyRejected(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	expected := resource.URN("urn:pulumi:s::p::pkg:typ::expected")
	other := resource.URN("urn:pulumi:s::p::pkg:typ::other")
	b.MarkExpected(expected)

	rejErr := errors.New("sweep")
	b.RejectUnresolved(rejErr)

	// A late Get for a non-Expected URN returns an already-rejected promise.
	_, err := b.Get(other).Result(t.Context())
	require.ErrorIs(t, err, rejErr)

	// A late Get for an Expected URN is still allowed to wait; a subsequent Resolve still works.
	expectedP := b.Get(expected)
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}
	b.Resolve(expected, "", outputs)
	got, err := expectedP.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}

// TestRegistrationObserver_ResolveDoesNotBlock checks that Resolve does not block even if no one has Got the
// URN yet; the resolved outputs should be observable by later Gets.
func TestRegistrationObserver_ResolveDoesNotBlock(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Resolve(urn, "", outputs)
	}()

	select {
	case <-done:
	case <-t.Context().Done():
		t.Fatal("Resolve should not block")
	}

	got, err := b.Get(urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}
