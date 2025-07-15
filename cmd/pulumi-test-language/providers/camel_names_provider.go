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

// CamelNamesProvider is used to test camel naming features in the Pulumi SDK, and that they are used
// correctly across sdk-gen and program-gen. We make sure every name makes use of "camelCase", the package,
// modules, types, etc.
type CamelNamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*CamelNamesProvider)(nil)

func (p *CamelNamesProvider) Close() error {
	return nil
}

func (p *CamelNamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *CamelNamesProvider) Pkg() tokens.Package {
	return "camelNames"
}

func (p *CamelNamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "camelNames",
		Version: "19.0.0",
		Resources: map[string]schema.ResourceSpec{
			"camelNames:CoolModule:SomeResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"theOutput": {
							TypeSpec: schema.TypeSpec{
								Type: "boolean",
							},
						},
					},
					Required: []string{"theOutput"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"theInput": {
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
				},
				RequiredInputs: []string{"theInput"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *CamelNamesProvider) CheckConfig(
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
	if version.StringValue() != "19.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 19.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *CamelNamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "camelNames:CoolModule:SomeResource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect just the boolean value
	value, ok := req.News["theInput"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("theInput", "missing theInput"),
		}, nil
	}
	if !value.IsBool() && !value.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("theInput", "theInput is not a boolean"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *CamelNamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "camelNames:CoolModule:SomeResource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	value, ok := req.Properties["theInput"]
	if !ok {
		return plugin.CreateResponse{}, fmt.Errorf("missing theInput property")
	}

	properties := resource.PropertyMap{
		"theOutput": value,
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *CamelNamesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	if req.URN.Type() != "camelNames:CoolModule:SomeResource" {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *CamelNamesProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("19.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *CamelNamesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *CamelNamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *CamelNamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *CamelNamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *CamelNamesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *CamelNamesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *CamelNamesProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	if req.URN.Type() != "camelNames:CoolModule:SomeResource" {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID: req.ID,
			Inputs: resource.PropertyMap{
				"theInput": resource.NewBoolProperty(true),
			},
			Outputs: resource.PropertyMap{
				"theOutput": resource.NewBoolProperty(true),
			},
		},
		Status: resource.StatusOK,
	}, nil
}
