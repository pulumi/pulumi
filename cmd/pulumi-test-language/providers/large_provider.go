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
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LargeProvider is a test provider that exercises the provider protocol by returning really large strings, lists, and
// maps.
type LargeProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*LargeProvider)(nil)

func (p *LargeProvider) Close() error {
	return nil
}

func (p *LargeProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *LargeProvider) Pkg() tokens.Package {
	return "large"
}

func (p *LargeProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "large",
		Version: "4.3.2",
		Resources: map[string]schema.ResourceSpec{
			"large:index:String": {
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

func (p *LargeProvider) CheckConfig(
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
	if version.StringValue() != "4.3.2" {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not 4.3.2")}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *LargeProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "large:index:String" {
		return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type()))}, nil
	}

	// Expect just the boolean value
	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{Failures: makeCheckFailure("value", "missing value")}, nil
	}
	if !value.IsString() {
		return plugin.CheckResponse{Failures: makeCheckFailure("value", "value is not a string")}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News))}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *LargeProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "large:index:String" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	// Take the input value and _massively_ expand it.
	value, ok := req.Properties["value"]
	if !ok {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, errors.New("missing value")
	}
	if !value.IsString() {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, errors.New("value is not a string")
	}

	// aim for 100mb of data (400mb is the size limit we normally set, but nodejs is far more limited)
	repeat := (100 * 1024 * 1024) / len(value.StringValue())
	result := resource.PropertyMap{
		"value": resource.NewStringProperty(
			strings.Repeat(value.StringValue(), repeat)),
	}
	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: result,
		Status:     resource.StatusOK,
	}, nil
}

func (p *LargeProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("4.3.2")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *LargeProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *LargeProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *LargeProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *LargeProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *LargeProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *LargeProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
