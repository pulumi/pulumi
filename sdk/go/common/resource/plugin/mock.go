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

package plugin

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type MockHost struct {
	ServerAddrF         func() string
	LoaderAddrF         func() string
	LogF                func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
	LogStatusF          func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
	AnalyzerF           func(nm tokens.QName) (Analyzer, error)
	PolicyAnalyzerF     func(name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error)
	ProviderF           func(descriptor workspace.PluginDescriptor, e env.Env) (Provider, error)
	LanguageRuntimeF    func(runtime string) (LanguageRuntime, error)
	ResolvePluginF      func(spec workspace.PluginDescriptor) (*workspace.PluginInfo, error)
	GetProjectPluginsF  func() []workspace.ProjectPlugin
	SignalCancellationF func() error
	CloseF              func() error
	StartDebuggingF     func(info DebuggingInfo) error
	AttachDebuggerF     func(spec DebugSpec) bool
}

var _ Host = (*MockHost)(nil)

func (m *MockHost) ServerAddr() string {
	if m.ServerAddrF != nil {
		return m.ServerAddrF()
	}
	return ""
}

func (m *MockHost) LoaderAddr() string {
	if m.LoaderAddrF != nil {
		return m.LoaderAddrF()
	}
	return ""
}

func (m *MockHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if m.LogF != nil {
		m.LogF(sev, urn, msg, streamID)
	}
}

func (m *MockHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if m.LogStatusF != nil {
		m.LogStatusF(sev, urn, msg, streamID)
	}
}

func (m *MockHost) Analyzer(nm tokens.QName) (Analyzer, error) {
	if m.AnalyzerF != nil {
		return m.AnalyzerF(nm)
	}
	return nil, status.Error(codes.Unimplemented, "Analyzer not implemented")
}

func (m *MockHost) PolicyAnalyzer(name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error) {
	if m.PolicyAnalyzerF != nil {
		return m.PolicyAnalyzerF(name, path, opts)
	}
	return nil, status.Error(codes.Unimplemented, "PolicyAnalyzer not implemented")
}

func (m *MockHost) Provider(descriptor workspace.PluginDescriptor, e env.Env) (Provider, error) {
	if m.ProviderF != nil {
		return m.ProviderF(descriptor, e)
	}
	return nil, status.Error(codes.Unimplemented, "Provider not implemented")
}

func (m *MockHost) LanguageRuntime(runtime string) (LanguageRuntime, error) {
	if m.LanguageRuntimeF != nil {
		return m.LanguageRuntimeF(runtime)
	}
	return nil, status.Error(codes.Unimplemented, "LanguageRuntime not implemented")
}

func (m *MockHost) ResolvePlugin(
	spec workspace.PluginDescriptor,
) (*workspace.PluginInfo, error) {
	if m.ResolvePluginF != nil {
		return m.ResolvePluginF(spec)
	}
	return nil, status.Error(codes.Unimplemented, "ResolvePlugin not implemented")
}

func (m *MockHost) GetProjectPlugins() []workspace.ProjectPlugin {
	if m.GetProjectPluginsF != nil {
		return m.GetProjectPluginsF()
	}
	return nil
}

func (m *MockHost) SignalCancellation() error {
	if m.SignalCancellationF != nil {
		return m.SignalCancellationF()
	}
	return nil
}

func (m *MockHost) Close() error {
	if m.CloseF != nil {
		return m.CloseF()
	}
	return nil
}

func (m *MockHost) StartDebugging(info DebuggingInfo) error {
	if m.StartDebuggingF != nil {
		return m.StartDebuggingF(info)
	}
	return nil
}

func (m *MockHost) AttachDebugger(spec DebugSpec) bool {
	if m.AttachDebuggerF != nil {
		return m.AttachDebuggerF(spec)
	}
	return false
}

