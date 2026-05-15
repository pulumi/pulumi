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

// ConstValuesProvider exercises properties that carry a `const` value in the
// schema. Const properties are present at every nesting level the schema
// supports: directly on a resource/function input, inside an object property,
// inside the items of an array property, and inside the values of a map
// property.
type ConstValuesProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ConstValuesProvider)(nil)

const (
	directConstValue = "direct-const"
	nestedConstValue = "nested-const"
)

func (p *ConstValuesProvider) Pkg() tokens.Package {
	return "const-values"
}

func (ConstValuesProvider) version() semver.Version {
	return semver.Version{Major: 40}
}

func (p *ConstValuesProvider) Close() error {
	return nil
}

func (p *ConstValuesProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ConstValuesProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{Version: ref(p.version())}, nil
}

func (p *ConstValuesProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ConstValuesProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ConstValuesProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ConstValuesProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConstValuesProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConstValuesProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *ConstValuesProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	nested := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"value": {TypeSpec: schema.TypeSpec{Type: "string"}},
			"constInNested": {
				TypeSpec: schema.TypeSpec{Type: "string"},
				Const:    nestedConstValue,
			},
		},
		Required: []string{"value", "constInNested"},
	}

	nestedRef := schema.TypeSpec{Type: "ref", Ref: "#/types/const-values:index:Nested"}

	properties := map[string]schema.PropertySpec{
		"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
		"directConst": {
			TypeSpec: schema.TypeSpec{Type: "string"},
			Const:    directConstValue,
		},
		"nested": {TypeSpec: nestedRef},
		"arrayItems": {
			TypeSpec: schema.TypeSpec{Type: "array", Items: &nestedRef},
		},
		"mapItems": {
			TypeSpec: schema.TypeSpec{Type: "object", AdditionalProperties: &nestedRef},
		},
	}
	required := []string{"name", "directConst", "nested", "arrayItems", "mapItems"}

	pkg := schema.PackageSpec{
		Name:    string(p.Pkg()),
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			"const-values:index:Nested": {ObjectTypeSpec: nested},
		},
		Resources: map[string]schema.ResourceSpec{
			"const-values:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: properties,
					Required:   required,
				},
				InputProperties: properties,
				RequiredInputs:  required,
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"const-values:index:applyConst": {
				Inputs: &schema.ObjectTypeSpec{
					Type:       "object",
					Properties: properties,
					Required:   required,
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type:       "object",
						Properties: properties,
						Required:   required,
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ConstValuesProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func validateInputs(props resource.PropertyMap) []plugin.CheckFailure {
	directConst, ok := props["directConst"]
	if !ok {
		return makeCheckFailure("directConst", "missing required property")
	}
	if !directConst.IsString() || directConst.StringValue() != directConstValue {
		return makeCheckFailure("directConst", fmt.Sprintf("expected %q, got %#v", directConstValue, directConst))
	}

	validateNested := func(key resource.PropertyKey, obj resource.PropertyValue) []plugin.CheckFailure {
		if !obj.IsObject() {
			return makeCheckFailure(key, "expected an object")
		}
		m := obj.ObjectValue()
		c, ok := m["constInNested"]
		if !ok {
			return makeCheckFailure(key, "missing constInNested")
		}
		if !c.IsString() || c.StringValue() != nestedConstValue {
			return makeCheckFailure(key, fmt.Sprintf("expected constInNested %q, got %#v", nestedConstValue, c))
		}
		return nil
	}

	nested, ok := props["nested"]
	if !ok {
		return makeCheckFailure("nested", "missing required property")
	}
	if failures := validateNested("nested", nested); failures != nil {
		return failures
	}

	arrayItems, ok := props["arrayItems"]
	if !ok {
		return makeCheckFailure("arrayItems", "missing required property")
	}
	if !arrayItems.IsArray() {
		return makeCheckFailure("arrayItems", "expected an array")
	}
	for i, item := range arrayItems.ArrayValue() {
		if failures := validateNested(resource.PropertyKey(fmt.Sprintf("arrayItems[%d]", i)), item); failures != nil {
			return failures
		}
	}

	mapItems, ok := props["mapItems"]
	if !ok {
		return makeCheckFailure("mapItems", "missing required property")
	}
	if !mapItems.IsObject() {
		return makeCheckFailure("mapItems", "expected a map")
	}
	for k, v := range mapItems.ObjectValue() {
		if failures := validateNested("mapItems."+k, v); failures != nil {
			return failures
		}
	}

	return nil
}

func (p *ConstValuesProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "const-values:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	if failures := validateInputs(req.News); failures != nil {
		return plugin.CheckResponse{Failures: failures}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ConstValuesProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "const-values:index:Resource" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	id := resource.ID("id")
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{
		ID:         id,
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ConstValuesProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "const-values:index:applyConst" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}
	if failures := validateInputs(req.Args); failures != nil {
		return plugin.InvokeResponse{Failures: failures}, nil
	}
	return plugin.InvokeResponse{Properties: req.Args}, nil
}
