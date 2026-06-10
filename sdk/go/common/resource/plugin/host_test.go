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

package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHostManagedProviderCloseSignalsCancellation locks in the contract that hostManagedProvider.Close sends
// SignalCancellation before tearing the underlying provider down. Without this, Plugin.Close treats the subsequent
// process exit as a premature crash (since shutdownAcknowledged is only flipped on Cancel RPC ack) and emits a
// misleading "exited prematurely" error to the user. defaultHost.Close does the same thing for plugins still
// registered at shutdown; callers that close individual providers (e.g. the convert mapper) bypass that path.
func TestHostManagedProviderCloseSignalsCancellation(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	ctx, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := ctx.Host.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	var calls []string
	mockProv := &MockProvider{
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
	require.NotContains(t, host.resourcePlugins, Provider(mockProv))
}

// TestContextCloseReleasesProviders locks in that closing a context releases the providers
// booted on its behalf without touching providers booted for other contexts sharing the same
// host. Release happens asynchronously once the context's base context is cancelled, so the
// test polls through the host's load channel, which serializes access to the plugin maps.
func TestContextCloseReleasesProviders(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	ctxA, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := ctxA.Host.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, ctxA.Close()) })

	ctxB := NewContextWithHost(t.Context(), sink, sink, ctxA.Host, "", "", nil)

	var mu sync.Mutex
	var aCalls, bCalls []string
	record := func(calls *[]string, call string) {
		mu.Lock()
		defer mu.Unlock()
		*calls = append(*calls, call)
	}
	provA := &MockProvider{
		SignalCancellationF: func(context.Context) error { record(&aCalls, "SignalCancellation"); return nil },
		CloseF:              func() error { record(&aCalls, "Close"); return nil },
	}
	provB := &MockProvider{
		SignalCancellationF: func(context.Context) error { record(&bCalls, "SignalCancellation"); return nil },
		CloseF:              func() error { record(&bCalls, "Close"); return nil },
	}
	host.resourcePlugins[provA] = &resourcePlugin{Plugin: provA, Name: "a", ctx: ctxA}
	host.resourcePlugins[provB] = &resourcePlugin{Plugin: provB, Name: "b", ctx: ctxB}
	host.watchContext(ctxA)
	host.watchContext(ctxB)

	readPlugins := func() (hasA, hasB bool) {
		_, err := host.loadPlugin(host.loadRequests, func() (any, error) {
			_, hasA = host.resourcePlugins[Provider(provA)]
			_, hasB = host.resourcePlugins[Provider(provB)]
			return nil, nil
		})
		require.NoError(t, err)
		return hasA, hasB
	}

	require.NoError(t, ctxB.Close())
	require.Eventually(t, func() bool {
		_, hasB := readPlugins()
		return !hasB
	}, 10*time.Second, 10*time.Millisecond)

	hasA, _ := readPlugins()
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
	ctxA, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := ctxA.Host.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, ctxA.Close()) })

	ctxB := NewContextWithHost(t.Context(), sink, sink, ctxA.Host, "", "", nil)

	var mu sync.Mutex
	var gotErr error
	var hasDeadline bool
	var gotBudget time.Duration
	prov := &MockProvider{
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
	host.watchContext(ctxB)

	require.NoError(t, ctxB.Close())
	require.Eventually(t, func() bool {
		var has bool
		_, err := host.loadPlugin(host.loadRequests, func() (any, error) {
			_, has = host.resourcePlugins[Provider(prov)]
			return nil, nil
		})
		require.NoError(t, err)
		return !has
	}, 10*time.Second, 10*time.Millisecond)

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
	LanguageRuntime
	closed bool
}

func (s *stubLanguageRuntime) Cancel(context.Context) error { return nil }
func (s *stubLanguageRuntime) Close() error                 { s.closed = true; return nil }

type stubAnalyzer struct {
	Analyzer
	closed bool
}

func (s *stubAnalyzer) Cancel(context.Context) error { return nil }
func (s *stubAnalyzer) Close() error                 { s.closed = true; return nil }

