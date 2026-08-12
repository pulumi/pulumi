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

// mapper-provider is a test resource provider used to verify that the engine sends a working mapper target as part
// of the provider handshake. It plays both sides of the mapping flow: it advertises a Terraform mapping for the
// source provider "sometf", and its `mapptest:index:getMapping` invoke dials the mapper service received during the
// handshake to retrieve that same mapping.
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
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

type mapperProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	mapper codegen.MapperClient
}

func (p *mapperProvider) Handshake(
	ctx context.Context, req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	if req.MapperTarget == nil || *req.MapperTarget == "" {
		return nil, fmt.Errorf("no mapper target received during handshake")
	}

	conn, err := grpc.NewClient(*req.MapperTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial mapper at %v: %w", *req.MapperTarget, err)
	}

	p.mapper = codegenrpc.NewMapperClient(conn)
	return &pulumirpc.ProviderHandshakeResponse{}, nil
}

func (p *mapperProvider) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

func (p *mapperProvider) Configure(
	ctx context.Context, req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{}, nil
}

func (p *mapperProvider) GetMappings(
	ctx context.Context, req *pulumirpc.GetMappingsRequest,
) (*pulumirpc.GetMappingsResponse, error) {
	if req.Key != "terraform" {
		return &pulumirpc.GetMappingsResponse{}, nil
	}
	return &pulumirpc.GetMappingsResponse{Providers: []string{"sometf"}}, nil
}

func (p *mapperProvider) GetMapping(
	ctx context.Context, req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	if req.Key == "terraform" && req.Provider == "sometf" {
		return &pulumirpc.GetMappingResponse{Provider: "sometf", Data: []byte(`{"hello":"world"}`)}, nil
	}
	return &pulumirpc.GetMappingResponse{}, nil
}

func (p *mapperProvider) Invoke(
	ctx context.Context, req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	if req.Tok != "mapptest:index:getMapping" {
		return nil, fmt.Errorf("unknown function %v", req.Tok)
	}

	res, err := p.mapper.GetMapping(ctx, &codegenrpc.GetMappingRequest{Provider: "sometf"})
	if err != nil {
		return nil, fmt.Errorf("get mapping: %w", err)
	}

	ret, err := structpb.NewStruct(map[string]any{"mapping": string(res.Data)})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.InvokeResponse{Return: ret}, nil
}

func main() {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, &mapperProvider{})
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
