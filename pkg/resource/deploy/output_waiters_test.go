// Copyright 2016-2025, Pulumi Corporation.
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
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputWaiterStoreBasic(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/stack-a", "org/proj/stack-b"})

	// Set outputs first, then wait -- should return immediately.
	outputs := property.NewMap(map[string]property.Value{
		"url": property.New("https://example.com"),
	})
	store.SetOutputs("org/proj/stack-a", outputs)
	assert.NotPanics(t, func() { store.SetOutputs("org/proj/stack-a", outputs) },
		"stack root finalization publishes outputs again")

	got, err := store.WaitForOutputs(context.Background(), "org/proj/stack-b", "org/proj/stack-a")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", got.Get("url").AsString())
}

func TestOutputWaiterStoreBlocking(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/stack-a", "org/proj/stack-b"})

	outputs := property.NewMap(map[string]property.Value{
		"port": property.New(8080.0),
	})

	var wg sync.WaitGroup
	wg.Add(1)

	var got property.Map
	var waitErr error
	go func() {
		defer wg.Done()
		got, waitErr = store.WaitForOutputs(context.Background(), "org/proj/stack-b", "org/proj/stack-a")
	}()

	// Give the goroutine time to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Now set outputs to unblock the waiter.
	store.SetOutputs("org/proj/stack-a", outputs)
	wg.Wait()

	require.NoError(t, waitErr)
	assert.Equal(t, 8080.0, got.Get("port").AsNumber())
}

func TestOutputWaiterStoreCycleDetection(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/stack-a", "org/proj/stack-b"})

	// Start stack-a waiting on stack-b in the background.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = store.WaitForOutputs(context.Background(), "org/proj/stack-a", "org/proj/stack-b")
	}()

	// Give the goroutine time to register in the wait graph.
	time.Sleep(50 * time.Millisecond)

	// Now stack-b tries to wait on stack-a -- this should detect a cycle.
	_, err := store.WaitForOutputs(context.Background(), "org/proj/stack-b", "org/proj/stack-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")

	// Clean up: unblock stack-a's wait.
	store.SetOutputs("org/proj/stack-b", property.NewMap(nil))
	wg.Wait()
}

func TestOutputWaiterStoreContextCancellation(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/stack-a", "org/proj/stack-b"})

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	var waitErr error
	go func() {
		defer wg.Done()
		_, waitErr = store.WaitForOutputs(ctx, "org/proj/stack-b", "org/proj/stack-a")
	}()

	// Give the goroutine time to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Cancel the context.
	cancel()
	wg.Wait()

	require.Error(t, waitErr)
	assert.Contains(t, waitErr.Error(), "timed out waiting for outputs from co-deployed stack")
}

func TestOutputWaiterStoreNotCoDeployed(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/stack-a"})

	assert.True(t, store.IsCoDeployed("org/proj/stack-a"))
	assert.False(t, store.IsCoDeployed("org/proj/stack-unknown"))
}

func TestOutputWaiterStoreBackendFallback(t *testing.T) {
	t.Parallel()

	// When outputWaiters is set but the target stack is NOT co-deployed,
	// readStackReference should fall through to the backend client.
	store := NewOutputWaiterStore([]string{"org/proj/stack-a"})

	var backendCalled bool
	p := newBuiltinProvider(
		&deploytest.BackendClient{
			GetStackOutputsF: func(ctx context.Context, name string, _ func(error) error) (property.Map, error) {
				backendCalled = true
				return property.NewMap(map[string]property.Value{
					"backend-key": property.New("backend-value"),
				}), nil
			},
		},
		nil, nil,
		&deploytest.NoopSink{},
	)
	p.WithOutputWaiters(store, "org/proj/stack-a")

	// Request a stack that is NOT co-deployed -- should use the backend.
	result, err := p.readStackReference(context.Background(), property.NewMap(map[string]property.Value{
		"name": property.New("org/proj/stack-other"),
	}), "org/proj/stack-a")
	require.NoError(t, err)
	assert.True(t, backendCalled, "expected backend client to be called for non-co-deployed stack")
	assert.Equal(t, "backend-value",
		result.Get("outputs").AsMap().Get("backend-key").AsString())
}

