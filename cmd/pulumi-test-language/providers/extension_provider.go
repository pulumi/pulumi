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
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const extensionProviderBaseName = "extension"

const extensionProviderBaseVersion = "17.17.17"

type extensionProvider struct {
	plugin.UnimplementedProvider

	replacement *plugin.ParameterizeValue
	extensions  map[string]*plugin.ParameterizeValue
}

var _ plugin.Provider = (*extensionProvider)(nil)

func NewExtensionProvider() plugin.Provider {
	return &extensionProvider{
		extensions: map[string]*plugin.ParameterizeValue{},
	}
}

func (p *extensionProvider) Close() error {
	return nil
}

func (p *extensionProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *extensionProvider) Pkg() tokens.Package {
	return tokens.Package(extensionProviderBaseName)
}

func (p *extensionProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	version := semver.MustParse(extensionProviderBaseVersion)
	info := workspace.PluginInfo{Version: &version}
	return info, nil
}

func (p *extensionProvider) Parameterize(
	_ context.Context,
	req plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	param, ok := req.Parameters.(*plugin.ParameterizeValue)
	if !ok {
		return plugin.ParameterizeResponse{}, fmt.Errorf("expected a ParameterizeValue, got %T", req.Parameters)
	}

	if param.Name == extensionProviderBaseName {
		return plugin.ParameterizeResponse{}, fmt.Errorf(
			"parameterization requested using the same package name as the base plugin: %s",
			param.Name,
		)
	}

	if req.Extension {
		if p.replacement != nil && p.replacement.Name == param.Name {
			return plugin.ParameterizeResponse{}, fmt.Errorf(
				"extension parameterization requested using the same package name as the base replacement: %s",
				param.Name,
			)
		}

		existing, exists := p.extensions[param.Name]
		if exists && !existing.Version.EQ(param.Version) {
			return plugin.ParameterizeResponse{}, fmt.Errorf(
				"extension parameterization requested for the same package name at different versions: have %s, want %s",
				existing.Version, param.Version,
			)
		}

		p.extensions[param.Name] = param
	} else {
		if p.replacement != nil {
			return plugin.ParameterizeResponse{}, errors.New("expected to be replacement parameterized only once")
		}

		p.replacement = param
	}

	return plugin.ParameterizeResponse{Name: param.Name, Version: param.Version}, nil
}

func (p *extensionProvider) GetSchema(
	_ context.Context,
	req plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg, err := p.getSchemaPackageSpec(req)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	res := plugin.GetSchemaResponse{Schema: jsonBytes}
	return res, nil
}

