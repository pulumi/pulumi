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
	"maps"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A small provider with a single resource "Resource" that has a single field "Data" which is a type with some primitive
// fields and a field to another nested type "InnerData" which is a type with some primitive fields.
type RefRefProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*RefRefProvider)(nil)

func (p *RefRefProvider) Close() error {
	return nil
}

func (p *RefRefProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *RefRefProvider) Pkg() tokens.Package {
	return "ref-ref"
}

func (p *RefRefProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("12.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *RefRefProvider) GetSchema(
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

	dataProperties := maps.Clone(typeProperties)
	dataProperties["innerData"] = schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type: "ref",
			Ref:  "#/types/ref-ref:index:InnerData",
		},
	}
	dataRequired := append(typeRequired, "innerData")

	resourceProperties := map[string]schema.PropertySpec{
		"data": {
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  "#/types/ref-ref:index:Data",
			},
		},
	}
	resourceRequired := []string{"data"}

	pkg := schema.PackageSpec{
		Name:    "ref-ref",
		Version: "12.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"ref-ref:index:InnerData": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: typeProperties,
					Required:   typeRequired,
				},
			},
			"ref-ref:index:Data": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: dataProperties,
					Required:   dataRequired,
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"ref-ref:index:Resource": {
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

func (p *RefRefProvider) CheckConfig(
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
	if version.StringValue() != "12.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 12.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *RefRefProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "ref-ref:index:Resource" {
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

	checkData := func(data resource.PropertyMap) *plugin.CheckResponse {
		// Expect all required properties
		check = assertField(data, "boolean", "boolean", resource.PropertyValue.IsBool)
		if check != nil {
			return check
		}
		check = assertField(data, "integer", "number", resource.PropertyValue.IsNumber)
		if check != nil {
			return check
		}
		check = assertField(data, "float", "number", resource.PropertyValue.IsNumber)
		if check != nil {
			return check
		}
		check = assertField(data, "string", "string", resource.PropertyValue.IsString)
		if check != nil {
			return check
		}
		check = assertField(data, "boolArray", "array", resource.PropertyValue.IsArray)
		if check != nil {
			return check
		}
		for _, v := range data["boolArray"].ArrayValue() {
			if !v.IsBool() {
				return &plugin.CheckResponse{
					Failures: makeCheckFailure("boolArray", "array element is not a boolean"),
				}
			}
		}
		check = assertField(data, "stringMap", "map", resource.PropertyValue.IsObject)
		if check != nil {
			return check
		}
		for _, v := range data["stringMap"].ObjectValue() {
			if !v.IsString() {
				return &plugin.CheckResponse{
					Failures: makeCheckFailure("stringMap", "map value is not a string"),
				}
			}
		}

		return nil
	}

	check = checkData(data)
	if check != nil {
		return *check, nil
	}

	inner := data["innerData"]
	if !inner.IsObject() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("innerData", "value is not an object"),
		}, nil
	}
	check = checkData(inner.ObjectValue())
	if check != nil {
		return *check, nil
	}
	if len(inner.ObjectValue()) != 6 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", data)),
		}, nil
	}

	if len(data) != 7 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", data)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *RefRefProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "ref-ref:index:Resource" {
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

func (p *RefRefProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *RefRefProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *RefRefProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *RefRefProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *RefRefProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *RefRefProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
