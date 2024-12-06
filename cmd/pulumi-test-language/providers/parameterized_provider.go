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
	"errors"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type ParameterizedProvider struct {
	plugin.UnimplementedProvider
	parameterValue   []byte
	parameterVersion string
	parameterPackage string
	config           string
}

var _ plugin.Provider = (*ParameterizedProvider)(nil)

func (p *ParameterizedProvider) Close() error {
	return nil
}

func (p *ParameterizedProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ParameterizedProvider) Pkg() tokens.Package {
	return "parameterized"
}

func (p *ParameterizedProvider) GetSchema(
	_ context.Context, req plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	if p.parameterPackage == "" {
		return plugin.GetSchemaResponse{}, errors.New("expected a parameter package name for a parameterized provider")
	}

	if p.parameterVersion == "" {
		return plugin.GetSchemaResponse{}, errors.New("expected a parameter version for a parameterized provider")
	}

	subpackage := p.parameterPackage
	version := p.parameterVersion

	// the name of the resource is the parameterized value
	parameterizedResource := string(p.parameterValue)
	if parameterizedResource == "" {
		return plugin.GetSchemaResponse{}, errors.New("expected parameter value to be non-empty")
	}

	token := fmt.Sprintf("%s:index:%s", subpackage, parameterizedResource)

	pkg := schema.PackageSpec{
		Name:    subpackage,
		Version: version,
		Provider: schema.ResourceSpec{
			InputProperties: map[string]schema.PropertySpec{
				"text": {
					TypeSpec: schema.TypeSpec{
						Type: "string",
					},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			token: {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"parameterValue": {
							TypeSpec: schema.TypeSpec{
								Type: "string",
							},
						},
					},
					Required: []string{"parameterValue"},
				},
			},
		},
		Parameterization: &schema.ParameterizationSpec{
			BaseProvider: schema.BaseProviderSpec{
				Name:    "parameterized",
				Version: "1.2.3",
			},
			Parameter: p.parameterValue,
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ParameterizedProvider) Parameterize(
	_ context.Context, req plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	param, ok := req.Parameters.(*plugin.ParameterizeValue)
	if !ok {
		err := fmt.Errorf("expected a ParameterizeValue when parameterizing, instead got %T", req.Parameters)
		return plugin.ParameterizeResponse{}, err
	}

	if param.Value == nil {
		return plugin.ParameterizeResponse{}, errors.New("expected a non-nil value when parameterizing")
	}

	if param.Name == "" {
		return plugin.ParameterizeResponse{}, errors.New("expected a non-empty name when parameterizing")
	}

	p.parameterPackage = param.Name
	p.parameterVersion = param.Version.String()
	p.parameterValue = param.Value

	return plugin.ParameterizeResponse{
		Name:    param.Name,
		Version: param.Version,
	}, nil
}

func (p *ParameterizedProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect the version
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

	// assert that the version is the parameterized version
	if version.StringValue() != p.parameterVersion {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version in CheckConfig is not the parameterized version"),
		}, nil
	}

	// Optionally expect the text config
	text, ok := req.News["text"]
	if ok {
		if !text.IsString() {
			return plugin.CheckConfigResponse{
				Failures: makeCheckFailure("text", "text is not a string"),
			}, nil
		}
		p.config = text.StringValue()
	}

	if len(req.News) > 2 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ParameterizedProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "{sub-package}:index:{parameterized-resource}"
	expectedToken := fmt.Sprintf("%s:index:%s", p.parameterPackage, string(p.parameterValue))
	if string(req.URN.Type()) != expectedToken {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("",
				fmt.Sprintf("invalid URN type: %s. Expected %s", req.URN.Type(), expectedToken)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ParameterizedProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "{sub-package}:index:{parameterized-resource}"
	expectedToken := fmt.Sprintf("%s:index:%s", p.parameterPackage, string(p.parameterValue))
	if string(req.URN.Type()) != expectedToken {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s. Expected %s", req.URN.Type(), expectedToken)
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	var configText string
	if p.config != "" {
		configText = " " + p.config
	}

	// parameterized resource outputs will include the parameter value that the provider was parameterized with
	outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"parameterValue": string(p.parameterValue) + configText,
	})

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: outputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ParameterizedProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("1.2.3")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ParameterizedProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ParameterizedProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ParameterizedProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ParameterizedProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ParameterizedProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ParameterizedProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
