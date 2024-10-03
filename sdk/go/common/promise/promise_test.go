// Copyright 2016-2024, Pulumi Corporation.
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

package promise

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZeroPanic(t *testing.T) {
	t.Parallel()

	var p Promise[int]
	assert.PanicsWithValue(t, "Promise must be initialized", func() {
		p.Result(context.Background()) //nolint:errcheck // Result is expected to panic
	})

	assert.PanicsWithValue(t, "Promise must be initialized", func() {
		p.TryResult() //nolint:errcheck // TryResult is expected to panic
	})
}

func TestFulfill(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	set := ps.Fulfill(42)
	assert.True(t, set, "set should be true")
	promise := ps.Promise()
	i, err := promise.Result(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, i)
	// Trying to fulfill again should fail
	set = ps.Fulfill(43)
	assert.False(t, set, "set should be false")
	// Asking for the promise again should give the same promise
	assert.Equal(t, promise, ps.Promise())
	// Result should still be 42
	i, err = promise.Result(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, i)
	// Trying to reject should fail
	set = ps.Reject(errors.New("boom"))
	assert.False(t, set, "set should be false")
}

func TestMustFulfill(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	// First call should succeed
	ps.MustFulfill(42)
	// And this promise should now resolve to 42
	i, err := ps.Promise().Result(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 42, i)
	// Second call should panic
	assert.PanicsWithValue(t, "CompletionSource already resolved", func() {
		ps.MustFulfill(43)
	})
}

func TestReject(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	boom := errors.New("boom")
	set := ps.Reject(boom)
	assert.True(t, set, "set should be true")
	promise := ps.Promise()
	_, err := promise.Result(context.Background())
	require.Equal(t, boom, err)
	// Trying to reject again should fail
	set = ps.Reject(errors.New("bigger boom"))
	assert.False(t, set, "set should be false")
	// Asking for the promise again should give the same promise
	assert.Equal(t, promise, ps.Promise())
	// Result should still be boom
	promise = ps.Promise()
	_, err = promise.Result(context.Background())
	require.Equal(t, boom, err)
	// Trying to fulfill should fail
	set = ps.Fulfill(42)
	assert.False(t, set, "set should be false")
}

func TestMustReject(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	// First call should succeed
	boom := errors.New("boom")
	ps.MustReject(boom)
	// And this promise should now resolve to boom
	_, err := ps.Promise().Result(context.Background())
	require.Equal(t, boom, err)
	// Second call should panic
	assert.PanicsWithValue(t, "CompletionSource already resolved", func() {
		ps.MustReject(errors.New("boom again"))
	})
}

func TestManyGets(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			got, err := ps.Promise().Result(ctx)
			assert.NoError(t, err)
			assert.Equal(t, 42, got)
		}()
	}

	ps.Fulfill(42)
	wg.Wait()
}

func TestAwaitCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	ps := &CompletionSource[int]{}
	p := ps.Promise()

	done := make(chan struct{})
	go func() {
		defer close(done)

		_, err := p.Result(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	}()

	cancel()
	<-done

	// The await was cancelled, not the promise so we should be able to fulfill and then wait again.
	set := ps.Fulfill(12)
	assert.True(t, set, "set should be true")

	i, err := p.Result(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 12, i)
}

func TestTryResult(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	p := ps.Promise()

	// TryResult should return false if the promise is not yet resolved.
	_, _, ok := p.TryResult()
	assert.False(t, ok)

	// Fulfilling the promise should return true.
	set := ps.Fulfill(42)
	assert.True(t, set, "set should be true")

	// TryResult should now return true and the result.
	i, err, ok := p.TryResult()
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, 42, i)
}

func TestTryResultRace(t *testing.T) {
	t.Parallel()

	ps := &CompletionSource[int]{}
	p := ps.Promise()

	// Start a promise that will keep trying to get the result of p.
	result := Run(func() (int, error) {
		for {
			i, err, ok := p.TryResult()
			if ok {
				return i, err
			}
		}
	})

	// Set the result of p, setting should return true.
	go func() {
		set := ps.Fulfill(42)
		assert.True(t, set, "set should be true")
	}()

	// Wait for the result promise.
	i, err := result.Result(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 42, i)
}

func TestRunError(t *testing.T) {
	t.Parallel()

	// Run should return a promise that is rejected if the function returns an error.
	result := Run(func() (int, error) {
		return 0, errors.New("boom")
	})

	_, err := result.Result(context.Background())
	assert.Error(t, err)
}