// TestContextCloseRefcountsSharedPlugins locks in that cached plugins shared by several
// contexts only close once the last context referencing them closes. The stubs' closed flags
// are only read through the load channels that serialize plugin map access, which orders those
// reads after the asynchronous release writes.
func TestContextCloseRefcountsSharedPlugins(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	ctxA, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := ctxA.Host.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, ctxA.Close()) })

	ctxB := NewContextWithHost(t.Context(), sink, sink, ctxA.Host, "", "", nil)
	ctxC := NewContextWithHost(t.Context(), sink, sink, ctxA.Host, "", "", nil)

	runtime := &stubLanguageRuntime{}
	langKey := languagePluginKey{runtime: "test", workingDirectory: ""}
	host.languagePlugins[langKey] = &languagePlugin{
		Plugin: runtime, Name: "test", refs: map[*Context]struct{}{ctxB: {}, ctxC: {}},
	}

	analyzer := &stubAnalyzer{}
	host.analyzerPlugins["test-analyzer"] = &analyzerPlugin{
		Plugin: analyzer, Name: "test-analyzer", refs: map[*Context]struct{}{ctxB: {}, ctxC: {}},
	}
	host.watchContext(ctxB)
	host.watchContext(ctxC)

	type pluginState struct {
		langCached, langClosed, analyzerCached, analyzerClosed bool
		langRefs, analyzerRefs                                 int
	}
	readState := func() pluginState {
		var state pluginState
		_, err := host.loadPlugin(host.loadRequests, func() (any, error) {
			plug, has := host.analyzerPlugins["test-analyzer"]
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
	require.NoError(t, ctxB.Close())
	require.Eventually(t, func() bool {
		state := readState()
		return state.langRefs == 1 && state.analyzerRefs == 1
	}, 10*time.Second, 10*time.Millisecond)
	state := readState()
	assert.Equal(t, pluginState{
		langCached:     true,
		analyzerCached: true,
		langRefs:       1,
		analyzerRefs:   1,
	}, state)

	// Closing the last referencing context closes them.
	require.NoError(t, ctxC.Close())
	require.Eventually(t, func() bool {
		state := readState()
		return state.langClosed && state.analyzerClosed
	}, 10*time.Second, 10*time.Millisecond)
	state = readState()
	assert.Equal(t, pluginState{
		langClosed:     true,
		analyzerClosed: true,
	}, state)
}

func TestClosePanic(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	ctx, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	host, ok := ctx.Host.(*defaultHost)
	require.True(t, ok)

	// Spin up a load of loadPlugin calls and then Close the context. This should not panic.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// We expect some of these to error that the host is shutting down, that's fine this test is just
			// checking nothing panics.
			_, _ = host.loadPlugin(host.loadRequests, func() (any, error) {
				return nil, nil
			})
		}()
	}
	err = host.Close()
	require.NoError(t, err)

	wg.Wait()
}

func TestIsLocalPluginPath(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "explicit relative path with ./",
			path:     "./my-plugin",
			expected: true,
		},
		{
			name:     "explicit relative path with ../",
			path:     "../my-plugin",
			expected: true,
		},
		{
			name:     "absolute path",
			path:     "/path/to/my-plugin",
			expected: true,
		},
		{
			name:     "windows absolute path",
			path:     "C:\\path\\to\\my-plugin",
			expected: true, // This will be true because it doesn't match plugin name regexp
		},
		{
			name:     "standard plugin name",
			path:     "aws",
			expected: false, // Standard plugin names match the regexp
		},
		{
			name:     "standard plugin name with version",
			path:     "aws@v4.0.0",
			expected: false,
		},
		{
			name:     "git URL",
			path:     "git://github.com/pulumi/pulumi-aws",
			expected: false,
		},
		{
			name:     "github URL",
			path:     "github.com/pulumi/pulumi-aws",
			expected: false,
		},
		{
			name:     "github HTTPS URL",
			path:     "https://github.com/pulumi/pulumi-aws",
			expected: false,
		},
		{
			name:     "plugin name",
			path:     "my-provider",
			expected: false,
		},
		{
			name:     "local path that looks like a plugin name",
			path:     "_my_local_path", // Doesn't match plugin name regexp
			expected: true,
		},
		{
			name:     "empty string",
			path:     "", // Can't be a valid plugin name
			expected: true,
		},
		{
			name:     "private github URL",
			path:     "github.com/pulumi/home",
			expected: false,
		},
		{
			name:     "non-existent repo URL",
			path:     "example.com/no-repo-exists/here",
			expected: false,
		},
		{
			name:     "git URL with a path",
			path:     "github.com/example/component.git/path-here",
			expected: false,
		},
		{
			name:     "git URL with a path with underscores",
			path:     "github.com/example/component.git/path_here",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsLocalPluginPath(ctx, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProjectPluginsFromProject_PackagesResolution(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a subdirectory to use as a local plugin path
	localPluginDir := filepath.Join(tempDir, "local-plugin")
	err := os.Mkdir(localPluginDir, 0o755)
	require.NoError(t, err)

	// Create another subdirectory to use as a relative plugin path
	relativePluginDir := filepath.Join(tempDir, "relative-path")
	err = os.Mkdir(relativePluginDir, 0o755)
	require.NoError(t, err)

	// Create a context for testing
	ctx := &Context{
		baseContext: t.Context(),
		Root:        tempDir,
		Diag: diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		}),
	}

	// Create packages map with various types of sources
	packages := map[string]workspace.PackageSpec{
		"local-plugin":    {Source: localPluginDir},
		"relative-plugin": {Source: "./relative-path"},
		"aws":             {Source: "aws"},                                // This should be skipped as it's not a local path
		"azure":           {Source: "azure@v4.0.0"},                       // This should be skipped as it's not a local path
		"git-plugin":      {Source: "git://github.com/pulumi/pulumi-aws"}, // This should be skipped
	}

	projectPlugins, err := projectPluginsFromProject(ctx, nil, packages)
	require.NoError(t, err)

	// We should have 2 plugins (local-plugin and relative-plugin)
	require.Len(t, projectPlugins, 2)

	// Create a map of plugin names to paths for easier verification
	pluginMap := make(map[string]string)
	for _, plugin := range projectPlugins {
		pluginMap[plugin.Name] = plugin.Path
	}

	// Verify the expected plugins are present with correct paths
	assert.Contains(t, pluginMap, "local-plugin")
	assert.Contains(t, pluginMap, "relative-plugin")
	assert.Equal(t, localPluginDir, pluginMap["local-plugin"])
	assert.Equal(t, filepath.Join(tempDir, "relative-path"), pluginMap["relative-plugin"])

	// Verify the unexpected plugins are not present
	assert.NotContains(t, pluginMap, "aws")
	assert.NotContains(t, pluginMap, "azure")
	assert.NotContains(t, pluginMap, "git-plugin")
}

