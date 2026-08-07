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
	"maps"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// DiscriminatedUnionInternalProvider exercises a discriminated union whose
// discriminator property is named "type__". Providers that surface a wire
// format with a reserved discriminator key (e.g. Jackson's "__type", as used
// by pulumi-pulumiservice) cannot put the leading-underscore name in a schema:
// Go renders it as an unexported struct field and Python name-mangles the
// constructor parameter. The trailing-underscore spelling is the escape hatch
// such providers translate to, so codegen must handle it in every language.
// The type also carries a schema-secret union property to exercise secret +
// union codegen together.
type DiscriminatedUnionInternalProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DiscriminatedUnionInternalProvider)(nil)

func (DiscriminatedUnionInternalProvider) version() semver.Version {
	return semver.Version{Major: 50}
}

func (p *DiscriminatedUnionInternalProvider) pkg() tokens.Package {
	return "discriminated-union-internal"
}

func (p *DiscriminatedUnionInternalProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	stringT := schema.TypeSpec{Type: "string"}
	intT := schema.TypeSpec{Type: "integer"}
	boolT := schema.TypeSpec{Type: "boolean"}

	// Three variants, tagged by a "type__" property whose value is the
	// variant's type name, the schema-safe spelling of the Jackson
	// @JsonTypeInfo "__type" wire key.
	type variantDef struct {
		token string
		extra map[string]schema.PropertySpec
	}

	variants := []variantDef{
		{"Alpha", map[string]schema.PropertySpec{"weight": {TypeSpec: intT}}},
		{"Beta", map[string]schema.PropertySpec{"tint": {TypeSpec: stringT}}},
		{"Gamma", map[string]schema.PropertySpec{"active": {TypeSpec: boolT}}},
	}

	types := map[string]schema.ComplexTypeSpec{}
	oneOf := make([]schema.TypeSpec, 0, len(variants))
	mapping := map[string]string{}
	for _, v := range variants {
		props := map[string]schema.PropertySpec{
			"type__":  {TypeSpec: stringT, Const: v.token},
			"payload": {TypeSpec: stringT},
		}
		maps.Copy(props, v.extra)
		types[fmt.Sprintf("%s:index:%s", p.pkg(), v.token)] = schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: props,
				Required:   []string{"type__"},
			},
		}
		ref := fmt.Sprintf("#/types/%s:index:%s", p.pkg(), v.token)
		oneOf = append(oneOf, schema.TypeSpec{Ref: ref})
		mapping[v.token] = ref
	}

	union := schema.TypeSpec{
		OneOf: oneOf,
		Discriminator: &schema.DiscriminatorSpec{
			PropertyName: "type__",
			Mapping:      mapping,
		},
	}

	resourceProperties := map[string]schema.PropertySpec{
		"unionOf":     {TypeSpec: union},
		"secretUnion": {TypeSpec: union, Secret: true},
	}

	pkg := schema.PackageSpec{
		Name:    string(p.pkg()),
		Version: p.version().String(),
		Types:   types,
		Resources: map[string]schema.ResourceSpec{
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

func (p *DiscriminatedUnionInternalProvider) Close() error {
	return nil
}

func (p *DiscriminatedUnionInternalProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DiscriminatedUnionInternalProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionInternalProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ptr(p.version()),
	}, nil
}

func (p *DiscriminatedUnionInternalProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *DiscriminatedUnionInternalProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DiscriminatedUnionInternalProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DiscriminatedUnionInternalProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DiscriminatedUnionInternalProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != tokens.Type(fmt.Sprintf("%s:index:Example", p.pkg())) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *DiscriminatedUnionInternalProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() == tokens.Type(fmt.Sprintf("%s:index:Example", p.pkg())) {
		return plugin.CreateResponse{
			ID:         resource.ID("new-resource-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
	return plugin.CreateResponse{Status: resource.StatusUnknown},
		fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
