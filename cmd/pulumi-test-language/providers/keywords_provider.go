// Copyright 2025, Pulumi Corporation.
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

// KeywordsProvider is used to test names that are keywords or standard library built-in names in certain languages,
// such as "builtins" and "property" in Python, ensuring such names can be used correctly across sdk-gen and
// program-gen.
type KeywordsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*KeywordsProvider)(nil)

func (p *KeywordsProvider) version() string {
	return "20.0.0"
}

func (p *KeywordsProvider) properties() []string {
	return []string{
		"builtins",
		"property",
	}
}

func (p *KeywordsProvider) Close() error {
	return nil
}

func (p *KeywordsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *KeywordsProvider) Pkg() tokens.Package {
	return "keywords"
}

func (p *KeywordsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	properties := make(map[string]schema.PropertySpec)
	for _, prop := range p.properties() {
		properties[prop] = schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		}
	}

	pkg := schema.PackageSpec{
		Name:    p.Pkg().String(),
		Version: p.version(),
		Resources: map[string]schema.ResourceSpec{
			"keywords:index:SomeResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: properties,
					Required:   p.properties(),
				},
				InputProperties: properties,
				RequiredInputs:  p.properties(),
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *KeywordsProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
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
	if version.StringValue() != p.version() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not "+p.version()),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *KeywordsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "keywords:index:SomeResource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	for _, prop := range p.properties() {
		propKey := resource.PropertyKey(prop)
		value, ok := req.News[propKey]
		if !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure(propKey, fmt.Sprintf("missing %s", propKey)),
			}, nil
		}
		if !value.IsString() && !value.IsComputed() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure(propKey, fmt.Sprintf("%s is not a string", propKey)),
			}, nil
		}
	}
	if len(req.News) != len(p.properties()) {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *KeywordsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "keywords:index:SomeResource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	properties := make(resource.PropertyMap)
	for _, prop := range p.properties() {
		propKey := resource.PropertyKey(prop)
		value, ok := req.Properties[propKey]
		if !ok {
			return plugin.CreateResponse{}, fmt.Errorf("missing property %s", propKey)
		}
		properties[propKey] = value
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *KeywordsProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	if req.URN.Type() != "keywords:index:SomeResource" {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *KeywordsProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse(p.version())
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *KeywordsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *KeywordsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *KeywordsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *KeywordsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *KeywordsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *KeywordsProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *KeywordsProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	if req.URN.Type() != "keywords:index:SomeResource" {
		return plugin.ReadResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	properties := make(resource.PropertyMap)
	for _, prop := range p.properties() {
		properties[resource.PropertyKey(prop)] = resource.NewProperty(prop)
	}

	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  properties,
			Outputs: properties,
		},
		Status: resource.StatusOK,
	}, nil
}
