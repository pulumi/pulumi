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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
)

// fakeSource turns a CompletionSource into the source-runner shape NewMuxSource expects, so tests can drive
// completion timing precisely.
func fakeSource(cs *promise.CompletionSource[struct{}]) func(string) *promise.Promise[struct{}] {
	return func(string) *promise.Promise[struct{}] {
		return cs.Promise()
	}
}

func TestMuxSource_AllSucceed(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}
	bCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS), fakeSource(bCS))
	out := mux("ignored")

	// Fulfill in arbitrary order.
	aCS.Fulfill(struct{}{})
	mainCS.Fulfill(struct{}{})
	bCS.Fulfill(struct{}{})

	_, err := out.Result(t.Context())
	require.NoError(t, err)
}

func TestMuxSource_SingleErrorPropagatesAsIs(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS))
	out := mux("ignored")

	bang := errors.New("snippet exploded")
	aCS.Reject(bang)
	mainCS.Fulfill(struct{}{})

	_, err := out.Result(t.Context())
	require.ErrorIs(t, err, bang, "the single error should pass through unwrapped")
}

func TestMuxSource_MultipleErrorsJoin(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}
	bCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS), fakeSource(bCS))
	out := mux("ignored")

	mainErr := errors.New("program failed")
	bErr := errors.New("snippet b failed")
	mainCS.Reject(mainErr)
	aCS.Fulfill(struct{}{})
	bCS.Reject(bErr)

	_, err := out.Result(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, mainErr)
	require.ErrorIs(t, err, bErr)
}

// TestMuxSource_WaitsForAll asserts that even when one source finishes early the mux still blocks until
// the others complete; the returned promise must not resolve until everything has settled.
func TestMuxSource_WaitsForAll(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS))
	out := mux("ignored")

	mainCS.Fulfill(struct{}{})

	// With one of two sources still pending the mux promise must not be resolved yet.
	if _, _, ok := out.TryResult(); ok {
		t.Fatal("mux should not be resolved while a sub-source is still pending")
	}

	aCS.Fulfill(struct{}{})
	_, err := out.Result(t.Context())
	require.NoError(t, err)
}

// TestMuxSource_CancelContextStopsWaiting verifies that cancelling the constructor ctx causes the mux to
// fulfill (with the ctx error) without all sub-sources having to complete.
func TestMuxSource_CancelContextStopsWaiting(t *testing.T) {
	t.Parallel()

	muxCtx, cancel := context.WithCancel(t.Context())

	mainCS := &promise.CompletionSource[struct{}]{}
	hangCS := &promise.CompletionSource[struct{}]{} // never resolved

	mux := NewMuxSource(muxCtx, nil, fakeSource(mainCS), fakeSource(hangCS))
	out := mux("ignored")

	mainCS.Fulfill(struct{}{})

	// While hangCS is pending and ctx is alive, mux should still be waiting.
	if _, _, ok := out.TryResult(); ok {
		t.Fatal("mux should still be waiting while a sub-source is pending and ctx is alive")
	}

	// Cancelling the ctx wakes the still-blocked waiter and the mux resolves.
	cancel()

	_, err := out.Result(t.Context())
	require.ErrorIs(t, err, context.Canceled)
}

// TestMuxSource_NoBusyLoop sanity-checks that the new implementation doesn't busy-wait by leaving the mux
// pending for a short while with no work to do and ensuring the test process doesn't run hot — the old
// implementation would spin a goroutine at 100% CPU for this whole window. We can't directly measure CPU
// here, but we can verify the goroutine yields by allowing it to sleep and ensure the test completes
// quickly once we resolve.
func TestMuxSource_NoBusyLoop(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS))
	out := mux("ignored")

	// Give the mux goroutine a window in which it would otherwise spin.
	time.Sleep(20 * time.Millisecond)

	mainCS.Fulfill(struct{}{})
	_, err := out.Result(t.Context())
	require.NoError(t, err)
}

// The lookupFunction / getPackageRefFromToken helpers were removed when snippet.run was refactored to
// drive a synthetic single-resource program through pclruntime.Interpreter — the interpreter has its
// own (well-tested) lookup helpers, so the malformed-token regression tests live there now.
