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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// A small provider with a single resource "Resource" that has a boolean, number, string, array, and map property.
// All inputs and outputs are optional to exercise optional value flow through resources and stack outputs.
type OptionalPrimitiveProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*OptionalPrimitiveProvider)(nil)

func (p *OptionalPrimitiveProvider) Close() error {
	return nil
}

func (p *OptionalPrimitiveProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *OptionalPrimitiveProvider) Pkg() tokens.Package {
	return "optionalprimitive"
}

func (p *OptionalPrimitiveProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	makePrimitive := func(t string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type: t,
			},
		}
	}
	resourceProperties := map[string]schema.PropertySpec{
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

	pkg := schema.PackageSpec{
		Name:    "optionalprimitive",
		Version: "34.0.0",
		Resources: map[string]schema.ResourceSpec{
			"optionalprimitive:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
				},
				InputProperties: resourceProperties,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *OptionalPrimitiveProvider) CheckConfig(
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
	if version.StringValue() != "34.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 34.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *OptionalPrimitiveProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "optionalprimitive:index:Resource"
	if req.URN.Type() != "optionalprimitive:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	unsecret := func(v resource.PropertyValue) resource.PropertyValue {
		if v.IsSecret() {
			return v.SecretValue().Element
		}
		return v
	}

	isNullLike := func(v resource.PropertyValue) bool {
		v = unsecret(v)
		if v.IsNull() {
			return true
		}
		if v.IsOutput() {
			return v.OutputValue().Element.IsNull()
		}
		return false
	}

	validate := func(key resource.PropertyKey, value resource.PropertyValue,
		assertType func(resource.PropertyValue) bool, typ string,
	) *plugin.CheckResponse {
		unwrapped := unsecret(value)
		if unwrapped.IsComputed() || unwrapped.IsOutput() || isNullLike(unwrapped) {
			return nil
		}
		if !assertType(unwrapped) {
			return &plugin.CheckResponse{
				Failures: makeCheckFailure(key, "value is not a "+typ),
			}
		}
		return nil
	}

	result := resource.PropertyMap{}
	for key, value := range req.News {
		switch key {
		case "boolean":
			check := validate("boolean", value, resource.PropertyValue.IsBool, "boolean")
			if check != nil {
				return *check, nil
			}
		case "integer":
			check := validate("integer", value, resource.PropertyValue.IsNumber, "number")
			if check != nil {
				return *check, nil
			}
		case "float":
			check := validate("float", value, resource.PropertyValue.IsNumber, "number")
			if check != nil {
				return *check, nil
			}
		case "string":
			check := validate("string", value, resource.PropertyValue.IsString, "string")
			if check != nil {
				return *check, nil
			}
		case "numberArray":
			check := validate("numberArray", value, resource.PropertyValue.IsArray, "array")
			if check != nil {
				return *check, nil
			}
			unwrapped := unsecret(value)
			if unwrapped.IsArray() {
				for _, v := range unwrapped.ArrayValue() {
					uv := unsecret(v)
					if uv.IsComputed() || uv.IsOutput() || uv.IsNull() {
						continue
					}
					if !uv.IsNumber() {
						return plugin.CheckResponse{
							Failures: makeCheckFailure("numberArray", "array element is not a number"),
						}, nil
					}
				}
			}
		case "booleanMap":
			check := validate("booleanMap", value, resource.PropertyValue.IsObject, "map")
			if check != nil {
				return *check, nil
			}
			unwrapped := unsecret(value)
			if unwrapped.IsObject() {
				for _, v := range unwrapped.ObjectValue() {
					uv := unsecret(v)
					if uv.IsComputed() || uv.IsOutput() || uv.IsNull() {
						continue
					}
					if !uv.IsBool() {
						return plugin.CheckResponse{
							Failures: makeCheckFailure("booleanMap", "map value is not a boolean"),
						}, nil
					}
				}
			}
		default:
			return plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("unexpected property: %s", key)),
			}, nil
		}

		if !isNullLike(value) {
			result[key] = value
		}
	}

	return plugin.CheckResponse{Properties: result}, nil
}

func (p *OptionalPrimitiveProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "optionalprimitive:index:Resource"
	if req.URN.Type() != "optionalprimitive:index:Resource" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	isMissingLike := func(v resource.PropertyValue) bool {
		if v.IsSecret() {
			v = v.SecretValue().Element
		}
		if v.IsNull() {
			return true
		}
		if v.IsOutput() {
			return v.OutputValue().Element.IsNull()
		}
		return false
	}

	properties := resource.PropertyMap{}
	for k, v := range req.Properties {
		if isMissingLike(v) {
			continue
		}
		properties[k] = v
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *OptionalPrimitiveProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("34.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *OptionalPrimitiveProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *OptionalPrimitiveProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *OptionalPrimitiveProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}
