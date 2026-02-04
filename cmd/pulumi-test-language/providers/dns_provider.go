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

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type DNSProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*DNSProvider)(nil)

func (p *DNSProvider) Close() error {
	return nil
}

func (p *DNSProvider) version() semver.Version {
	return semver.Version{Major: 1, Minor: 42}
}

func (p *DNSProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *DNSProvider) Pkg() tokens.Package {
	return "dns"
}

func (p *DNSProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "dns",
		Version: p.version().String(),
		Types: map[string]schema.ComplexTypeSpec{
			"dns:index:DnsChallenge": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"domainName": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
						"recordName": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"domainName", "recordName"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"dns:index:Subscription": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"domains": {
							TypeSpec: schema.TypeSpec{
								Type:  "array",
								Items: &schema.TypeSpec{Type: "string"},
							},
						},
						"challenges": {
							TypeSpec: schema.TypeSpec{
								Type: "array",
								Items: &schema.TypeSpec{
									Ref: "#/types/dns:index:DnsChallenge",
								},
							},
						},
					},
					Required: []string{"domains", "challenges"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"domains": {
						TypeSpec: schema.TypeSpec{
							Type:  "array",
							Items: &schema.TypeSpec{Type: "string"},
						},
					},
				},
				RequiredInputs: []string{"domains"},
			},
			"dns:index:Record": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"name": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"name"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"name": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"name"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *DNSProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *DNSProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *DNSProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}

	outputs := req.Properties.Copy()

	if req.URN.Type() == "dns:index:Subscription" {
		// Compute challenges from domains: for each domain, create a challenge object.
		domains := req.Properties["domains"]
		if domains.IsArray() {
			challenges := make([]resource.PropertyValue, len(domains.ArrayValue()))
			for i, domain := range domains.ArrayValue() {
				challenges[i] = resource.NewProperty(resource.PropertyMap{
					"domainName": domain,
					"recordName": resource.NewProperty("_acme-challenge." + domain.StringValue()),
				})
			}
			outputs["challenges"] = resource.NewProperty(challenges)
		}
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: outputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *DNSProvider) Update(
	_ context.Context, req plugin.UpdateRequest,
) (plugin.UpdateResponse, error) {
	return plugin.UpdateResponse{
		Properties: req.NewInputs,
		Status:     resource.StatusOK,
	}, nil
}

func (p *DNSProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: ref(p.version()),
	}, nil
}

func (p *DNSProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *DNSProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *DNSProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *DNSProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DNSProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *DNSProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *DNSProvider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  req.Inputs,
			Outputs: req.State,
		},
		Status: resource.StatusOK,
	}, nil
}
