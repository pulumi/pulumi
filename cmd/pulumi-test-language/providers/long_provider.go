// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 8.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-8.0
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
	"math/big"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LongProvider is a small provider with a single resource that takes a int64 value aka "long".
type LongProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*LongProvider)(nil)

func (p *LongProvider) Close() error {
	return nil
}

func (p *LongProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *LongProvider) Pkg() tokens.Package {
	return "long"
}

func (p *LongProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "bigInteger",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "long",
		Version: "8.0.0",
		Resources: map[string]schema.ResourceSpec{
			"long:index:Resource": {
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

func (p *LongProvider) CheckConfig(
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
	if version.StringValue() != "8.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 8.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *LongProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "long:index:Resource"
	if req.URN.Type() != "long:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	// Expect just the integer value
	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "missing value"),
		}, nil
	}
	if !value.IsInteger() && !value.IsNumber() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("value", "value is not an integer"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *LongProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "long:index:Resource"
	if req.URN.Type() != "long:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	// If the input is a number, we'll convert it to an integer.
	value := req.Properties["value"]
	if value.IsNumber() {
		bf := big.NewFloat(value.NumberValue())
		bi, _ := bf.Int(nil)
		value = resource.NewIntegerProperty(bi)
	}
	req.Properties["value"] = value

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *LongProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	// URN should be of the form "long:index:Resource"
	if req.URN.Type() != "long:index:Resource" {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	// If the input is a number, we'll convert it to an integer.
	value := req.NewInputs["value"]
	if value.IsNumber() {
		bf := big.NewFloat(value.NumberValue())
		bi, _ := bf.Int(nil)
		value = resource.NewIntegerProperty(bi)
	}
	req.NewInputs["value"] = value

	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *LongProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("8.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *LongProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *LongProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *LongProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *LongProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *LongProvider) Diff(
	_ context.Context, req plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	// Check if the value has changed.
	oldValue := req.OldOutputs["value"]
	newValue := req.NewInputs["value"]

	// If the old output is a number report a diff so that we update to an integer
	if oldValue.IsNumber() {
		return plugin.DiffResponse{
			Changes:     plugin.DiffSome,
			ChangedKeys: []resource.PropertyKey{"value"},
		}, nil
	}

	// Or if the value has changed in any way.
	if oldValue.DeepEquals(newValue) {
		return plugin.DiffResponse{
			Changes: plugin.DiffNone,
		}, nil
	}

	return plugin.DiffResponse{
		Changes:     plugin.DiffSome,
		ChangedKeys: []resource.PropertyKey{"value"},
	}, nil
}

func (p *LongProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