type MockProvider struct {
	NotForwardCompatibleProvider

	CloseF              func() error
	PkgF                func() tokens.Package
	HandshakeF          func(context.Context, ProviderHandshakeRequest) (*ProviderHandshakeResponse, error)
	ParameterizeF       func(context.Context, ParameterizeRequest) (ParameterizeResponse, error)
	GetSchemaF          func(context.Context, GetSchemaRequest) (GetSchemaResponse, error)
	CheckConfigF        func(context.Context, CheckConfigRequest) (CheckConfigResponse, error)
	DiffConfigF         func(context.Context, DiffConfigRequest) (DiffConfigResponse, error)
	ConfigureF          func(context.Context, ConfigureRequest) (ConfigureResponse, error)
	CheckF              func(context.Context, CheckRequest) (CheckResponse, error)
	DiffF               func(context.Context, DiffRequest) (DiffResponse, error)
	CreateF             func(context.Context, CreateRequest) (CreateResponse, error)
	ReadF               func(context.Context, ReadRequest) (ReadResponse, error)
	UpdateF             func(context.Context, UpdateRequest) (UpdateResponse, error)
	DeleteF             func(context.Context, DeleteRequest) (DeleteResponse, error)
	ListF               func(context.Context, ListRequest) (*ListStream, error)
	ConstructF          func(context.Context, ConstructRequest) (ConstructResponse, error)
	InvokeF             func(context.Context, InvokeRequest) (InvokeResponse, error)
	CallF               func(context.Context, CallRequest) (CallResponse, error)
	GetPluginInfoF      func(context.Context) (PluginInfo, error)
	SignalCancellationF func(context.Context) error
	GetMappingF         func(context.Context, GetMappingRequest) (GetMappingResponse, error)
	GetMappingsF        func(context.Context, GetMappingsRequest) (GetMappingsResponse, error)
}

var _ Provider = (*MockProvider)(nil)

func (m *MockProvider) Close() error {
	if m.CloseF != nil {
		return m.CloseF()
	}
	return nil
}

func (m *MockProvider) Pkg() tokens.Package {
	if m.PkgF != nil {
		return m.PkgF()
	}
	return ""
}

func (m *MockProvider) Handshake(
	ctx context.Context, req ProviderHandshakeRequest,
) (*ProviderHandshakeResponse, error) {
	if m.HandshakeF != nil {
		return m.HandshakeF(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "Handshake not implemented")
}

func (m *MockProvider) Parameterize(ctx context.Context, req ParameterizeRequest) (ParameterizeResponse, error) {
	if m.ParameterizeF != nil {
		return m.ParameterizeF(ctx, req)
	}
	return ParameterizeResponse{}, status.Error(codes.Unimplemented, "Parameterize not implemented")
}

func (m *MockProvider) GetSchema(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
	if m.GetSchemaF != nil {
		return m.GetSchemaF(ctx, req)
	}
	return GetSchemaResponse{}, status.Error(codes.Unimplemented, "GetSchema not implemented")
}

func (m *MockProvider) CheckConfig(ctx context.Context, req CheckConfigRequest) (CheckConfigResponse, error) {
	if m.CheckConfigF != nil {
		return m.CheckConfigF(ctx, req)
	}
	return CheckConfigResponse{}, status.Error(codes.Unimplemented, "CheckConfig not implemented")
}

func (m *MockProvider) DiffConfig(ctx context.Context, req DiffConfigRequest) (DiffConfigResponse, error) {
	if m.DiffConfigF != nil {
		return m.DiffConfigF(ctx, req)
	}
	return DiffConfigResponse{}, status.Error(codes.Unimplemented, "DiffConfig not implemented")
}

func (m *MockProvider) Configure(ctx context.Context, req ConfigureRequest) (ConfigureResponse, error) {
	if m.ConfigureF != nil {
		return m.ConfigureF(ctx, req)
	}
	return ConfigureResponse{}, status.Error(codes.Unimplemented, "Configure not implemented")
}

func (m *MockProvider) Check(ctx context.Context, req CheckRequest) (CheckResponse, error) {
	if m.CheckF != nil {
		return m.CheckF(ctx, req)
	}
	return CheckResponse{}, status.Error(codes.Unimplemented, "Check not implemented")
}

func (m *MockProvider) Diff(ctx context.Context, req DiffRequest) (DiffResponse, error) {
	if m.DiffF != nil {
		return m.DiffF(ctx, req)
	}
	return DiffResponse{}, status.Error(codes.Unimplemented, "Diff not implemented")
}

func (m *MockProvider) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	if m.CreateF != nil {
		return m.CreateF(ctx, req)
	}
	return CreateResponse{}, status.Error(codes.Unimplemented, "Create not implemented")
}

