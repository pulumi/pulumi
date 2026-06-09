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
// provider's namespace (matching kubernetes/crd2pulumi semantics).
//
// Parameterize and the readers below run concurrently because both the
// custom-resource step path and the remote-component path can target the
// same provider instance; the mutex guards the shared parameter fields.
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
	extensionBaseVersion = "42.0.0"
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
	_ context.Context, _ plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	name, version, value := p.snapshot()
	if name == "" {
		// Base-provider schema before parameterization.
		return plugin.GetSchemaResponse{Schema: []byte(
			`{ "name": "` + extensionBaseName + `", "version": "` + extensionBaseVersion + `" }`,
		)}, nil
	}

	// Resource lives in the base provider's namespace; this is the defining
	// trait of extension parameterization vs replacement.
	token := extensionBaseName + ":index:Greeting"
	componentToken := token + "Component"

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
		// Provider intentionally left nil and the parameterization is declared in
		// the ExtensionParameterization slot — this is what marks the package as
		// extension- rather than replacement-parameterized.
		Resources: map[string]schema.ResourceSpec{
			token:          {ObjectTypeSpec: greetingSpec},
			componentToken: {IsComponent: true, ObjectTypeSpec: greetingSpec},
		},
		ExtensionParameterization: &schema.ParameterizationSpec{
			BaseProvider: schema.BaseProviderSpec{
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
	expected := extensionBaseName + ":index:Greeting"
	if string(req.URN.Type()) != expected {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("",
				fmt.Sprintf("invalid URN type %s, expected %s", req.URN.Type(), expected)),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ExtensionParameterizedProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	expected := extensionBaseName + ":index:Greeting"
	if string(req.URN.Type()) != expected {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type %s, expected %s", req.URN.Type(), expected)
	}
	id := "id"
	if req.Preview {
		id = ""
	}
	_, _, value := p.snapshot()
	return plugin.CreateResponse{
		ID: resource.ID(id),
		Properties: resource.NewPropertyMapFromMap(map[string]any{
			"parameterValue": string(value),
		}),
		Status: resource.StatusOK,
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

func (p *ExtensionParameterizedProvider) SignalCancellation(context.Context) error { return nil }

func (p *ExtensionParameterizedProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}
