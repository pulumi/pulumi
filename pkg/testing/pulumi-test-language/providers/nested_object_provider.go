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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type NestedObjectProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*NestedObjectProvider)(nil)

func (p *NestedObjectProvider) Close() error {
	return nil
}

func (p *NestedObjectProvider) version() semver.Version {
	return semver.Version{Major: 1, Minor: 42}
}

func (p *NestedObjectProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *NestedObjectProvider) Pkg() tokens.Package {
	return "nestedobject"
}

func (p *NestedObjectProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "nestedobject",
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			"nestedobject:index:Detail": {
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
					Required: []string{"key", "value"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"nestedobject:index:Container": {
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
									Ref: "#/types/nestedobject:index:Detail",
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
			"nestedobject:index:Target": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"name": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"name"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"name": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"name"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *NestedObjectProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *NestedObjectProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *NestedObjectProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	outputs := req.Properties.Copy()

	if req.URN.Type() == "nestedobject:index:Container" {
		// Compute details from inputs: for each input, create a detail object.
		inputs := req.Properties["inputs"]
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
		Properties: outputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *NestedObjectProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *NestedObjectProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ref(p.version()),
	}, nil
}

func (p *NestedObjectProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *NestedObjectProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *NestedObjectProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *NestedObjectProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *NestedObjectProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *NestedObjectProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *NestedObjectProvider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  req.Inputs,
			Outputs: req.State,
		},
		Status: resource.StatusOK,
	}, nil
}