func (p *extensionProvider) getSchemaPackageSpec(req plugin.GetSchemaRequest) (schema.PackageSpec, error) {
	primitiveType := func(t string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type: t,
			},
		}
	}

	resource := func(isComponent bool) func(
		description string,
		inputs map[string]schema.PropertySpec,
		outputs map[string]schema.PropertySpec,
	) schema.ResourceSpec {
		return func(
			description string,
			inputs map[string]schema.PropertySpec,
			outputs map[string]schema.PropertySpec,
		) schema.ResourceSpec {
			requiredInputs := slices.Sorted(maps.Keys(inputs))
			requiredOutputs := slices.Sorted(maps.Keys(outputs))

			return schema.ResourceSpec{
				IsComponent: isComponent,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: description,
					Type:        "object",
					Properties:  outputs,
					Required:    requiredOutputs,
				},
				InputProperties: inputs,
				RequiredInputs:  requiredInputs,
			}
		}
	}

	customResource := resource(false)

	if req.SubpackageName == "" {
		pkg := schema.PackageSpec{
			Name: extensionProviderBaseName,
			Provider: customResource(
				fmt.Sprintf("The `%s` package's provider resource", extensionProviderBaseName),
				map[string]schema.PropertySpec{
					"value": primitiveType("string"),
				},
				map[string]schema.PropertySpec{
					"value": primitiveType("string"),
				},
			),
			Resources: map[string]schema.ResourceSpec{
				extensionProviderBaseName + ":index:Custom": customResource(
					fmt.Sprintf("A custom resource in the `%s` package", extensionProviderBaseName),
					map[string]schema.PropertySpec{
						"value": primitiveType("string"),
					},
					map[string]schema.PropertySpec{
						"value": primitiveType("string"),
					},
				),
			},
		}

		return pkg, nil
	}

	if req.SubpackageVersion == nil {
		return schema.PackageSpec{}, errors.New("expected subpackage to have a version, but got nil")
	}

	if p.replacement != nil && req.SubpackageName == p.replacement.Name {
		if !req.SubpackageVersion.EQ(p.replacement.Version) {
			return schema.PackageSpec{}, fmt.Errorf(
				"expected replacement parameterization to be the same version as the request: have %s, want %s",
				p.replacement.Version, req.SubpackageVersion,
			)
		}

		pkg := schema.PackageSpec{
			Name:    req.SubpackageName,
			Version: req.SubpackageVersion.String(),
			Provider: customResource(
				fmt.Sprintf("The `%s` package's provider resource", req.SubpackageName),
				map[string]schema.PropertySpec{
					"value": primitiveType("string"),
				},
				map[string]schema.PropertySpec{
					"value": primitiveType("string"),
				},
			),
			Resources: map[string]schema.ResourceSpec{
				req.SubpackageName + ":index:Custom": customResource(
					fmt.Sprintf("A custom resource in the `%s` package", req.SubpackageName),
					map[string]schema.PropertySpec{
						"value": primitiveType("string"),
					},
					map[string]schema.PropertySpec{
						"value": primitiveType("string"),
					},
				),
			},
			Parameterization: &schema.ParameterizationSpec{
				BaseProvider: schema.BaseProviderSpec{
					Name:    extensionProviderBaseName,
					Version: extensionProviderBaseVersion,
				},
				Parameter: p.replacement.Value,
			},
		}

		return pkg, nil
	}

	if ext, has := p.extensions[req.SubpackageName]; has {
		if !req.SubpackageVersion.EQ(ext.Version) {
			return schema.PackageSpec{}, fmt.Errorf(
				"expected extension parameterization to be the same version as the request: have %s, want %s",
				ext.Version, req.SubpackageVersion,
			)
		}

		baseProviderSpec := schema.BaseProviderSpec{
			Name:    extensionProviderBaseName,
			Version: extensionProviderBaseVersion,
		}

		var replacement *schema.ParameterizationSpec
		if p.replacement != nil {
			replacement = &schema.ParameterizationSpec{
				BaseProvider: baseProviderSpec,
				Parameter:    p.replacement.Value,
			}
		}

		pkg := schema.PackageSpec{
			Name:    req.SubpackageName,
			Version: req.SubpackageVersion.String(),
			Resources: map[string]schema.ResourceSpec{
				req.SubpackageName + ":index:Custom": customResource(
					fmt.Sprintf("A custom resource in the `%s` package", req.SubpackageName),
					map[string]schema.PropertySpec{
						"value": primitiveType("string"),
					},
					map[string]schema.PropertySpec{
						"value": primitiveType("string"),
					},
				),
			},
			Parameterization: replacement,
			Extension: &schema.ParameterizationSpec{
				BaseProvider: baseProviderSpec,
				Parameter:    ext.Value,
			},
		}

		return pkg, nil
	}

	return schema.PackageSpec{}, fmt.Errorf("could not produce schema package for request %v", req)
}

func (p *extensionProvider) CheckConfig(
	_ context.Context,
	req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// TODO

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *extensionProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *extensionProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *extensionProvider) Check(
	_ context.Context,
	req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// TODO

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *extensionProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *extensionProvider) Create(
	_ context.Context,
	req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// TODO

	id := "id-" + req.URN.Name()
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}
