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

// NestedCollectionsProvider exercises deeply nested collection types: a resource whose
// properties are a list of lists of lists of an object type and a map of maps of maps of
// strings (pulumi/pulumi#9887).
type NestedCollectionsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*NestedCollectionsProvider)(nil)

func (p *NestedCollectionsProvider) Close() error {
	return nil
}

func (p *NestedCollectionsProvider) version() semver.Version {
	return semver.Version{Major: 50}
}

func (p *NestedCollectionsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *NestedCollectionsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	conditionSets := schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type: "array",
			Items: &schema.TypeSpec{
				Type: "array",
				Items: &schema.TypeSpec{
					Type: "array",
					Items: &schema.TypeSpec{
						Ref: "#/types/nestedcollections:index:Bar",
					},
				},
			},
		},
	}
	privateEndpoint := schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type: "object",
			AdditionalProperties: &schema.TypeSpec{
				Type: "object",
				AdditionalProperties: &schema.TypeSpec{
					Type:                 "object",
					AdditionalProperties: &schema.TypeSpec{Type: "string"},
				},
			},
		},
	}

	pkg := schema.PackageSpec{
		Name:    "nestedcollections",
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			"nestedcollections:index:Bar": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"prop": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"prop"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"nestedcollections:index:Foo": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"conditionSets":   conditionSets,
						"privateEndpoint": privateEndpoint,
					},
					Required: []string{"conditionSets", "privateEndpoint"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"conditionSets":   conditionSets,
					"privateEndpoint": privateEndpoint,
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *NestedCollectionsProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *NestedCollectionsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *NestedCollectionsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	outputs := req.Properties.Copy()

	if req.URN.Type() == "nestedcollections:index:Foo" {
		bar := func(prop string) resource.PropertyValue {
			return resource.NewProperty(resource.PropertyMap{
				"prop": resource.NewProperty(prop),
			})
		}
		outputs["conditionSets"] = resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty([]resource.PropertyValue{bar("first"), bar("second")}),
			}),
		})
		outputs["privateEndpoint"] = resource.NewProperty(resource.PropertyMap{
			"outer": resource.NewProperty(resource.PropertyMap{
				"inner": resource.NewProperty(resource.PropertyMap{
					"leaf": resource.NewProperty("deep"),
				}),
			}),
		})
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: outputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *NestedCollectionsProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *NestedCollectionsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: new(p.version()),
	}, nil
}

func (p *NestedCollectionsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *NestedCollectionsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *NestedCollectionsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *NestedCollectionsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *NestedCollectionsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *NestedCollectionsProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *NestedCollectionsProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  req.Inputs,
			Outputs: req.State,
		},
		Status: resource.StatusOK,
	}, nil
}
