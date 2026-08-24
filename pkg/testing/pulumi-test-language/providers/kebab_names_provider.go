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

// KebabNamesProvider is used to test that kebab-case names are handled correctly across sdk-gen and
// program-gen: in the package name, module names, resource names, object type names, function names
// and property names. The defaults-input type carries a property with a schema default value, so
// generated provide-defaults helpers are exercised for kebab-case property names.
type KebabNamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*KebabNamesProvider)(nil)

func (p *KebabNamesProvider) Close() error {
	return nil
}

func (p *KebabNamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *KebabNamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "kebab-names",
		Version: "52.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"kebab-names:kebab-module:nested-input": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"nested-value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"nested-value"},
				},
			},
			// The only always-set property is the single-word "value": Python TypedDict inputs
			// are serialized without input type information for invokes, so multi-word keys
			// would reach the provider untranslated.
			"kebab-names:kebab-module:defaults-input": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"default-value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
							Default:  "defaulted",
						},
					},
					Required: []string{"value"},
				},
			},
			"kebab-names:kebab-module:output-item": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"nested-output": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"nested-output"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"kebab-names:kebab-module:some-resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the-output": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/kebab-names:kebab-module:output-item",
							},
						},
					},
					Required: []string{"the-output"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"the-input": {
						TypeSpec: schema.TypeSpec{Type: "boolean"},
					},
					"nested": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/kebab-names:kebab-module:nested-input",
						},
					},
				},
				RequiredInputs: []string{"the-input", "nested"},
			},
			"kebab-names:kebab-module:another-resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the-input": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"the-input"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"the-input": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"the-input"},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"kebab-names:kebab-module:do-something": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"the-input": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"nested": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/kebab-names:kebab-module:defaults-input",
							},
						},
					},
					Required: []string{"the-input"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"the-output": {
								TypeSpec: schema.TypeSpec{Type: "string"},
							},
						},
						Required: []string{"the-output"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *KebabNamesProvider) CheckConfig(
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
	if version.AsString() != "52.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 52.0.0"),
		}, nil
	}
	if req.News.Len() != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *KebabNamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch typ := req.URN.Type(); typ {
	case "kebab-names:kebab-module:some-resource":
		if _, ok := req.News["the-input"]; !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("the-input", "missing the-input"),
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
	case "kebab-names:kebab-module:another-resource":
		if _, ok := req.News["the-input"]; !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("the-input", "missing the-input"),
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

func (p *KebabNamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	switch typ := req.URN.Type(); typ {
	case "kebab-names:kebab-module:some-resource":
		nested, ok := req.Properties["nested"]
		if !ok {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("missing nested property")
		}
		nestedValue := nested.ObjectValue()["nested-value"].StringValue()
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"the-output": resource.NewProperty(resource.PropertyMap{
					"nested-output": resource.NewProperty(nestedValue),
				}),
			},
			Status: resource.StatusOK,
		}, nil
	case "kebab-names:kebab-module:another-resource":
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

func (p *KebabNamesProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "kebab-names:kebab-module:do-something" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}

	theInput, ok := req.Args.GetOk("the-input")
	if !ok || !theInput.IsString() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("the-input", fmt.Sprintf("missing string the-input: %v", req.Args)),
		}, nil
	}

	nested, ok := req.Args.GetOk("nested")
	if !ok || !nested.IsMap() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("nested", fmt.Sprintf("missing object nested: %v", req.Args)),
		}, nil
	}
	nestedValue, ok := nested.AsMap().GetOk("value")
	if !ok || !nestedValue.IsString() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("nested", fmt.Sprintf("missing string value: %v", nested)),
		}, nil
	}
	// Whether the schema default for default-value is applied client-side varies by SDK: typed
	// inputs (Node.js, Go, Python classes) apply it, but Python dict inputs bypass the input
	// class __init__ where defaults are set. Tolerate its absence, but reject any other value.
	if defaultValue, ok := nested.AsMap().GetOk("default-value"); ok {
		if !defaultValue.IsString() || defaultValue.AsString() != "defaulted" {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("nested", fmt.Sprintf("default-value is not \"defaulted\": %v", nested)),
			}, nil
		}
	}

	return plugin.InvokeResponse{
		Properties: property.NewMap(map[string]property.Value{
			"the-output": property.New(theInput.AsString() + " " + nestedValue.AsString()),
		}),
	}, nil
}

func (p *KebabNamesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.Version{Major: 52}
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *KebabNamesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *KebabNamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *KebabNamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *KebabNamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *KebabNamesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *KebabNamesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *KebabNamesProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		Status: resource.StatusUnknown,
	}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}

func (p *KebabNamesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Status: resource.StatusUnknown,
	}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
