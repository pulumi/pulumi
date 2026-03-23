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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	mapset "github.com/deckarep/golang-set/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type testHost struct {
	engine      *languageTestServer
	ctx         *plugin.Context
	host        plugin.Host
	runtime     plugin.LanguageRuntime
	runtimeName string
	providers   map[string]func() (plugin.Provider, error)

	connectionsMutex sync.Mutex
	connections      map[plugin.Provider]io.Closer

	policies []plugin.Analyzer

	closeMutex sync.Mutex

	skipEnsurePluginsValidation bool

	loaderAddress string
}

var _ plugin.Host = (*testHost)(nil)

func (h *testHost) ServerAddr() string {
	return h.engine.addr
}

func (h *testHost) LoaderAddr() string {
	return h.loaderAddress
}

func (h *testHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	var rpcsev pulumirpc.LogSeverity
	switch sev {
	case diag.Debug:
		rpcsev = pulumirpc.LogSeverity_DEBUG
	case diag.Info:
		rpcsev = pulumirpc.LogSeverity_INFO
	case diag.Infoerr:
		rpcsev = pulumirpc.LogSeverity_INFO
	case diag.Warning:
		rpcsev = pulumirpc.LogSeverity_WARNING
	case diag.Error:
		rpcsev = pulumirpc.LogSeverity_ERROR
	default:
		contract.Failf("unexpected severity %v", sev)
	}

	_, err := h.engine.Log(context.TODO(),
		&pulumirpc.LogRequest{
			Severity: rpcsev,
			Urn:      string(urn),
			Message:  msg,
			StreamId: streamID,
		})
	contract.IgnoreError(err)
}

func (h *testHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	panic("not implemented")
}

func (h *testHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	panic("not implemented")
}

func (h *testHost) PolicyAnalyzer(
	name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	hasPlugin := func(spec workspace.PluginDescriptor) bool {
		// This is only called for the language runtime, so we can just do a simple check.
		return spec.Kind == apitype.LanguagePlugin && spec.Name == h.runtimeName
	}
	analyzer, err := plugin.NewPolicyAnalyzer(h, h.ctx, name, path, opts, hasPlugin)
	if err != nil {
		return nil, err
	}
	h.policies = append(h.policies, analyzer)
	return analyzer, nil
}

func (h *testHost) ListAnalyzers() []plugin.Analyzer {
	return h.policies
}

func (h *testHost) Provider(descriptor workspace.PluginDescriptor, e env.Env) (plugin.Provider, error) {
	factory, key, err := h.findProviderFactory(descriptor)
	if err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("unknown provider %s", key)
	}

	provider, err := factory()
	if err != nil {
		return nil, fmt.Errorf("initializing provider %s: %w", key, err)
	}

	grpcProvider, closer, err := wrapProviderWithGrpc(provider)
	if err != nil {
		return nil, err
	}
	h.connectionsMutex.Lock()
	defer h.connectionsMutex.Unlock()
	h.connections[grpcProvider] = closer

	return grpcProvider, nil
}

// findProviderFactory finds the best matching provider factory for the given descriptor.
// It returns the factory, the key, and any error. When version is nil, it picks the latest version.
// This avoids creating provider instances just to compare versions.
func (h *testHost) findProviderFactory(
	descriptor workspace.PluginDescriptor,
) (func() (plugin.Provider, error), string, error) {
	if descriptor.Version == nil {
		var bestKey string
		var bestVersion semver.Version
		var found bool
		for k := range h.providers {
			parts := strings.Split(k, "@")
			if len(parts) != 2 {
				return nil, k, fmt.Errorf("unexpected provider key %s", k)
			}
			if parts[0] == descriptor.Name {
				v := semver.MustParse(parts[1])
				if !found || v.GT(bestVersion) {
					bestKey = k
					bestVersion = v
					found = true
				}
			}
		}
		if !found {
			return nil, descriptor.Name, nil
		}
		return h.providers[bestKey], bestKey, nil
	}

	key := fmt.Sprintf("%s@%s", descriptor.Name, descriptor.Version)
	factory, ok := h.providers[key]
	if !ok {
		return nil, key, nil
	}
	return factory, key, nil
}

// RawProvider returns a provider instance without gRPC wrapping. This is useful for
// operations that only need to call GetSchema or other metadata methods, avoiding the
// overhead of starting a gRPC server just for a schema query.
func (h *testHost) RawProvider(descriptor workspace.PluginDescriptor) (plugin.Provider, error) {
	factory, key, err := h.findProviderFactory(descriptor)
	if err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, fmt.Errorf("unknown provider %s", key)
	}

	provider, err := factory()
	if err != nil {
		return nil, fmt.Errorf("initializing provider %s: %w", key, err)
	}
	return provider, nil
}

// LanguageRuntime returns the language runtime initialized by the test host.
// ProgramInfo is only used here for compatibility reasons and will be removed from this function.
func (h *testHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	if runtime != h.runtimeName {
		return nil, fmt.Errorf("unexpected runtime %s", runtime)
	}
	return h.runtime, nil
}