// TestProjectPluginsFromProject_BothPluginsAndPackages tests the combined resolution of plugins and packages
func TestProjectPluginsFromProject_BothPluginsAndPackages(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for our test
	tempDir := t.TempDir()

	// Create a subdirectory to use as a local plugin path
	localPluginDir := filepath.Join(tempDir, "local-plugin")
	err := os.Mkdir(localPluginDir, 0o755)
	require.NoError(t, err)

	awsProviderDir := filepath.Join(tempDir, "aws-provider")
	err = os.Mkdir(awsProviderDir, 0o755)
	require.NoError(t, err)

	// Create a context for testing
	ctx := &Context{
		baseContext: t.Context(),
		Root:        tempDir,
		Diag: diag.DefaultSink(os.Stderr, os.Stderr, diag.FormatOptions{
			Color: colors.Never,
		}),
	}

	// Create test plugins
	plugins := &workspace.Plugins{
		Providers: []workspace.PluginOptions{
			{Name: "aws", Path: "./aws-provider"},
		},
	}

	// Create test packages
	packages := map[string]workspace.PackageSpec{
		"local-plugin": {Source: localPluginDir},
		"azure":        {Source: "azure"}, // This should be skipped as it's not a local path
	}

	projectPlugins, err := projectPluginsFromProject(ctx, plugins, packages)
	require.NoError(t, err)

	// We should have 2 plugins (1 from plugins, 1 from packages)
	require.Len(t, projectPlugins, 2)

	// Check that all expected plugins are present
	pluginNames := map[string]bool{}
	for _, plugin := range projectPlugins {
		pluginNames[plugin.Name] = true
	}

	assert.True(t, pluginNames["aws"])
	assert.True(t, pluginNames["local-plugin"])
	assert.False(t, pluginNames["azure"])
}

// TestContextLoaderAddr locks in that a context constructed with a NewLoaderFunc serves the
// loader for the lifetime of the context, bound to that context's workspace view.
func TestContextLoaderAddr(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)

	var captureCtx *Context
	mockLoader := func(ctx *Context) codegenrpc.LoaderServer {
		captureCtx = ctx
		return codegenrpc.UnimplementedLoaderServer{}
	}

	ctx, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, mockLoader, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, ctx, captureCtx, "the loader is bound to the context it was constructed with")
	assert.NotEmpty(t, ctx.LoaderAddr())
	assert.Equal(t, "", ctx.MapperAddr(), "a context built without a mapper should have no mapper address")

	require.NoError(t, ctx.Close())
}

func TestContextMapperAddr(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)

	var captureCtx *Context
	mockMapper := func(ctx *Context) codegenrpc.MapperServer {
		captureCtx = ctx
		return codegenrpc.UnimplementedMapperServer{}
	}

	ctx, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, mockMapper, nil)
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

	ctx, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil, nil, nil, installLang)
	require.NoError(t, err)
	host, ok := ctx.Host.(*defaultHost)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	lang, err := host.LanguageRuntime(ctx, "test-lang")

	// The installer ran exactly once, for the requested runtime, and its error gated the load so we never
	// got a runtime back.
	require.ErrorIs(t, err, errInstall)
	assert.Nil(t, lang)
	assert.Equal(t, 1, installCalls)
	assert.Equal(t, "test-lang", gotRuntime)
}
