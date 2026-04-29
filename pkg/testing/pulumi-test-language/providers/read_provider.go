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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type ReadProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ReadProvider)(nil)

func (p *ReadProvider) Close() error {
	return nil
}

func (p *ReadProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ReadProvider) Pkg() tokens.Package {
	return "read"
}

func (p *ReadProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "read",
		Version: "39.0.0",
		Resources: map[string]schema.ResourceSpec{
			"read:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"lookup": {TypeSpec: schema.TypeSpec{Type: "string"}},
						"value":  {TypeSpec: schema.TypeSpec{Type: "boolean"}},
					},
					Required: []string{"lookup", "value"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": {TypeSpec: schema.TypeSpec{Type: "boolean"}},
				},
				RequiredInputs: []string{"value"},
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

func (p *ReadProvider) CheckConfig(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{}, nil
}

func (p *ReadProvider) Check(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	if req.URN.Type() != "read:index:Resource" {
		return plugin.CheckResponse{}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ReadProvider) Create(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	if req.URN.Type() != "read:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	id := "created"
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ReadProvider) DiffConfig(context.Context, plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReadProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReadProvider) Update(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ReadProvider) Delete(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *ReadProvider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	if req.URN.Type() != "read:index:Resource" {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	lookup, ok := req.Inputs["lookup"]
	if !ok || !lookup.IsString() {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, errors.New("lookup input is required and must be a string")
	}

	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID: req.ID,
			Inputs: resource.PropertyMap{
				"lookup": lookup,
			},
			Outputs: resource.PropertyMap{
				"lookup": lookup,
				"value":  resource.NewProperty(true),
			},
		},
		Status: resource.StatusOK,
	}, nil
}

func (p *ReadProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ref(semver.MustParse("39.0.0")),
	}, nil
}

func (p *ReadProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ReadProvider) GetMapping(context.Context, plugin.GetMappingRequest) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ReadProvider) GetMappings(context.Context, plugin.GetMappingsRequest) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}