func TestOutputWaiterStoreCoDeployedReadStackReference(t *testing.T) {
	t.Parallel()

	// When outputWaiters is set and the target stack IS co-deployed,
	// readStackReference should use the waiter and NOT call the backend.
	store := NewOutputWaiterStore([]string{"org/proj/stack-a", "org/proj/stack-b"})

	// Pre-set outputs for stack-b so we don't block.
	store.SetOutputs("org/proj/stack-b", property.NewMap(map[string]property.Value{
		"url":    property.New("https://example.com"),
		"secret": property.New("s3cret").WithSecret(true),
	}))

	var backendCalled bool
	p := newBuiltinProvider(
		&deploytest.BackendClient{
			GetStackOutputsF: func(ctx context.Context, name string, _ func(error) error) (property.Map, error) {
				backendCalled = true
				return property.Map{}, nil
			},
		},
		nil, nil,
		&deploytest.NoopSink{},
	)
	p.WithOutputWaiters(store, "org/proj/stack-a")

	result, err := p.readStackReference(context.Background(), property.NewMap(map[string]property.Value{
		"name": property.New("org/proj/stack-b"),
	}), "org/proj/stack-a")
	require.NoError(t, err)
	assert.False(t, backendCalled, "backend client should NOT be called for co-deployed stack")

	// Verify outputs are correctly translated.
	outputs := result.Get("outputs").AsMap()
	assert.Equal(t, "https://example.com", outputs.Get("url").AsString())

	// Verify secret output names are populated.
	secretNames := result.Get("secretOutputNames").AsArray()
	require.Equal(t, 1, secretNames.Len())
	assert.Equal(t, "secret", secretNames.Get(0).AsString())
}

// TestOutputWaiterStoreCoDeployedReadStackReferencePreservesUnknown proves an unknown (computed)
// co-deployed output survives the waiter path unresolved rather than being dropped or coerced to
// a zero value -- the shape a StackReference sees during preview of a producer resource that has
// not been created yet. A real not-yet-created resource requires a language host and provider
// backing an actual resource type, which the auto/driver.go end-to-end yaml tests in this repo
// don't have available (the yaml programs there only register static, always-known outputs), so
// this exercises the same readStackReference/WaitForOutputs code the constructor wiring uses,
// directly, the same way TestOutputWaiterStoreCoDeployedReadStackReference already does for the
// secret case.
func TestOutputWaiterStoreCoDeployedReadStackReferencePreservesUnknown(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/stack-a", "org/proj/stack-b"})
	store.SetOutputs("org/proj/stack-b", property.NewMap(map[string]property.Value{
		"pending": property.New(property.Computed),
	}))

	p := newBuiltinProvider(nil, nil, nil, &deploytest.NoopSink{})
	p.WithOutputWaiters(store, "org/proj/stack-a")

	result, err := p.readStackReference(context.Background(), property.NewMap(map[string]property.Value{
		"name": property.New("org/proj/stack-b"),
	}), "org/proj/stack-a")
	require.NoError(t, err)

	outputs := result.Get("outputs").AsMap()
	assert.True(t, outputs.Get("pending").IsComputed(),
		"an unknown co-deployed output must stay unknown, not be resolved to some other value")
}

// waitForEdgeCount polls the store's internal wait graph, under its own lock, until the edge
// from waiter to target has reached at least the given count, failing the test after a bounded
// timeout. This deterministically detects that a WaitForOutputs call has registered (or retired)
// its edge, instead of guessing with a fixed sleep and a scheduler-dependent race.
func waitForEdgeCount(t *testing.T, store *OutputWaiterStore, waiter, target string, count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		store.mu.Lock()
		n := store.waitGraph[waiter][target]
		store.mu.Unlock()
		if (count == 0 && n == 0) || (count > 0 && n >= count) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for wait edge %s->%s to reach count %d (have %d)", waiter, target, count, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestOutputWaiterStoreConcurrentWaitsFromOneStack proves that one stack can be blocked
// waiting on two other co-deployed stacks at the same time without the two wait edges
// clobbering each other in the wait graph. Before the fix, waitGraph stored a single
// waiter->target string per waiter, so the second concurrent wait overwrote the first
// edge; cleaning up either wait then deleted the other's still-pending edge, corrupting
// cycle detection for any wait started afterwards.
func TestOutputWaiterStoreConcurrentWaitsFromOneStack(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/a", "org/proj/b", "org/proj/c"})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	results := make([]property.Map, 2)

	// Stack "a" waits on both "b" and "c" concurrently.
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0], errs[0] = store.WaitForOutputs(context.Background(), "org/proj/a", "org/proj/b")
	}()
	go func() {
		defer wg.Done()
		results[1], errs[1] = store.WaitForOutputs(context.Background(), "org/proj/a", "org/proj/c")
	}()

	// Deterministically wait for both edges to register instead of guessing with a sleep.
	waitForEdgeCount(t, store, "org/proj/a", "org/proj/b", 1)
	waitForEdgeCount(t, store, "org/proj/a", "org/proj/c", 1)

	// While both waits are in flight, a genuine cycle must still be detected: "b" waiting on
	// "a" would close the loop a -> b -> a in the (still corrupted, pre-fix) single-edge graph.
	_, cycleErr := store.WaitForOutputs(context.Background(), "org/proj/b", "org/proj/a")
	require.Error(t, cycleErr)
	assert.Contains(t, cycleErr.Error(), "circular dependency detected")

	store.SetOutputs("org/proj/b", property.NewMap(map[string]property.Value{"from": property.New("b")}))
	store.SetOutputs("org/proj/c", property.NewMap(map[string]property.Value{"from": property.New("c")}))
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, "b", results[0].Get("from").AsString())
	assert.Equal(t, "c", results[1].Get("from").AsString())
}

