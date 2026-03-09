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
)

// SnakeNamesProvider is used to test snake_case naming features in the Pulumi SDK, and that they are used
// correctly across sdk-gen and program-gen.
type SnakeNamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SnakeNamesProvider)(nil)

func (p *SnakeNamesProvider) Close() error {
	return nil
}

func (p *SnakeNamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SnakeNamesProvider) Pkg() tokens.Package {
	return "snake_names"
}

func (p *SnakeNamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "snake_names",
		Version: "33.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"snake_names:cool_module:nested_input": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"nested_value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"nested_value"},
				},
			},
			"snake_names:cool_module:output_item": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"nested_output": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"nested_output"},
				},
			},
			"snake_names:cool_module:entry": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"value"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"snake_names:cool_module:some_resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the_output": {
							TypeSpec: schema.TypeSpec{
								Type: "object",
								AdditionalProperties: &schema.TypeSpec{
									Type: "array",
									Items: &schema.TypeSpec{
										Ref: "#/types/snake_names:cool_module:output_item",
									},
								},
							},
						},
					},
					Required: []string{"the_output"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"the_input": {
						TypeSpec: schema.TypeSpec{Type: "boolean"},
					},
					"nested": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/snake_names:cool_module:nested_input",
						},
					},
				},
				RequiredInputs: []string{"the_input", "nested"},
			},
			"snake_names:cool_module:another_resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the_input": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"the_input"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"the_input": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"the_input"},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"snake_names:cool_module:some_data": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the_input": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"nested": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Ref: "#/types/snake_names:cool_module:entry",
								},
							},
						},
					},
					Required: []string{"the_input", "nested"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"nested_output": {
								TypeSpec: schema.TypeSpec{
									Type: "array",
									Items: &schema.TypeSpec{
										Type: "object",
										AdditionalProperties: &schema.TypeSpec{
											Ref: "#/types/snake_names:cool_module:entry",
										},
									},
								},
							},
						},
						Required: []string{"nested_output"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SnakeNamesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
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
	if version.StringValue() != "33.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 33.0.0"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SnakeNamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch typ := req.URN.Type(); typ {
	case "snake_names:cool_module:some_resource":
		if _, ok := req.News["the_input"]; !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("the_input", "missing the_input"),
			}, nil
		}
		if _, ok := req.News["nested"]; !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("nested", "missing nested"),
			}, nil
		}
		if len(req.News) != 2 {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("unexpected properties: %v", req.News)),
			}, nil
		}
		return plugin.CheckResponse{Properties: req.News}, nil
	case "snake_names:cool_module:another_resource":
		if _, ok := req.News["the_input"]; !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("the_input", "missing the_input"),
			}, nil
		}
		if len(req.News) != 1 {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("unexpected properties: %v", req.News)),
			}, nil
		}
		return plugin.CheckResponse{Properties: req.News}, nil
	case tokens.RootStackType:
		return plugin.CheckResponse{Properties: req.News}, nil
	default:
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", typ)),
		}, nil
	}
}

func (p *SnakeNamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	switch typ := req.URN.Type(); typ {
	case "snake_names:cool_module:some_resource":
		nested, ok := req.Properties["nested"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing nested property")
		}
		nestedValue := nested.ObjectValue()["nested_value"].StringValue()
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"the_output": resource.NewProperty(resource.PropertyMap{
					"someKey": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(resource.PropertyMap{
							"nested_output": resource.NewProperty(nestedValue),
						}),
					}),
				}),
			},
			Status: resource.StatusOK,
		}, nil
	case "snake_names:cool_module:another_resource":
		theInput, ok := req.Properties["the_input"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing the_input property")
		}
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"the_input": theInput,
			},
			Status: resource.StatusOK,
		}, nil
	case tokens.RootStackType:
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", typ)
	default:
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", typ)
	}
}

func (p *SnakeNamesProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "snake_names:cool_module:some_data" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}

	nestedArr, ok := req.Args["nested"]
	if !ok {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("nested", "missing nested"),
		}, nil
	}

	firstEntry := nestedArr.ArrayValue()[0].ObjectValue()
	firstValue := firstEntry["value"].StringValue()

	return plugin.InvokeResponse{
		Properties: resource.PropertyMap{
			"nested_output": resource.NewProperty([]resource.PropertyValue{
				resource.NewProperty(resource.PropertyMap{
					"key": resource.NewProperty(resource.PropertyMap{
						"value": resource.NewProperty(firstValue),
					}),
				}),
			}),
		},
	}, nil
}

func (p *SnakeNamesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.Version{Major: 33}
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SnakeNamesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SnakeNamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SnakeNamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SnakeNamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SnakeNamesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SnakeNamesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *SnakeNamesProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		Status: resource.StatusUnknown,
	}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}

func (p *SnakeNamesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Status: resource.StatusUnknown,
	}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
