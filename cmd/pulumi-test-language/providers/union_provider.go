// Copyright 2016-2025, Pulumi Corporation.
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

// Stress-test union type support in the schema.

type UnionProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*UnionProvider)(nil)

func (UnionProvider) version() string {
	return "18.0.0"
}

func (p *UnionProvider) Pkg() tokens.Package {
	return "union"
}

func (p *UnionProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	oneOf := func(variants ...schema.TypeSpec) schema.TypeSpec {
		return schema.TypeSpec{
			OneOf: variants,
		}
	}

	stringType := schema.TypeSpec{Type: "string"}
	integerType := schema.TypeSpec{Type: "integer"}

	arrayOf := func(elementType schema.TypeSpec) schema.TypeSpec {
		return schema.TypeSpec{
			Type:  "array",
			Items: &elementType,
		}
	}

	mapOf := func(elementType schema.TypeSpec) schema.TypeSpec {
		return schema.TypeSpec{
			Type:                 "object",
			AdditionalProperties: &elementType,
		}
	}

	mapMapUnionType := mapOf(mapOf(oneOf(stringType, arrayOf(stringType))))

	resourceProperties := map[string]schema.PropertySpec{
		"stringOrIntegerProperty": {
			TypeSpec: oneOf(stringType, integerType),
		},
		"mapMapUnionProperty": {
			TypeSpec: mapMapUnionType,
		},
	}

	pkg := schema.PackageSpec{
		Name:    string(p.Pkg()),
		Version: p.version(),
		Resources: map[string]schema.ResourceSpec{
			fmt.Sprintf("%s:index:Example", p.Pkg()): {
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

func (p *UnionProvider) Close() error {
	return nil
}

func (p *UnionProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *UnionProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *UnionProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse(p.version())
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *UnionProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *UnionProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *UnionProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *UnionProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *UnionProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if string(req.URN.Type()) != fmt.Sprintf("%s:index:Example", p.Pkg()) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *UnionProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if string(req.URN.Type()) == fmt.Sprintf("%s:index:Example", p.Pkg()) {
		return plugin.CreateResponse{
			ID:         resource.ID("new-resource-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
	return plugin.CreateResponse{Status: resource.StatusUnknown},
		fmt.Errorf("invalid URN type: %s", req.URN.Type())
}
