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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type SimpleInvokeWithScalarReturnProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SimpleInvokeWithScalarReturnProvider)(nil)

func (p *SimpleInvokeWithScalarReturnProvider) Close() error {
	return nil
}

func (p *SimpleInvokeWithScalarReturnProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SimpleInvokeWithScalarReturnProvider) Pkg() tokens.Package {
	return "simple-invoke-with-scalar-return"
}

func (p *SimpleInvokeWithScalarReturnProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("17.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SimpleInvokeWithScalarReturnProvider) GetSchema(
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
		Name:    "simple-invoke-with-scalar-return",
		Version: "17.0.0",
		Resources: map[string]schema.ResourceSpec{
			// A small resource that just has a single string property.
			"simple-invoke-with-scalar-return:index:StringResource": {
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
			"simple-invoke-with-scalar-return:index:myInvokeScalar": {
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
						Type: "boolean",
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SimpleInvokeWithScalarReturnProvider) CheckConfig(
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
	if version.StringValue() != "17.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 17.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SimpleInvokeWithScalarReturnProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok == "simple-invoke-with-scalar-return:index:myInvokeScalar" {
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
			Properties: resource.PropertyMap{
				"result": resource.NewBoolProperty(true),
			},
		}, nil
	}

	return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}

func (p *SimpleInvokeWithScalarReturnProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "simple-invoke-with-scalar-return:index:StringResource" {
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

func (p *SimpleInvokeWithScalarReturnProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "simple-invoke-with-scalar-return:index:StringResource" {
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
			"text": resource.NewStringProperty("Goodbye"),
		},
		Status: resource.StatusOK,
	}, nil
}
