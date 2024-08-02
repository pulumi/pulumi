// Copyright 2016-2024, Pulumi Corporation.
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

package providers

import (
	"context"
	"encoding/json"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// ParameterizedProvider is a test provider that supports tests checking the
// behavior of parameterized providers and their methods.
type ParameterizedProvider struct {
	plugin.NotForwardCompatibleProvider
}

var _ plugin.Provider = (*ParameterizedProvider)(nil)

func (p *ParameterizedProvider) Parameterize(
	context.Context,
	plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	return plugin.ParameterizeResponse{}, nil
}

func (p *ParameterizedProvider) Close() error {
	return nil
}

func (p *ParameterizedProvider) Configure(
	context.Context,
	plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ParameterizedProvider) Pkg() tokens.Package {
	return "parameterized"
}

func (p *ParameterizedProvider) GetSchema(
	context.Context,
	plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	stringT := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}

	pkg := schema.PackageSpec{
		Name:    "parameterized",
		Version: "1.3.7",
		Resources: map[string]schema.ResourceSpec{
			"parameterized:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": stringT,
					},
					Required: []string{"value"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": stringT,
				},
				RequiredInputs: []string{"value"},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"parameterized:index:invoke": {
				Inputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"value": stringT,
					},
				},
				Outputs: &schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"value": stringT,
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ParameterizedProvider) CheckConfig(
	_ context.Context,
	req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ParameterizedProvider) Check(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ParameterizedProvider) Create(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{}, nil
}

func (p *ParameterizedProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("1.3.7")
	return workspace.PluginInfo{Version: &ver}, nil
}

func (p *ParameterizedProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ParameterizedProvider) GetMapping(
	context.Context,
	plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ParameterizedProvider) GetMappings(
	context.Context,
	plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ParameterizedProvider) DiffConfig(
	context.Context,
	plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ParameterizedProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ParameterizedProvider) Delete(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *ParameterizedProvider) Construct(
	context.Context,
	plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	return plugin.ConstructResponse{}, nil
}

func (p *ParameterizedProvider) Invoke(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	return plugin.InvokeResponse{}, nil
}

func (p *ParameterizedProvider) StreamInvoke(
	context.Context,
	plugin.StreamInvokeRequest,
) (plugin.StreamInvokeResponse, error) {
	return plugin.StreamInvokeResponse{}, nil
}

func (p *ParameterizedProvider) Call(context.Context, plugin.CallRequest) (plugin.CallResponse, error) {
	return plugin.CallResponse{}, nil
}

func (p *ParameterizedProvider) Read(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{}, nil
}

func (p *ParameterizedProvider) Update(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{}, nil
}