// TestOutputWaiterStoreDuplicateWaitRefCounted proves that when two concurrent waits from the
// same waiter to the same target are in flight, retiring ONE of them (by cancellation here) does
// not remove the wait edge while the other is still active. Before the reference-count fix, the
// edge was a single boolean per (waiter, target) pair, so the first wait to exit deleted it
// outright -- letting a later target->waiter wait evade checkCycle entirely and deadlock instead
// of failing fast.
func TestOutputWaiterStoreDuplicateWaitRefCounted(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/a", "org/proj/b"})

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	var wg sync.WaitGroup
	wg.Add(2)
	var err1, err2 error
	go func() {
		defer wg.Done()
		_, err1 = store.WaitForOutputs(ctx1, "org/proj/a", "org/proj/b")
	}()
	go func() {
		defer wg.Done()
		_, err2 = store.WaitForOutputs(context.Background(), "org/proj/a", "org/proj/b")
	}()

	// Deterministically wait for BOTH a->b waits to register before cancelling one.
	waitForEdgeCount(t, store, "org/proj/a", "org/proj/b", 2)

	// Cancel only the first wait. The second a->b wait is still active, so the edge must survive.
	cancel1()
	waitForEdgeCount(t, store, "org/proj/a", "org/proj/b", 1)

	// b -> a must still be rejected as a cycle: the a->b edge from the still-active second wait
	// is still in the graph.
	_, cycleErr := store.WaitForOutputs(context.Background(), "org/proj/b", "org/proj/a")
	require.Error(t, cycleErr)
	assert.Contains(t, cycleErr.Error(), "circular dependency detected")

	// Clean up: unblock the remaining a->b wait.
	store.SetOutputs("org/proj/b", property.NewMap(nil))
	wg.Wait()
	require.Error(t, err1, "the cancelled wait must report its own timeout error")
	require.NoError(t, err2, "the still-active wait must complete successfully once b publishes")
}

// TestOutputWaiterStoreFailAfterPublishedOutputs proves that once a co-deployed stack fails,
// every subsequent (and still-pending) waiter sees the failure -- not the provisional outputs
// that stack published before failing later in its own deployment. A failed member after
// publishing provisional outputs must still fail the whole preview.
func TestOutputWaiterStoreFailAfterPublishedOutputs(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{"org/proj/a", "org/proj/b"})

	store.SetOutputs("org/proj/a", property.NewMap(map[string]property.Value{
		"provisional": property.New("value"),
	}))
	store.FailStack("org/proj/a", assert.AnError)

	// A waiter arriving after both the (stale) success and the failure must see the failure.
	_, err := store.WaitForOutputs(context.Background(), "org/proj/b", "org/proj/a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `co-deployed stack "org/proj/a" failed`)
}

func TestOutputWaiterStoreMultipleWaiters(t *testing.T) {
	t.Parallel()

	store := NewOutputWaiterStore([]string{
		"org/proj/stack-a",
		"org/proj/stack-b",
		"org/proj/stack-c",
	})

	outputs := property.NewMap(map[string]property.Value{
		"endpoint": property.New("api.example.com"),
	})

	var wg sync.WaitGroup

	// Both stack-b and stack-c wait on stack-a.
	for _, waiter := range []string{"org/proj/stack-b", "org/proj/stack-c"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := store.WaitForOutputs(context.Background(), waiter, "org/proj/stack-a")
			require.NoError(t, err)
			assert.Equal(t, "api.example.com", got.Get("endpoint").AsString())
		}()
	}

	// Give goroutines time to start waiting.
	time.Sleep(50 * time.Millisecond)

	// Set outputs to unblock both waiters.
	store.SetOutputs("org/proj/stack-a", outputs)
	wg.Wait()
}
