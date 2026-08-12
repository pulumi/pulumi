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

// ConfigEnumProvider exposes a string and a string-typed enumeration as provider
// configuration, and a single resource that re-emits those values. The provider's
// own output properties mirror the configuration so that downstream resources can
// reference them via PCL property access (e.g. `prov.aEnum`).
type ConfigEnumProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ConfigEnumProvider)(nil)

func (*ConfigEnumProvider) version() semver.Version {
	return semver.Version{Major: 41}
}

func (p *ConfigEnumProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	enumToken := "config-enum:index:MyEnum" //nolint:gosec // not a credential

	configProperties := map[string]schema.PropertySpec{
		"aString": {TypeSpec: schema.TypeSpec{Type: "string"}},
		"aEnum":   {TypeSpec: schema.TypeSpec{Ref: "#/types/" + enumToken}},
	}
	configRequired := []string{"aString", "aEnum"}

	resourceProperties := map[string]schema.PropertySpec{
		"theString": {TypeSpec: schema.TypeSpec{Type: "string"}},
		"theEnum":   {TypeSpec: schema.TypeSpec{Ref: "#/types/" + enumToken}},
	}
	resourceRequired := []string{"theString", "theEnum"}

	pkg := schema.PackageSpec{
		Name:    "config-enum",
		Version: p.version().String(),
		Config: schema.ConfigSpec{
			Variables: configProperties,
			Required:  configRequired,
		},
		Provider: &schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: configProperties,
				Required:   configRequired,
			},
			InputProperties: configProperties,
			RequiredInputs:  configRequired,
		},
		Types: map[string]schema.ComplexTypeSpec{
			enumToken: {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "string"},
				Enum: []schema.EnumValueSpec{
					{Name: "One", Value: "one"},
					{Name: "Two", Value: "two"},
					{Name: "Three", Value: "three"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"config-enum:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ConfigEnumProvider) Close() error { return nil }

func (p *ConfigEnumProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ConfigEnumProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ConfigEnumProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "config-enum:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ConfigEnumProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "config-enum:index:Resource" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	return plugin.CreateResponse{
		ID:         "id",
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ConfigEnumProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{Version: ptr(p.version())}, nil
}

func (p *ConfigEnumProvider) SignalCancellation(context.Context) error { return nil }

func (p *ConfigEnumProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ConfigEnumProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ConfigEnumProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConfigEnumProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConfigEnumProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
