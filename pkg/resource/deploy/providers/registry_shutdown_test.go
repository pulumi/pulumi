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

package providers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type mockProvider struct {
	plugin.UnimplementedProvider

	name                     tokens.PackageName
	signalCancellationCalled bool
	signalCancellationCount  int
	signalCancellationErr    error
	signalCancellationDelay  time.Duration
	mu                       sync.Mutex
}

func (m *mockProvider) Pkg() tokens.Package {
	return tokens.Package(m.name)
}

func (m *mockProvider) Close() error {
	return nil
}

func (m *mockProvider) SignalCancellation(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.signalCancellationCalled = true
	m.signalCancellationCount++

	if m.signalCancellationDelay > 0 {
		select {
		case <-time.After(m.signalCancellationDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return m.signalCancellationErr
}

func (m *mockProvider) wasCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.signalCancellationCalled
}

func (m *mockProvider) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.signalCancellationCount
}

func TestRegistryClose_CallsSignalCancellationOnAllProviders(t *testing.T) {
	t.Parallel()

	mock1 := &mockProvider{name: "provider1"}
	mock2 := &mockProvider{name: "provider2"}
	mock3 := &mockProvider{name: "provider3"}

	registry := &Registry{
		providers: map[Reference]plugin.Provider{
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider1::prov1", "id1"): mock1,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider2::prov2", "id2"): mock2,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider3::prov3", "id3"): mock3,
		},
	}

	err := registry.Close()

	assert.NoError(t, err)
	assert.True(t, mock1.wasCalled(), "provider1 SignalCancellation should have been called")
	assert.True(t, mock2.wasCalled(), "provider2 SignalCancellation should have been called")
	assert.True(t, mock3.wasCalled(), "provider3 SignalCancellation should have been called")
}

func TestRegistryClose_RespectsCancellationTimeout(t *testing.T) {
	t.Parallel()

	slowProvider := &mockProvider{
		name:                    "slow-provider",
		signalCancellationDelay: 60 * time.Second,
	}

	registry := &Registry{
		providers: map[Reference]plugin.Provider{
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:slow::slow", "id1"): slowProvider,
		},
	}

	start := time.Now()
	err := registry.Close()
	elapsed := time.Since(start)

	assert.Error(t, err, "expected timeout error")
	assert.Contains(t, err.Error(), "context deadline exceeded", "error should indicate timeout")
	assert.True(t, slowProvider.wasCalled(), "provider SignalCancellation should have been called")
	assert.Less(t, elapsed, 35*time.Second, "should timeout around 30 seconds, not wait full 60s")
}

func TestRegistryClose_ContinuesOnProviderError(t *testing.T) {
	t.Parallel()

	mock1 := &mockProvider{
		name:                  "provider1",
		signalCancellationErr: errors.New("provider1 cancellation failed"),
	}
	mock2 := &mockProvider{
		name: "provider2",
	}
	mock3 := &mockProvider{
		name:                  "provider3",
		signalCancellationErr: errors.New("provider3 cancellation failed"),
	}

	registry := &Registry{
		providers: map[Reference]plugin.Provider{
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider1::prov1", "id1"): mock1,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider2::prov2", "id2"): mock2,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider3::prov3", "id3"): mock3,
		},
	}

	err := registry.Close()

	require.Error(t, err, "should return error when providers fail")
	assert.True(t, mock1.wasCalled(), "provider1 should have been called despite error")
	assert.True(t, mock2.wasCalled(), "provider2 should have been called")
	assert.True(t, mock3.wasCalled(), "provider3 should have been called despite provider1 error")

	multiErr, ok := err.(*multierror.Error)
	require.True(t, ok, "error should be multierror")
	assert.Len(t, multiErr.Errors, 2, "should contain errors from both failing providers")

	errStr := err.Error()
	assert.Contains(t, errStr, "provider1", "error should mention provider1")
	assert.Contains(t, errStr, "provider3", "error should mention provider3")
}

func TestRegistryClose_EmptyRegistry(t *testing.T) {
	t.Parallel()

	registry := &Registry{
		providers: map[Reference]plugin.Provider{},
	}

	err := registry.Close()
	assert.NoError(t, err, "closing empty registry should not error")
}

