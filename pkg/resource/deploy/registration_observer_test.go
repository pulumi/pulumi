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

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// observeRegistration returns a promise for urn without affecting source liveness. Tests use this
// to inspect observer publication independently of source waiting behavior.
func observeRegistration(o *RegistrationObserver, urn resource.URN) *promise.Promise[URNRegistration] {
	o.mu.Lock()
	defer o.mu.Unlock()
	cs := o.entry(urn)
	if o.closedErr != nil {
		cs.Reject(o.closedErr)
	}
	return cs.Promise()
}

func TestRegistrationObserver_ResolveBeforeObserve(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	b.Resolve(urn, "", outputs)

	got, err := observeRegistration(b, urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}

func TestRegistrationObserver_ObserveBeforeResolve(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	p := observeRegistration(b, urn)

	// Resolve from another goroutine; Result should unblock.
	go b.Resolve(urn, "", outputs)

	got, err := p.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}

// TestRegistrationObserver_ResolvePropagatesID pins the contract that the provider-assigned ID supplied to
// Resolve round-trips through URNRegistration. The other tests use "" for ID since they're
// about other behavior (sticky rejection, expected URNs, etc.).
func TestRegistrationObserver_ResolvePropagatesID(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	id := resource.ID("res-id-42")
	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}

	b.Resolve(urn, id, outputs)

	got, err := observeRegistration(b, urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, id, got.ID)
	require.Equal(t, outputs, got.Outputs)
}

func TestRegistrationObserver_ObserveReturnsSamePromise(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	p1 := observeRegistration(b, urn)
	p2 := observeRegistration(b, urn)
	require.Same(t, p1, p2, "repeated observations of the same URN should return the same promise")
}

func TestRegistrationObserver_FirstResolveWins(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	first := resource.PropertyMap{"k": resource.NewProperty("first")}
	second := resource.PropertyMap{"k": resource.NewProperty("second")}

	b.Resolve(urn, "", first)
	b.Resolve(urn, "", second)

	got, err := observeRegistration(b, urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, first, got.Outputs, "the first Resolve should win; later ones are no-ops")
}

func TestRegistrationObserver_Reject(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	rejErr := errors.New("nope")

	p := observeRegistration(b, urn)
	b.Reject(urn, rejErr)

	_, err := p.Result(t.Context())
	require.ErrorIs(t, err, rejErr)
}

// TestRegistrationObserver_ConcurrentObserversAndResolves stresses the observer with many goroutines
// all racing to observe and Resolve the same set of URNs.
func TestRegistrationObserver_ConcurrentObserversAndResolves(t *testing.T) {
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
			wg.Go(func() {
				got, err := observeRegistration(b, urn).Result(ctx)
				require.NoError(t, err)
				require.Equal(t, want[urn], got.Outputs)
			})
		}
	}

	// Resolve every URN from a separate goroutine.
	for _, urn := range all {
		go b.Resolve(urn, "", want[urn])
	}

	wg.Wait()
}

func TestRegistrationObserver_RunnableSourceProtectsWaiter(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	producer := b.NewSource()
	consumer := b.NewSource()
	b.SourcesReady()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	p := consumer.Wait(urn)

	_, _, ok := p.TryResult()
	require.False(t, ok, "a runnable producer should keep the reference pending")

	outputs := resource.PropertyMap{"k": resource.NewProperty("v")}
	b.Resolve(urn, "", outputs)
	got, err := p.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)

	consumer.Done()
	producer.Done()
}

func TestRegistrationObserver_WaitResolvedURNKeepsSourceRunnable(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	producer := b.NewSource()
	consumer := b.NewSource()
	b.SourcesReady()

	resolvedURN := resource.URN("urn:pulumi:s::p::pkg:typ::resolved")
	pendingURN := resource.URN("urn:pulumi:s::p::pkg:typ::pending")
	b.Resolve(resolvedURN, "", resource.PropertyMap{})
	_, err := consumer.Wait(resolvedURN).Result(t.Context())
	require.NoError(t, err)

	producer.Done()
	pending := observeRegistration(b, pendingURN)
	_, _, ok := pending.TryResult()
	require.False(t, ok, "waiting on a pending URN should leave the source blocked")

	b.Resolve(pendingURN, "", resource.PropertyMap{})
	_, err = pending.Result(t.Context())
	require.NoError(t, err)
	consumer.Done()
}

