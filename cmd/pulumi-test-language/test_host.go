// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	mapset "github.com/deckarep/golang-set/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type testHost struct {
	stderr      *bytes.Buffer
	host        plugin.Host
	runtime     plugin.LanguageRuntime
	runtimeName string
	providers   map[string]plugin.Provider

	connections map[plugin.Provider]io.Closer
}

var _ plugin.Host = (*testHost)(nil)

func (h *testHost) ServerAddr() string {
	panic("not implemented")
}

func (h *testHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	prefix := ""
	if urn != "" {
		prefix = fmt.Sprintf(" %s: ", urn)
	}
	_, err := fmt.Fprintf(h.stderr, "[%s]%s\n", prefix, msg)
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
	panic("not implemented")
}

func (h *testHost) ListAnalyzers() []plugin.Analyzer {
	// We're not using analyzers for matrix tests, yet.
	return nil
}

func (h *testHost) Provider(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
	// Look in the providers map for this provider
	if descriptor.Version == nil {
		return nil, errors.New("unexpected provider request with no version")
	}

	key := fmt.Sprintf("%s@%s", descriptor.Name, descriptor.Version)
	provider, has := h.providers[key]
	if !has {
		return nil, fmt.Errorf("unknown provider %s", key)
	}

	grpcProvider, closer, err := wrapProviderWithGrpc(provider)
	if err != nil {
		return nil, err
	}
	h.connections[grpcProvider] = closer

	return grpcProvider, nil
}

func (h *testHost) CloseProvider(provider plugin.Provider) error {
	closer, ok := h.connections[provider]
	if !ok {
		return fmt.Errorf("unknown provider %v", provider)
	}
	delete(h.connections, provider)
	return closer.Close()
}

// LanguageRuntime returns the language runtime initialized by the test host.
// ProgramInfo is only used here for compatibility reasons and will be removed from this function.
func (h *testHost) LanguageRuntime(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
	if runtime != h.runtimeName {
		return nil, fmt.Errorf("unexpected runtime %s", runtime)
	}
	return h.runtime, nil
}

func (h *testHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds plugin.Flags) error {
	// EnsurePlugins will be called with the result of GetRequiredPlugins, so we can use this to check
	// that that returned the expected plugins (with expected versions).
	expected := mapset.NewSet(
		fmt.Sprintf("language-%s@<nil>", h.runtimeName),
	)
	for _, provider := range h.providers {
		pkg := provider.Pkg()
		version, err := getProviderVersion(provider)
		if err != nil {
			return fmt.Errorf("get provider version %s: %w", pkg, err)
		}
		expected.Add(fmt.Sprintf("resource-%s@%s", pkg, version))
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
	kind apitype.PluginKind, name string, version *semver.Version,
) (*workspace.PluginInfo, error) {
	if kind == apitype.ResourcePlugin {
		for _, provider := range h.providers {
			pkg := provider.Pkg()
			providerVersion, err := getProviderVersion(provider)
			if err != nil {
				return nil, fmt.Errorf("get provider version %s: %w", pkg, err)
			}
			if name == string(pkg) && version == nil || version.EQ(providerVersion) {
				return &workspace.PluginInfo{
					Name:    name,
					Kind:    kind,
					Version: version,
				}, nil
			}
		}
		return nil, fmt.Errorf("unknown provider %s@%s", name, version)
	}

	return &workspace.PluginInfo{
		Name:    name,
		Kind:    kind,
		Version: version,
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
	return nil
}

func (h *testHost) StartDebugging(plugin.DebuggingInfo) error {
	panic("not implemented")
}

type grpcWrapper struct {
	stop chan bool
}

func (w *grpcWrapper) Close() error {
	go func() { w.stop <- true }()
	return nil
}

func wrapProviderWithGrpc(provider plugin.Provider) (plugin.Provider, io.Closer, error) {
	wrapper := &grpcWrapper{stop: make(chan bool)}
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: wrapper.stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, plugin.NewProviderServer(provider))
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not start resource provider service: %w", err)
	}
	conn, err := grpc.Dial(
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
