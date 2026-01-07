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

// A small provider that uses documentation references for various properties. The correctness of documentation can't
// really be conformance tested, but having snapshots of the SDK generated for each language will help catch
// documentation rendering regressions.
type DocsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DocsProvider)(nil)

func (p *DocsProvider) Close() error {
	return nil
}

func (p *DocsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DocsProvider) Pkg() tokens.Package {
	return "docs"
}

func (p *DocsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "docs",
		Version: "25.0.0",
		Resources: map[string]schema.ResourceSpec{
			"docs:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:        "object",
					Description: "<pulumi ref=\"#/resources/docs:index:Resource\"/> is a basic resource. Use <pulumi ref=\"#/functions/docs:index:fun\"/> to set <pulumi ref=\"#/resources/docs:index:Resource/inputProperties/in\"/> using <pulumi ref=\"#/functions/docs:index:fun/outputProperties/out\"/>.",
					Properties: map[string]schema.PropertySpec{
						"in": {
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
							Description: "Will be set to the same as <pulumi ref=\"#/resources/docs:index:Resource/inputProperties/in\"/>.",
						},
						"out": {
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
							Description: "Will be set to the opposite of <pulumi ref=\"#/resources/docs:index:Resource/inputProperties/in\"/>.",
						},
					},
					Required: []string{"out"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"in": {
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
						Description: "Will be used to set <pulumi ref=\"#/resources/docs:index:Resource/properties/in\"/> and <pulumi ref=\"#/resources/docs:index:Resource/properties/out\"/>.",
					},
				},
				RequiredInputs: []string{"in"},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"docs:index:fun": {
				Description: "<pulumi ref=\"#/functions/docs:index:fun\"/> is a basic function for setting <pulumi ref=\"#/resources/docs:index:Resource/properties/in\"/> on <pulumi ref=\"#/resources/docs:index:Resource\"/>.",
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"in": {
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
							Description: "Will be used to set <pulumi ref=\"#/functions/docs:index:fun/outputProperties/out\"/>.",
						},
					},
					Required: []string{"in"},
				},
				Outputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"out": {
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
							Description: "Will be the opposite of <pulumi ref=\"#/functions/docs:index:fun/inputProperties/in\"/> can be used to set <pulumi ref=\"#/resources/docs:index:Resource/inputProperties/in\"/>.",
						},
					},
					Required: []string{"out"},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *DocsProvider) CheckConfig(
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
	if version.StringValue() != "25.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 25.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DocsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "docs:index:Resource"
	if req.URN.Type() != "docs:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	assertField := func(key resource.PropertyKey, typ string,
		assertType func(resource.PropertyValue) bool,
	) *plugin.CheckResponse {
		v, ok := req.News[key]
		if !ok {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure(key, "missing value"),
			}
		}
		if !assertType(v) {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure(key, "value is not a "+typ),
			}
		}

		return nil
	}

	// Expect all required properties
	check := assertField("in", "boolean", resource.PropertyValue.IsBool)
	if check != nil {
		return *check, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *DocsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "docs:index:Resource"
	if req.URN.Type() != "docs:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	properties := resource.NewPropertyMapFromMap(map[string]any{
		"in":  req.Properties["in"],
		"out": !req.Properties["in"].BoolValue(),
	})

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *DocsProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	// Function should be of the form "docs:index:fun"
	if req.Tok != "docs:index:fun" {
		return plugin.InvokeResponse{}, fmt.Errorf("invalid function token: %s", req.Tok)
	}

	in := req.Args["in"]
	if !in.IsBool() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("in", fmt.Sprintf("invalid argument 'in': %v", in)),
		}, nil
	}

	out := !in.BoolValue()

	return plugin.InvokeResponse{
		Properties: resource.NewPropertyMapFromMap(map[string]any{
			"out": out,
		}),
	}, nil
}

func (p *DocsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("25.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *DocsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *DocsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DocsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DocsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DocsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DocsProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
