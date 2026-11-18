// Copyright 2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// InvokeResultProvider has functions that return an object with the same shape as the Settings type, and a
// function and a resource that take Settings values as inputs. A program can pass the whole result of
// makeSettings to them.
type InvokeResultProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*InvokeResultProvider)(nil)

func (p *InvokeResultProvider) Close() error { return nil }

func (p *InvokeResultProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *InvokeResultProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("64.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *InvokeResultProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	// The property names have two words, so that their SDK names differ from their wire names in languages that
	// rename properties.
	settings := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"displayName": {TypeSpec: schema.TypeSpec{Type: "string"}},
			"maxItems":    {TypeSpec: schema.TypeSpec{Type: "integer"}},
		},
		Required: []string{"displayName", "maxItems"},
	}
	settingsRef := schema.TypeSpec{Ref: "#/types/invokeresult:index:Settings"}
	settingsList := schema.TypeSpec{Type: "array", Items: &settingsRef}
	holder := map[string]schema.PropertySpec{
		"settings": {TypeSpec: settingsRef},
		"all":      {TypeSpec: settingsList},
	}

	pkg := schema.PackageSpec{
		Name:    "invokeresult",
		Version: "64.0.0",
		Resources: map[string]schema.ResourceSpec{
			"invokeresult:index:Holder": {
				ObjectTypeSpec:  schema.ObjectTypeSpec{Type: "object", Properties: holder},
				InputProperties: holder,
			},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"invokeresult:index:Settings": {ObjectTypeSpec: settings},
		},
		Functions: map[string]schema.FunctionSpec{
			// makeSettings returns its arguments. The return type is an object of its own, not a reference to
			// Settings.
			"invokeresult:index:makeSettings": {
				Inputs:     &settings,
				ReturnType: &schema.ReturnTypeSpec{ObjectTypeSpec: &settings},
			},
			// listOf returns its items unchanged.
			"invokeresult:index:listOf": {
				Inputs: &schema.ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]schema.PropertySpec{"items": {TypeSpec: settingsList}},
					Required:   []string{"items"},
				},
				ReturnType: &schema.ReturnTypeSpec{ObjectTypeSpec: &schema.ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]schema.PropertySpec{"items": {TypeSpec: settingsList}},
					Required:   []string{"items"},
				}},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *InvokeResultProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

// checkSettings fails unless value is an object with exactly the wire names of Settings.
func checkSettings(value property.Value) error {
	if !value.IsMap() {
		return fmt.Errorf("expected an object, got %v", value)
	}
	m := value.AsMap()
	if displayName, ok := m.GetOk("displayName"); !ok || !displayName.IsString() {
		return fmt.Errorf("expected a string displayName, got %v", value)
	}
	if maxItems, ok := m.GetOk("maxItems"); !ok || !maxItems.IsNumber() {
		return fmt.Errorf("expected a number maxItems, got %v", value)
	}
	if m.Len() != 2 {
		return fmt.Errorf("expected only displayName and maxItems, got %v", value)
	}
	return nil
}

func (p *InvokeResultProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	switch req.Tok {
	case "invokeresult:index:makeSettings":
		if err := checkSettings(property.New(req.Args)); err != nil {
			return plugin.InvokeResponse{Failures: makeCheckFailure("", err.Error())}, nil
		}
		return plugin.InvokeResponse{Properties: req.Args}, nil
	case "invokeresult:index:listOf":
		items, ok := req.Args.GetOk("items")
		if !ok || !items.IsArray() {
			return plugin.InvokeResponse{Failures: makeCheckFailure("items", "expected a list")}, nil
		}
		for _, item := range items.AsArray().All {
			if err := checkSettings(item); err != nil {
				return plugin.InvokeResponse{Failures: makeCheckFailure("items", err.Error())}, nil
			}
		}
		return plugin.InvokeResponse{Properties: req.Args}, nil
	}
	return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}

func (p *InvokeResultProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "invokeresult:index:Holder" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *InvokeResultProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := resource.ID("id-" + req.URN.Name())
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{
		ID:         id,
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *InvokeResultProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InvokeResultProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
