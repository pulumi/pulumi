// Copyright 2016, Pulumi Corporation.
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

// Like PrimitiveProvider but with all fields optional in schema and defaults specified there.
// The language host/SDK is expected to materialize defaults before calling Check.
type PrimitiveDefaultsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PrimitiveDefaultsProvider)(nil)

func (p *PrimitiveDefaultsProvider) Close() error {
	return nil
}

func (p *PrimitiveDefaultsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PrimitiveDefaultsProvider) Pkg() tokens.Package {
	return "primitive-defaults"
}

func (p *PrimitiveDefaultsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"boolean": {
			TypeSpec: schema.TypeSpec{Type: "boolean"},
			Default:  false,
		},
		"float": {
			TypeSpec: schema.TypeSpec{Type: "number"},
			Default:  0.5,
		},
		"integer": {
			TypeSpec: schema.TypeSpec{Type: "integer"},
			Default:  1,
		},
		"string": {
			TypeSpec: schema.TypeSpec{Type: "string"},
			Default:  "default",
		},
	}

	pkg := schema.PackageSpec{
		Name:    "primitive-defaults",
		Version: "8.0.0",
		Resources: map[string]schema.ResourceSpec{
			"primitive-defaults:index:Resource": {
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

func (p *PrimitiveDefaultsProvider) CheckConfig(
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
	if version.StringValue() != "8.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 8.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PrimitiveDefaultsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "primitive-defaults:index:Resource" {
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

	// Start with user-provided values.
	props := resource.PropertyMap{}
	for k, v := range req.News {
		props[k] = v
	}

	// For each optional property: assert it is present and validate its type.
	assertPresentAndType := func(
		key resource.PropertyKey,
		typeName string,
		assertType func(resource.PropertyValue) bool,
	) *plugin.CheckResponse {
		v, ok := props[key]
		if !ok {
			resp := plugin.CheckResponse{
				Failures: makeCheckFailure(key, "missing required property"),
			}
			return &resp
		}
		if !assertType(unsecret(v)) {
			resp := plugin.CheckResponse{
				Failures: makeCheckFailure(key, "value is not a "+typeName),
			}
			return &resp
		}
		return nil
	}

	if resp := assertPresentAndType("boolean", "boolean", resource.PropertyValue.IsBool); resp != nil {
		return *resp, nil
	}
	if resp := assertPresentAndType("float", "number", resource.PropertyValue.IsNumber); resp != nil {
		return *resp, nil
	}
	if resp := assertPresentAndType("integer", "number", resource.PropertyValue.IsNumber); resp != nil {
		return *resp, nil
	}
	if resp := assertPresentAndType("string", "string", resource.PropertyValue.IsString); resp != nil {
		return *resp, nil
	}

	return plugin.CheckResponse{Properties: props}, nil
}

func (p *PrimitiveDefaultsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "primitive-defaults:index:Resource" {
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

func (p *PrimitiveDefaultsProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	if req.URN.Type() != "primitive-defaults:index:Resource" {
		return plugin.UpdateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *PrimitiveDefaultsProvider) Delete(
	_ context.Context, req plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	if req.URN.Type() != "primitive-defaults:index:Resource" {
		return plugin.DeleteResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	return plugin.DeleteResponse{
		Status: resource.StatusOK,
	}, nil
}

func (p *PrimitiveDefaultsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("8.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *PrimitiveDefaultsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *PrimitiveDefaultsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PrimitiveDefaultsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PrimitiveDefaultsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PrimitiveDefaultsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}
