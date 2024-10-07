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

// SimpleOverlappingNameProvider is a simple provider that has a same named
// package the SimpleProvider.
type SimpleOverlappingNameProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SimpleOverlappingNameProvider)(nil)

func (p *SimpleOverlappingNameProvider) Close() error {
	return nil
}

func (p *SimpleOverlappingNameProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SimpleOverlappingNameProvider) Pkg() tokens.Package {
	return "simpleoverlap"
}

func (p *SimpleOverlappingNameProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "simpleoverlap",
		Version: "2.0.0",
		Resources: map[string]schema.ResourceSpec{
			"simpleoverlap:overlapping_pkg:OverlapResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"out": {
							TypeSpec: schema.TypeSpec{
								Ref: "/simple/v2.0.0/schema.json#/resources/simple:index:Resource",
							},
						},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": {
						TypeSpec: schema.TypeSpec{
							Type: "boolean",
						},
					},
				},
				RequiredInputs: []string{"value"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SimpleOverlappingNameProvider) CheckConfig(
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
	if version.StringValue() != "2.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 2.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SimpleOverlappingNameProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "simpleoverlap:index:Resource"
	if req.URN.Type() != "simpleoverlap:overlapping_pkg:OverlapResource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect the inner simple provider's resource.
	value, ok := req.News["out"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("out", "missing out"),
		}, nil
	}
	if !value.IsObject() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("out", "out is not an object"),
		}, nil
	}
	if len(req.News) != 2 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *SimpleOverlappingNameProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "simpleoverlap:index:Resource"
	if req.URN.Type() != "simpleoverlap:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *SimpleOverlappingNameProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("2.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SimpleOverlappingNameProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SimpleOverlappingNameProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SimpleOverlappingNameProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SimpleOverlappingNameProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleOverlappingNameProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleOverlappingNameProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
