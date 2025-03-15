// Copyright 2016-2018, Pulumi Corporation.
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

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type MockHost struct {
	ServerAddrF         func() string
	LogF                func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
	LogStatusF          func(sev diag.Severity, urn resource.URN, msg string, streamID int32)
	AnalyzerF           func(nm tokens.QName) (Analyzer, error)
	PolicyAnalyzerF     func(name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error)
	ListAnalyzersF      func() []Analyzer
	ProviderF           func(descriptor workspace.PackageDescriptor) (Provider, error)
	CloseProviderF      func(provider Provider) error
	LanguageRuntimeF    func(runtime string, info ProgramInfo) (LanguageRuntime, error)
	EnsurePluginsF      func(plugins []workspace.PluginSpec, kinds Flags) error
	ResolvePluginF      func(spec workspace.PluginSpec) (*workspace.PluginInfo, error)
	GetProjectPluginsF  func() []workspace.ProjectPlugin
	SignalCancellationF func() error
	CloseF              func() error
	StartDebuggingF     func(DebuggingInfo) error
}

var _ Host = (*MockHost)(nil)

func (m *MockHost) ServerAddr() string {
	if m.ServerAddrF != nil {
		return m.ServerAddrF()
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
	return nil, errors.New("Analyzer not implemented")
}

func (m *MockHost) PolicyAnalyzer(name tokens.QName, path string, opts *PolicyAnalyzerOptions) (Analyzer, error) {
	if m.PolicyAnalyzerF != nil {
		return m.PolicyAnalyzerF(name, path, opts)
	}
	return nil, errors.New("PolicyAnalyzer not implemented")
}

func (m *MockHost) ListAnalyzers() []Analyzer {
	if m.ListAnalyzersF != nil {
		return m.ListAnalyzersF()
	}
	return nil
}

func (m *MockHost) Provider(descriptor workspace.PackageDescriptor) (Provider, error) {
	if m.ProviderF != nil {
		return m.ProviderF(descriptor)
	}
	return nil, errors.New("Provider not implemented")
}

func (m *MockHost) CloseProvider(provider Provider) error {
	if m.CloseProviderF != nil {
		return m.CloseProviderF(provider)
	}
	return nil
}

func (m *MockHost) LanguageRuntime(runtime string, info ProgramInfo) (LanguageRuntime, error) {
	if m.LanguageRuntimeF != nil {
		return m.LanguageRuntimeF(runtime, info)
	}
	return nil, errors.New("LanguageRuntime not implemented")
}

func (m *MockHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds Flags) error {
	if m.EnsurePluginsF != nil {
		return m.EnsurePluginsF(plugins, kinds)
	}
	return nil
}

func (m *MockHost) ResolvePlugin(
	spec workspace.PluginSpec,
) (*workspace.PluginInfo, error) {
	if m.ResolvePluginF != nil {
		return m.ResolvePluginF(spec)
	}
	return nil, errors.New("ResolvePlugin not implemented")
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
	ConstructF          func(context.Context, ConstructRequest) (ConstructResponse, error)
	InvokeF             func(context.Context, InvokeRequest) (InvokeResponse, error)
	StreamInvokeF       func(context.Context, StreamInvokeRequest) (StreamInvokeResponse, error)
	CallF               func(context.Context, CallRequest) (CallResponse, error)
	GetPluginInfoF      func(context.Context) (workspace.PluginInfo, error)
	SignalCancellationF func(context.Context) error
	GetMappingF         func(context.Context, GetMappingRequest) (GetMappingResponse, error)
	GetMappingsF        func(context.Context, GetMappingsRequest) (GetMappingsResponse, error)
	MigrateF            func(context.Context, MigrateRequest) (MigrateResponse, error)
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
	return nil, errors.New("Handshake not implemented")
}

func (m *MockProvider) Parameterize(ctx context.Context, req ParameterizeRequest) (ParameterizeResponse, error) {
	if m.ParameterizeF != nil {
		return m.ParameterizeF(ctx, req)
	}
	return ParameterizeResponse{}, errors.New("Parameterize not implemented")
}

func (m *MockProvider) GetSchema(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
	if m.GetSchemaF != nil {
		return m.GetSchemaF(ctx, req)
	}
	return GetSchemaResponse{}, errors.New("GetSchema not implemented")
}

func (m *MockProvider) CheckConfig(ctx context.Context, req CheckConfigRequest) (CheckConfigResponse, error) {
	if m.CheckConfigF != nil {
		return m.CheckConfigF(ctx, req)
	}
	return CheckConfigResponse{}, errors.New("CheckConfig not implemented")
}

func (m *MockProvider) DiffConfig(ctx context.Context, req DiffConfigRequest) (DiffConfigResponse, error) {
	if m.DiffConfigF != nil {
		return m.DiffConfigF(ctx, req)
	}
	return DiffConfigResponse{}, errors.New("DiffConfig not implemented")
}

func (m *MockProvider) Configure(ctx context.Context, req ConfigureRequest) (ConfigureResponse, error) {
	if m.ConfigureF != nil {
		return m.ConfigureF(ctx, req)
	}
	return ConfigureResponse{}, errors.New("Configure not implemented")
}

func (m *MockProvider) Check(ctx context.Context, req CheckRequest) (CheckResponse, error) {
	if m.CheckF != nil {
		return m.CheckF(ctx, req)
	}
	return CheckResponse{}, errors.New("Check not implemented")
}

func (m *MockProvider) Diff(ctx context.Context, req DiffRequest) (DiffResponse, error) {
	if m.DiffF != nil {
		return m.DiffF(ctx, req)
	}
	return DiffResponse{}, errors.New("Diff not implemented")
}

func (m *MockProvider) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	if m.CreateF != nil {
		return m.CreateF(ctx, req)
	}
	return CreateResponse{}, errors.New("Create not implemented")
}

