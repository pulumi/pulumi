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

func (DiscriminatedUnionProvider) version() semver.Version {
	return semver.Version{Major: 31}
}

func (p *DiscriminatedUnionProvider) Pkg() tokens.Package {
	return "discriminated-union"
}

func (p *DiscriminatedUnionProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	variantOneType := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"discriminantKind": {
				TypeSpec: schema.TypeSpec{Type: "string"},
				Const:    "variant1",
			},
			"field1": {
				TypeSpec: schema.TypeSpec{Type: "string"},
			},
		},
		Required: []string{"discriminantKind"},
	}

	variantTwoType := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"discriminantKind": {
				TypeSpec: schema.TypeSpec{Type: "string"},
				Const:    "variant2",
			},
			"field2": {
				TypeSpec: schema.TypeSpec{Type: "string"},
			},
		},
		Required: []string{"discriminantKind"},
	}

	union := schema.TypeSpec{
		OneOf: []schema.TypeSpec{
			{Ref: fmt.Sprintf("#/types/%s:index:VariantOne", p.Pkg())},
			{Ref: fmt.Sprintf("#/types/%s:index:VariantTwo", p.Pkg())},
		},
		Discriminator: &schema.DiscriminatorSpec{
			PropertyName: "discriminantKind",
			Mapping: map[string]string{
				"variant1": fmt.Sprintf("#/types/%s:index:VariantOne", p.Pkg()),
				"variant2": fmt.Sprintf("#/types/%s:index:VariantTwo", p.Pkg()),
			},
		},
	}

	arrayOfUnion := schema.TypeSpec{
		Type:  "array",
		Items: &union,
	}

	resourceProperties := map[string]schema.PropertySpec{
		"unionOf": {
			TypeSpec: union,
		},
		"arrayOfUnionOf": {
			TypeSpec: arrayOfUnion,
		},
	}

	pkg := schema.PackageSpec{
		Name:    string(p.Pkg()),
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			fmt.Sprintf("%s:index:VariantOne", p.Pkg()): {ObjectTypeSpec: variantOneType},
			fmt.Sprintf("%s:index:VariantTwo", p.Pkg()): {ObjectTypeSpec: variantTwoType},
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
	return plugin.PluginInfo{
		Version: ref(p.version()),
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
