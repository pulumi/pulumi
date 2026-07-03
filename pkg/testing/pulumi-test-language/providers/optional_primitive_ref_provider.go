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

// A small provider with a single resource "Resource" that has a single field "Data" which is a type whose
// inner primitive fields are all optional. The resource's "data" property itself is required (both as input
// and output). This exercises the codegen path where a program traverses an output object to reach an
// optional scalar (e.g. `res.data.string` where the inner field is *string).
type OptionalPrimitiveRefProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*OptionalPrimitiveRefProvider)(nil)

func (p *OptionalPrimitiveRefProvider) Close() error {
	return nil
}

func (p *OptionalPrimitiveRefProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *OptionalPrimitiveRefProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("40.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *OptionalPrimitiveRefProvider) GetSchema(
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
		"numberArray": {
			TypeSpec: schema.TypeSpec{
				Type:  "array",
				Items: &schema.TypeSpec{Type: "number"},
			},
		},
		"booleanMap": {
			TypeSpec: schema.TypeSpec{
				Type:                 "object",
				AdditionalProperties: &schema.TypeSpec{Type: "boolean"},
			},
		},
	}
	// No "required" array: all inner fields are optional.

	resourceProperties := map[string]schema.PropertySpec{
		"data": {
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  "#/types/optional-primitive-ref:index:Data",
			},
		},
	}
	resourceRequired := []string{"data"}

	pkg := schema.PackageSpec{
		Name:    "optional-primitive-ref",
		Version: "40.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"optional-primitive-ref:index:Data": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: typeProperties,
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"optional-primitive-ref:index:Resource": {
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

func (p *OptionalPrimitiveRefProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
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
	if version.StringValue() != "40.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 40.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *OptionalPrimitiveRefProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "optional-primitive-ref:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	data, ok := req.News["data"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("data", "missing value"),
		}, nil
	}
	if !data.IsObject() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("data", "value is not an object"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	inner := data.ObjectValue()
	for key, v := range inner {
		switch key {
		case "boolean":
			if !v.IsBool() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure(key, "value is not a boolean"),
				}, nil
			}
		case "float":
			if !v.IsNumber() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure(key, "value is not a number"),
				}, nil
			}
		case "integer":
			if !v.IsNumber() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure(key, "value is not a number"),
				}, nil
			}
		case "string":
			if !v.IsString() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure(key, "value is not a string"),
				}, nil
			}
		case "numberArray":
			if !v.IsArray() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure(key, "value is not an array"),
				}, nil
			}
			for _, e := range v.ArrayValue() {
				if !e.IsNumber() {
					return plugin.CheckResponse{
						Failures: makeCheckFailure(key, "array element is not a number"),
					}, nil
				}
			}
		case "booleanMap":
			if !v.IsObject() {
				return plugin.CheckResponse{
					Failures: makeCheckFailure(key, "value is not a map"),
				}, nil
			}
			for _, e := range v.ObjectValue() {
				if !e.IsBool() {
					return plugin.CheckResponse{
						Failures: makeCheckFailure(key, "map value is not a boolean"),
					}, nil
				}
			}
		default:
			return plugin.CheckResponse{
				Failures: makeCheckFailure(resource.PropertyKey("data."+string(key)),
					fmt.Sprintf("unexpected property: %s", key)),
			}, nil
		}
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *OptionalPrimitiveRefProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "optional-primitive-ref:index:Resource" {
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

func (p *OptionalPrimitiveRefProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *OptionalPrimitiveRefProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *OptionalPrimitiveRefProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *OptionalPrimitiveRefProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *OptionalPrimitiveRefProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *OptionalPrimitiveRefProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
