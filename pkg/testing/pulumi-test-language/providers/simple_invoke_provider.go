// Copyright 2016, Pulumi Corporation.
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
	"fmt"
	"slices"
	"sync"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

type SimpleInvokeProvider struct {
	plugin.UnimplementedProvider

	// The texts of the StringResources created (not previewed) by this
	// provider instance. getText fails when called with a text that has not
	// been created, so tests can detect invokes that run before the resource
	// providing their argument exists.
	mu      sync.Mutex
	created []string
}

var _ plugin.Provider = (*SimpleInvokeProvider)(nil)

func (p *SimpleInvokeProvider) Close() error {
	return nil
}

func (p *SimpleInvokeProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SimpleInvokeProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("10.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SimpleInvokeProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"text": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	resourceRequired := []string{"text"}

	pkg := schema.PackageSpec{
		Name:    "simple-invoke",
		Version: "10.0.0",
		Resources: map[string]schema.ResourceSpec{
			// A small resource that just has a single string property.
			"simple-invoke:index:StringResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"simple-invoke:index:myInvoke": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"value"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
			"simple-invoke:index:invokeWithDefault": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
							Default: "default",
						},
					},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
			"simple-invoke:index:unit": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
			"simple-invoke:index:secretInvoke": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						// indicates that the response should be wrapped as a secret
						"secretResponse": {
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
						},
					},
					Required: []string{"value", "secretResponse"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"response": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
							"secret": {
								TypeSpec: schema.TypeSpec{
									Type: "boolean",
								},
							},
						},
						Required: []string{"response", "secret"},
					},
				},
			},
			// echoMap returns its stringMap argument unchanged. Used to verify that map keys
			// (including keys starting with "__") survive a round-trip through an invoke.
			"simple-invoke:index:echoMap": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"stringMap": {
							TypeSpec: schema.TypeSpec{
								Type:                 "object",
								AdditionalProperties: &schema.TypeSpec{Type: "string"},
							},
						},
					},
					Required: []string{"stringMap"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"stringMap": {
								TypeSpec: schema.TypeSpec{
									Type:                 "object",
									AdditionalProperties: &schema.TypeSpec{Type: "string"},
								},
							},
						},
						Required: []string{"stringMap"},
					},
				},
			},
			"simple-invoke:index:getText": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"text": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"text"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SimpleInvokeProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
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
	if version.AsString() != "10.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 10.0.0"),
		}, nil
	}

	if req.News.Len() != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SimpleInvokeProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	switch req.Tok {
	case "simple-invoke:index:myInvoke":
		value, ok := req.Args.GetOk("value")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "missing value"),
			}, nil
		}

		if value.IsComputed() {
			return plugin.InvokeResponse{
				// providers should not get computed values (during preview)
				// since we bail out early in the core SDKs or generated provider SDKs
				// when we encounter unknowns
				Failures: makeCheckFailure("value", "value is unknown when calling myInvoke"),
			}, nil
		}

		if !value.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "is not a string"),
			}, nil
		}

		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{
				"result": property.New(value.AsString() + " world"),
			}),
		}, nil
	case "simple-invoke:index:invokeWithDefault":
		value, ok := req.Args.GetOk("value")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "missing value"),
			}, nil
		}

		if !value.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "is not a string"),
			}, nil
		}

		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{
				"result": value,
			}),
		}, nil
	case "simple-invoke:index:myInvokeScalar":
		value, ok := req.Args.GetOk("value")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "missing value"),
			}, nil
		}

		if value.IsComputed() {
			return plugin.InvokeResponse{
				// providers should not get computed values (during preview)
				// since we bail out early in the core SDKs or generated provider SDKs
				// when we encounter unknowns
				Failures: makeCheckFailure("value", "value is unknown when calling myInvokeScalar"),
			}, nil
		}

		if !value.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "is not a string"),
			}, nil
		}

		// Single value returns work because SDKs automatically extract single value returns in their
		// invoke implementations.
		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{
				"result": property.New(true),
			}),
		}, nil
	case "simple-invoke:index:unit":
		if req.Args.Len() > 0 {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.Args)),
			}, nil
		}

		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{
				"result": property.New("Hello world"),
			}),
		}, nil
	case "simple-invoke:index:secretInvoke":
		value, ok := req.Args.GetOk("value")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "missing value"),
			}, nil
		}

		if value.IsComputed() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "value is unknown when calling secretInvoke"),
			}, nil
		}

		valueIsSecret := value.Secret()

		if !value.IsString() {
			reason := fmt.Sprintf("value is not a string: %#v", value)
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", reason),
			}, nil
		}

		secretResponse, ok := req.Args.GetOk("secretResponse")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("secretResponse", "missing secretResponse"),
			}, nil
		}
		if !secretResponse.IsBool() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("secretResponse", "secretResponse is not a bool"),
			}, nil
		}

		// if the secretResponse is true, wrap the response as a secret
		response := property.New(value.AsString() + " world")
		if secretResponse.AsBool() || valueIsSecret {
			response = response.WithSecret(true)
		}
		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{
				"response": response,
				"secret":   secretResponse,
			}),
		}, nil
	case "simple-invoke:index:echoMap":
		sm, ok := req.Args.GetOk("stringMap")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("stringMap", "missing stringMap"),
			}, nil
		}
		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{"stringMap": sm}),
		}, nil
	case "simple-invoke:index:getText":
		text, ok := req.Args.GetOk("text")
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("text", "missing text"),
			}, nil
		}

		if text.IsComputed() {
			return plugin.InvokeResponse{
				// providers should not get computed values (during preview)
				// since we bail out early in the core SDKs or generated provider SDKs
				// when we encounter unknowns
				Failures: makeCheckFailure("text", "text is unknown when calling getText"),
			}, nil
		}

		if !text.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("text", "text is not a string"),
			}, nil
		}

		p.mu.Lock()
		created := slices.Contains(p.created, text.AsString())
		p.mu.Unlock()
		if !created {
			// SDKs must not call this invoke before the StringResource
			// providing the text argument has been created, e.g. during
			// preview, when the argument is known but the resource does not
			// exist yet.
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("text",
					fmt.Sprintf("no StringResource with text %q has been created", text.AsString())),
			}, nil
		}

		return plugin.InvokeResponse{
			Properties: property.NewMap(map[string]property.Value{
				"result": property.New(text.AsString() + " world"),
			}),
		}, nil
	}
	return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}

func (p *SimpleInvokeProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "simple-invoke:index:StringResource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("expected exactly one property: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *SimpleInvokeProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "simple-invoke:index:StringResource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	text := "Goodbye"
	if req.Preview {
		id = ""
	} else {
		p.mu.Lock()
		p.created = append(p.created, text)
		p.mu.Unlock()
	}

	return plugin.CreateResponse{
		ID: resource.ID(id),
		Properties: resource.PropertyMap{
			"text": resource.NewProperty(text),
		},
		Status: resource.StatusOK,
	}, nil
}
