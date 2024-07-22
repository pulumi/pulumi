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

// A small provider with a single resource "Resource" that has a boolean, number, string, array, and map property.
// This is for testing that simple "primitive" values can be sent to and from the engine.
type PrimitiveProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PrimitiveProvider)(nil)

func (p *PrimitiveProvider) Close() error {
	return nil
}

func (p *PrimitiveProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PrimitiveProvider) Pkg() tokens.Package {
	return "primitive"
}

func (p *PrimitiveProvider) GetSchema(
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
	resourceRequired := []string{"boolean", "float", "integer", "string", "numberArray", "booleanMap"}

	pkg := schema.PackageSpec{
		Name:    "primitive",
		Version: "7.0.0",
		Resources: map[string]schema.ResourceSpec{
			"primitive:index:Resource": {
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

func (p *PrimitiveProvider) CheckConfig(
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
	if version.StringValue() != "7.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 7.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PrimitiveProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "primitive:index:Resource"
	if req.URN.Type() != "primitive:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	assertField := func(key resource.PropertyKey, typ string,
		assertType func(resource.PropertyValue) bool,
	) *plugin.CheckResponse {
		v, ok := req.News[key]
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

	// Expect all required properties
	check := assertField("boolean", "boolean", resource.PropertyValue.IsBool)
	if check != nil {
		return *check, nil
	}
	check = assertField("integer", "number", resource.PropertyValue.IsNumber)
	if check != nil {
		return *check, nil
	}
	check = assertField("float", "number", resource.PropertyValue.IsNumber)
	if check != nil {
		return *check, nil
	}
	check = assertField("string", "string", resource.PropertyValue.IsString)
	if check != nil {
		return *check, nil
	}
	check = assertField("numberArray", "array", resource.PropertyValue.IsArray)
	if check != nil {
		return *check, nil
	}
	// Check the array is numbers
	for _, v := range req.News["numberArray"].ArrayValue() {
		if !v.IsNumber() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("numberArray", "array element is not a number"),
			}, nil
		}
	}
	check = assertField("booleanMap", "map", resource.PropertyValue.IsObject)
	if check != nil {
		return *check, nil
	}
	// Check the map values are booleans
	for _, v := range req.News["booleanMap"].ObjectValue() {
		if !v.IsBool() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("booleanMap", "map value is not a boolean"),
			}, nil
		}
	}

	if len(req.News) != 6 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *PrimitiveProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "primitive:index:Resource"
	if req.URN.Type() != "primitive:index:Resource" {
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

func (p *PrimitiveProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("7.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *PrimitiveProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *PrimitiveProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PrimitiveProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PrimitiveProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PrimitiveProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PrimitiveProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
