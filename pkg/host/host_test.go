// Copyright 2016, Pulumi Corporation.
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

package host

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHost constructs a default host and returns its concrete type so tests can poke at its
// internals. The host is closed when the test finishes.
func newHost(t *testing.T, installLang plugin.LanguageInstaller) *defaultHost {
	sink := diagtest.LogSink(t)
	h, err := New(t.Context(), sink, sink, nil, installLang, nil, nil, nil)
	require.NoError(t, err)
	host, ok := h.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, host.Close()) })
	return host
}

// TestLogWithNilSink is a regression test for https://github.com/pulumi/pulumi/issues/23646.
//
// Schema binding instantiates a throwaway host with nil diagnostic sinks (see newBinder in
// pkg/codegen/schema/bind.go) when it needs to load a referenced package. If that reference is to
// an uninstalled plugin, the plugin download-progress callback logs through the host. A host with a
// nil diag sink must not panic when logged to.
func TestLogWithNilSink(t *testing.T) {
	t.Parallel()

	h, err := New(t.Context(), nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := h.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	require.NotPanics(t, func() {
		host.Log(diag.Info, "", "downloading plugin", 0)
		host.LogStatus(diag.Info, "", "downloading plugin", 0)
	})
}

// TestHostManagedProviderCloseSignalsCancellation locks in the contract that hostManagedProvider.Close sends
// SignalCancellation before tearing the underlying provider down. Without this, Plugin.Close treats the subsequent
// process exit as a premature crash (since shutdownAcknowledged is only flipped on Cancel RPC ack) and emits a
// misleading "exited prematurely" error to the user. defaultHost.Close does the same thing for plugins still
// registered at shutdown; callers that close individual providers (e.g. the convert mapper) bypass that path.
func TestHostManagedProviderCloseSignalsCancellation(t *testing.T) {
	t.Parallel()

	host := newHost(t, nil)

	var calls []string
	mockProv := &plugin.MockProvider{
		SignalCancellationF: func(context.Context) error {
			calls = append(calls, "SignalCancellation")
			return nil
		},
		CloseF: func() error {
			calls = append(calls, "Close")
			return nil
		},
	}

	host.resourcePlugins[mockProv] = &resourcePlugin{Plugin: mockProv, Name: "mock"}

	managed := hostManagedProvider{Provider: mockProv, host: host}
	require.NoError(t, managed.Close())

	require.Equal(t, []string{"SignalCancellation", "Close"}, calls)
	require.NotContains(t, host.resourcePlugins, plugin.Provider(mockProv))
}

// TestContextCloseReleasesProviders locks in that closing a context releases the providers
// booted on its behalf without touching providers booted for other contexts sharing the same
// host. Release happens asynchronously once the context's base context is cancelled, so the
// test polls through the host's load channel, which serializes access to the plugin maps.
func TestContextCloseReleasesProviders(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	host := newHost(t, nil)

	ctxA, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)
	ctxB, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ctxA.Close()) })

	var mu sync.Mutex
	var aCalls, bCalls []string
	record := func(calls *[]string, call string) {
		mu.Lock()
		defer mu.Unlock()
		*calls = append(*calls, call)
	}
	provA := &plugin.MockProvider{
		SignalCancellationF: func(context.Context) error { record(&aCalls, "SignalCancellation"); return nil },
		CloseF:              func() error { record(&aCalls, "Close"); return nil },
	}
	provB := &plugin.MockProvider{
		SignalCancellationF: func(context.Context) error { record(&bCalls, "SignalCancellation"); return nil },
		CloseF:              func() error { record(&bCalls, "Close"); return nil },
	}
	host.resourcePlugins[provA] = &resourcePlugin{Plugin: provA, Name: "a", ctx: ctxA}
	host.resourcePlugins[provB] = &resourcePlugin{Plugin: provB, Name: "b", ctx: ctxB}

	readPlugins := func() (hasA, hasB bool) {
		_, err := host.loadPlugin(host.loadRequests, func() (any, error) {
			_, hasA = host.resourcePlugins[plugin.Provider(provA)]
			_, hasB = host.resourcePlugins[plugin.Provider(provB)]
			return nil, nil
		})
		require.NoError(t, err)
		return hasA, hasB
	}

	// ReleaseContext, driven synchronously by Context.Close, has released ctxB's provider by the
	// time Close returns.
	require.NoError(t, ctxB.Close())
	hasA, hasB := readPlugins()
	assert.False(t, hasB, "released context's provider must be closed")
	assert.True(t, hasA, "provider booted for another context must survive")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"SignalCancellation", "Close"}, bCalls)
	assert.Empty(t, aCalls)
}

