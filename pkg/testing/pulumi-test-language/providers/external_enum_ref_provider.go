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

type ExternalEnumRefProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ExternalEnumRefProvider)(nil)

func (p *ExternalEnumRefProvider) Pkg() tokens.Package {
	return "extenumref"
}

func (*ExternalEnumRefProvider) version() semver.Version {
	return semver.Version{Major: 32}
}

func (p *ExternalEnumRefProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	enumProvider := &EnumProvider{}
	enumVersion := enumProvider.version()
	pkg := schema.PackageSpec{
		Name:    p.Pkg().String(),
		Version: p.version().String(),
		Dependencies: []schema.PackageDescriptor{
			{Name: enumProvider.Pkg().String(), Version: &enumVersion},
		},
		Resources: map[string]schema.ResourceSpec{
			p.Pkg().String() + ":index:Sink": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Properties: map[string]schema.PropertySpec{
						"stringEnum": {TypeSpec: schema.TypeSpec{
							Ref: fmt.Sprintf("/%s/v%s/schema.json#/types/%s:index:StringEnum",
								enumProvider.Pkg(), enumProvider.version(), enumProvider.Pkg()),
						}},
					},
				},
				InputProperties: map[string]schema.PropertySpec{
					"stringEnum": {TypeSpec: schema.TypeSpec{
						Ref: fmt.Sprintf("/%s/v%s/schema.json#/types/%s:index:StringEnum",
							enumProvider.Pkg(), enumProvider.version(), enumProvider.Pkg()),
					}},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ExternalEnumRefProvider) Close() error {
	return nil
}

func (p *ExternalEnumRefProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ExternalEnumRefProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ExternalEnumRefProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ref(p.version()),
	}, nil
}

func (p *ExternalEnumRefProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ExternalEnumRefProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ExternalEnumRefProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ExternalEnumRefProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ExternalEnumRefProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	switch req.URN.Type().String() {
	case fmt.Sprintf("%s:index:Sink", p.Pkg()):
		return plugin.CheckResponse{Properties: req.News}, nil
	default:
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
}

func (p *ExternalEnumRefProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	switch req.URN.Type().String() {
	case fmt.Sprintf("%s:index:Sink", p.Pkg()):
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
