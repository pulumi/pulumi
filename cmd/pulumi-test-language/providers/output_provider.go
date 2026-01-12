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
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// A provider where the inputs are a subset of outputs for testing unknown values in programs.
type OutputProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*OutputProvider)(nil)

func (p *OutputProvider) Close() error {
	return nil
}

func (p *OutputProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *OutputProvider) Pkg() tokens.Package {
	return "output"
}

func (p *OutputProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("23.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *OutputProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "output",
		Version: "23.0.0",
		Resources: map[string]schema.ResourceSpec{
			"output:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{
								Type: "number",
							},
						},
						"output": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"value", "output"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": {
						TypeSpec: schema.TypeSpec{
							Type: "number",
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

func (p *OutputProvider) CheckConfig(
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
	if version.StringValue() != "23.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 23.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *OutputProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "output:index:Resource"
	if req.URN.Type() != "output:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect just the number value
	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "missing value"),
		}, nil
	}
	if !value.IsNumber() && !value.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "value is not a number"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *OutputProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "output:index:Resource"
	if req.URN.Type() != "output:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	out := resource.NewProperty(resource.Computed{})
	// Only generate the output property during an actual up
	if !req.Preview {
		out = resource.NewProperty(
			strings.Repeat("hello", int(req.Properties["value"].NumberValue())))
	}

	properties := resource.PropertyMap{
		"value":  req.Properties["value"],
		"output": out,
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *OutputProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	// URN should be of the form "output:index:Resource"
	if req.URN.Type() != "output:index:Resource" {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	out := resource.NewProperty(resource.Computed{})
	// Only generate the output property during an actual up
	if !req.Preview {
		out = resource.NewProperty(
			strings.Repeat("hello", int(req.NewInputs["value"].NumberValue())))
	}

	properties := resource.PropertyMap{
		"value":  req.NewInputs["value"],
		"output": out,
	}

	return plugin.UpdateResponse{
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *OutputProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *OutputProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *OutputProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *OutputProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *OutputProvider) Diff(
	ctx context.Context, req plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	// URN should be of the form "output:index:Resource"
	if req.URN.Type() != "output:index:Resource" {
		return plugin.DiffResponse{}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	changes := plugin.DiffNone
	var changedKeys []resource.PropertyKey
	if !req.OldInputs["value"].DeepEquals(req.NewInputs["value"]) {
		changes = plugin.DiffSome
		changedKeys = append(changedKeys, "value")
	}

	return plugin.DiffResponse{
		Changes:     changes,
		ChangedKeys: changedKeys,
	}, nil
}

func (p *OutputProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *OutputProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	if req.URN.Type() != "output:index:Resource" {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  req.Inputs,
			Outputs: req.State,
		},
		Status: resource.StatusOK,
	}, nil
}
