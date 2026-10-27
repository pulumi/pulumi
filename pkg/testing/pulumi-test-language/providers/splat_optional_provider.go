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

package providers

import (
	"context"
	"encoding/json"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// SplatOptionalProvider is a container whose detail.value is an optional string.
// It exists so program-gen can splat that field (pulumi/pulumi#25356).
type SplatOptionalProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SplatOptionalProvider)(nil)

func (p *SplatOptionalProvider) Close() error {
	return nil
}

func (p *SplatOptionalProvider) version() semver.Version {
	return semver.Version{Major: 1}
}

func (p *SplatOptionalProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SplatOptionalProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "splatoptional",
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			"splatoptional:index:Detail": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"key": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					// value is optional. key is required.
					Required: []string{"key"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"splatoptional:index:Container": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"inputs": {
							TypeSpec: schema.TypeSpec{
								Type:  "array",
								Items: &schema.TypeSpec{Type: "string"},
							},
						},
						"details": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Ref: "#/types/splatoptional:index:Detail",
								},
							},
						},
					},
					Required: []string{"inputs", "details"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"inputs": {
						TypeSpec: schema.TypeSpec{
							Type:  "array",
							Items: &schema.TypeSpec{Type: "string"},
						},
					},
				},
				RequiredInputs: []string{"inputs"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SplatOptionalProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SplatOptionalProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	news := resource.ToResourcePropertyMap(req.NewInputs)
	return plugin.CheckResponse{Properties: resource.FromResourcePropertyMap(news)}, nil
}

func (p *SplatOptionalProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	properties := resource.ToResourcePropertyMap(req.Properties)
	id := "id"
	if req.Preview {
		id = ""
	}

	outputs := properties.Copy()
	if req.URN.Type() == "splatoptional:index:Container" {
		inputs := properties["inputs"]
		if inputs.IsArray() {
			details := make([]resource.PropertyValue, len(inputs.ArrayValue()))
			for i, input := range inputs.ArrayValue() {
				details[i] = resource.NewProperty(resource.PropertyMap{
					"key":   input,
					"value": resource.NewProperty("computed-" + input.StringValue()),
				})
			}
			outputs["details"] = resource.NewProperty(details)
		}
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: resource.FromResourcePropertyMap(outputs),
		Status:     resource.StatusOK,
	}, nil
}

func (p *SplatOptionalProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *SplatOptionalProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: new(p.version()),
	}, nil
}

func (p *SplatOptionalProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SplatOptionalProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SplatOptionalProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SplatOptionalProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SplatOptionalProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SplatOptionalProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *SplatOptionalProvider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  &req.Inputs,
			Outputs: coalesceReadOutputs(req),
		},
		Status: resource.StatusOK,
	}, nil
}
