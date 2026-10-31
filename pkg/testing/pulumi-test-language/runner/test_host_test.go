// Copyright 2026, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type handshakeRecorder struct {
	plugin.UnimplementedProvider

	mu         sync.Mutex
	handshakes []plugin.ProviderHandshakeRequest
}

func (p *handshakeRecorder) Handshake(
	_ context.Context, req plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handshakes = append(p.handshakes, req)
	return &plugin.ProviderHandshakeResponse{}, nil
}

// pluginLanguageHost runs every plugin as an in-process gRPC server for provider, in the way a
// language host runs a provider from source.
type pluginLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	provider plugin.Provider
}

func (h *pluginLanguageHost) RunPlugin(
	_ *pulumirpc.RunPluginRequest, server grpc.ServerStreamingServer[pulumirpc.RunPluginResponse],
) error {
	stop := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: stop,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, plugin.NewProviderServer(h.provider))
			return nil
		},
	})
	if err != nil {
		return err
	}
	err = server.Send(&pulumirpc.RunPluginResponse{
		Output: &pulumirpc.RunPluginResponse_Stdout{Stdout: fmt.Appendf(nil, "%d\n", handle.Port)},
	})
	if err == nil {
		<-server.Context().Done()
	}
	close(stop)
	return err
}

// A provider that the host boots from a path must receive exactly one handshake, and that handshake
// must carry the addresses of the plugin context.
func TestProviderFromPathHandshakesOnce(t *testing.T) {
	t.Parallel()

	recorder := &handshakeRecorder{}
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, &pluginLanguageHost{provider: recorder})
			return nil
		},
	})
	require.NoError(t, err)
	conn, err := grpc.NewClient(
		fmt.Sprintf("127.0.0.1:%d", handle.Port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	host := &testHost{
		engine:           &languageTestServer{addr: "127.0.0.1:1"},
		runtime:          plugin.NewLanguageRuntimeClient("mock", pulumirpc.NewLanguageRuntimeClient(conn)),
		runtimeName:      "mock",
		providers:        make(map[string]func() (plugin.Provider, error)),
		connections:      make(map[plugin.Provider]io.Closer),
		contextServices:  make(map[*plugin.Context][]*plugin.GrpcServer),
		contextProviders: make(map[*plugin.Context][]plugin.Provider),
	}
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	tempDir := t.TempDir()
	snk := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	pctx, err := plugin.NewContextWithRoot(
		t.Context(), snk, snk, host, tempDir, tempDir, nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, host.ReleaseContext(pctx))
		require.NoError(t, pctx.Close())
	})

	providerDir := filepath.Join(tempDir, "providers", "recorder")
	require.NoError(t, os.MkdirAll(providerDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(providerDir, "PulumiPlugin.yaml"), []byte("runtime: mock\n"), 0o600))

	host.providers["recorder@1.0.0"] = func() (plugin.Provider, error) {
		return host.ProviderFromPath(pctx, providerDir, env.Global())
	}
	version := semver.MustParse("1.0.0")
	_, err = host.Provider(pctx, workspace.PluginDescriptor{Name: "recorder", Version: &version}, env.Global())
	require.NoError(t, err)

	rootDir := filepath.Dir(providerDir)
	mapperAddr := pctx.MapperAddr()
	loaderAddr := pctx.LoaderAddr()
	resolverAddr := pctx.ResolverAddr()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	assert.Equal(t, []plugin.ProviderHandshakeRequest{{
		EngineAddress:               host.ServerAddr(),
		RootDirectory:               &rootDir,
		ProgramDirectory:            &rootDir,
		ConfigureWithUrn:            true,
		SupportsViews:               true,
		SupportsRefreshBeforeUpdate: true,
		InvokeWithPreview:           true,
		MapperTarget:                &mapperAddr,
		LoaderTarget:                &loaderAddr,
		ResolverTarget:              &resolverAddr,
		AcceptsByteString:           true,
		SendsOldOutputsToCheck:      true,
		AcceptsOutputsInInvoke:      true,
		AcceptsOutputsInCall:        true,
	}}, recorder.handshakes)
}

// The resolver that the host serves for a context resolves a package spec to one of the test's
// providers. A spec without a version resolves to the latest version of the named provider.
func TestResolverResolvesTestProviders(t *testing.T) {
	t.Parallel()

	v3 := semver.MustParse("3.0.0")
	host := &testHost{
		engine: &languageTestServer{addr: "127.0.0.1:1"},
		providers: map[string]func() (plugin.Provider, error){
			"simple@2.0.0": func() (plugin.Provider, error) { return &providers.SimpleProvider{}, nil },
			"simple@3.0.0": func() (plugin.Provider, error) { return &providers.SimpleProvider{Version: &v3}, nil },
		},
		connections:      make(map[plugin.Provider]io.Closer),
		contextServices:  make(map[*plugin.Context][]*plugin.GrpcServer),
		contextProviders: make(map[*plugin.Context][]plugin.Provider),
	}
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	tempDir := t.TempDir()
	snk := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	pctx, err := plugin.NewContextWithRoot(
		t.Context(), snk, snk, host, tempDir, tempDir, nil, false, nil, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, host.ReleaseContext(pctx))
		require.NoError(t, pctx.Close())
	})

	conn, err := grpc.NewClient(pctx.ResolverAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	resolver := pulumirpc.NewPackageResolverClient(conn)

	versioned, err := resolver.ResolvePackage(t.Context(), &pulumirpc.PackageSpec{Source: "simple", Version: "2.0.0"})
	require.NoError(t, err)
	assertProtoEqual(t, &pulumirpc.PackageDependency{
		Name:    "simple",
		Kind:    "resource",
		Version: "2.0.0",
	}, versioned)

	latest, err := resolver.ResolvePackage(t.Context(), &pulumirpc.PackageSpec{Source: "simple"})
	require.NoError(t, err)
	assertProtoEqual(t, &pulumirpc.PackageDependency{
		Name:    "simple",
		Kind:    "resource",
		Version: "3.0.0",
	}, latest)
}

func assertProtoEqual(t *testing.T, expected, actual proto.Message) {
	t.Helper()
	assert.Truef(t, proto.Equal(expected, actual), "expected: %v\nactual: %v", expected, actual)
}
