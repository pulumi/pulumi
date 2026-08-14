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

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ReservedNamesProvider exercises schema names that collide with members generated SDKs
// must define themselves: a resource and a type sharing the `ElementType` token, with an
// `elementType` property that collides with the `ElementType()` method every generated Go
// SDK type must implement.
type ReservedNamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ReservedNamesProvider)(nil)

func (p *ReservedNamesProvider) Close() error {
	return nil
}

func (p *ReservedNamesProvider) version() semver.Version {
	return semver.Version{Major: 51}
}

func (p *ReservedNamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ReservedNamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "reservednames",
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			// A type with the same token as the ElementType resource, whose property
			// shares the type's own name.
			"reservednames:index:ElementType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"elementType": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"elementType"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"reservednames:index:ElementType": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"elementType": {
							TypeSpec: schema.TypeSpec{
								Ref: "#/types/reservednames:index:ElementType",
							},
						},
					},
					Required: []string{"elementType"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"elementType": {
						TypeSpec: schema.TypeSpec{
							Ref: "#/types/reservednames:index:ElementType",
						},
					},
				},
				RequiredInputs: []string{"elementType"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ReservedNamesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ReservedNamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ReservedNamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties.Copy(),
		Status:     resource.StatusOK,
	}, nil
}

func (p *ReservedNamesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ReservedNamesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ptr(p.version()),
	}, nil
}

func (p *ReservedNamesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ReservedNamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ReservedNamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ReservedNamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReservedNamesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReservedNamesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *ReservedNamesProvider) Read(
	_ context.Context, req plugin.ReadRequest,
) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  req.Inputs,
			Outputs: req.State,
		},
		Status: resource.StatusOK,
	}, nil
}
