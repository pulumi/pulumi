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
	"errors"
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ReadIdRewriteProvider is a provider whose Read method returns a resolved ID that differs from
// the ID supplied on the request (prefixed with "rewritten-"). This exercises the SDK contract
// that a resource's .id reflects the id returned by the provider, not merely the one passed in.
type ReadIdRewriteProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ReadIdRewriteProvider)(nil)

func (p *ReadIdRewriteProvider) Close() error {
	return nil
}

func (p *ReadIdRewriteProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ReadIdRewriteProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "read-id-rewrite",
		Version: "58.0.0",
		Resources: map[string]schema.ResourceSpec{
			"read-id-rewrite:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"lookup": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"lookup"},
				},
				InputProperties: map[string]schema.PropertySpec{},
				StateInputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"lookup": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"lookup"},
				},
			},
		},
	}

	bytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: bytes}, err
}

func (p *ReadIdRewriteProvider) CheckConfig(
	context.Context, plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{}, nil
}

func (p *ReadIdRewriteProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "read-id-rewrite:index:Resource" {
		return plugin.CheckResponse{}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *ReadIdRewriteProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReadIdRewriteProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReadIdRewriteProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *ReadIdRewriteProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	if req.URN.Type() != "read-id-rewrite:index:Resource" {
		return plugin.ReadResponse{Status: resource.StatusUnknown}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	inputs := resource.ToResourcePropertyMap(req.Inputs)
	lookup, ok := inputs["lookup"]
	if !ok || !lookup.IsString() {
		return plugin.ReadResponse{Status: resource.StatusUnknown},
			errors.New("lookup input is required and must be a string")
	}

	readInputs := resource.FromResourcePropertyMap(resource.PropertyMap{"lookup": lookup})
	readOutputs := resource.FromResourcePropertyMap(resource.PropertyMap{"lookup": lookup})
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      "rewritten-" + req.ID,
			Inputs:  &readInputs,
			Outputs: &readOutputs,
		},
		Status: resource.StatusOK,
	}, nil
}

func (p *ReadIdRewriteProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{Version: new(semver.MustParse("58.0.0"))}, nil
}

func (p *ReadIdRewriteProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ReadIdRewriteProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ReadIdRewriteProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}
