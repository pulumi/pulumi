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

package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
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
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type testHost struct {
	engine       *languageTestServer
	runtime      plugin.LanguageRuntime
	runtimeName  string
	languageInfo string
	providers    map[string]func() (plugin.Provider, error)

	// servicesMu guards loader and contextServices, which are written from Loader/Mapper as the
	// engine boots contexts concurrently.
	servicesMu sync.Mutex

	// loader is the provider-backed schema loader this host binds onto a context. It is captured
	// here when Loader runs so the conformance runner can reuse it to bind PCL programs.
	loader *providerLoader

	// contextServices holds the loader/mapper gRPC servers this host hosts for each context; they
	// are shut down in that context's ReleaseContext.
	contextServices map[*plugin.Context][]*plugin.GrpcServer

	connectionsMutex sync.Mutex
	connections      map[plugin.Provider]io.Closer

	policies []plugin.Analyzer

	closeMutex sync.Mutex
}

var _ plugin.Host = (*testHost)(nil)

func (h *testHost) ServerAddr() string {
	return h.engine.addr
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

func (h *testHost) Analyzer(ctx *plugin.Context, nm tokens.QName) (plugin.Analyzer, error) {
	panic("not implemented")
}

func (h *testHost) PolicyAnalyzer(
	ctx *plugin.Context, name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	hasPlugin := func(spec workspace.PluginDescriptor) bool {
		// This is only called for the language runtime, so we can just do a simple check.
		return spec.Kind == apitype.LanguagePlugin && spec.Name == h.runtimeName
	}
	analyzer, err := plugin.NewPolicyAnalyzer(h, ctx, name, path, opts, hasPlugin)
	if err != nil {
		return nil, err
	}
	h.policies = append(h.policies, analyzer)
	return analyzer, nil
}

func (h *testHost) Provider(
	ctx *plugin.Context, descriptor workspace.PluginDescriptor, e env.Env,
) (plugin.Provider, error) {
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
					var err error
					provider, err = p()
					if err != nil {
						return nil, fmt.Errorf("initializing provider %s: %w", k, err)
					}
					version = v
				}
			}
		}
	} else {
		key = fmt.Sprintf("%s@%s", descriptor.Name, descriptor.Version)
		var err error
		providerFactory, ok := h.providers[key]
		if !ok {
			return nil, fmt.Errorf("unknown provider %s", key)
		}

		provider, err = providerFactory()
		if err != nil {
			return nil, fmt.Errorf("initializing provider %s: %w", key, err)
		}
	}

	if provider == nil {
		return nil, fmt.Errorf("unknown provider %s", key)
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

// LanguageRuntime returns the language runtime initialized by the test host.
// ProgramInfo is only used here for compatibility reasons and will be removed from this function.
func (h *testHost) LanguageRuntime(ctx *plugin.Context, runtime string) (plugin.LanguageRuntime, error) {
	if runtime != h.runtimeName {
		return nil, fmt.Errorf("unexpected runtime %s", runtime)
	}
	return h.runtime, nil
}

func (h *testHost) ResolvePlugin(
	ctx *plugin.Context, spec workspace.PluginDescriptor,
) (*workspace.PluginInfo, error) {
	if spec.Kind == apitype.ResourcePlugin {
		for key, provider := range h.providers {
			p, err := provider()
			if err != nil {
				return nil, fmt.Errorf("initializing provider %s for resolve plugin: %w", key, err)
			}
			providerVersion, err := GetProviderVersion(context.TODO(), p)
			if err != nil {
				return nil, fmt.Errorf("get provider version %s: %w", key, err)
			}
			name, err := GetProviderName(context.TODO(), p)
			if err != nil {
				return nil, fmt.Errorf("get provider name %s: %w", key, err)
			}
			if spec.Name == name && (spec.Version == nil || spec.Version.EQ(providerVersion)) {
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

// ReleaseContext shuts down the loader and mapper gRPC servers this host hosts for the context.
// The test host's providers are not scoped to a context and are torn down when it closes.
func (h *testHost) ReleaseContext(ctx *plugin.Context) error {
	h.servicesMu.Lock()
	servers := h.contextServices[ctx]
	delete(h.contextServices, ctx)
	h.servicesMu.Unlock()

	var errs []error
	for _, srv := range servers {
		if err := srv.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Loader serves the conformance runner's provider-backed schema loader, bound to ctx. The loader
// resolves schemas from the test's own providers via this host.
func (h *testHost) Loader(ctx *plugin.Context) (*plugin.GrpcServer, error) {
	loader := &providerLoader{
		language:     h.runtimeName,
		languageInfo: h.languageInfo,
		pctx:         ctx,
		host:         h,
	}
	srv, err := plugin.NewServer(ctx, func(srv *grpc.Server) {
		codegenrpc.RegisterLoaderServer(srv, schema.NewLoaderServer(loader))
	})
	if err != nil {
		return nil, err
	}
	h.servicesMu.Lock()
	h.loader = loader
	h.contextServices[ctx] = append(h.contextServices[ctx], srv)
	h.servicesMu.Unlock()
	return srv, nil
}

// Mapper serves the standard conversion mapper bound to ctx, sourcing mappings from the plugins
// installed in the global plugin storage.
func (h *testHost) Mapper(ctx *plugin.Context) (*plugin.GrpcServer, error) {
	srv, err := plugin.NewServer(ctx, func(srv *grpc.Server) {
		codegenrpc.RegisterMapperServer(srv, convert.NewMapperServerFromContext(ctx))
	})
	if err != nil {
		return nil, err
	}
	h.servicesMu.Lock()
	h.contextServices[ctx] = append(h.contextServices[ctx], srv)
	h.servicesMu.Unlock()
	return srv, nil
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
		nil, pulumirpc.NewResourceProviderClient(conn), false)
	return wrapped, wrapper, nil
}
