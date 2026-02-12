// Copyright 2016-2025, Pulumi Corporation.
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

type DiscriminatedUnionProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DiscriminatedUnionProvider)(nil)

func (DiscriminatedUnionProvider) version() string {
	return "30.0.0"
}

func (p *DiscriminatedUnionProvider) Pkg() tokens.Package {
	return "discriminated-union"
}

func (p *DiscriminatedUnionProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	catType := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"petType": {
				TypeSpec: schema.TypeSpec{Type: "string"},
				Const:    "cat",
			},
			"meow": {
				TypeSpec: schema.TypeSpec{Type: "string"},
			},
		},
		Required: []string{"petType"},
	}

	dogType := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"petType": {
				TypeSpec: schema.TypeSpec{Type: "string"},
				Const:    "dog",
			},
			"bark": {
				TypeSpec: schema.TypeSpec{Type: "string"},
			},
		},
		Required: []string{"petType"},
	}

	petUnion := schema.TypeSpec{
		OneOf: []schema.TypeSpec{
			{Ref: fmt.Sprintf("#/types/%s:index:Cat", p.Pkg())},
			{Ref: fmt.Sprintf("#/types/%s:index:Dog", p.Pkg())},
		},
		Discriminator: &schema.DiscriminatorSpec{
			PropertyName: "petType",
			Mapping: map[string]string{
				"cat": fmt.Sprintf("#/types/%s:index:Cat", p.Pkg()),
				"dog": fmt.Sprintf("#/types/%s:index:Dog", p.Pkg()),
			},
		},
	}

	petsArray := schema.TypeSpec{
		Type:  "array",
		Items: &petUnion,
	}

	resourceProperties := map[string]schema.PropertySpec{
		"pet": {
			TypeSpec: petUnion,
		},
		"pets": {
			TypeSpec: petsArray,
		},
	}

	pkg := schema.PackageSpec{
		Name:    string(p.Pkg()),
		Version: p.version(),
		Types: map[string]schema.ComplexTypeSpec{
			fmt.Sprintf("%s:index:Cat", p.Pkg()): {ObjectTypeSpec: catType},
			fmt.Sprintf("%s:index:Dog", p.Pkg()): {ObjectTypeSpec: dogType},
		},
		Resources: map[string]schema.ResourceSpec{
			fmt.Sprintf("%s:index:Example", p.Pkg()): {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
				},
				InputProperties: resourceProperties,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *DiscriminatedUnionProvider) Close() error {
	return nil
}

func (p *DiscriminatedUnionProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DiscriminatedUnionProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse(p.version())
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *DiscriminatedUnionProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *DiscriminatedUnionProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DiscriminatedUnionProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DiscriminatedUnionProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DiscriminatedUnionProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if string(req.URN.Type()) != fmt.Sprintf("%s:index:Example", p.Pkg()) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if string(req.URN.Type()) == fmt.Sprintf("%s:index:Example", p.Pkg()) {
		return plugin.CreateResponse{
			ID:         resource.ID("new-resource-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
	return plugin.CreateResponse{Status: resource.StatusUnknown},
		fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
