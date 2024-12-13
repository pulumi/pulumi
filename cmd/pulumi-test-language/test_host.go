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
	"fmt"
	"io"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
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
	// If we've not been given a version, we'll try and find the provider by name alone, picking the latest if there are
	// multiple versions of the named provider. Otherwise, we can attempt to find an exact match.
	var key string
	var provider plugin.Provider
	if descriptor.Version == nil {
		key = descriptor.Name

		var version semver.Version
		for k, p := range h.providers {
			parts := strings.Split(k, "@")
			if len(parts) != 2 {
				return nil, fmt.Errorf("unexpected provider key %s", k)
			}

			if parts[0] == key {
				v := semver.MustParse(parts[1])
				if provider == nil || v.GT(version) {
					provider = p
					version = v
				}
			}
		}
	} else {
		key = fmt.Sprintf("%s@%s", descriptor.Name, descriptor.Version)
		provider = h.providers[key]
	}

	if provider == nil {
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
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
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