func TestRegistrationObserver_ProducerCanResolveAnyURN(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	producer := b.NewSource()
	consumer := b.NewSource()
	b.SourcesReady()

	// These are representative of URNs whose names or qualified types cannot be inferred from
	// Snippet.Name and Snippet.Type alone.
	parented := resource.URN("urn:pulumi:s::p::pkg:parent$pkg:typ::child")
	ranged := resource.URN("urn:pulumi:s::p::pkg:typ::child-0")
	parentedP := consumer.Wait(parented)

	b.Resolve(parented, "", resource.PropertyMap{"kind": resource.NewProperty("parented")})
	parentedResult, err := parentedP.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, "parented", parentedResult.Outputs["kind"].StringValue())

	rangedP := consumer.Wait(ranged)
	b.Resolve(ranged, "", resource.PropertyMap{"kind": resource.NewProperty("ranged")})
	rangedResult, err := rangedP.Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, "ranged", rangedResult.Outputs["kind"].StringValue())

	consumer.Done()
	producer.Done()
}

func TestRegistrationObserver_ResolvedWaiterBecomesRunnableProducer(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	producer := b.NewSource()
	middle := b.NewSource()
	consumer := b.NewSource()
	b.SourcesReady()

	middleDependency := resource.URN("urn:pulumi:s::p::pkg:typ::producer")
	consumerDependency := resource.URN("urn:pulumi:s::p::pkg:typ::middle")
	middleP := middle.Wait(middleDependency)
	consumerP := consumer.Wait(consumerDependency)

	b.Resolve(middleDependency, "", resource.PropertyMap{})
	// The producer may finish before the middle source's goroutine observes its resolved promise.
	// Resolve must make middle runnable atomically so this does not falsely reject consumerP.
	producer.Done()
	_, err := middleP.Result(t.Context())
	require.NoError(t, err)

	_, _, ok := consumerP.TryResult()
	require.False(t, ok, "the newly runnable middle source should keep its consumer pending")

	b.Resolve(consumerDependency, "", resource.PropertyMap{})
	_, err = consumerP.Result(t.Context())
	require.NoError(t, err)

	middle.Done()
	consumer.Done()
}

func TestRegistrationObserver_LastRunnableSourceDoneRejectsWaiter(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	producer := b.NewSource()
	consumer := b.NewSource()
	b.SourcesReady()

	urn := resource.URN("urn:pulumi:s::p::pkg:typ::a")
	p := consumer.Wait(urn)
	producer.Done()

	_, err := p.Result(t.Context())
	require.ErrorContains(t, err, "no source registered this URN")

	consumer.Done()
}

func TestRegistrationObserver_AllWaitingRejectsCycle(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	a := b.NewSource()
	c := b.NewSource()
	b.SourcesReady()

	aP := a.Wait(resource.URN("urn:pulumi:s::p::pkg:typ::a"))
	cP := c.Wait(resource.URN("urn:pulumi:s::p::pkg:typ::c"))

	_, err := aP.Result(t.Context())
	require.ErrorContains(t, err, "no source registered this URN")
	_, err = cP.Result(t.Context())
	require.ErrorContains(t, err, "no source registered this URN")

	a.Done()
	c.Done()
}

func TestRegistrationObserver_ObserveAfterQuiescenceIsStickyRejected(t *testing.T) {
	t.Parallel()
	b := NewRegistrationObserver()

	source := b.NewSource()
	b.SourcesReady()
	source.Done()

	_, err := observeRegistration(b, resource.URN("urn:pulumi:s::p::pkg:typ::other")).Result(t.Context())
	require.ErrorContains(t, err, "no source registered this URN")
}

// TestRegistrationObserver_ResolveDoesNotBlock checks that Resolve does not block even if no one is
// observing the URN yet; the resolved outputs should be observable later.
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

	got, err := observeRegistration(b, urn).Result(t.Context())
	require.NoError(t, err)
	require.Equal(t, outputs, got.Outputs)
}