func (m *MockProvider) Read(ctx context.Context, req ReadRequest) (ReadResponse, error) {
	if m.ReadF != nil {
		return m.ReadF(ctx, req)
	}
	return ReadResponse{}, errors.New("Read not implemented")
}

func (m *MockProvider) Update(ctx context.Context, req UpdateRequest) (UpdateResponse, error) {
	if m.UpdateF != nil {
		return m.UpdateF(ctx, req)
	}
	return UpdateResponse{}, errors.New("Update not implemented")
}

func (m *MockProvider) Delete(ctx context.Context, req DeleteRequest) (DeleteResponse, error) {
	if m.DeleteF != nil {
		return m.DeleteF(ctx, req)
	}
	return DeleteResponse{}, errors.New("Delete not implemented")
}

func (m *MockProvider) Construct(ctx context.Context, req ConstructRequest) (ConstructResponse, error) {
	if m.ConstructF != nil {
		return m.ConstructF(ctx, req)
	}
	return ConstructResponse{}, errors.New("Construct not implemented")
}

func (m *MockProvider) Invoke(ctx context.Context, req InvokeRequest) (InvokeResponse, error) {
	if m.InvokeF != nil {
		return m.InvokeF(ctx, req)
	}
	return InvokeResponse{}, errors.New("Invoke not implemented")
}

func (m *MockProvider) StreamInvoke(ctx context.Context, req StreamInvokeRequest) (StreamInvokeResponse, error) {
	if m.StreamInvokeF != nil {
		return m.StreamInvokeF(ctx, req)
	}
	return StreamInvokeResponse{}, errors.New("StreamInvoke not implemented")
}

func (m *MockProvider) Call(ctx context.Context, req CallRequest) (CallResponse, error) {
	if m.CallF != nil {
		return m.CallF(ctx, req)
	}
	return CallResponse{}, errors.New("Call not implemented")
}

func (m *MockProvider) GetPluginInfo(ctx context.Context) (workspace.PluginInfo, error) {
	if m.GetPluginInfoF != nil {
		return m.GetPluginInfoF(ctx)
	}
	return workspace.PluginInfo{}, errors.New("GetPluginInfo not implemented")
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
	return GetMappingResponse{}, errors.New("GetMapping not implemented")
}

func (m *MockProvider) GetMappings(ctx context.Context, req GetMappingsRequest) (GetMappingsResponse, error) {
	if m.GetMappingsF != nil {
		return m.GetMappingsF(ctx, req)
	}
	return GetMappingsResponse{}, errors.New("GetMappings not implemented")
}

func (m *MockProvider) Migrate(ctx context.Context, req MigrateRequest) (MigrateResponse, error) {
	if m.GetMappingsF != nil {
		return m.MigrateF(ctx, req)
	}
	return MigrateResponse{}, errors.New("Migrate not implemented")
}