func TestRegistryClose_CollectsAllErrors(t *testing.T) {
	t.Parallel()

	mock1 := &mockProvider{
		name:                  "provider1",
		signalCancellationErr: errors.New("error1"),
	}
	mock2 := &mockProvider{
		name:                  "provider2",
		signalCancellationErr: errors.New("error2"),
	}
	mock3 := &mockProvider{
		name:                  "provider3",
		signalCancellationErr: errors.New("error3"),
	}
	mock4 := &mockProvider{
		name: "provider4",
	}

	registry := &Registry{
		providers: map[Reference]plugin.Provider{
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider1::prov1", "id1"): mock1,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider2::prov2", "id2"): mock2,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider3::prov3", "id3"): mock3,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:provider4::prov4", "id4"): mock4,
		},
	}

	err := registry.Close()

	require.Error(t, err)
	assert.True(t, mock1.wasCalled())
	assert.True(t, mock2.wasCalled())
	assert.True(t, mock3.wasCalled())
	assert.True(t, mock4.wasCalled())

	multiErr, ok := err.(*multierror.Error)
	require.True(t, ok, "error should be multierror")
	assert.Len(t, multiErr.Errors, 3, "should contain all three errors")
}

func TestRegistryClose_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	const numProviders = 10
	providers := make(map[Reference]plugin.Provider, numProviders)
	mocks := make([]*mockProvider, numProviders)

	for i := 0; i < numProviders; i++ {
		mock := &mockProvider{
			name:                    tokens.PackageName("provider" + string(rune('0'+i))),
			signalCancellationDelay: 10 * time.Millisecond,
		}
		mocks[i] = mock
		ref := mustNewReferenceForTest(
			resource.URN("urn:pulumi:stack::project::pulumi:providers:provider"+string(rune('0'+i))+"::prov"),
			resource.ID("id"+string(rune('0'+i))),
		)
		providers[ref] = mock
	}

	registry := &Registry{
		providers: providers,
	}

	err := registry.Close()
	assert.NoError(t, err)

	for i, mock := range mocks {
		assert.True(t, mock.wasCalled(), "provider %d should have been called", i)
		assert.Equal(t, 1, mock.callCount(), "provider %d should be called exactly once", i)
	}
}

func mustNewReferenceForTest(urn resource.URN, id resource.ID) Reference {
	ref, err := NewReference(urn, id)
	if err != nil {
		panic(err)
	}
	return ref
}

func TestRegistryClose_NilProvider(t *testing.T) {
	t.Parallel()

	registry := &Registry{
		providers: nil,
	}

	err := registry.Close()
	assert.NoError(t, err, "closing registry with nil providers map should not panic")
}

func TestRegistryClose_MixedSuccessAndTimeout(t *testing.T) {
	t.Parallel()

	fastProvider := &mockProvider{
		name: "fast-provider",
	}
	slowProvider := &mockProvider{
		name:                    "slow-provider",
		signalCancellationDelay: 60 * time.Second,
	}
	normalProvider := &mockProvider{
		name: "normal-provider",
	}

	registry := &Registry{
		providers: map[Reference]plugin.Provider{
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:fast::fast", "id1"):     fastProvider,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:slow::slow", "id2"):     slowProvider,
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:normal::normal", "id3"): normalProvider,
		},
	}

	err := registry.Close()

	assert.Error(t, err, "should return error due to timeout")
	assert.True(t, fastProvider.wasCalled(), "fast provider should have been called")
	assert.True(t, slowProvider.wasCalled(), "slow provider should have been called")
	assert.True(t, normalProvider.wasCalled(), "normal provider should have been called")
}

func TestRegistryClose_ContextPropagation(t *testing.T) {
	t.Parallel()

	contextCheckProvider := &mockProvider{
		name:                    "context-check",
		signalCancellationDelay: 100 * time.Millisecond,
	}

	registry := &Registry{
		providers: map[Reference]plugin.Provider{
			mustNewReferenceForTest("urn:pulumi:stack::project::pulumi:providers:ctx::ctx", "id1"): contextCheckProvider,
		},
	}

	err := registry.Close()
	assert.NoError(t, err)
	assert.True(t, contextCheckProvider.wasCalled())
}
