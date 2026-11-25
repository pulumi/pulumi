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
)

// ModuleFormatEnumProvider is a provider whose enum types are in modules that need to be parsed with the module
// format, so that programs referencing them must resolve each enum's module through the schema's moduleFormat.
type ModuleFormatEnumProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ModuleFormatEnumProvider)(nil)

const moduleFormatEnumResourceType = "moduleformatenum:mod_Res:Res"

func (p *ModuleFormatEnumProvider) Close() error {
	return nil
}

func (p *ModuleFormatEnumProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ModuleFormatEnumProvider) version() semver.Version {
	return semver.Version{Major: 64}
}

func (p *ModuleFormatEnumProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := p.version()
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ModuleFormatEnumProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	const (
		colorToken = "moduleformatenum:mod_Color:Color"
		shadeToken = "moduleformatenum:index_Shade:Shade"
	)

	properties := map[string]schema.PropertySpec{
		"color": {TypeSpec: schema.TypeSpec{Ref: "#/types/" + colorToken}},
		"shade": {TypeSpec: schema.TypeSpec{Ref: "#/types/" + shadeToken}},
	}

	pkg := schema.PackageSpec{
		Name:    "moduleformatenum",
		Version: p.version().String(),
		Meta: &schema.MetadataSpec{
			// Like the module-format provider, this isn't the module format that the Terraform bridge uses, so
			// codegen has to apply the regex rather than split off a trailing /xyz.
			ModuleFormat: "(.*)(?:_[^_]*)",
		},
		Types: map[string]schema.ComplexTypeSpec{
			// An enum in the "mod" module.
			colorToken: {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "string"},
				Enum: []schema.EnumValueSpec{
					{Name: "Red", Value: "red"},
					{Name: "Blue", Value: "blue"},
				},
			},
			// An enum in the "index" module.
			shadeToken: {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "string"},
				Enum: []schema.EnumValueSpec{
					{Name: "Light", Value: "light"},
					{Name: "Dark", Value: "dark"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			moduleFormatEnumResourceType: {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: properties,
				},
				InputProperties: properties,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ModuleFormatEnumProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ModuleFormatEnumProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type().String() != moduleFormatEnumResourceType {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *ModuleFormatEnumProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type().String() != moduleFormatEnumResourceType {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
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
