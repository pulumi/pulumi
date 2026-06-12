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
)

// AnyHandledProvider exposes a single resource with a property of type `any`, to exercise generating
// and running code that passes an object literal to an `any`-typed input.
type AnyHandledProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*AnyHandledProvider)(nil)

func (p *AnyHandledProvider) Close() error { return nil }

func (p *AnyHandledProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *AnyHandledProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("42.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *AnyHandledProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	props := map[string]schema.PropertySpec{
		"value": {TypeSpec: schema.TypeSpec{Ref: "pulumi.json#/Any"}},
	}
	required := []string{"value"}
	pkg := schema.PackageSpec{
		Name:    "any-handled",
		Version: "42.0.0",
		Resources: map[string]schema.ResourceSpec{
			"any-handled:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: props,
					Required:   required,
				},
				InputProperties: props,
				RequiredInputs:  required,
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *AnyHandledProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *AnyHandledProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "any-handled:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	if _, ok := req.News["value"]; !ok {
		return plugin.CheckResponse{Failures: makeCheckFailure("value", "missing value")}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *AnyHandledProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "any-handled:index:Resource" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
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

func (p *AnyHandledProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *AnyHandledProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
