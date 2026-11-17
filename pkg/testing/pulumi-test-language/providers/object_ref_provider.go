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

// ObjectRefProvider has object types that reference a resource of the same module. Each module has a
// resource with an object-type input, and a second resource that references the first through an object
// type. In the module "referencingFirst" the referencing resource sorts first, and in the module
// "referencedFirst" the referenced resource sorts first.
type ObjectRefProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ObjectRefProvider)(nil)

var objectRefResources = map[string]bool{
	"objectref:referencingFirst:Widget": true,
	"objectref:referencingFirst:Gadget": true,
	"objectref:referencedFirst:Alpha":   true,
	"objectref:referencedFirst:Beta":    true,
}

func (p *ObjectRefProvider) Close() error { return nil }

func (p *ObjectRefProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ObjectRefProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("62.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *ObjectRefProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	ref := func(token string) schema.PropertySpec {
		return schema.PropertySpec{TypeSpec: schema.TypeSpec{Ref: token}}
	}
	object := func(name, typ string) schema.ComplexTypeSpec {
		return schema.ComplexTypeSpec{ObjectTypeSpec: schema.ObjectTypeSpec{
			Type:       "object",
			Properties: map[string]schema.PropertySpec{name: ref(typ)},
			Required:   []string{name},
		}}
	}
	res := func(name, typ string) schema.ResourceSpec {
		return schema.ResourceSpec{
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: map[string]schema.PropertySpec{name: ref(typ)},
				Required:   []string{name},
			},
			InputProperties: map[string]schema.PropertySpec{name: ref(typ)},
			RequiredInputs:  []string{name},
		}
	}
	settings := schema.ComplexTypeSpec{ObjectTypeSpec: schema.ObjectTypeSpec{
		Type:       "object",
		Properties: map[string]schema.PropertySpec{"name": {TypeSpec: schema.TypeSpec{Type: "string"}}},
		Required:   []string{"name"},
	}}

	pkg := schema.PackageSpec{
		Name:    "objectref",
		Version: "62.0.0",
		Resources: map[string]schema.ResourceSpec{
			"objectref:referencingFirst:Widget": res("settings", "#/types/objectref:referencingFirst:WidgetSettings"),
			"objectref:referencingFirst:Gadget": res("widgetRef", "#/types/objectref:referencingFirst:WidgetRef"),
			"objectref:referencedFirst:Alpha":   res("settings", "#/types/objectref:referencedFirst:AlphaSettings"),
			"objectref:referencedFirst:Beta":    res("alphaRef", "#/types/objectref:referencedFirst:AlphaRef"),
		},
		Types: map[string]schema.ComplexTypeSpec{
			"objectref:referencingFirst:WidgetSettings": settings,
			"objectref:referencingFirst:WidgetRef": object(
				"widget", "#/resources/objectref:referencingFirst:Widget"),
			"objectref:referencedFirst:AlphaSettings": settings,
			"objectref:referencedFirst:AlphaRef": object(
				"alpha", "#/resources/objectref:referencedFirst:Alpha"),
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ObjectRefProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ObjectRefProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if !objectRefResources[string(req.URN.Type())] {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *ObjectRefProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if !objectRefResources[string(req.URN.Type())] {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
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

func (p *ObjectRefProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ObjectRefProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
