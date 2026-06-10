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

package plugin

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// TestHandshakePassesLoaderMapperResolver is an integration test that locks in the contract that, when the engine's
// host serves a schema loader, a provider mapper, and a package resolver, the addresses of all three are handed to a
// provider during Handshake -- and that each address is dialable as the service it claims to be.
//
// It exercises the real path: a host serving the three services, a provider attached over gRPC, and the
// engine->provider Handshake call that carries the three targets.
func TestHandshakePassesLoaderMapperResolver(t *testing.T) {
	// Not parallel: this test sets PULUMI_DEBUG_PROVIDERS via t.Setenv to attach to the fake provider.

	sink := diagtest.LogSink(t)

	// Construct a host that serves all three services on its own gRPC server. With all three constructors non-nil,
	// LoaderAddr/MapperAddr/PackageResolverAddr all resolve to the host's server address.
	newLoader := func(Host) codegenrpc.LoaderServer { return &fakeLoaderServer{} }
	newMapper := func(Host) codegenrpc.MapperServer { return &fakeMapperServer{} }
	newResolver := func(Host) pulumirpc.PackageResolverServer { return &fakeResolverServer{} }

	pctx, err := NewContext(t.Context(), sink, sink, nil, nil, "", nil, false, nil,
		newLoader, nil /* installLang */, newMapper, newResolver)
	require.NoError(t, err)
	host := pctx.Host
	t.Cleanup(func() { require.NoError(t, host.Close()) })

	require.NotEmpty(t, host.ServerAddr())
	require.Equal(t, host.ServerAddr(), host.LoaderAddr())
	require.Equal(t, host.ServerAddr(), host.MapperAddr())
	require.Equal(t, host.ServerAddr(), host.PackageResolverAddr())

	// Stand up a fake provider that records the Handshake request it receives, and serve it over gRPC.
	var mu sync.Mutex
	var got *ProviderHandshakeRequest
	mock := &MockProvider{
		HandshakeF: func(_ context.Context, req ProviderHandshakeRequest) (*ProviderHandshakeResponse, error) {
			mu.Lock()
			defer mu.Unlock()
			got = &req
			return &ProviderHandshakeResponse{
				AcceptSecrets:   true,
				AcceptResources: true,
				AcceptOutputs:   true,
			}, nil
		},
		GetPluginInfoF: func(context.Context) (PluginInfo, error) {
			v := semver.MustParse("1.0.0")
			return PluginInfo{Version: &v}, nil
		},
	}

	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, &attachableProviderServer{NewProviderServer(mock)})
			return nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { close(cancel); <-handle.Done })

	// Attach the host to the fake provider so that loading it triggers a Handshake against our server.
	const pkg = "handshaketest"
	t.Setenv("PULUMI_DEBUG_PROVIDERS", fmt.Sprintf("%s:%d", pkg, handle.Port))

	_, err = host.Provider(workspace.PluginDescriptor{Name: pkg, Kind: apitype.ResourcePlugin}, env.Global())
	require.NoError(t, err)

	// The provider must have been handed all three service addresses, each pointing at the host's server.
	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, got, "provider did not receive a Handshake")
	assert.Equal(t, ProviderHandshakeRequest{
		EngineAddress:               host.ServerAddr(),
		ConfigureWithUrn:            true,
		SupportsViews:               true,
		SupportsRefreshBeforeUpdate: supportsRefreshBeforeUpdate,
		InvokeWithPreview:           true,
		LoaderTarget:                host.ServerAddr(),
		MapperTarget:                host.ServerAddr(),
		PackageResolverTarget:       host.ServerAddr(),
	}, *got)

	// Each target must be dialable as the service it claims to be. We dial the reported targets directly (rather than
	// the known host address) to prove the addresses handed to the provider are correct and live.
	conn, err := grpc.NewClient(got.LoaderTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()), rpcutil.GrpcChannelOptions())
	require.NoError(t, err)
	defer conn.Close()

	loaderResp, err := codegenrpc.NewLoaderClient(conn).GetSchema(t.Context(), &codegenrpc.GetSchemaRequest{})
	require.NoError(t, err)
	assert.Equal(t, []byte(fakeLoaderSchema), loaderResp.Schema)

	mapperResp, err := codegenrpc.NewMapperClient(conn).GetMapping(t.Context(), &codegenrpc.GetMappingRequest{})
	require.NoError(t, err)
	assert.Equal(t, []byte(fakeMapperData), mapperResp.Data)

	resolverResp, err := pulumirpc.NewPackageResolverClient(conn).ResolvePackage(
		t.Context(), &pulumirpc.ResolvePackageRequest{Spec: &pulumirpc.PackageSpec{Source: "irrelevant"}})
	require.NoError(t, err)
	assert.Equal(t, fakeResolvedName, resolverResp.GetPackage().GetName())
}

// attachableProviderServer adds a no-op Attach to a ResourceProviderServer. NewProviderServer leaves Attach
// unimplemented (a built provider is attached *to*, it does not attach), but the engine's attach path calls Attach to
// hand the provider the engine address after Handshake, so the fake provider must accept it.
type attachableProviderServer struct {
	pulumirpc.ResourceProviderServer
}

func (*attachableProviderServer) Attach(context.Context, *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

const (
	fakeLoaderSchema = `{"name":"fake-loader"}`
	fakeMapperData   = "fake-mapper-data"
	fakeResolvedName = "fake-resolved-package"
)

type fakeLoaderServer struct {
	codegenrpc.UnimplementedLoaderServer
}

func (*fakeLoaderServer) GetSchema(
	context.Context, *codegenrpc.GetSchemaRequest,
) (*codegenrpc.GetSchemaResponse, error) {
	return &codegenrpc.GetSchemaResponse{Schema: []byte(fakeLoaderSchema)}, nil
}

type fakeMapperServer struct {
	codegenrpc.UnimplementedMapperServer
}

func (*fakeMapperServer) GetMapping(
	context.Context, *codegenrpc.GetMappingRequest,
) (*codegenrpc.GetMappingResponse, error) {
	return &codegenrpc.GetMappingResponse{Data: []byte(fakeMapperData)}, nil
}

type fakeResolverServer struct {
	pulumirpc.UnimplementedPackageResolverServer
}

func (*fakeResolverServer) ResolvePackage(
	context.Context, *pulumirpc.ResolvePackageRequest,
) (*pulumirpc.ResolvePackageResponse, error) {
	return &pulumirpc.ResolvePackageResponse{Package: &pulumirpc.PackageDependency{Name: fakeResolvedName}}, nil
}
