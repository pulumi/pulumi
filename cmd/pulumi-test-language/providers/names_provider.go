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
	"slices"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// This provider offers resources with names that could potentially cause problems in go
// code.
type NamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*NamesProvider)(nil)

func (p *NamesProvider) Types() []string {
	prefix := p.Pkg().Name().String() + ":index:"
	return []string{
		prefix + "ResMap",
		prefix + "ResArray",
		prefix + "ResList",
		prefix + "ResResource",
	}
}

func (*NamesProvider) Close() error {
	return nil
}

func (p *NamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *NamesProvider) Pkg() tokens.Package {
	return "names"
}

func (p *NamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceRequired := []string{"value"}

	resources := map[string]schema.ResourceSpec{}
	for _, urn := range p.Types() {
		resources[urn] = schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: resourceProperties,
				Required:   resourceRequired,
			},
			InputProperties: resourceProperties,
			RequiredInputs:  resourceRequired,
		}
	}

	pkg := schema.PackageSpec{
		Name:      p.Pkg().String(),
		Version:   "6.0.0",
		Resources: resources,
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *NamesProvider) CheckConfig(
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
	if version.StringValue() != "6.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 6.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *NamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if !slices.Contains(p.Types(), req.URN.Type().String()) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect just the boolean value
	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "missing value"),
		}, nil
	}
	if !value.IsBool() && !value.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "value is not a boolean"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *NamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if !slices.Contains(p.Types(), req.URN.Type().String()) {
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

func (p *NamesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	if !slices.Contains(p.Types(), req.URN.Type().String()) {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *NamesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("6.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *NamesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *NamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *NamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *NamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *NamesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *NamesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *NamesProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	if !slices.Contains(p.Types(), req.URN.Type().String()) {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID: req.ID,
			Inputs: resource.PropertyMap{
				"value": resource.NewProperty(true),
			},
			Outputs: resource.PropertyMap{
				"value": resource.NewProperty(true),
			},
		},
		Status: resource.StatusOK,
	}, nil
}
