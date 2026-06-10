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
)

// ManifestProvider is a small provider with a single resource "Resource" shaped like a typical
// "manifest": a `kind` property with a constant value in the schema and several levels of nested
// object types. It backs tests for reading constant-valued properties and for binding programs
// that write nested typed properties with quoted keys.
type ManifestProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ManifestProvider)(nil)

func (p *ManifestProvider) Close() error {
	return nil
}

func (p *ManifestProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ManifestProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("43.0.0")
	return plugin.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ManifestProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	ref := func(token string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{Type: "ref", Ref: "#/types/" + token},
		}
	}
	stringMap := schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Type:                 "object",
			AdditionalProperties: &schema.TypeSpec{Type: "string"},
		},
	}

	resourceProperties := map[string]schema.PropertySpec{
		"kind": {
			Const:    "Manifest",
			TypeSpec: schema.TypeSpec{Type: "string"},
		},
		"metadata": ref("manifest:index:Metadata"),
		"spec":     ref("manifest:index:Spec"),
	}
	resourceRequired := []string{"kind", "metadata"}

	pkg := schema.PackageSpec{
		Name:    "manifest",
		Version: "43.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"manifest:index:Metadata": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"name":   {TypeSpec: schema.TypeSpec{Type: "string"}},
						"labels": stringMap,
					},
					Required: []string{"name"},
				},
			},
			"manifest:index:Spec": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"replicas": {TypeSpec: schema.TypeSpec{Type: "integer"}},
						"template": ref("manifest:index:Template"),
					},
					Required: []string{"template"},
				},
			},
			"manifest:index:Template": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"metadata": ref("manifest:index:Metadata"),
						"containers": {
							TypeSpec: schema.TypeSpec{
								Type:  "array",
								Items: &schema.TypeSpec{Type: "ref", Ref: "#/types/manifest:index:Container"},
							},
						},
					},
					Required: []string{"containers"},
				},
			},
			"manifest:index:Container": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"name":  {TypeSpec: schema.TypeSpec{Type: "string"}},
						"image": {TypeSpec: schema.TypeSpec{Type: "string"}},
						"ports": {
							TypeSpec: schema.TypeSpec{
								Type:  "array",
								Items: &schema.TypeSpec{Type: "integer"},
							},
						},
					},
					Required: []string{"name", "image"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"manifest:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *ManifestProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ManifestProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "manifest:index:Resource" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	kind, ok := req.News["kind"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("kind", "missing value"),
		}, nil
	}
	if !kind.IsString() || kind.StringValue() != "Manifest" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("kind", fmt.Sprintf("value is not the constant \"Manifest\": %v", kind)),
		}, nil
	}

	metadata, ok := req.News["metadata"]
	if !ok {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("metadata", "missing value"),
		}, nil
	}
	if !metadata.IsObject() {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("metadata", "value is not an object"),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ManifestProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "manifest:index:Resource" {
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

func (p *ManifestProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ManifestProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
