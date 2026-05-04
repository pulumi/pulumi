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
	"errors"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type PartialValuesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PartialValuesProvider)(nil)

func (p *PartialValuesProvider) Close() error {
	return nil
}

func (p *PartialValuesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PartialValuesProvider) Pkg() tokens.Package {
	return "partial"
}

func (p *PartialValuesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: &semver.Version{Major: 40},
	}, nil
}

func (p *PartialValuesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	// Define the complex object type with mixed known/unknown and secret/non-secret fields
	dataObjectSpec := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"knownField": {
				TypeSpec: schema.TypeSpec{
					Type: "string",
				},
				Description: "A field that is known during preview",
			},
			"knownSecretField": {
				Secret: true,
				TypeSpec: schema.TypeSpec{
					Type: "string",
				},
				Description: "A secret field that is known during preview",
			},
			"unknownField": {
				TypeSpec: schema.TypeSpec{
					Type: "string",
				},
				Description: "A field that is unknown during preview",
			},
			"unknownSecretField": {
				Secret: true,
				TypeSpec: schema.TypeSpec{
					Type: "string",
				},
				Description: "A secret field that is unknown during preview",
			},
		},
		Required: []string{"knownField", "knownSecretField", "unknownField", "unknownSecretField"},
	}

	pkg := schema.PackageSpec{
		Name:    "partial",
		Version: "40.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"partial:index:DataObject": {
				ObjectTypeSpec: dataObjectSpec,
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"partial:index:Source": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"data": {
							TypeSpec: schema.TypeSpec{
								Type: "ref",
								Ref:  "#/types/partial:index:DataObject",
							},
							Description: "Object with mixed known/unknown and secret/non-secret fields",
						},
						"dataList": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Type: "string",
								},
							},
							Description: "List with mixed known/unknown and secret/non-secret values",
						},
						"dataMap": {
							TypeSpec: schema.TypeSpec{
								Type: "object",
								AdditionalProperties: &schema.TypeSpec{
									Type: "string",
								},
							},
							Description: "Map with mixed known/unknown and secret/non-secret values",
						},
					},
					Required: []string{"data", "dataList", "dataMap"},
				},
			},
			"partial:index:Consumer": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]schema.PropertySpec{},
				},
				InputProperties: map[string]schema.PropertySpec{
					"values": {
						TypeSpec: schema.TypeSpec{
							Type: "object",
							AdditionalProperties: &schema.TypeSpec{
								Type: "string",
							},
						},
						Description: "Flat map of all unpacked source values to verify",
					},
				},
				RequiredInputs: []string{"values"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *PartialValuesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "missing version"),
		}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not a string"),
		}, nil
	}
	if version.StringValue() != "40.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 40.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PartialValuesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "partial:index:Source" && req.URN.Type() != "partial:index:Consumer" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Basic validation - just accept the inputs
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *PartialValuesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	switch req.URN.Type() { //nolint:exhaustive // Default covers the other case
	case "partial:index:Source":
		return p.createSource(req)
	case "partial:index:Consumer":
		return p.createConsumer(req)
	default:
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
}

