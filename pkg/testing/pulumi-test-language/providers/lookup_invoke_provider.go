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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// LookupInvokeProvider has a resource, Widget, and a root-module function, getWidget, whose name
// clashes with the Widget resource's static getter in some SDKs. The Go SDK, for example, exports
// the function as LookupWidget because GetWidget is the resource getter.
type LookupInvokeProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*LookupInvokeProvider)(nil)

func (p *LookupInvokeProvider) Close() error {
	return nil
}

func (p *LookupInvokeProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *LookupInvokeProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("63.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *LookupInvokeProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	nameProperties := map[string]schema.PropertySpec{
		"name": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	nameRequired := []string{"name"}

	pkg := schema.PackageSpec{
		Name:    "lookup-invoke",
		Version: "63.0.0",
		Resources: map[string]schema.ResourceSpec{
			"lookup-invoke:index:Widget": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: nameProperties,
					Required:   nameRequired,
				},
				InputProperties: nameProperties,
				RequiredInputs:  nameRequired,
			},
		},
		Functions: map[string]schema.FunctionSpec{
			"lookup-invoke:index:getWidget": {
				Inputs: &schema.ObjectTypeSpec{
					Type:       "object",
					Properties: nameProperties,
					Required:   nameRequired,
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"result": {
								TypeSpec: schema.TypeSpec{
									Type: "string",
								},
							},
						},
						Required: []string{"result"},
					},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *LookupInvokeProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
	version, ok := req.News.GetOk("version")
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckConfigFailure("version", "missing version"),
		}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckConfigFailure("version", "version is not a string"),
		}, nil
	}
	if version.AsString() != "63.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckConfigFailure("version", "version is not 63.0.0"),
		}, nil
	}

	if req.News.Len() != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckConfigFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *LookupInvokeProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "lookup-invoke:index:getWidget" {
		return plugin.InvokeResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}

	name, ok := req.Args.GetOk("name")
	if !ok {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("name", "missing name"),
		}, nil
	}

	if name.IsComputed() {
		return plugin.InvokeResponse{
			// providers should not get computed values (during preview)
			// since we bail out early in the core SDKs or generated provider SDKs
			// when we encounter unknowns
			Failures: makeCheckFailure("name", "name is unknown when calling getWidget"),
		}, nil
	}

	if !name.IsString() {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("name", "name is not a string"),
		}, nil
	}

	return plugin.InvokeResponse{
		Properties: property.NewMap(map[string]property.Value{
			"result": property.New("widget " + name.AsString()),
		}),
	}, nil
}

func (p *LookupInvokeProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "lookup-invoke:index:Widget" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	if req.NewInputs.Len() != 1 {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("expected exactly one property: %v", req.NewInputs)),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *LookupInvokeProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "lookup-invoke:index:Widget" {
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