// TestContextCloseGracefulShutdownBudget locks in that plugins released because their context
// was closed still get the full graceful-shutdown budget: the Cancel RPC runs under the host's
// lifetime context, not the (already cancelled) workspace context. Closing a workspace context
// is graceful; only the host's own lifetime context is a hard stop.
func TestContextCloseGracefulShutdownBudget(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	host := newHost(t, nil)

	ctxB, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)

	var mu sync.Mutex
	var gotErr error
	var hasDeadline bool
	var gotBudget time.Duration
	prov := &plugin.MockProvider{
		SignalCancellationF: func(cancelCtx context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			gotErr = cancelCtx.Err()
			var deadline time.Time
			if deadline, hasDeadline = cancelCtx.Deadline(); hasDeadline {
				gotBudget = time.Until(deadline)
			}
			return nil
		},
		CloseF: func() error { return nil },
	}
	host.resourcePlugins[prov] = &resourcePlugin{Plugin: prov, Name: "b", ctx: ctxB}

	require.NoError(t, ctxB.Close())
	var has bool
	_, err = host.loadPlugin(host.loadRequests, func() (any, error) {
		_, has = host.resourcePlugins[plugin.Provider(prov)]
		return nil, nil
	})
	require.NoError(t, err)
	assert.False(t, has, "released context's provider must be closed")

	mu.Lock()
	defer mu.Unlock()
	// The workspace context was already cancelled when the shutdown RPC ran, yet the RPC's
	// context must be alive with (roughly) the full 5 second budget remaining.
	require.NoError(t, gotErr)
	require.True(t, hasDeadline)
	assert.Greater(t, gotBudget, 2*time.Second)
	assert.LessOrEqual(t, gotBudget, 5*time.Second)
}

type stubLanguageRuntime struct {
	plugin.LanguageRuntime
	closed bool
}

func (s *stubLanguageRuntime) Cancel(context.Context) error { return nil }
func (s *stubLanguageRuntime) Close() error                 { s.closed = true; return nil }

type stubAnalyzer struct {
	plugin.Analyzer
	closed bool
}

func (s *stubAnalyzer) Cancel(context.Context) error { return nil }
func (s *stubAnalyzer) Close() error                 { s.closed = true; return nil }

// TestContextCloseRefcountsSharedPlugins locks in that cached plugins shared by several
// contexts only close once the last context referencing them closes. The stubs' closed flags
// are only read through the load channels that serialize plugin map access, which orders those
// reads after the release writes.
func TestContextCloseRefcountsSharedPlugins(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	host := newHost(t, nil)

	ctxB, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)
	ctxC, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)

	runtime := &stubLanguageRuntime{}
	langKey := languagePluginKey{runtime: "test", workingDirectory: ""}
	host.languagePlugins[langKey] = &languagePlugin{
		Plugin: runtime, Name: "test", refs: map[*plugin.Context]struct{}{ctxB: {}, ctxC: {}},
	}

	analyzer := &stubAnalyzer{}
	analyzerKey := analyzerPluginKey{name: "test-analyzer"}
	host.analyzerPlugins[analyzerKey] = &analyzerPlugin{
		Plugin: analyzer, Name: "test-analyzer", refs: map[*plugin.Context]struct{}{ctxB: {}, ctxC: {}},
	}

	type pluginState struct {
		langCached, langClosed, analyzerCached, analyzerClosed bool
		langRefs, analyzerRefs                                 int
	}
	readState := func() pluginState {
		var state pluginState
		_, err := host.loadPlugin(host.loadRequests, func() (any, error) {
			plug, has := host.analyzerPlugins[analyzerKey]
			state.analyzerCached = has
			state.analyzerClosed = analyzer.closed
			if has {
				state.analyzerRefs = len(plug.refs)
			}
			return nil, nil
		})
		require.NoError(t, err)
		_, err = host.loadPlugin(host.languageLoadRequests, func() (any, error) {
			plug, has := host.languagePlugins[langKey]
			state.langCached = has
			state.langClosed = runtime.closed
			if has {
				state.langRefs = len(plug.refs)
			}
			return nil, nil
		})
		require.NoError(t, err)
		return state
	}

	// Closing the first context must not close the shared plugins, only drop its references.
	// Release is synchronous, so the dropped reference is visible as soon as Close returns.
	require.NoError(t, ctxB.Close())
	state := readState()
	assert.Equal(t, pluginState{
		langCached:     true,
		analyzerCached: true,
		langRefs:       1,
		analyzerRefs:   1,
	}, state)

	// Closing the last referencing context closes them.
	require.NoError(t, ctxC.Close())
	state = readState()
	assert.Equal(t, pluginState{
		langClosed:     true,
		analyzerClosed: true,
	}, state)
}

