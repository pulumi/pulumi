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
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type componentProvider struct {
	host      *HostClient
	name      string
	version   string
	schema    []byte
	construct provider.ConstructFunc
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
func (p *componentProvider) GetPluginInfo(context.Context, *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

// GetSchema returns the JSON-encoded schema for this provider's package.
func (p *componentProvider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	if v := req.GetVersion(); v != 0 {
		return nil, errors.Errorf("unsupported schema version %d", v)
	}
	schema := string(p.schema)
	if schema == "" {
		schema = "{}"
	}
	return &pulumirpc.GetSchemaResponse{Schema: schema}, nil
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *componentProvider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
	}, nil
}

// Construct creates a new instance of the provided component resource and returns its state.
func (p *componentProvider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return provider.Construct(ctx, req, p.host.conn, p.construct)
}

// CheckConfig validates the configuration for this provider.
func (p *componentProvider) CheckConfig(ctx context.Context,
	req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")
}

// DiffConfig diffs the configuration for this provider.
func (p *componentProvider) DiffConfig(ctx context.Context,
	req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

// StreamInvoke dynamically executes a built-in function in the provider. The result is streamed
// back as a series of messages.
func (p *componentProvider) StreamInvoke(req *pulumirpc.InvokeRequest,
	server pulumirpc.ResourceProvider_StreamInvokeServer) error {
	return status.Error(codes.Unimplemented, "StreamInvoke is not yet implemented")
}

// Check validates that the given property bag is valid for a resource of the given type and returns
// the inputs that should be passed to successive calls to Diff, Create, or Update for this
// resource. As a rule, the provider inputs returned by a call to Check should preserve the original
// representation of the properties as present in the program inputs. Though this rule is not
// required for correctness, violations thereof can negatively impact the end-user experience, as
// the provider inputs are using for detecting and rendering diffs.
func (p *componentProvider) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Check is not yet implemented")
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *componentProvider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Diff is not yet implemented")
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.
// (The input ID must be blank.)  If this call fails, the resource must not have been created (i.e.,
// it is "transactional").
func (p *componentProvider) Create(ctx context.Context,
	req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Create is not yet implemented")
}

// Read the current live state associated with a resource.  Enough state must be include in the
// inputs to uniquely identify the resource; this is typically just the resource ID, but may also
// include some properties.
func (p *componentProvider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Read is not yet implemented")
}

// Update updates an existing resource with new values.
func (p *componentProvider) Update(ctx context.Context,
	req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Update is not yet implemented")
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed
// to still exist.
func (p *componentProvider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*pbempty.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "Delete is not yet implemented")
}

// Invoke dynamically executes a built-in function in the provider.
func (p *componentProvider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Invoke is not yet implemented")
}

// Call dynamically executes a method in the provider associated with a component resource.
func (p *componentProvider) Call(ctx context.Context,
	req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either a
// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
// to the host to decide how long to wait after Cancel is called before (e.g.)
// hard-closing any gRPC connection.
func (p *componentProvider) Cancel(context.Context, *pbempty.Empty) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}
