// Copyright 2016-2022, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NotForwardCompatible can be embedded to explicitly opt out of forward compatibility.
//
// Either NotForwardCompatibleProvider or [UnimplementedProvider] must be embedded to
// implement [Provider].
type NotForwardCompatibleProvider struct{}

// UnimplementedProvider can be embedded to have a forward compatible implementation of
// [Provider].
//
// Either NotForwardCompatibleProvider or [UnimplementedProvider] must be embedded to
// implement [Provider].
type UnimplementedProvider struct{ NotForwardCompatibleProvider }

func (p *UnimplementedProvider) Handshake(
	context.Context, ProviderHandshakeRequest,
) (*ProviderHandshakeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Handshake is not yet implemented")
}

func (p *UnimplementedProvider) Close() error {
	return status.Error(codes.Unimplemented, "Close is not yet implemented")
}

func (p *UnimplementedProvider) SignalCancellation(context.Context) error {
	return status.Error(codes.Unimplemented, "SignalCancellation is not yet implemented")
}

func (p *UnimplementedProvider) Pkg() tokens.Package {
	return tokens.Package("")
}

func (p *UnimplementedProvider) Parameterize(context.Context, ParameterizeRequest) (ParameterizeResponse, error) {
	return ParameterizeResponse{}, status.Error(codes.Unimplemented, "Parameterize is not yet implemented")
}

func (p *UnimplementedProvider) GetSchema(context.Context, GetSchemaRequest) (GetSchemaResponse, error) {
	return GetSchemaResponse{}, status.Error(codes.Unimplemented, "GetSchema is not yet implemented")
}

func (p *UnimplementedProvider) CheckConfig(context.Context, CheckConfigRequest) (CheckConfigResponse, error) {
	return CheckConfigResponse{}, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")
}

func (p *UnimplementedProvider) DiffConfig(context.Context, DiffConfigRequest) (DiffConfigResponse, error) {
	return DiffResult{}, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

func (p *UnimplementedProvider) Configure(context.Context, ConfigureRequest) (ConfigureResponse, error) {
	return ConfigureResponse{}, status.Error(codes.Unimplemented, "Configure is not yet implemented")
}

func (p *UnimplementedProvider) Check(context.Context, CheckRequest) (CheckResponse, error) {
	return CheckResponse{}, status.Error(codes.Unimplemented, "Check is not yet implemented")
}

func (p *UnimplementedProvider) Diff(context.Context, DiffRequest) (DiffResponse, error) {
	return DiffResponse{}, status.Error(codes.Unimplemented, "Diff is not yet implemented")
}

func (p *UnimplementedProvider) Create(context.Context, CreateRequest) (CreateResponse, error) {
	return CreateResponse{}, status.Error(codes.Unimplemented, "Create is not yet implemented")
}

func (p *UnimplementedProvider) Read(context.Context, ReadRequest) (ReadResponse, error) {
	return ReadResponse{}, status.Error(codes.Unimplemented, "Read is not yet implemented")
}

func (p *UnimplementedProvider) Update(context.Context, UpdateRequest) (UpdateResponse, error) {
	return UpdateResponse{}, status.Error(codes.Unimplemented, "Update is not yet implemented")
}

func (p *UnimplementedProvider) Delete(context.Context, DeleteRequest) (DeleteResponse, error) {
	return DeleteResponse{}, status.Error(codes.Unimplemented, "Delete is not yet implemented")
}

func (p *UnimplementedProvider) Construct(context.Context, ConstructRequest) (ConstructResponse, error) {
	return ConstructResponse{}, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

func (p *UnimplementedProvider) Invoke(context.Context, InvokeRequest) (InvokeResponse, error) {
	return InvokeResponse{}, status.Error(codes.Unimplemented, "Invoke is not yet implemented")
}

func (p *UnimplementedProvider) Call(context.Context, CallRequest) (CallResponse, error) {
	return CallResponse{}, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

func (p *UnimplementedProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	return workspace.PluginInfo{}, status.Error(codes.Unimplemented, "GetPluginInfo is not yet implemented")
}

func (p *UnimplementedProvider) GetMapping(context.Context, GetMappingRequest) (GetMappingResponse, error) {
	return GetMappingResponse{}, status.Error(codes.Unimplemented, "GetMapping is not yet implemented")
}

func (p *UnimplementedProvider) GetMappings(context.Context, GetMappingsRequest) (GetMappingsResponse, error) {
	return GetMappingsResponse{}, status.Error(codes.Unimplemented, "GetMappings is not yet implemented")
}

func (p NotForwardCompatibleProvider) mustEmbedAForwardCompatibilityOption(
	UnimplementedProvider, NotForwardCompatibleProvider) {
}
