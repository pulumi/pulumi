// Copyright 2016-2021, Pulumi Corporation.
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

package provider

import (
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type componentProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host      *HostClient
	name      string
	version   string
	schema    []byte
	construct provider.ConstructFunc
	call      provider.CallFunc
}

type Options struct {
	Name      string
	Version   string
	Schema    []byte
	Construct provider.ConstructFunc
	Call      provider.CallFunc
}

// MainWithOptions is an entrypoint for a resource provider plugin that implements `Construct` and optionally also
// `Call` for component resources.
//
// Using it isn't required but can cut down significantly on the amount of boilerplate necessary to fire up a new
// resource provider for components.
func MainWithOptions(opts Options) error {
	return Main(opts.Name, func(host *HostClient) (pulumirpc.ResourceProviderServer, error) {
		return &componentProvider{
			host:      host,
			name:      opts.Name,
			version:   opts.Version,
			schema:    opts.Schema,
			construct: opts.Construct,
			call:      opts.Call,
		}, nil
	})
}

// ComponentMain is an entrypoint for a resource provider plugin that implements `Construct` for component resources.
// Using it isn't required but can cut down significantly on the amount of boilerplate necessary to fire up a new
// resource provider for components.
func ComponentMain(name, version string, schema []byte, construct provider.ConstructFunc) error {
	return Main(name, func(host *HostClient) (pulumirpc.ResourceProviderServer, error) {
		return &componentProvider{
			host:      host,
			name:      name,
			version:   version,
			schema:    schema,
			construct: construct,
		}, nil
	})
}

// GetPluginInfo returns generic information about this plugin, like its version.
func (p *componentProvider) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

// GetSchema returns the JSON-encoded schema for this provider's package.
func (p *componentProvider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	if v := req.GetVersion(); v != 0 {
		return nil, fmt.Errorf("unsupported schema version %d", v)
	}
	schema := string(p.schema)
	if schema == "" {
		schema = "{}"
	}
	return &pulumirpc.GetSchemaResponse{Schema: schema}, nil
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *componentProvider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
		AcceptOutputs:   true,
	}, nil
}

// Construct creates a new instance of the provided component resource and returns its state.
func (p *componentProvider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	if p.construct != nil {
		return provider.Construct(ctx, req, p.host.conn, p.construct)
	}
	return nil, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

// Call dynamically executes a method in the provider associated with a component resource.
func (p *componentProvider) Call(ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	if p.call != nil {
		return provider.Call(ctx, req, p.host.conn, p.call)
	}
	return nil, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either a
// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
// to the host to decide how long to wait after Cancel is called before (e.g.)
// hard-closing any gRPC connection.
func (p *componentProvider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Attach attaches to the engine for an already running provider.
func (p *componentProvider) Attach(ctx context.Context,
	req *pulumirpc.PluginAttach,
) (*emptypb.Empty, error) {
	host, err := NewHostClient(req.GetAddress())
	if err != nil {
		return nil, err
	}
	p.host = host
	return &emptypb.Empty{}, nil
}

// GetMapping fetches the conversion mapping (if any) for this resource provider.
func (p *componentProvider) GetMapping(ctx context.Context,
	req *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	return &pulumirpc.GetMappingResponse{Provider: "", Data: nil}, nil
}
