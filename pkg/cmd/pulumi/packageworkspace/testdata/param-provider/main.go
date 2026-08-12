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

// param-provider is a test resource provider that supports parameterization. It stands in for a
// bridged provider (such as a Terraform provider): resolving it with parameters yields a distinct
// package, and its schema reports the parameterization that produced that package. It is used to
// verify that the engine's package resolver surfaces parameterization in the dependency it returns.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// paramValue is the parameter the provider produces for the requested package. The resolver reads
// this back out of the schema, so the test asserts on it.
const paramValue = "random-param-value"

type paramProvider struct {
	pulumirpc.UnimplementedResourceProviderServer
}

func (p *paramProvider) Handshake(
	ctx context.Context, req *pulumirpc.ProviderHandshakeRequest,
) (*pulumirpc.ProviderHandshakeResponse, error) {
	return &pulumirpc.ProviderHandshakeResponse{}, nil
}

func (p *paramProvider) GetPluginInfo(ctx context.Context, req *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "1.0.0"}, nil
}

func (p *paramProvider) Parameterize(
	ctx context.Context, req *pulumirpc.ParameterizeRequest,
) (*pulumirpc.ParameterizeResponse, error) {
	return &pulumirpc.ParameterizeResponse{Name: "random", Version: "3.0.0"}, nil
}

func (p *paramProvider) GetSchema(
	ctx context.Context, req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	schema, err := json.Marshal(map[string]any{
		"name":    req.SubpackageName,
		"version": req.SubpackageVersion,
		"parameterization": map[string]any{
			"baseProvider": map[string]any{"name": "paramtest", "version": "1.0.0"},
			"parameter":    []byte(paramValue),
		},
	})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.GetSchemaResponse{Schema: string(schema)}, nil
}

func main() {
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceProviderServer(srv, &paramProvider{})
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
