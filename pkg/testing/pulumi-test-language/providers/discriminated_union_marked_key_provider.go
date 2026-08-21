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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// DiscriminatedUnionMarkedKeyProvider tests discriminated unions whose discriminator property is
// not a plain value: the "unionOut" output is returned with an unknown discriminator during
// preview and a secret discriminator during the actual operation.
type DiscriminatedUnionMarkedKeyProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DiscriminatedUnionMarkedKeyProvider)(nil)

func (DiscriminatedUnionMarkedKeyProvider) version() semver.Version {
	return semver.Version{Major: 53}
}

func (p *DiscriminatedUnionMarkedKeyProvider) pkg() tokens.Package {
	return "discriminated-union-marked-key"
}

func (p *DiscriminatedUnionMarkedKeyProvider) GetSchema(
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
			{Ref: fmt.Sprintf("#/types/%s:index:VariantOne", p.pkg())},
			{Ref: fmt.Sprintf("#/types/%s:index:VariantTwo", p.pkg())},
		},
		Discriminator: &schema.DiscriminatorSpec{
			PropertyName: "discriminantKind",
			Mapping: map[string]string{
				"variant1": fmt.Sprintf("#/types/%s:index:VariantOne", p.pkg()),
				"variant2": fmt.Sprintf("#/types/%s:index:VariantTwo", p.pkg()),
			},
		},
	}

	pkg := schema.PackageSpec{
		Name:    string(p.pkg()),
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			fmt.Sprintf("%s:index:VariantOne", p.pkg()): {ObjectTypeSpec: variantOneType},
			fmt.Sprintf("%s:index:VariantTwo", p.pkg()): {ObjectTypeSpec: variantTwoType},
		},
		Resources: map[string]schema.ResourceSpec{
			fmt.Sprintf("%s:index:Example", p.pkg()): {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"unionIn":  {TypeSpec: union},
						"unionOut": {TypeSpec: union},
					},
					Required: []string{"unionIn", "unionOut"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"unionIn": {TypeSpec: union},
				},
				RequiredInputs: []string{"unionIn"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *DiscriminatedUnionMarkedKeyProvider) Close() error {
	return nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ptr(p.version()),
	}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if string(req.URN.Type()) != fmt.Sprintf("%s:index:Example", p.pkg()) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionMarkedKeyProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if string(req.URN.Type()) != fmt.Sprintf("%s:index:Example", p.pkg()) {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	// The discriminator of unionOut is unknown during preview and secret once the resource is
	// actually created.
	discriminant := resource.MakeSecret(resource.NewProperty("variant1"))
	id := resource.ID("new-resource-id")
	if req.Preview {
		discriminant = resource.MakeComputed(resource.NewProperty(""))
		id = ""
	}

	return plugin.CreateResponse{
		ID: id,
		Properties: resource.PropertyMap{
			"unionIn": req.Properties["unionIn"],
			"unionOut": resource.NewProperty(resource.PropertyMap{
				"discriminantKind": discriminant,
				"field1":           resource.NewProperty("hello"),
			}),
		},
		Status: resource.StatusOK,
	}, nil
}
