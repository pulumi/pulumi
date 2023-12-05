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

//nolint:lll
package plugin

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnimplementedProvider can be embedded to have forward compatible implementations.
type UnimplementedProvider struct{}

func (p *UnimplementedProvider) Close() error {
	return status.Error(codes.Unimplemented, "Close is not yet implemented")
}

func (p *UnimplementedProvider) SignalCancellation() error {
	return status.Error(codes.Unimplemented, "SignalCancellation is not yet implemented")
}

func (p *UnimplementedProvider) Pkg() tokens.Package {
	return tokens.Package("")
}

func (p *UnimplementedProvider) GetSchema(version int) ([]byte, error) {
	return nil, status.Error(codes.Unimplemented, "GetSchema is not yet implemented")
}

func (p *UnimplementedProvider) CheckConfig(urn resource.URN, olds resource.PropertyMap, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []CheckFailure, error) {
	return resource.PropertyMap{}, nil, status.Error(codes.Unimplemented, "CheckConfig is not yet implemented")
}

func (p *UnimplementedProvider) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (DiffResult, error) {
	return DiffResult{}, status.Error(codes.Unimplemented, "DiffConfig is not yet implemented")
}

func (p *UnimplementedProvider) Configure(inputs resource.PropertyMap) error {
	return status.Error(codes.Unimplemented, "Configure is not yet implemented")
}

func (p *UnimplementedProvider) Check(urn resource.URN, olds resource.PropertyMap, news resource.PropertyMap, allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []CheckFailure, error) {
	return resource.PropertyMap{}, nil, status.Error(codes.Unimplemented, "Check is not yet implemented")
}

func (p *UnimplementedProvider) Diff(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (DiffResult, error) {
	return DiffResult{}, status.Error(codes.Unimplemented, "Diff is not yet implemented")
}

func (p *UnimplementedProvider) Create(urn resource.URN, news resource.PropertyMap, timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {
	return resource.ID(""), resource.PropertyMap{}, resource.StatusUnknown, status.Error(codes.Unimplemented, "Create is not yet implemented")
}

func (p *UnimplementedProvider) Read(urn resource.URN, id resource.ID, inputs resource.PropertyMap, state resource.PropertyMap) (ReadResult, resource.Status, error) {
	return ReadResult{}, resource.StatusUnknown, status.Error(codes.Unimplemented, "Read is not yet implemented")
}

func (p *UnimplementedProvider) Update(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap, timeout float64, ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {
	return resource.PropertyMap{}, resource.StatusUnknown, status.Error(codes.Unimplemented, "Update is not yet implemented")
}

func (p *UnimplementedProvider) Delete(urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap, timeout float64) (resource.Status, error) {
	return resource.StatusUnknown, status.Error(codes.Unimplemented, "Delete is not yet implemented")
}

func (p *UnimplementedProvider) Construct(info ConstructInfo, typ tokens.Type, name string, parent resource.URN, inputs resource.PropertyMap, options ConstructOptions) (ConstructResult, error) {
	return ConstructResult{}, status.Error(codes.Unimplemented, "Construct is not yet implemented")
}

func (p *UnimplementedProvider) Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error) {
	return resource.PropertyMap{}, nil, status.Error(codes.Unimplemented, "Invoke is not yet implemented")
}

func (p *UnimplementedProvider) StreamInvoke(tok tokens.ModuleMember, args resource.PropertyMap, onNext func(resource.PropertyMap) error) ([]CheckFailure, error) {
	return nil, status.Error(codes.Unimplemented, "StreamInvoke is not yet implemented")
}

func (p *UnimplementedProvider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info CallInfo, options CallOptions) (CallResult, error) {
	return CallResult{}, status.Error(codes.Unimplemented, "Call is not yet implemented")
}

func (p *UnimplementedProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{}, status.Error(codes.Unimplemented, "GetPluginInfo is not yet implemented")
}

func (p *UnimplementedProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", status.Error(codes.Unimplemented, "GetMapping is not yet implemented")
}

func (p *UnimplementedProvider) GetMappings(key string) ([]string, error) {
	return nil, status.Error(codes.Unimplemented, "GetMappings is not yet implemented")
}
