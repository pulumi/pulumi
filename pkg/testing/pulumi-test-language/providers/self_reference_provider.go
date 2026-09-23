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

// SelfReferenceProvider exposes resource inputs that reference the same resource type.
type SelfReferenceProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SelfReferenceProvider)(nil)

func (p *SelfReferenceProvider) Close() error { return nil }

func (p *SelfReferenceProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SelfReferenceProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("1.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *SelfReferenceProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	ref := schema.TypeSpec{Ref: "#/resources/selfref:index:Node"}
	pkg := schema.PackageSpec{
		Name:    "selfref",
		Version: "1.0.0",
		Language: map[string]schema.RawMessage{
			"go": schema.RawMessage(`{"generateResourceContainerTypes": true}`),
		},
		Resources: map[string]schema.ResourceSpec{
			"selfref:index:Node": {
				InputProperties: map[string]schema.PropertySpec{
					"parent":       {TypeSpec: ref},
					"parents":      {TypeSpec: schema.TypeSpec{Type: "array", Items: &ref}},
					"namedParents": {TypeSpec: schema.TypeSpec{Type: "object", AdditionalProperties: &ref}},
					"parentOrName": {TypeSpec: schema.TypeSpec{OneOf: []schema.TypeSpec{ref, {Type: "string"}}}},
				},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SelfReferenceProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SelfReferenceProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "selfref:index:Node" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *SelfReferenceProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "selfref:index:Node" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
	id := req.URN.Name()
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *SelfReferenceProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SelfReferenceProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
