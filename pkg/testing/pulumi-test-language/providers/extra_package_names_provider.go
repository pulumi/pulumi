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

// A provider whose schema declares a resource, function, enum type, and
// object type whose tokens all name a foreign package ("other"), declared via
// allowedPackageNames.
type ExtraPackageNamesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ExtraPackageNamesProvider)(nil)

func (p *ExtraPackageNamesProvider) pkg() tokens.Package {
	return "extra-package-names"
}

func (*ExtraPackageNamesProvider) version() semver.Version {
	return semver.Version{Major: 47}
}

func (p *ExtraPackageNamesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	choiceToken := "other:mod:Choice" //nolint:gosec // not a credential
	objToken := "other:mod:Obj"       //nolint:gosec // not a credential

	pkg := schema.PackageSpec{
		Name:                p.pkg().String(),
		Version:             p.version().String(),
		AllowedPackageNames: []string{"other"},
		Types: map[string]schema.ComplexTypeSpec{
			choiceToken: {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "string"},
				Enum: []schema.EnumValueSpec{
					{Name: "First", Value: "first"},
					{Name: "Second", Value: "second"},
				},
			},
			objToken: {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"label":  {TypeSpec: schema.TypeSpec{Type: "string"}},
						"choice": {TypeSpec: schema.TypeSpec{Ref: "#/types/" + choiceToken}},
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"other:mod:Res": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"choice": {TypeSpec: schema.TypeSpec{Ref: "#/types/" + choiceToken}},
						"obj":    {TypeSpec: schema.TypeSpec{Ref: "#/types/" + objToken}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"choice": {TypeSpec: schema.TypeSpec{Ref: "#/types/" + choiceToken}},
					"obj":    {TypeSpec: schema.TypeSpec{Ref: "#/types/" + objToken}},
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"other:mod:getThing": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"text": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"text"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
						Required: []string{"result"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ExtraPackageNamesProvider) Close() error {
	return nil
}

func (p *ExtraPackageNamesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ExtraPackageNamesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ExtraPackageNamesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ptr(p.version()),
	}, nil
}

func (p *ExtraPackageNamesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ExtraPackageNamesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ExtraPackageNamesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ExtraPackageNamesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ExtraPackageNamesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "other:mod:Res" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ExtraPackageNamesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "other:mod:Res" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	return plugin.CreateResponse{
		ID:         resource.ID("new-resource-id"),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ExtraPackageNamesProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "other:mod:getThing" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}
	text, ok := req.Args["text"]
	if !ok || !text.IsString() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("text", "missing string property text"),
		}, nil
	}
	return plugin.InvokeResponse{
		Properties: resource.PropertyMap{
			"result": resource.NewProperty("got: " + text.StringValue()),
		},
	}, nil
}
