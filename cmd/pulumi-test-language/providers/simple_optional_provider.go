// Copyright 2016-2023, Pulumi Corporation.
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

const SimpleOptionalProviderVersion = "17.0.0"

type SimpleOptionalProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SimpleOptionalProvider)(nil)

func (p *SimpleOptionalProvider) Close() error {
	return nil
}

func (p *SimpleOptionalProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SimpleOptionalProvider) Pkg() tokens.Package {
	return "simple-optional"
}

func (p *SimpleOptionalProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
		"text": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}

	pkg := schema.PackageSpec{
		Name:    "simple-optional",
		Version: SimpleOptionalProviderVersion,
		Resources: map[string]schema.ResourceSpec{
			"simple-optional:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   []string{},
				},
				InputProperties: resourceProperties,
				RequiredInputs:  []string{},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SimpleOptionalProvider) CheckConfig(
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
	if version.StringValue() != SimpleOptionalProviderVersion {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not "+SimpleOptionalProviderVersion),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SimpleOptionalProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "simple-optional:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	seen := 0
	value, ok := req.News["value"]
	if ok {
		seen += 1
		if !value.IsBool() && !value.IsComputed() && !value.IsNull() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("value", "value is not a boolean"),
			}, nil
		}
	}

	text, ok := req.News["text"]
	if ok {
		seen += 1
		if !text.IsString() && !text.IsComputed() && !text.IsNull() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("text", "text is not a string"),
			}, nil
		}
	}

	if len(req.News) != seen {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *SimpleOptionalProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "simple-optional:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
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

func (p *SimpleOptionalProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse(SimpleOptionalProviderVersion)
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SimpleOptionalProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SimpleOptionalProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SimpleOptionalProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SimpleOptionalProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleOptionalProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleOptionalProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