func (p *PartialValuesProvider) createSource(req plugin.CreateRequest) (plugin.CreateResponse, error) {
	id := "source-id"
	if req.Preview {
		id = ""
	}

	properties := resource.PropertyMap{}

	if req.Preview {
		// During preview: return partial values
		properties["data"] = resource.NewProperty(resource.PropertyMap{
			"knownField":       resource.NewProperty("known-value"),
			"knownSecretField": resource.MakeSecret(resource.NewProperty("known-secret-value")),
			// unknownField and unknownSecretField are computed
			"unknownField": resource.NewProperty(resource.Computed{Element: resource.NewProperty("")}),
			"unknownSecretField": resource.MakeSecret(
				resource.NewProperty(resource.Computed{Element: resource.NewProperty("")}),
			),
		})

		// List with mixed values
		properties["dataList"] = resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("known-item-0"),                                                            // known
			resource.MakeSecret(resource.NewProperty("known-secret-item-1")),                                // known secret
			resource.NewProperty(resource.Computed{Element: resource.NewProperty("")}),                      // unknown
			resource.MakeSecret(resource.NewProperty(resource.Computed{Element: resource.NewProperty("")})), // unknown secret
		})

		// Map with mixed values
		properties["dataMap"] = resource.NewProperty(resource.PropertyMap{
			"knownKey":         resource.NewProperty("known-map-value"),
			"knownSecretKey":   resource.MakeSecret(resource.NewProperty("known-secret-map-value")),
			"unknownKey":       resource.NewProperty(resource.Computed{Element: resource.NewProperty("")}),
			"unknownSecretKey": resource.MakeSecret(resource.NewProperty(resource.Computed{Element: resource.NewProperty("")})),
		})
	} else {
		// During actual execution: return concrete values
		properties["data"] = resource.NewProperty(resource.PropertyMap{
			"knownField":         resource.NewProperty("known-value"),
			"knownSecretField":   resource.MakeSecret(resource.NewProperty("known-secret-value")),
			"unknownField":       resource.NewProperty("computed-value"),
			"unknownSecretField": resource.MakeSecret(resource.NewProperty("computed-secret-value")),
		})

		properties["dataList"] = resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("known-item-0"),
			resource.MakeSecret(resource.NewProperty("known-secret-item-1")),
			resource.NewProperty("computed-item-2"),
			resource.MakeSecret(resource.NewProperty("computed-secret-item-3")),
		})

		properties["dataMap"] = resource.NewProperty(resource.PropertyMap{
			"knownKey":         resource.NewProperty("known-map-value"),
			"knownSecretKey":   resource.MakeSecret(resource.NewProperty("known-secret-map-value")),
			"unknownKey":       resource.NewProperty("computed-map-value"),
			"unknownSecretKey": resource.MakeSecret(resource.NewProperty("computed-secret-map-value")),
		})
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *PartialValuesProvider) createConsumer(req plugin.CreateRequest) (plugin.CreateResponse, error) {
	id := "consumer-id"
	if req.Preview {
		id = ""
	}

	valuesProp, ok := req.Properties["values"]
	if !ok || !valuesProp.IsObject() {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, errors.New("createConsumer: missing or non-object 'values' input")
	}
	actual := resource.FromResourcePropertyMap(valuesProp.ObjectValue())
	var expected property.Map
	if req.Preview {
		expected = property.NewMap(map[string]property.Value{
			"dataKnownField":         property.New("known-value"),
			"dataKnownSecretField":   property.New("known-secret-value").WithSecret(true),
			"dataUnknownField":       property.New(property.Computed),
			"dataUnknownSecretField": property.New(property.Computed).WithSecret(true),
			"listKnown":              property.New("known-item-0"),
			"listKnownSecret":        property.New("known-secret-item-1").WithSecret(true),
			"listUnknown":            property.New(property.Computed),
			"listUnknownSecret":      property.New(property.Computed).WithSecret(true),
			"mapKnown":               property.New("known-map-value"),
			"mapKnownSecret":         property.New("known-secret-map-value").WithSecret(true),
			"mapUnknown":             property.New(property.Computed),
			"mapUnknownSecret":       property.New(property.Computed).WithSecret(true),
		})
	} else {
		expected = property.NewMap(map[string]property.Value{
			"dataKnownField":         property.New("known-value"),
			"dataKnownSecretField":   property.New("known-secret-value").WithSecret(true),
			"dataUnknownField":       property.New("computed-value"),
			"dataUnknownSecretField": property.New("computed-secret-value").WithSecret(true),
			"listKnown":              property.New("known-item-0"),
			"listKnownSecret":        property.New("known-secret-item-1").WithSecret(true),
			"listUnknown":            property.New("computed-item-2"),
			"listUnknownSecret":      property.New("computed-secret-item-3").WithSecret(true),
			"mapKnown":               property.New("known-map-value"),
			"mapKnownSecret":         property.New("known-secret-map-value").WithSecret(true),
			"mapUnknown":             property.New("computed-map-value"),
			"mapUnknownSecret":       property.New("computed-secret-map-value").WithSecret(true),
		})
	}

	if !property.New(expected).Equals(property.New(actual)) {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("mismatch: expected: %#v, actual: %#v", expected, actual)
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: resource.PropertyMap{},
		Status:     resource.StatusOK,
	}, nil
}

func (p *PartialValuesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *PartialValuesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PartialValuesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PartialValuesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PartialValuesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PartialValuesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
