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

type ReplaceOnChangesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ReplaceOnChangesProvider)(nil)

func (p *ReplaceOnChangesProvider) Close() error {
	return nil
}

func (p *ReplaceOnChangesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ReplaceOnChangesProvider) Pkg() tokens.Package {
	return "replaceonchanges"
}

func (p *ReplaceOnChangesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: &semver.Version{Major: 25},
	}, nil
}

func (p *ReplaceOnChangesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceAProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
		"replaceProp": {
			ReplaceOnChanges: true,
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceARequired := []string{"value"}

	resourceBProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceBRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "replaceonchanges",
		Version: "25.0.0",
		Resources: map[string]schema.ResourceSpec{
			"replaceonchanges:index:ResourceA": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceAProperties,
					Required:   resourceARequired,
				},
				InputProperties: resourceAProperties,
				RequiredInputs:  resourceARequired,
			},
			"replaceonchanges:index:ResourceB": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceBProperties,
					Required:   resourceBRequired,
				},
				InputProperties: resourceBProperties,
				RequiredInputs:  resourceBRequired,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ReplaceOnChangesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "missing version"),
		}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not a string"),
		}, nil
	}
	if version.StringValue() != "25.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 25.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ReplaceOnChangesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	urnType := req.URN.Type()
	if urnType != "replaceonchanges:index:ResourceA" && urnType != "replaceonchanges:index:ResourceB" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", urnType)),
		}, nil
	}

	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "missing value"),
		}, nil
	}
	if !value.IsBool() && !value.IsComputed() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "value is not a boolean"),
		}, nil
	}

	if urnType == "replaceonchanges:index:ResourceA" {
		if len(req.News) < 1 || len(req.News) > 2 {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("wrong number of properties: %v", req.News)),
			}, nil
		}
		if len(req.News) == 2 {
			replaceProp, ok := req.News["replaceProp"]
			if !ok {
				return plugin.CheckResponse{
					Failures: makeCheckFailure("", fmt.Sprintf("unexpected properties: %v", req.News)),
				}, nil
			}
			if !replaceProp.IsBool() && !replaceProp.IsComputed() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure("replaceProp", "replaceProp is not a boolean"),
				}, nil
			}
		}
	} else {
		if len(req.News) != 1 {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
			}, nil
		}
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ReplaceOnChangesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	urnType := req.URN.Type()
	if urnType != "replaceonchanges:index:ResourceA" && urnType != "replaceonchanges:index:ResourceB" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", urnType)
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

func (p *ReplaceOnChangesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ReplaceOnChangesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ReplaceOnChangesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ReplaceOnChangesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReplaceOnChangesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ReplaceOnChangesProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	urnType := req.URN.Type()
	if urnType != "replaceonchanges:index:ResourceA" && urnType != "replaceonchanges:index:ResourceB" {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", urnType)
	}

	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ReplaceOnChangesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
