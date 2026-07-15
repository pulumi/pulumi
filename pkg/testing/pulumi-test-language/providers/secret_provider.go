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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// A small provider with a single resource "Resource" that has a "public" and "private" field, plus two nested
// objects "privateData" and "publicData" that have the same fields. The "private" fields are marked secret at
// various levels. The resource also has secret-marked collection fields: arrays and maps of both plain strings
// ("privateArray", "privateMap") and object types ("privateDataArray", "privateDataMap").
type SecretProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SecretProvider)(nil)

func (p *SecretProvider) Close() error {
	return nil
}

func (p *SecretProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SecretProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("14.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SecretProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	dataProperties := map[string]schema.PropertySpec{
		"private": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
		"public": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	dataRequired := []string{"private", "public"}

	resourceProperties := map[string]schema.PropertySpec{
		"private": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
		"public": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
		"privateData": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  "#/types/secret:index:Data",
			},
		},
		"publicData": {
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  "#/types/secret:index:Data",
			},
		},
		"privateArray": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "array",
				Items: &schema.TypeSpec{
					Type: "string",
				},
			},
		},
		"privateMap": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "object",
				AdditionalProperties: &schema.TypeSpec{
					Type: "string",
				},
			},
		},
		"privateDataArray": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "array",
				Items: &schema.TypeSpec{
					Type: "ref",
					Ref:  "#/types/secret:index:Data",
				},
			},
		},
		"privateDataMap": {
			Secret: true,
			TypeSpec: schema.TypeSpec{
				Type: "object",
				AdditionalProperties: &schema.TypeSpec{
					Type: "ref",
					Ref:  "#/types/secret:index:Data",
				},
			},
		},
	}
	resourceRequired := []string{
		"private", "public", "privateData", "publicData",
		"privateArray", "privateMap", "privateDataArray", "privateDataMap",
	}

	pkg := schema.PackageSpec{
		Name:    "secret",
		Version: "14.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"secret:index:Data": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: dataProperties,
					Required:   dataRequired,
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"secret:index:Resource": {
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

func (p *SecretProvider) CheckConfig(
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
	if version.StringValue() != "14.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 14.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SecretProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "secret:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	unsecret := func(v resource.PropertyValue) resource.PropertyValue {
		for {
			if v.IsSecret() {
				v = v.SecretValue().Element
				continue
			}
			if v.IsOutput() {
				v = v.OutputValue().Element
				continue
			}
			break
		}
		return v
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

	// isSecretString, isSecretObject and isSecretArray require the outermost wrapper to be a
	// secret, but accept any number of additional secret layers beneath it.
	isSecretString := func(v resource.PropertyValue) bool {
		return v.IsSecret() && unsecret(v).IsString()
	}

	isSecretObject := func(v resource.PropertyValue) bool {
		return v.IsSecret() && unsecret(v).IsObject()
	}

	isSecretArray := func(v resource.PropertyValue) bool {
		return v.IsSecret() && unsecret(v).IsArray()
	}

	// isDataObject reports whether v unwraps to a Data object: exactly a "private" and a
	// "public" field, each unwrapping to a string.
	isDataObject := func(v resource.PropertyValue) bool {
		v = unsecret(v)
		if !v.IsObject() {
			return false
		}
		data := v.ObjectValue()
		return len(data) == 2 &&
			unsecret(data["private"]).IsString() &&
			unsecret(data["public"]).IsString()
	}

	check := assertField(req.News, "privateData", "secret object", isSecretObject)
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "publicData", "object", resource.PropertyValue.IsObject)
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "private", "secret string", isSecretString)
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "public", "string", func(v resource.PropertyValue) bool {
		return unsecret(v).IsString()
	})
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "privateArray", "secret array of strings", func(v resource.PropertyValue) bool {
		if !isSecretArray(v) {
			return false
		}
		for _, elem := range unsecret(v).ArrayValue() {
			if !unsecret(elem).IsString() {
				return false
			}
		}
		return true
	})
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "privateMap", "secret map of strings", func(v resource.PropertyValue) bool {
		if !isSecretObject(v) {
			return false
		}
		for _, value := range unsecret(v).ObjectValue() {
			if !unsecret(value).IsString() {
				return false
			}
		}
		return true
	})
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "privateDataArray", "secret array of Data", func(v resource.PropertyValue) bool {
		if !isSecretArray(v) {
			return false
		}
		for _, elem := range unsecret(v).ArrayValue() {
			if !isDataObject(elem) {
				return false
			}
		}
		return true
	})
	if check != nil {
		return *check, nil
	}
	check = assertField(req.News, "privateDataMap", "secret map of Data", func(v resource.PropertyValue) bool {
		if !isSecretObject(v) {
			return false
		}
		for _, value := range unsecret(v).ObjectValue() {
			if !isDataObject(value) {
				return false
			}
		}
		return true
	})
	if check != nil {
		return *check, nil
	}

	if len(req.News) != 8 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	publicData := req.News["publicData"].ObjectValue()
	check = assertField(publicData, "private", "string", func(v resource.PropertyValue) bool {
		return unsecret(v).IsString()
	})
	if check != nil {
		return *check, nil
	}
	check = assertField(publicData, "public", "string", func(v resource.PropertyValue) bool {
		return unsecret(v).IsString()
	})
	if check != nil {
		return *check, nil
	}
	if len(publicData) != 2 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", publicData)),
		}, nil
	}

	privateData := unsecret(req.News["privateData"]).ObjectValue()
	check = assertField(privateData, "private", "string", func(v resource.PropertyValue) bool {
		return unsecret(v).IsString()
	})
	if check != nil {
		return *check, nil
	}
	check = assertField(privateData, "public", "string", func(v resource.PropertyValue) bool {
		return unsecret(v).IsString()
	})
	if check != nil {
		return *check, nil
	}
	if len(privateData) != 2 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", privateData)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *SecretProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "secret:index:Resource" {
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

func (p *SecretProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SecretProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SecretProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SecretProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SecretProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SecretProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
