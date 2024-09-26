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
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A small provider with a single resource "Resource" that has a single field "Data" which is a type with
// some primitive fields.
type PrimitiveRefProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PrimitiveRefProvider)(nil)

func (p *PrimitiveRefProvider) Close() error {
	return nil
}

func (p *PrimitiveRefProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PrimitiveRefProvider) Pkg() tokens.Package {
	return "primitive-ref"
}

func (p *PrimitiveRefProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("11.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *PrimitiveRefProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	makePrimitive := func(t string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type: t,
			},
		}
	}
	typeProperties := map[string]schema.PropertySpec{
		"boolean": makePrimitive("boolean"),
		"float":   makePrimitive("number"),
		"integer": makePrimitive("integer"),
		"string":  makePrimitive("string"),
		"boolArray": {
			TypeSpec: schema.TypeSpec{
				Type:  "array",
				Items: &schema.TypeSpec{Type: "boolean"},
			},
		},
		"stringMap": {
			TypeSpec: schema.TypeSpec{
				Type:                 "object",
				AdditionalProperties: &schema.TypeSpec{Type: "string"},
			},
		},
	}
	typeRequired := []string{"boolean", "float", "integer", "string", "boolArray", "stringMap"}

	resourceProperties := map[string]schema.PropertySpec{
		"data": {
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  "#/types/primitive-ref:index:Data",
			},
		},
	}
	resourceRequired := []string{"data"}

	pkg := schema.PackageSpec{
		Name:    "primitive-ref",
		Version: "11.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"primitive-ref:index:Data": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: typeProperties,
					Required:   typeRequired,
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"primitive-ref:index:Resource": {
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

func (p *PrimitiveRefProvider) CheckConfig(
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
	if version.StringValue() != "11.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 11.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PrimitiveRefProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "primitive-ref:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	assertField := func(props resource.PropertyMap, key resource.PropertyKey, typ string,
		assertType func(resource.PropertyValue) bool,
	) *plugin.CheckResponse {
		v, ok := props[key]
		if !ok {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure(key, "missing value"),
			}
		}
		if !assertType(v) {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure(key, "value is not a "+typ),
			}
		}

		return nil
	}

	check := assertField(req.News, "data", "object", resource.PropertyValue.IsObject)
	if check != nil {
		return *check, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	data := req.News["data"].ObjectValue()

	// Expect all required properties
	check = assertField(data, "boolean", "boolean", resource.PropertyValue.IsBool)
	if check != nil {
		return *check, nil
	}
	check = assertField(data, "integer", "number", resource.PropertyValue.IsNumber)
	if check != nil {
		return *check, nil
	}
	check = assertField(data, "float", "number", resource.PropertyValue.IsNumber)
	if check != nil {
		return *check, nil
	}
	check = assertField(data, "string", "string", resource.PropertyValue.IsString)
	if check != nil {
		return *check, nil
	}
	check = assertField(data, "boolArray", "array", resource.PropertyValue.IsArray)
	if check != nil {
		return *check, nil
	}
	for _, v := range data["boolArray"].ArrayValue() {
		if !v.IsBool() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("boolArray", "array element is not a boolean"),
			}, nil
		}
	}
	check = assertField(data, "stringMap", "map", resource.PropertyValue.IsObject)
	if check != nil {
		return *check, nil
	}
	for _, v := range data["stringMap"].ObjectValue() {
		if !v.IsString() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("stringMap", "map value is not a string"),
			}, nil
		}
	}

	if len(data) != 6 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", data)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *PrimitiveRefProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "primitive-ref:index:Resource" {
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

func (p *PrimitiveRefProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *PrimitiveRefProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PrimitiveRefProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PrimitiveRefProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PrimitiveRefProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PrimitiveRefProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
