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

// resolver-provider is a test resource provider used to verify that the engine sends a working
// package-resolver target as part of the provider handshake. On Create it dials the resolver service
// received during the handshake and asks it to resolve the package named by the "source" input,
// returning the resolved dependency's coordinates as the created resource's state.
package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type resolverProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	resolver pulumirpc.PackageResolverClient
}

func (p *resolverProvider) Handshake(
	ctx context.Context, req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	if req.ResolverTarget == nil || *req.ResolverTarget == "" {
		return nil, fmt.Errorf("no resolver target received during handshake")
	}

	conn, err := grpc.NewClient(*req.ResolverTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial resolver at %v: %w", *req.ResolverTarget, err)
	}

	p.resolver = pulumirpc.NewPackageResolverClient(conn)
	return &pulumirpc.ProviderHandshakeResponse{}, nil
}

func (p *resolverProvider) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

func (p *resolverProvider) Configure(
	ctx context.Context, req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{}, nil
}

func (p *resolverProvider) Create(
	ctx context.Context, req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	source := req.Properties.Fields["source"].GetStringValue()

	dep, err := p.resolver.ResolvePackage(ctx, &pulumirpc.PackageSpec{Source: source})
	if err != nil {
		return nil, fmt.Errorf("resolve package %q: %w", source, err)
	}

	props, err := structpb.NewStruct(map[string]any{
		"name":    dep.Name,
		"kind":    dep.Kind,
		"version": dep.Version,
		"server":  dep.Server,
	})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.CreateResponse{Id: "created", Properties: props}, nil
}

func main() {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, &resolverProvider{})
			return nil
		},
	})
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
}
