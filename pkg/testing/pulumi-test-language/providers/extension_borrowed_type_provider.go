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
	"errors"
	"fmt"
	"sync"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ExtensionBorrowedTypeProvider models a base plugin whose extension packages
// borrow a type from the base package instead of defining their own. The
// extension schema carries an external reference to the base type and names the
// base package in Dependencies.
type ExtensionBorrowedTypeProvider struct {
	plugin.UnimplementedProvider
	mu               sync.Mutex
	extensionName    string
	extensionVersion string
	extensionValue   []byte
}

const (
	borrowedTypeBaseName    = "borrowbase"
	borrowedTypeBaseVersion = "56.0.0"
)

var _ plugin.Provider = (*ExtensionBorrowedTypeProvider)(nil)

func (p *ExtensionBorrowedTypeProvider) snapshot() (string, string, []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.extensionName, p.extensionVersion, p.extensionValue
}

func (p *ExtensionBorrowedTypeProvider) Close() error { return nil }

func (p *ExtensionBorrowedTypeProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ExtensionBorrowedTypeProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	v := semver.MustParse(borrowedTypeBaseVersion)
	return plugin.PluginInfo{Version: &v}, nil
}

func (p *ExtensionBorrowedTypeProvider) Parameterize(
	_ context.Context, req plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	param, ok := req.Parameters.(*plugin.ParameterizeValue)
	if !ok {
		return plugin.ParameterizeResponse{}, fmt.Errorf(
			"expected ParameterizeValue, got %T", req.Parameters)
	}
	if param.Name == "" || param.Value == nil {
		return plugin.ParameterizeResponse{}, errors.New("extension parameterize requires name and value")
	}
	p.mu.Lock()
	p.extensionName = param.Name
	p.extensionVersion = param.Version.String()
	p.extensionValue = param.Value
	p.mu.Unlock()
	return plugin.ParameterizeResponse{Name: param.Name, Version: param.Version}, nil
}

const metadataToken = borrowedTypeBaseName + ":index:Metadata"

func (p *ExtensionBorrowedTypeProvider) GetSchema(
	_ context.Context, req plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	metadataSpec := schema.ComplexTypeSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]schema.PropertySpec{
				"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
			},
			Required: []string{"name"},
		},
	}

	if req.SubpackageName == "" {
		base := schema.PackageSpec{
			Name:    borrowedTypeBaseName,
			Version: borrowedTypeBaseVersion,
			Types: map[string]schema.ComplexTypeSpec{
				metadataToken: metadataSpec,
			},
			Resources: map[string]schema.ResourceSpec{
				borrowedTypeBaseName + ":index:Base": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"metadata": {TypeSpec: schema.TypeSpec{
								Ref: "#/types/" + metadataToken,
							}},
						},
						Required: []string{"metadata"},
					},
				},
			},
		}
		out, err := json.Marshal(base)
		return plugin.GetSchemaResponse{Schema: out}, err
	}

	_, _, value := p.snapshot()
	name := req.SubpackageName
	version := borrowedTypeBaseVersion
	if req.SubpackageVersion != nil {
		version = req.SubpackageVersion.String()
	}

	baseVersion := semver.MustParse(borrowedTypeBaseVersion)
	pkg := schema.PackageSpec{
		Name:    name,
		Version: version,
		Resources: map[string]schema.ResourceSpec{
			name + ":index:Widget": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"metadata": {TypeSpec: schema.TypeSpec{
							Ref: fmt.Sprintf("/%s/v%s/schema.json#/types/%s",
								borrowedTypeBaseName, borrowedTypeBaseVersion, metadataToken),
						}},
					},
					Required: []string{"metadata"},
				},
			},
		},
		Dependencies: []schema.PackageDescriptor{
			{Name: borrowedTypeBaseName, Version: &baseVersion},
		},
		ExtensionParameterization: &schema.ExtensionParameterizationSpec{
			BaseProvider: schema.BaseProviderRefSpec{
				Name:    borrowedTypeBaseName,
				Version: borrowedTypeBaseVersion,
			},
			Parameter: value,
		},
	}

	out, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: out}, err
}

func (p *ExtensionBorrowedTypeProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ExtensionBorrowedTypeProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	news := resource.ToResourcePropertyMap(req.NewInputs)
	extName, _, _ := p.snapshot()
	widget := extName + ":index:Widget"
	base := borrowedTypeBaseName + ":index:Base"
	if t := string(req.URN.Type()); t != widget && t != base {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("",
				fmt.Sprintf("invalid URN type %s, expected %s or %s", t, widget, base)),
		}, nil
	}
	return plugin.CheckResponse{Properties: resource.FromResourcePropertyMap(news)}, nil
}

func (p *ExtensionBorrowedTypeProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}
	extName, _, value := p.snapshot()
	var properties resource.PropertyMap
	switch t := string(req.URN.Type()); t {
	case extName + ":index:Widget":
		properties = resource.NewPropertyMapFromMap(map[string]any{
			"metadata": map[string]any{"name": string(value)},
		})
	case borrowedTypeBaseName + ":index:Base":
		properties = resource.NewPropertyMapFromMap(map[string]any{
			"metadata": map[string]any{"name": "base"},
		})
	default:
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type %s", t)
	}
	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: resource.FromResourcePropertyMap(properties),
		Status:     resource.StatusOK,
	}, nil
}

func (p *ExtensionBorrowedTypeProvider) SignalCancellation(context.Context) error { return nil }

func (p *ExtensionBorrowedTypeProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}
