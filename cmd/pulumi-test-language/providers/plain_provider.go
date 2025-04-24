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

// A small provider with a single resource "Resource" that takes most of its inputs via plain properties.
// It includes nested types "Data" and "InnerData" that are also plain.
// The nonPlainData input tests nesting of plain and non-plain types.
type PlainProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PlainProvider)(nil)

func (p *PlainProvider) Close() error {
	return nil
}

func (p *PlainProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PlainProvider) Pkg() tokens.Package {
	return "plain"
}

func (p *PlainProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("13.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *PlainProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	makePrimitive := func(t string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type:  t,
				Plain: true,
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
				Type: "array",
				Items: &schema.TypeSpec{
					Type:  "boolean",
					Plain: true,
				},
				Plain: true,
			},
		},
		"stringMap": {
			TypeSpec: schema.TypeSpec{
				Type: "object",
				AdditionalProperties: &schema.TypeSpec{
					Type:  "string",
					Plain: true,
				},
				Plain: true,
			},
		},
	}
	typeRequired := []string{"boolean", "float", "integer", "string", "boolArray", "stringMap"}

	dataProperties := maps.Clone(typeProperties)
	dataProperties["innerData"] = schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type:  "ref",
			Ref:   "#/types/plain:index:InnerData",
			Plain: true,
		},
	}
	dataRequired := append(typeRequired, "innerData")

	resourceProperties := map[string]schema.PropertySpec{
		"data": {
			TypeSpec: schema.TypeSpec{
				Type:  "ref",
				Ref:   "#/types/plain:index:Data",
				Plain: true,
			},
		},
		"nonPlainData": {
			Description: "A non plain input to compare against the plain inputs, as well as testing plain/non-plain nesting.",
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  "#/types/plain:index:Data",
			},
		},
	}
	resourceRequired := []string{"data"}

	pkg := schema.PackageSpec{
		Name:    "plain",
		Version: "13.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"plain:index:InnerData": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: typeProperties,
					Required:   typeRequired,
				},
			},
			"plain:index:Data": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: dataProperties,
					Required:   dataRequired,
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"plain:index:Resource": {
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

func (p *PlainProvider) CheckConfig(
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
	if version.StringValue() != "13.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 13.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PlainProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "plain:index:Resource" {
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

	// Should have one or two properties: data is required, nonPlainData is optional
	if len(req.News) > 2 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

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

	checkInnerData := func(data resource.PropertyMap) *plugin.CheckResponse {
		inner := data["innerData"]
		if !inner.IsObject() {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure("innerData", "value is not an object"),
			}
		}
		check = checkData(inner.ObjectValue())
		if check != nil {
			return check
		}
		if len(inner.ObjectValue()) != 6 {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", data)),
			}
		}

		if len(data) != 7 {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", data)),
			}
		}

		return nil
	}

	// Check data
	data := req.News["data"].ObjectValue()
	check = checkData(data)
	if check != nil {
		return *check, nil
	}
	check = checkInnerData(data)
	if check != nil {
		return *check, nil
	}

	// Check nonPlainData
	if _, ok := req.News["nonPlainData"]; ok {
		nonPlainData := req.News["nonPlainData"].ObjectValue()
		check = checkData(nonPlainData)
		if check != nil {
			return *check, nil
		}
		check = checkInnerData(nonPlainData)
		if check != nil {
			return *check, nil
		}
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *PlainProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "plain:index:Resource" {
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

func (p *PlainProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *PlainProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PlainProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PlainProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PlainProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PlainProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
