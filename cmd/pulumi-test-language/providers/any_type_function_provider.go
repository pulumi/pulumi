// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type AnyTypeFunctionProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*AnyTypeFunctionProvider)(nil)

func (p *AnyTypeFunctionProvider) Close() error {
	return nil
}

func (p *AnyTypeFunctionProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *AnyTypeFunctionProvider) Pkg() tokens.Package {
	return "any-type-function"
}

func (p *AnyTypeFunctionProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "any-type-function",
		Version: "15.0.0",
		Functions: map[string]schema.FunctionSpec{
			"any-type-function:index:dynListToDyn": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"inputs": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Ref: "pulumi.json#/Any",
								},
							},
						},
					},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Ref: "pulumi.json#/Any",
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

func (p *AnyTypeFunctionProvider) CheckConfig(
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
	if version.StringValue() != "15.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 15.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *AnyTypeFunctionProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{}, nil
}

func (p *AnyTypeFunctionProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{}, nil
}

func (p *AnyTypeFunctionProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("15.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *AnyTypeFunctionProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *AnyTypeFunctionProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *AnyTypeFunctionProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *AnyTypeFunctionProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *AnyTypeFunctionProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *AnyTypeFunctionProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *AnyTypeFunctionProvider) Invoke(
	ctx context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok == "any-type-function:index:dynListToDyn" {
		value, ok := req.Args["inputs"]
		if !ok {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("inputs", "missing inputs"),
			}, nil
		}

		if !value.IsArray() {
			return plugin.InvokeResponse{
				Failures: makeCheckFailure("inputs", "inputs is not an array"),
			}, nil
		}

		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{
				"result": resource.NewObjectProperty(resource.PropertyMap{
					"resultProperty": resource.NewStringProperty("resultValue"),
				}),
			},
		}, nil
	}

	return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}
