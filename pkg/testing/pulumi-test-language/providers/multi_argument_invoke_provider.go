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
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// MultiArgumentInvokeProvider exposes a function that uses multiArgumentInputs, so its inputs are
// passed positionally rather than as a single argument bag.
type MultiArgumentInvokeProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*MultiArgumentInvokeProvider)(nil)

func (p *MultiArgumentInvokeProvider) Close() error {
	return nil
}

func (p *MultiArgumentInvokeProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *MultiArgumentInvokeProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("44.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *MultiArgumentInvokeProvider) GetSchema(
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
		Name:    "multi-argument-invoke",
		Version: "44.0.0",
		Resources: map[string]schema.ResourceSpec{
			// A small resource that just has a single string property.
			"multi-argument-invoke:index:StringResource": {
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
			// A function whose inputs are passed positionally. "first" is required and "second" is
			// optional, exercising trailing-optional handling in positional program generation.
			"multi-argument-invoke:index:multiArgumentInvoke": {
				MultiArgumentInputs: []string{"first", "second"},
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"first": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
						"second": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"first"},
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

func (p *MultiArgumentInvokeProvider) CheckConfig(
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
	if version.StringValue() != "44.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 44.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *MultiArgumentInvokeProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "multi-argument-invoke:index:multiArgumentInvoke" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}

	first, ok := req.Args["first"]
	if !ok {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("first", "missing first"),
		}, nil
	}
	if !first.IsString() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("first", "first is not a string"),
		}, nil
	}

	result := first.StringValue()
	// "second" is optional; when provided it is appended to the result.
	if second, ok := req.Args["second"]; ok && !second.IsNull() {
		if !second.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("second", "second is not a string"),
			}, nil
		}
		result += " " + second.StringValue()
	}

	return plugin.InvokeResponse{
		Properties: resource.PropertyMap{
			"result": resource.NewProperty(result),
		},
	}, nil
}

func (p *MultiArgumentInvokeProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "multi-argument-invoke:index:StringResource" {
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

func (p *MultiArgumentInvokeProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "multi-argument-invoke:index:StringResource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID: resource.ID(id),
		Properties: resource.PropertyMap{
			"text": resource.NewProperty("Goodbye"),
		},
		Status: resource.StatusOK,
	}, nil
}
