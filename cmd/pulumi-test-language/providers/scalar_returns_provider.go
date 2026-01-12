// Copyright 2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type ScalarReturnsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ScalarReturnsProvider)(nil)

func (p *ScalarReturnsProvider) Close() error {
	return nil
}

func (p *ScalarReturnsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ScalarReturnsProvider) Pkg() tokens.Package {
	return "scalar-returns"
}

func (p *ScalarReturnsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("21.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ScalarReturnsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "scalar-returns",
		Version: "21.0.0",
		Functions: map[string]schema.FunctionSpec{
			"scalar-returns:index:invokeSecret": {
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
					TypeSpec: &schema.TypeSpec{
						Type: "number",
					},
				},
			},
			"scalar-returns:index:invokeArray": {
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
					TypeSpec: &schema.TypeSpec{
						Type: "array",
						Items: &schema.TypeSpec{
							Type: "boolean",
						},
					},
				},
			},
			"scalar-returns:index:invokeMap": {
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
					TypeSpec: &schema.TypeSpec{
						Type: "object",
						AdditionalProperties: &schema.TypeSpec{
							Type: "string",
						},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ScalarReturnsProvider) CheckConfig(
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
	if version.StringValue() != "21.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 21.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ScalarReturnsProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok == "scalar-returns:index:invokeSecret" {
		value, ok := req.Args["value"]
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
				Failures: makeCheckFailure("value", "value is unknown when calling invokeSecret"),
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
			Properties: resource.PropertyMap{
				"return": resource.MakeSecret(resource.NewProperty(float64(len(value.StringValue())))),
			},
		}, nil
	}
	if req.Tok == "scalar-returns:index:invokeArray" {
		value, ok := req.Args["value"]
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
				Failures: makeCheckFailure("value", "value is unknown when calling invokeMap"),
			}, nil
		}

		if !value.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "is not a string"),
			}, nil
		}

		result := []resource.PropertyValue{}
		for i := 0; i < len(value.StringValue()); i++ {
			c := value.StringValue()[i]
			if c == 'i' || c == 'o' || c == 'u' || c == 'e' || c == 'a' {
				result = append(result, resource.NewProperty(true))
			} else {
				result = append(result, resource.NewProperty(false))
			}
		}

		// Single value returns work because SDKs automatically extract single value returns in their
		// invoke implementations.
		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{
				"thisCanBeAnything": resource.NewProperty(result),
			},
		}, nil
	}
	if req.Tok == "scalar-returns:index:invokeMap" {
		value, ok := req.Args["value"]
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
				Failures: makeCheckFailure("value", "value is unknown when calling invokeMap"),
			}, nil
		}

		if !value.IsString() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("value", "is not a string"),
			}, nil
		}

		result := resource.NewProperty(resource.PropertyMap{
			"value": resource.NewProperty(value.StringValue() + " world"),
		})

		if value.StringValue() == "secret" {
			result = resource.MakeSecret(result)
		}

		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{
				"result": result,
			},
		}, nil
	}

	return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}
