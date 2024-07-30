// Copyright 2016-2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Config provider is a small provider to test things related to provider configuration and explicit provider resources.
// It has one small resource that takes a text input and returns it with a prefix based on provider configuration.
type ConfigProvider struct {
	plugin.UnimplementedProvider

	prefix string
}

var _ plugin.Provider = (*ConfigProvider)(nil)

func (p *ConfigProvider) Close() error {
	return nil
}

func (p *ConfigProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ConfigProvider) Pkg() tokens.Package {
	return "config"
}

func (p *ConfigProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"text": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	resourceRequired := []string{"text"}

	providerProperties := map[string]schema.PropertySpec{
		"name": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
		"pluginDownloadURL": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}

	pkg := schema.PackageSpec{
		Name:    "config",
		Version: "9.0.0",
		Config: schema.ConfigSpec{
			Variables: providerProperties,
			Required:  []string{"name"},
		},
		PluginDownloadURL: "http://example.com",
		Provider: schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type: "object",
				Properties: map[string]schema.PropertySpec{
					"name":              providerProperties["name"],
					"pluginDownloadURL": providerProperties["pluginDownloadURL"],
					"version": {
						TypeSpec: schema.TypeSpec{
							Type: "string",
						},
					},
				},
				Required: []string{"name", "version"},
			},
			InputProperties: providerProperties,
			RequiredInputs:  []string{"name"},
		},
		Resources: map[string]schema.ResourceSpec{
			"config:index:Resource": {
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

func (p *ConfigProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// We should have the version but also name and pluginDownloadURL

	check := func(required bool, key resource.PropertyKey, expected string) *plugin.CheckConfigResponse {
		value, ok := req.News[key]
		if !ok {
			if required {
				return &plugin.CheckConfigResponse{
					Failures: makeCheckFailure(key, fmt.Sprintf("missing %s", key)),
				}
			}
			return nil
		}
		if !value.IsString() {
			return &plugin.CheckConfigResponse{
				Failures: makeCheckFailure(key, fmt.Sprintf("%s is not a string", key)),
			}
		}
		if expected != "" && value.StringValue() != expected {
			return &plugin.CheckConfigResponse{
				Failures: makeCheckFailure(key, fmt.Sprintf("%s is not %s", key, expected)),
			}
		}
		return nil
	}

	ok := check(true, "version", "9.0.0")
	if ok != nil {
		return *ok, nil
	}

	ok = check(true, "name", "")
	if ok != nil {
		return *ok, nil
	}

	ok = check(false, "pluginDownloadURL", "")
	if ok != nil {
		return *ok, nil
	}

	if len(req.News) > 3 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ConfigProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "config:index:Resource"
	if req.URN.Type() != "config:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect just the text string value
	value, ok := req.News["text"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("text", "missing text"),
		}, nil
	}
	if !value.IsString() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("text", "text is not a string"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ConfigProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "config:index:Resource"
	if req.URN.Type() != "config:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	// Check should have already checked this, good practice would be to check again but for tests we can just panic
	// here.
	text := req.Properties["text"].StringValue()

	props := resource.PropertyMap{
		"text": resource.NewStringProperty(p.prefix + ": " + text),
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: props,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ConfigProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("9.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ConfigProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ConfigProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ConfigProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ConfigProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConfigProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConfigProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
