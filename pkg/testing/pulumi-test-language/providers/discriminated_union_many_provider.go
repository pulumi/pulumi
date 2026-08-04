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

// DiscriminatedUnionManyProvider exercises a discriminated union with many cases
// (10 variants) and cases that share property names. Variants "variant1" and
// "variant2" have exactly the same set of properties (payload + extra), which
// should be valid because the discriminator disambiguates them.
type DiscriminatedUnionManyProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DiscriminatedUnionManyProvider)(nil)

func (DiscriminatedUnionManyProvider) version() semver.Version {
	return semver.Version{Major: 42}
}

func (p *DiscriminatedUnionManyProvider) pkg() tokens.Package {
	return "discriminated-union-many"
}

func (p *DiscriminatedUnionManyProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	stringT := schema.TypeSpec{Type: "string"}
	intT := schema.TypeSpec{Type: "integer"}
	boolT := schema.TypeSpec{Type: "boolean"}

	// Build 10 variants. All variants share the property "payload" (a string).
	// variant1 and variant2 additionally share the property "extra" (a string) —
	// i.e. their property shapes are identical, and the discriminator is the
	// only thing distinguishing them.
	type variantDef struct {
		token string
		kind  string
		extra map[string]schema.PropertySpec
	}

	variants := []variantDef{
		{"Variant1", "variant1", map[string]schema.PropertySpec{"extra": {TypeSpec: stringT}}},
		{"Variant2", "variant2", map[string]schema.PropertySpec{"extra": {TypeSpec: stringT}}},
		{"Variant3", "variant3", map[string]schema.PropertySpec{"count": {TypeSpec: intT}}},
		{"Variant4", "variant4", map[string]schema.PropertySpec{"enabled": {TypeSpec: boolT}}},
		{"Variant5", "variant5", map[string]schema.PropertySpec{"label": {TypeSpec: stringT}}},
		{"Variant6", "variant6", map[string]schema.PropertySpec{"code": {TypeSpec: intT}}},
		{"Variant7", "variant7", map[string]schema.PropertySpec{"message": {TypeSpec: stringT}}},
		{"Variant8", "variant8", map[string]schema.PropertySpec{"size": {TypeSpec: intT}}},
		{"Variant9", "variant9", map[string]schema.PropertySpec{"flag": {TypeSpec: boolT}}},
		{"Variant10", "variant10", map[string]schema.PropertySpec{"note": {TypeSpec: stringT}}},
	}

	types := map[string]schema.ComplexTypeSpec{}
	oneOf := make([]schema.TypeSpec, 0, len(variants))
	mapping := map[string]string{}
	for _, v := range variants {
		props := map[string]schema.PropertySpec{
			"discriminantKind": {TypeSpec: stringT, Const: v.kind},
			"payload":          {TypeSpec: stringT},
		}
		for k, s := range v.extra {
			props[k] = s
		}
		types[fmt.Sprintf("%s:index:%s", p.pkg(), v.token)] = schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: props,
				Required:   []string{"discriminantKind"},
			},
		}
		ref := fmt.Sprintf("#/types/%s:index:%s", p.pkg(), v.token)
		oneOf = append(oneOf, schema.TypeSpec{Ref: ref})
		mapping[v.kind] = ref
	}

	union := schema.TypeSpec{
		OneOf: oneOf,
		Discriminator: &schema.DiscriminatorSpec{
			PropertyName: "discriminantKind",
			Mapping:      mapping,
		},
	}

	// A subset union over the first three variants. A value of this type should
	// be assignable to a field typed as the full union above, so we can chain
	// a SubsetExample's output into an Example's input.
	subsetOneOf := oneOf[:3]
	subsetMapping := map[string]string{}
	for _, v := range variants[:3] {
		subsetMapping[v.kind] = mapping[v.kind]
	}
	subsetUnion := schema.TypeSpec{
		OneOf: subsetOneOf,
		Discriminator: &schema.DiscriminatorSpec{
			PropertyName: "discriminantKind",
			Mapping:      subsetMapping,
		},
	}

	resourceProperties := map[string]schema.PropertySpec{
		"unionOf": {TypeSpec: union},
	}
	subsetResourceProperties := map[string]schema.PropertySpec{
		"unionOf": {TypeSpec: subsetUnion},
	}

	pkg := schema.PackageSpec{
		Name:    string(p.pkg()),
		Version: p.version().String(),
		Types:   types,
		Resources: map[string]schema.ResourceSpec{
			fmt.Sprintf("%s:index:SubsetExample", p.pkg()): {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: subsetResourceProperties,
				},
				InputProperties: subsetResourceProperties,
			},
			fmt.Sprintf("%s:index:Example", p.pkg()): {
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

func (p *DiscriminatedUnionManyProvider) Close() error {
	return nil
}

func (p *DiscriminatedUnionManyProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DiscriminatedUnionManyProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionManyProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ptr(p.version()),
	}, nil
}

func (p *DiscriminatedUnionManyProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *DiscriminatedUnionManyProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DiscriminatedUnionManyProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DiscriminatedUnionManyProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DiscriminatedUnionManyProvider) isKnownType(t tokens.Type) bool {
	s := string(t)
	return s == fmt.Sprintf("%s:index:Example", p.pkg()) ||
		s == fmt.Sprintf("%s:index:SubsetExample", p.pkg())
}

func (p *DiscriminatedUnionManyProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if !p.isKnownType(req.URN.Type()) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionManyProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if p.isKnownType(req.URN.Type()) {
		return plugin.CreateResponse{
			ID:         resource.ID("new-resource-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
	return plugin.CreateResponse{Status: resource.StatusUnknown},
		fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
