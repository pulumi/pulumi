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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ArrayOrAnyProvider exposes resources whose input property is typed as a union of an array and the
// catch-all `pulumi.json#/Any` type. This mirrors the shape produced by schemas like aws-native's
// DynamoDB Table.keySchema (see https://github.com/pulumi/pulumi/issues/17181) and exercises the
// language SDKs' handling of `Union[Sequence[T], Any]` when the caller passes a plain list.
type ArrayOrAnyProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ArrayOrAnyProvider)(nil)

func (p *ArrayOrAnyProvider) Close() error { return nil }

func (p *ArrayOrAnyProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ArrayOrAnyProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("64.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *ArrayOrAnyProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	stringArrayOrAny := schema.TypeSpec{
		OneOf: []schema.TypeSpec{
			{Type: "array", Items: &schema.TypeSpec{Type: "string"}},
			{Ref: "pulumi.json#/Any"},
		},
	}
	itemArrayOrAny := schema.TypeSpec{
		OneOf: []schema.TypeSpec{
			{Type: "array", Items: &schema.TypeSpec{Ref: "#/types/array-or-any:index:Item"}},
			{Ref: "pulumi.json#/Any"},
		},
	}

	stringResourceProps := map[string]schema.PropertySpec{
		"values": {TypeSpec: stringArrayOrAny},
	}
	objectResourceProps := map[string]schema.PropertySpec{
		"values": {TypeSpec: itemArrayOrAny},
	}
	required := []string{"values"}

	pkg := schema.PackageSpec{
		Name:    "array-or-any",
		Version: "64.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"array-or-any:index:Item": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"itemName": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"itemName"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"array-or-any:index:StringList": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: stringResourceProps,
					Required:   required,
				},
				InputProperties: stringResourceProps,
				RequiredInputs:  required,
			},
			"array-or-any:index:ObjectList": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: objectResourceProps,
					Required:   required,
				},
				InputProperties: objectResourceProps,
				RequiredInputs:  required,
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ArrayOrAnyProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ArrayOrAnyProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	news := resource.ToResourcePropertyMap(req.NewInputs)
	urnType := string(req.URN.Type())
	if urnType != "array-or-any:index:StringList" && urnType != "array-or-any:index:ObjectList" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	if _, ok := news["values"]; !ok {
		return plugin.CheckResponse{Failures: makeCheckFailure("values", "missing value")}, nil
	}
	return plugin.CheckResponse{Properties: resource.FromResourcePropertyMap(news)}, nil
}

func (p *ArrayOrAnyProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	properties := resource.ToResourcePropertyMap(req.Properties)
	urnType := string(req.URN.Type())
	if urnType != "array-or-any:index:StringList" && urnType != "array-or-any:index:ObjectList" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	id := "id"
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: resource.FromResourcePropertyMap(properties),
		Status:     resource.StatusOK,
	}, nil
}

func (p *ArrayOrAnyProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ArrayOrAnyProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
