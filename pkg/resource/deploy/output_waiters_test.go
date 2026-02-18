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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
	result, err := p.readStackReference(resource.PropertyMap{
		"name": resource.NewProperty("org/proj/stack-other"),
	}, "org/proj/stack-a")
	require.NoError(t, err)
	assert.True(t, backendCalled, "expected backend client to be called for non-co-deployed stack")
	assert.Equal(t, "backend-value",
		result["outputs"].ObjectValue()["backend-key"].StringValue())
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

	result, err := p.readStackReference(resource.PropertyMap{
		"name": resource.NewProperty("org/proj/stack-b"),
	}, "org/proj/stack-a")
	require.NoError(t, err)
	assert.False(t, backendCalled, "backend client should NOT be called for co-deployed stack")

	// Verify outputs are correctly translated.
	outputs := result["outputs"].ObjectValue()
	assert.Equal(t, "https://example.com", outputs["url"].StringValue())

	// Verify secret output names are populated.
	secretNames := result["secretOutputNames"].ArrayValue()
	require.Len(t, secretNames, 1)
	assert.Equal(t, "secret", secretNames[0].StringValue())
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
