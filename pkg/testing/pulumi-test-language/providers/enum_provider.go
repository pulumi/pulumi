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

// Stress-test enum type support in the schema.

type EnumProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*EnumProvider)(nil)

func (p *EnumProvider) Pkg() tokens.Package {
	return "enum"
}

func (*EnumProvider) version() semver.Version {
	return semver.Version{Major: 30}
}

func (p *EnumProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:      p.Pkg().String(),
		Version:   p.version().String(),
		Resources: map[string]schema.ResourceSpec{},
		Types:     map[string]schema.ComplexTypeSpec{},
	}

	add := func(mod string) {
		intEnumToken := pkg.Name + ":" + mod + ":IntEnum"
		pkg.Types[intEnumToken] = schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{Type: "integer"},
			Enum: []schema.EnumValueSpec{
				{Name: "IntOne", Value: 1},
				{Name: "IntTwo", Value: 2},
			},
		}

		stringEnumToken := pkg.Name + ":" + mod + ":StringEnum"
		pkg.Types[stringEnumToken] = schema.ComplexTypeSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{Type: "string"},
			Enum: []schema.EnumValueSpec{
				{Name: "StringOne", Value: "one"},
				{Name: "StringTwo", Value: "two"},
			},
		}

		resToken := pkg.Name + ":" + mod + ":Res"
		props := map[string]schema.PropertySpec{
			"intEnum":    {TypeSpec: schema.TypeSpec{Ref: "#/types/" + intEnumToken}},
			"stringEnum": {TypeSpec: schema.TypeSpec{Ref: "#/types/" + stringEnumToken}},
		}
		pkg.Resources[resToken] = schema.ResourceSpec{
			ObjectTypeSpec:  schema.ObjectTypeSpec{Properties: props},
			InputProperties: props,
		}
	}

	add("index")
	add("mod")
	add("mod/nested")

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *EnumProvider) Close() error {
	return nil
}

func (p *EnumProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *EnumProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *EnumProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ref(p.version()),
	}, nil
}

func (p *EnumProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *EnumProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *EnumProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *EnumProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *EnumProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch req.URN.Type().String() {
	case fmt.Sprintf("%s:index:Res", p.Pkg()),
		fmt.Sprintf("%s:mod:Res", p.Pkg()),
		fmt.Sprintf("%s:mod/nested:Res", p.Pkg()):
		return plugin.CheckResponse{Properties: req.News}, nil
	default:
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
}

func (p *EnumProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	switch req.URN.Type().String() {
	case fmt.Sprintf("%s:index:Res", p.Pkg()),
		fmt.Sprintf("%s:mod:Res", p.Pkg()),
		fmt.Sprintf("%s:mod/nested:Res", p.Pkg()):
		return plugin.CreateResponse{
			ID:         resource.ID("new-resource-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	default:
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
}