func (h *testHost) EnsurePlugins(plugins []workspace.PluginDescriptor, kinds plugin.Flags) error {
	// Remove the builtin "pulumi" provider, as that's always available.
	filtered := make([]workspace.PluginDescriptor, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.Kind == apitype.ResourcePlugin && plugin.Name == "pulumi" {
			continue
		}
		filtered = append(filtered, plugin)
	}
	plugins = filtered

	// Skip validation if requested (e.g., for tests using version resource option)
	if h.skipEnsurePluginsValidation {
		return nil
	}

	// EnsurePlugins will be called with the result of GetRequiredPlugins, so we can use this to check
	// that that returned the expected plugins (with expected versions).
	// We derive the expected set from the provider keys (which are already "name@version")
	// to avoid creating provider instances just for metadata queries.
	expected := mapset.NewSet[string]()
	for key := range h.providers {
		parts := strings.Split(key, "@")
		if len(parts) != 2 {
			return fmt.Errorf("unexpected provider key %s", key)
		}
		expected.Add(fmt.Sprintf("resource-%s@%s", parts[0], parts[1]))
	}

	actual := mapset.NewSetWithSize[string](len(plugins))
	for _, plugin := range plugins {
		actual.Add(fmt.Sprintf("%s-%s@%s", plugin.Kind, plugin.Name, plugin.Version))
	}

	// Symmetric difference, we want to know if there are any unexpected plugins, or any missing plugins.
	diff := expected.SymmetricDifference(actual)
	if !diff.IsEmpty() {
		expectedSlice := expected.ToSlice()
		slices.Sort(expectedSlice)
		actualSlice := actual.ToSlice()
		slices.Sort(actualSlice)
		return fmt.Errorf("unexpected required plugins: actual %v, expected %v", actualSlice, expectedSlice)
	}

	return nil
}

func (h *testHost) ResolvePlugin(
	spec workspace.PluginDescriptor,
) (*workspace.PluginInfo, error) {
	if spec.Kind == apitype.ResourcePlugin {
		// Resolve from the provider keys (which are "name@version") to avoid
		// creating provider instances just for metadata queries.
		for key := range h.providers {
			parts := strings.Split(key, "@")
			if len(parts) != 2 {
				return nil, fmt.Errorf("unexpected provider key %s", key)
			}
			name := parts[0]
			version := semver.MustParse(parts[1])
			if spec.Name == name && (spec.Version == nil || spec.Version.EQ(version)) {
				return &workspace.PluginInfo{
					Name:    spec.Name,
					Kind:    spec.Kind,
					Version: spec.Version,
				}, nil
			}
		}
		return nil, fmt.Errorf("unknown provider %s@%s", spec.Name, spec.Version)
	}

	return &workspace.PluginInfo{
		Name:    spec.Name,
		Kind:    spec.Kind,
		Version: spec.Version,
	}, nil
}

func (h *testHost) GetProjectPlugins() []workspace.ProjectPlugin {
	// We're not using project plugins, in fact this method shouldn't even really exists on Host given it's
	// just reading off Pulumi.yaml.
	return nil
}

func (h *testHost) SignalCancellation() error {
	panic("not implemented")
}

func (h *testHost) Close() error {
	h.closeMutex.Lock()
	defer h.closeMutex.Unlock()
	errs := make([]error, 0)
	h.connectionsMutex.Lock()
	defer h.connectionsMutex.Unlock()
	for _, closer := range h.connections {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	h.connections = make(map[plugin.Provider]io.Closer)

	for _, policy := range h.policies {
		if err := policy.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	h.policies = nil

	err := errors.Join(errs...)
	if err != nil {
		return fmt.Errorf("failed to close plugins: %w", err)
	}
	return nil
}

func (h *testHost) StartDebugging(plugin.DebuggingInfo) error {
	panic("not implemented")
}

func (h *testHost) AttachDebugger(plugin.DebugSpec) bool {
	return false
}

type grpcWrapper struct {
	stop chan bool
}

func (w *grpcWrapper) Close() error {
	go func() { w.stop <- true }()
	return nil
}

func newProviderServer(provider plugin.Provider) pulumirpc.ResourceProviderServer {
	if pwcs, ok := provider.(providers.ProviderWithCustomServer); ok {
		return pwcs.NewProviderServer()
	}
	return plugin.NewProviderServer(provider)
}

func wrapProviderWithGrpc(provider plugin.Provider) (plugin.Provider, io.Closer, error) {
	wrapper := &grpcWrapper{stop: make(chan bool)}
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: wrapper.stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, newProviderServer(provider))
			return nil
		},
		Options: rpcutil.TracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not start resource provider service: %w", err)
	}
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%v", handle.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		contract.IgnoreClose(wrapper)
		return nil, nil, fmt.Errorf("could not connect to resource provider service: %w", err)
	}
	wrapped := plugin.NewProviderWithClient(
		nil, provider.Pkg(), pulumirpc.NewResourceProviderClient(conn), false)
	return wrapped, wrapper, nil
}
