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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// ExtensionParameterizedProvider models a base plugin that accepts extension
// parameterization arguments. The emitted schema sets Name to the extension
// identity, leaves Provider nil, and keeps every resource token in the base
// provider's namespace.
type ExtensionParameterizedProvider struct {
	plugin.UnimplementedProvider
	mu               sync.Mutex
	extensionName    string
	extensionVersion string
	extensionValue   []byte
}

func (p *ExtensionParameterizedProvider) snapshot() (string, string, []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.extensionName, p.extensionVersion, p.extensionValue
}

const (
	extensionBaseName    = "extbase"
	extensionBaseVersion = "45.0.0"
)

var _ plugin.Provider = (*ExtensionParameterizedProvider)(nil)

func (p *ExtensionParameterizedProvider) Close() error { return nil }

func (p *ExtensionParameterizedProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ExtensionParameterizedProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	v := semver.MustParse(extensionBaseVersion)
	return plugin.PluginInfo{Version: &v}, nil
}

func (p *ExtensionParameterizedProvider) Parameterize(
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

func (p *ExtensionParameterizedProvider) GetSchema(
	_ context.Context, req plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	if req.SubpackageName == "" {
		base := schema.PackageSpec{
			Name:    extensionBaseName,
			Version: extensionBaseVersion,
			Resources: map[string]schema.ResourceSpec{
				extensionBaseName + ":index:Base": {
					ObjectTypeSpec: schema.ObjectTypeSpec{
						Type: "object",
						Properties: map[string]schema.PropertySpec{
							"baseValue": {TypeSpec: schema.TypeSpec{Type: "string"}},
						},
						Required: []string{"baseValue"},
					},
				},
			},
		}
		out, err := json.Marshal(base)
		return plugin.GetSchemaResponse{Schema: out}, err
	}

	_, _, value := p.snapshot()
	name := req.SubpackageName
	version := extensionBaseVersion
	if req.SubpackageVersion != nil {
		version = req.SubpackageVersion.String()
	}

	token := extensionBaseName + ":index:Greeting"
	componentToken := token + "Component"
	greetToken := extensionBaseName + ":index:greet"

	greetingSpec := schema.ObjectTypeSpec{
		Type: "object",
		Properties: map[string]schema.PropertySpec{
			"parameterValue": {TypeSpec: schema.TypeSpec{Type: "string"}},
		},
		Required: []string{"parameterValue"},
	}

	pkg := schema.PackageSpec{
		Name:    name,
		Version: version,
		Resources: map[string]schema.ResourceSpec{
			token:          {ObjectTypeSpec: greetingSpec},
			componentToken: {IsComponent: true, ObjectTypeSpec: greetingSpec},
		},
		Functions: map[string]schema.FunctionSpec{
			greetToken: {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"name": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"name"},
				},
				Outputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"greeting": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"greeting"},
				},
			},
		},
		ExtensionParameterization: &schema.ExtensionParameterizationSpec{
			BaseProvider: schema.BaseProviderRefSpec{
				Name:    extensionBaseName,
				Version: extensionBaseVersion,
			},
			Parameter: value,
		},
	}

	out, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: out}, err
}

func (p *ExtensionParameterizedProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ExtensionParameterizedProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	greeting := extensionBaseName + ":index:Greeting"
	base := extensionBaseName + ":index:Base"
	if t := string(req.URN.Type()); t != greeting && t != base {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("",
				fmt.Sprintf("invalid URN type %s, expected %s or %s", t, greeting, base)),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ExtensionParameterizedProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := "id"
	if req.Preview {
		id = ""
	}
	_, _, value := p.snapshot()
	var properties resource.PropertyMap
	switch t := string(req.URN.Type()); t {
	case extensionBaseName + ":index:Greeting":
		properties = resource.NewPropertyMapFromMap(map[string]any{
			"parameterValue": string(value),
		})
	case extensionBaseName + ":index:Base":
		properties = resource.NewPropertyMapFromMap(map[string]any{
			"baseValue": "base",
		})
	default:
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type %s", t)
	}
	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *ExtensionParameterizedProvider) Construct(
	_ context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	token := extensionBaseName + ":index:GreetingComponent"
	_, _, value := p.snapshot()
	return plugin.ConstructResponse{
		URN: resource.CreateURN(req.Name, token, req.Parent, req.Info.Project, req.Info.Stack),
		Outputs: resource.PropertyMap{
			"parameterValue": resource.NewProperty(string(value) + "Component"),
		},
	}, nil
}

func (p *ExtensionParameterizedProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	expected := extensionBaseName + ":index:greet"
	if string(req.Tok) != expected {
		return plugin.InvokeResponse{}, fmt.Errorf("invalid invoke token %s, expected %s", req.Tok, expected)
	}
	_, _, value := p.snapshot()
	return plugin.InvokeResponse{
		Properties: resource.NewPropertyMapFromMap(map[string]any{
			"greeting": string(value) + ", " + req.Args["name"].StringValue(),
		}),
	}, nil
}

func (p *ExtensionParameterizedProvider) SignalCancellation(context.Context) error { return nil }

func (p *ExtensionParameterizedProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}