func TestClosePanic(t *testing.T) {
	t.Parallel()

	host := newHost(t, nil)

	// Spin up a load of loadPlugin calls and then Close the context. This should not panic.
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			// We expect some of these to error that the host is shutting down, that's fine this test is just
			// checking nothing panics.
			_, _ = host.loadPlugin(host.loadRequests, func() (any, error) {
				return nil, nil
			})
		})
	}
	err := host.Close()
	require.NoError(t, err)

	wg.Wait()
}

// TestContextLoaderAddr locks in that a context constructed with a NewLoaderFunc serves the
// loader for the lifetime of the context, bound to that context's workspace view.
func TestContextLoaderAddr(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)

	var captureCtx *plugin.Context
	mockLoader := func(ctx *plugin.Context) codegenrpc.LoaderServer {
		captureCtx = ctx
		return codegenrpc.UnimplementedLoaderServer{}
	}

	host, err := New(t.Context(), sink, sink, nil, nil, mockLoader, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	ctx, err := plugin.NewContext(t.Context(), sink, sink, host, nil, "", nil, false, nil)
	require.NoError(t, err)

	assert.Equal(t, ctx, captureCtx, "the loader is bound to the context it was constructed with")
	assert.NotEmpty(t, ctx.LoaderAddr())
	assert.Equal(t, "", ctx.MapperAddr(), "a context built without a mapper should have no mapper address")

	require.NoError(t, ctx.Close())
}

func TestContextMapperAddr(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)

	var captureCtx *plugin.Context
	mockMapper := func(ctx *plugin.Context) codegenrpc.MapperServer {
		captureCtx = ctx
		return codegenrpc.UnimplementedMapperServer{}
	}

	host, err := New(t.Context(), sink, sink, nil, nil, nil, mockMapper, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	ctx, err := plugin.NewContext(t.Context(), sink, sink, host, nil, "", nil, false, nil)
	require.NoError(t, err)

	assert.Equal(t, ctx, captureCtx, "the mapper is bound to the context it was constructed with")
	assert.NotEmpty(t, ctx.MapperAddr())
	assert.Equal(t, "", ctx.LoaderAddr(), "a context built without a loader should have no loader address")

	require.NoError(t, ctx.Close())
}

func TestDefaultHostLanguageRuntimeInstallsOnDemand(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)

	errInstall := errors.New("install boom")
	var (
		installCalls int
		gotRuntime   string
	)
	installLang := func(_ context.Context, runtime string) error {
		installCalls++
		gotRuntime = runtime
		return errInstall
	}

	host := newHost(t, installLang)
	ctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)

	lang, err := host.LanguageRuntime(ctx, "test-lang")

	// The installer ran exactly once, for the requested runtime, and its error gated the load so we never
	// got a runtime back.
	require.ErrorIs(t, err, errInstall)
	assert.Nil(t, lang)
	assert.Equal(t, 1, installCalls)
	assert.Equal(t, "test-lang", gotRuntime)
}
