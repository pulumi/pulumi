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

// UnionRefProvider has a Pipeline resource whose inputs are typed as a union of two resource types: at the
// top level, inside an object type, and as the elements of a list.
type UnionRefProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*UnionRefProvider)(nil)

var unionRefResources = map[string]bool{
	"unionref:index:Github":   true,
	"unionref:index:Gitlab":   true,
	"unionref:index:Pipeline": true,
}

func (p *UnionRefProvider) Close() error { return nil }

func (p *UnionRefProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *UnionRefProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("63.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *UnionRefProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	source := schema.TypeSpec{OneOf: []schema.TypeSpec{
		{Ref: "#/resources/unionref:index:Github"},
		{Ref: "#/resources/unionref:index:Gitlab"},
	}}
	properties := map[string]schema.PropertySpec{
		"source":  {TypeSpec: source},
		"program": {TypeSpec: schema.TypeSpec{Ref: "#/types/unionref:index:Program"}},
		"sources": {TypeSpec: schema.TypeSpec{Type: "array", Items: &source}},
	}
	empty := schema.ResourceSpec{ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"}}

	pkg := schema.PackageSpec{
		Name:    "unionref",
		Version: "63.0.0",
		Resources: map[string]schema.ResourceSpec{
			"unionref:index:Github": empty,
			"unionref:index:Gitlab": empty,
			"unionref:index:Pipeline": {
				ObjectTypeSpec:  schema.ObjectTypeSpec{Type: "object", Properties: properties},
				InputProperties: properties,
			},
		},
		Types: map[string]schema.ComplexTypeSpec{
			"unionref:index:Program": {ObjectTypeSpec: schema.ObjectTypeSpec{
				Type:       "object",
				Properties: map[string]schema.PropertySpec{"source": {TypeSpec: source}},
			}},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *UnionRefProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *UnionRefProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if !unionRefResources[string(req.URN.Type())] {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *UnionRefProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if !unionRefResources[string(req.URN.Type())] {
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

func (p *UnionRefProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *UnionRefProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
