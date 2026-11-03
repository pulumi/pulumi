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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// SpecialCharsProvider is used to test that property names containing characters that are not
// legal identifier characters in most languages (such as "@timestamp" and "entity.name", as seen
// in bridged Terraform providers like Elasticstack) are handled correctly across sdk-gen and at
// runtime. The special names only appear in a nested object type: they cannot appear as top-level
// resource inputs in PCL programs, and real-world schemas carry them in nested types.
type SpecialCharsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SpecialCharsProvider)(nil)

func (p *SpecialCharsProvider) Close() error {
	return nil
}

func (p *SpecialCharsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SpecialCharsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "specialchars",
		Version: "54.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"specialchars:index:item": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"@timestamp": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"entity.name": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"@timestamp", "entity.name"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"specialchars:index:Thing": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"item": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/specialchars:index:item",
							},
						},
					},
					Required: []string{"value", "item"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
					// data is never set by test programs, but input types are still generated
					// for the item type, so sdk-gen of input types is exercised as well.
					"data": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/specialchars:index:item",
						},
					},
				},
				RequiredInputs: []string{"value"},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"specialchars:index:getItem": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"value"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"item": {
								TypeSpec: schema.TypeSpec{
									Ref: "#/types/specialchars:index:item",
								},
							},
						},
						Required: []string{"item"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SpecialCharsProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News.GetOk("version")
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
	if version.AsString() != "54.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 54.0.0"),
		}, nil
	}
	if req.News.Len() != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SpecialCharsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch typ := req.URN.Type(); typ {
	case "specialchars:index:Thing":
		if _, ok := req.News["value"]; !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("value", "missing value"),
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

func (p *SpecialCharsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	switch typ := req.URN.Type(); typ {
	case "specialchars:index:Thing":
		value, ok := req.Properties["value"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing value property")
		}
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"value": value,
				"item": resource.NewProperty(resource.PropertyMap{
					"@timestamp":  resource.NewProperty("2026-01-02T03:04:05Z"),
					"entity.name": resource.NewProperty(value.StringValue()),
				}),
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

func (p *SpecialCharsProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "specialchars:index:getItem" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}

	value, ok := req.Args.GetOk("value")
	if !ok || !value.IsString() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("value", fmt.Sprintf("missing string value: %v", req.Args)),
		}, nil
	}

	return plugin.InvokeResponse{
		Properties: property.NewMap(map[string]property.Value{
			"item": property.New(map[string]property.Value{
				"@timestamp":  property.New("2026-01-02T03:04:05Z"),
				"entity.name": property.New(value.AsString()),
			}),
		}),
	}, nil
}

func (p *SpecialCharsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.Version{Major: 54}
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SpecialCharsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SpecialCharsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SpecialCharsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SpecialCharsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SpecialCharsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SpecialCharsProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *SpecialCharsProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		Status: resource.StatusUnknown,
	}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}

func (p *SpecialCharsProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Status: resource.StatusUnknown,
	}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
