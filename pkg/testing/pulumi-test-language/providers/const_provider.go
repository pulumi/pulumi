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

// ConstProvider is a minimal provider with a single resource whose only property has a constant
// value in the schema. It backs tests for reading constant-valued properties.
type ConstProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ConstProvider)(nil)

func (p *ConstProvider) Close() error {
	return nil
}

func (p *ConstProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ConstProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("43.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ConstProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	properties := map[string]schema.PropertySpec{
		"kind": {
			Const:    "Constant",
			TypeSpec: schema.TypeSpec{Type: "string"},
		},
	}

	pkg := schema.PackageSpec{
		Name:    "constant",
		Version: "43.0.0",
		Resources: map[string]schema.ResourceSpec{
			"constant:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: properties,
				},
				InputProperties: properties,
				RequiredInputs:  []string{"kind"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ConstProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ConstProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "constant:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	kind, ok := req.News["kind"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("kind", "missing value"),
		}, nil
	}
	if !kind.IsString() || kind.StringValue() != "Constant" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("kind", fmt.Sprintf("value is not the constant \"Constant\": %v", kind)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ConstProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "constant:index:Resource" {
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

func (p *ConstProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConstProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
