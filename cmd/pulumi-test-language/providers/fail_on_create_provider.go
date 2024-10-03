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
	"errors"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type FailOnCreateProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*FailOnCreateProvider)(nil)

func (p *FailOnCreateProvider) Close() error {
	return nil
}

func (p *FailOnCreateProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *FailOnCreateProvider) Pkg() tokens.Package {
	return "fail_on_create"
}

func (p *FailOnCreateProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "fail_on_create",
		Version: "4.0.0",
		Resources: map[string]schema.ResourceSpec{
			"fail_on_create:index:Resource": {
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

func (p *FailOnCreateProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "missing version")}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not a string")}, nil
	}
	if version.StringValue() != "4.0.0" {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not 4.0.0")}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *FailOnCreateProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "fail_on_create:index:Resource"
	if req.URN.Type() != "fail_on_create:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect just the boolean value
	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "missing value"),
		}, nil
	}
	if !value.IsBool() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "value is not a boolean"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *FailOnCreateProvider) Create(
	context.Context, plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{}, errors.New("failed create")
}

func (p *FailOnCreateProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("4.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *FailOnCreateProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *FailOnCreateProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *FailOnCreateProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *FailOnCreateProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *FailOnCreateProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *FailOnCreateProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