func (m *MockProvider) Read(ctx context.Context, req ReadRequest) (ReadResponse, error) {
	if m.ReadF != nil {
		return m.ReadF(ctx, req)
	}
	return ReadResponse{}, status.Error(codes.Unimplemented, "Read not implemented")
}

func (m *MockProvider) Update(ctx context.Context, req UpdateRequest) (UpdateResponse, error) {
	if m.UpdateF != nil {
		return m.UpdateF(ctx, req)
	}
	return UpdateResponse{}, status.Error(codes.Unimplemented, "Update not implemented")
}

func (m *MockProvider) Delete(ctx context.Context, req DeleteRequest) (DeleteResponse, error) {
	if m.DeleteF != nil {
		return m.DeleteF(ctx, req)
	}
	return DeleteResponse{}, status.Error(codes.Unimplemented, "Delete not implemented")
}

func (m *MockProvider) List(ctx context.Context, req ListRequest) (*ListStream, error) {
	if m.ListF != nil {
		return m.ListF(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "List not implemented")
}

func (m *MockProvider) Construct(ctx context.Context, req ConstructRequest) (ConstructResponse, error) {
	if m.ConstructF != nil {
		return m.ConstructF(ctx, req)
	}
	return ConstructResponse{}, status.Error(codes.Unimplemented, "Construct not implemented")
}

func (m *MockProvider) Invoke(ctx context.Context, req InvokeRequest) (InvokeResponse, error) {
	if m.InvokeF != nil {
		return m.InvokeF(ctx, req)
	}
	return InvokeResponse{}, status.Error(codes.Unimplemented, "Invoke not implemented")
}

func (m *MockProvider) Call(ctx context.Context, req CallRequest) (CallResponse, error) {
	if m.CallF != nil {
		return m.CallF(ctx, req)
	}
	return CallResponse{}, status.Error(codes.Unimplemented, "Call not implemented")
}

func (m *MockProvider) GetPluginInfo(ctx context.Context) (PluginInfo, error) {
	if m.GetPluginInfoF != nil {
		return m.GetPluginInfoF(ctx)
	}
	return PluginInfo{}, status.Error(codes.Unimplemented, "GetPluginInfo not implemented")
}

func (m *MockProvider) SignalCancellation(ctx context.Context) error {
	if m.SignalCancellationF != nil {
		return m.SignalCancellationF(ctx)
	}
	return nil
}

func (m *MockProvider) GetMapping(ctx context.Context, req GetMappingRequest) (GetMappingResponse, error) {
	if m.GetMappingF != nil {
		return m.GetMappingF(ctx, req)
	}
	return GetMappingResponse{}, status.Error(codes.Unimplemented, "GetMapping not implemented")
}

func (m *MockProvider) GetMappings(ctx context.Context, req GetMappingsRequest) (GetMappingsResponse, error) {
	if m.GetMappingsF != nil {
		return m.GetMappingsF(ctx, req)
	}
	return GetMappingsResponse{}, status.Error(codes.Unimplemented, "GetMappings not implemented")
}

type MockConverter struct {
	CloseF          func() error
	ConvertStateF   func(context.Context, *ConvertStateRequest) (*ConvertStateResponse, error)
	ConvertProgramF func(context.Context, *ConvertProgramRequest) (*ConvertProgramResponse, error)
	ConvertSnippetF func(context.Context, *ConvertSnippetRequest) (*ConvertSnippetResponse, error)
}

var _ Converter = (*MockConverter)(nil)

func (m *MockConverter) Close() error {
	if m.CloseF != nil {
		return m.CloseF()
	}
	return nil
}

func (m *MockConverter) ConvertState(ctx context.Context, req *ConvertStateRequest) (*ConvertStateResponse, error) {
	if m.ConvertStateF != nil {
		return m.ConvertStateF(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "ConvertState not implemented")
}

func (m *MockConverter) ConvertProgram(
	ctx context.Context, req *ConvertProgramRequest,
) (*ConvertProgramResponse, error) {
	if m.ConvertProgramF != nil {
		return m.ConvertProgramF(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "ConvertProgram not implemented")
}

func (m *MockConverter) ConvertSnippet(
	ctx context.Context, req *ConvertSnippetRequest,
) (*ConvertSnippetResponse, error) {
	if m.ConvertSnippetF != nil {
		return m.ConvertSnippetF(ctx, req)
	}
	return nil, status.Error(codes.Unimplemented, "ConvertSnippet not implemented")
}
