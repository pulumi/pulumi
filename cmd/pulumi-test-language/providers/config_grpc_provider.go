// Copyright 2016-2024, Pulumi Corporation.
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
	"strings"

	"github.com/blang/semver"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// ConfigGrpcProvider helps testing gRPC-level payloads over CheckConfig, Configure, and DiffConfig methods.
//
// In particular this provider is used to verify that all languages have parity in how the payloads that get sent on the
// wire to the provider.
//
// The schema for configuration is intentionally interesting and has the following properties:
//
//	"string1", "bool1", "int1", "num1": string, boolean, integer and number-typed properties.
//	"listString1", "listBool1", "listInt1", "listNum1": primitives wrapped in a list
//	"mapString1", "mapBool1", "mapInt1", "mapNum1": primitives wrapped in a map
//	"objString1", "objBool1", "objInt1", "objNum1": primitives wrapped in an object type {"x": t}
//
// Each property is suffixed by 1,2,3 to allow more simultaneous tests.
//
// A special resource Config can be invoked to retrieve a JSON-encoded representation of what the provider received over
// the wire for configuration. It exposes this as the "config" output property.
type ConfigGrpcProvider struct {
	plugin.UnimplementedProvider
	lastCheckConfigRequest RPCRequest
	lastConfigureRequest   RPCRequest
}

var (
	_ plugin.Provider          = (*ConfigGrpcProvider)(nil)
	_ ProviderWithCustomServer = (*ConfigGrpcProvider)(nil)
)

func (*ConfigGrpcProvider) Pkg() tokens.Package {
	return "config-grpc"
}

func (*ConfigGrpcProvider) version() string {
	return "1.0.0"
}

func (p *ConfigGrpcProvider) generateSchema(
	types map[string]pschema.ComplexTypeSpec,
	minN int,
	maxN int,
) map[string]pschema.PropertySpec {
	spec := map[string]pschema.PropertySpec{}
	for n := minN; n <= maxN; n++ {
		p.populateSchema(types, n, spec)
	}
	return spec
}

func (p *ConfigGrpcProvider) populateSchema(
	types map[string]pschema.ComplexTypeSpec,
	n int,
	spec map[string]pschema.PropertySpec,
) {
	for name, t := range map[string]string{
		"string": "string",
		"bool":   "boolean",
		"int":    "integer",
		"num":    "number",
	} {
		ts := pschema.TypeSpec{Type: t}
		c := name
		if n != 0 {
			c = fmt.Sprintf("%s%d", c, n)
		}
		cUpper := strings.ToUpper(c[0:1]) + c[1:]
		spec[c] = pschema.PropertySpec{TypeSpec: ts}
		spec["list"+cUpper] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:  "array",
				Items: &ts,
			},
		}
		spec["map"+cUpper] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:                 "object",
				AdditionalProperties: &ts,
			},
		}
		typeToken := fmt.Sprintf("%s:index:T%s", p.Pkg(), c)
		typeRef := "#/types/" + typeToken
		spec["obj"+cUpper] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Ref: typeRef,
			},
		}
		types[typeToken] = pschema.ComplexTypeSpec{
			ObjectTypeSpec: pschema.ObjectTypeSpec{
				Type: "object",
				Properties: map[string]pschema.PropertySpec{
					"x": {TypeSpec: ts},
				},
			},
		}
	}
}

func (p *ConfigGrpcProvider) schema() pschema.PackageSpec {
	types := map[string]pschema.ComplexTypeSpec{}
	configSpec := pschema.ConfigSpec{
		Variables: p.generateSchema(types, 1, 3),
	}

	schema := pschema.PackageSpec{
		Name:    string(p.Pkg()),
		Version: p.version(),
		Config:  configSpec,
		Provider: pschema.ResourceSpec{
			InputProperties: configSpec.Variables,
			ObjectTypeSpec: pschema.ObjectTypeSpec{
				Properties: configSpec.Variables,
			},
		},
		Types: types,
		Resources: map[string]pschema.ResourceSpec{
			fmt.Sprintf("%s:index:Config", p.Pkg()): {
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Properties: map[string]pschema.PropertySpec{
						"config": {
							TypeSpec: pschema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"config"},
				},
			},
		},
		Functions: map[string]pschema.FunctionSpec{},
		Language: map[string]pschema.RawMessage{
			"nodejs": []byte(`{"respectSchemaVersion": true}`),
		},
	}

	toSecretSchema := p.generateSchema(types, 1, 3)
	allProps := []string{}
	for k := range toSecretSchema {
		allProps = append(allProps, k)
	}

	schema.Functions[fmt.Sprintf("%s:index:toSecret", p.Pkg())] = pschema.FunctionSpec{
		Inputs: &pschema.ObjectTypeSpec{
			Properties: toSecretSchema,
		},
		Outputs: &pschema.ObjectTypeSpec{
			Properties: toSecretSchema,
			Required:   allProps,
		},
	}

	return schema
}

func (p *ConfigGrpcProvider) NewProviderServer() pulumirpc.ResourceProviderServer {
	return &grpcCapturingProviderServer{
		ResourceProviderServer: plugin.NewProviderServer(p),
		onRequest: func(r RPCRequest) {
			switch r.Method {
			case ConfigureMethod:
				p.lastConfigureRequest = r
			case CheckConfigMethod:
				p.lastCheckConfigRequest = r
			}
		},
	}
}

func (p *ConfigGrpcProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	schema := p.schema()
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	return plugin.GetSchemaResponse{Schema: schemaBytes}, nil
}

func (p *ConfigGrpcProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse(p.version())
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *ConfigGrpcProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ConfigGrpcProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ConfigGrpcProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ConfigGrpcProvider) Create(
	ctx context.Context,
	req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if string(req.Type) == fmt.Sprintf("%s:index:Config", p.Pkg()) {
		requestsJSON, err := json.Marshal([]RPCRequest{
			p.lastCheckConfigRequest,
			p.lastConfigureRequest,
		})
		contract.AssertNoErrorf(err, "json.Marshal failed")

		id := ""
		if !req.Preview {
			id = "id0"
		}

		// Send out Config-related requests.
		return plugin.CreateResponse{
			ID: resource.ID(id),
			Properties: resource.PropertyMap{
				"config": resource.NewStringProperty(string(requestsJSON)),
			},
			Status: resource.StatusOK,
		}, nil
	}
	return plugin.CreateResponse{}, errors.New("Unknown resource")
}

func (p *ConfigGrpcProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConfigGrpcProvider) Invoke(
	ctx context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	switch {
	case string(req.Tok) == fmt.Sprintf("%s:index:toSecret", p.Pkg()):
		secreted := req.Args.Copy()
		for k, v := range secreted {
			secreted[k] = resource.MakeSecret(v)
		}
		return plugin.InvokeResponse{Properties: secreted}, nil
	default:
		return plugin.InvokeResponse{}, errors.New("Unknown function")
	}
}
