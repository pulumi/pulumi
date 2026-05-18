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

// DashNamesProvider verifies that dash-style package, module, member, type, and property names
// are preserved on the provider wire protocol while SDKs expose language-safe identifiers.
type DashNamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DashNamesProvider)(nil)

func (p *DashNamesProvider) Close() error { return nil }

func (p *DashNamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DashNamesProvider) Pkg() tokens.Package { return "dash-names" }

func (p *DashNamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "dash-names",
		Version: "41.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"dash-names:dash-module:nested-input": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"nested-value": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"nested-value"},
				},
			},
			"dash-names:dash-module:output-item": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"nested-output": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"nested-output"},
				},
			},
			"dash-names:dash-module:entry-value": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"entry-value": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"entry-value"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"dash-names:dash-module:some-resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the-output": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Ref: "#/types/dash-names:dash-module:output-item",
								},
							},
						},
					},
					Required: []string{"the-output"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"the-input": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
					"nested-value": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/dash-names:dash-module:nested-input",
						},
					},
				},
				RequiredInputs: []string{"the-input", "nested-value"},
			},
			"dash-names:dash-module:another-resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the-input": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"the-input"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"the-input": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
				RequiredInputs: []string{"the-input"},
			},
			"dash-names:dash-module:trailing-resource-": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"trailing-output-": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"trailing-output-"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"trailing-input-": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
				RequiredInputs: []string{"trailing-input-"},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"dash-names:dash-module:some-data": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the-input": {TypeSpec: schema.TypeSpec{Type: "string"}},
						"entry-values": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Type: "string",
								},
							},
						},
					},
					Required: []string{"the-input", "entry-values"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"nested-output": {
								TypeSpec: schema.TypeSpec{
									Type: "array",
									Items: &schema.TypeSpec{
										Ref: "#/types/dash-names:dash-module:entry-value",
									},
								},
							},
						},
						Required: []string{"nested-output"},
					},
				},
			},
			"dash-names:dash-module:trailing-data-": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"trailing-input-": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"trailing-input-"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"trailing-output-": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
						Required: []string{"trailing-output-"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *DashNamesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "missing version")}, nil
	}
	if !version.IsString() || version.StringValue() != "41.0.0" {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not 41.0.0")}, nil
	}
	if len(req.News) != 1 {
		failures := makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News))
		return plugin.CheckConfigResponse{Failures: failures}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DashNamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch typ := req.URN.Type(); typ {
	case "dash-names:dash-module:some-resource":
		for _, key := range []resource.PropertyKey{"the-input", "nested-value"} {
			if _, ok := req.News[key]; !ok {
				return plugin.CheckResponse{Failures: makeCheckFailure(key, "missing "+string(key))}, nil
			}
		}
		if len(req.News) != 2 {
			return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("unexpected properties: %v", req.News))}, nil
		}
		return plugin.CheckResponse{Properties: req.News}, nil
	case "dash-names:dash-module:another-resource":
		if _, ok := req.News["the-input"]; !ok {
			return plugin.CheckResponse{Failures: makeCheckFailure("the-input", "missing the-input")}, nil
		}
		if len(req.News) != 1 {
			return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("unexpected properties: %v", req.News))}, nil
		}
		return plugin.CheckResponse{Properties: req.News}, nil
	case "dash-names:dash-module:trailing-resource-":
		if _, ok := req.News["trailing-input-"]; !ok {
			return plugin.CheckResponse{Failures: makeCheckFailure("trailing-input-", "missing trailing-input-")}, nil
		}
		if len(req.News) != 1 {
			return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("unexpected properties: %v", req.News))}, nil
		}
		return plugin.CheckResponse{Properties: req.News}, nil
	case tokens.RootStackType:
		return plugin.CheckResponse{Failures: makeCheckFailure("", "invalid root stack resource")}, nil
	default:
		return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", typ))}, nil
	}
}

func (p *DashNamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	switch typ := req.URN.Type(); typ {
	case "dash-names:dash-module:some-resource":
		nested, ok := req.Properties["nested-value"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing nested-value property")
		}
		nestedValue := nested.ObjectValue()["nested-value"].StringValue()
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"the-output": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"nested-output": resource.NewProperty(nestedValue),
					}),
				}),
			},
			Status: resource.StatusOK,
		}, nil
	case "dash-names:dash-module:another-resource":
		theInput, ok := req.Properties["the-input"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing the-input property")
		}
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"the-input": theInput,
			},
			Status: resource.StatusOK,
		}, nil
	case "dash-names:dash-module:trailing-resource-":
		trailingInput, ok := req.Properties["trailing-input-"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing trailing-input- property")
		}
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"trailing-output-": trailingInput,
			},
			Status: resource.StatusOK,
		}, nil
	case tokens.RootStackType:
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, errors.New("invalid root stack resource")
	default:
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", typ)
	}
}

func (p *DashNamesProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	switch req.Tok {
	case "dash-names:dash-module:some-data":
		entries, ok := req.Args["entry-values"]
		if !ok {
			return plugin.InvokeResponse{Failures: makeCheckFailure("entry-values", "missing entry-values")}, nil
		}
		firstValue := entries.ArrayValue()[0].StringValue()

		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{
				"nested-output": resource.NewProperty([]resource.PropertyValue{
					resource.NewProperty(resource.PropertyMap{
						"entry-value": resource.NewProperty(firstValue),
					}),
				}),
			},
		}, nil
	case "dash-names:dash-module:trailing-data-":
		trailingInput, ok := req.Args["trailing-input-"]
		if !ok {
			return plugin.InvokeResponse{Failures: makeCheckFailure("trailing-input-", "missing trailing-input-")}, nil
		}

		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{
				"trailing-output-": trailingInput,
			},
		}, nil
	default:
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}
}

func (p *DashNamesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.Version{Major: 41}
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *DashNamesProvider) SignalCancellation(context.Context) error { return nil }

func (p *DashNamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DashNamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DashNamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DashNamesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DashNamesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *DashNamesProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{Status: resource.StatusUnknown}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}

func (p *DashNamesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{Status: resource.StatusUnknown}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
