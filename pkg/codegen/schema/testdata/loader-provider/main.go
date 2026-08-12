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

// loader-provider is a test resource provider used to verify that the engine sends a working loader target as part
// of the provider handshake, and that the loader stays usable across the provider's lifecycle. It plays both sides
// of the loading flow: it serves its own schema via GetSchema, and its Create method dials the loader service
// received during the handshake to load that same schema back.
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
	"github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// schemaJSON is the schema this provider both advertises via GetSchema and loads back through the handshake loader.
// It carries a top-level version so the loader returns it verbatim rather than defaulting the version.
const schemaJSON = `{"name":"loadtest","version":"1.0.0"}`

type loaderProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	loader codegen.LoaderClient
}

func (p *loaderProvider) Handshake(
	ctx context.Context, req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	if req.LoaderTarget == nil || *req.LoaderTarget == "" {
		return nil, fmt.Errorf("no loader target received during handshake")
	}

	conn, err := grpc.NewClient(*req.LoaderTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial loader at %v: %w", *req.LoaderTarget, err)
	}

	p.loader = codegen.NewLoaderClient(conn)
	return &pulumirpc.ProviderHandshakeResponse{}, nil
}

func (p *loaderProvider) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

func (p *loaderProvider) Configure(
	ctx context.Context, req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{}, nil
}

func (p *loaderProvider) GetSchema(
	ctx context.Context, req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{Schema: schemaJSON}, nil
}

func (p *loaderProvider) Create(
	ctx context.Context, req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	res, err := p.loader.GetSchema(ctx, &codegen.GetSchemaRequest{Package: "loadtest", Version: "1.0.0"})
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}

	props, err := structpb.NewStruct(map[string]any{"schema": string(res.Schema)})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.CreateResponse{Id: "created", Properties: props}, nil
}

func main() {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, &loaderProvider{})
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
