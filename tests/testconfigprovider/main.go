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

// This provider is used in tests to verify that provider configuration (CheckConfig, Configure, DiffConfig) works as
// expected across various programming languages, and the gRPC protocol and SDK generators are in alignment.
//
// The schema has the following properties:
//
//	"s", "b", "i", "n": string, boolean, integer and number-typed properties.
//	"ls", "lb", "li", "ln": primitives wrapped in a list
//	"ms", "mb", "mi", "mn": primitives wrapped in a map
//	"os", "ob", "oi", "on": primitives wrapped in an object type {"x": t}
//
// After configuring the provider, the test suite may invoke testprovider:index:getConfig to get back a {"result": ".."}
// where ".." would contain the protojson-encoded gRPC ConfigureRequest.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/encoding/protojson"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	providerName = "testconfigprovider"
	version      = "0.0.1"
)

func schema() pschema.PackageSpec {
	configSpec := pschema.ConfigSpec{
		Variables: map[string]pschema.PropertySpec{},
	}
	types := map[string]pschema.ComplexTypeSpec{}

	for _, t := range []string{"string", "boolean", "integer", "number"} {
		ts := pschema.TypeSpec{Type: t}
		c := t[0:1]
		configSpec.Variables[c] = pschema.PropertySpec{TypeSpec: ts}
		configSpec.Variables["l"+c] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:  "array",
				Items: &ts,
			},
		}
		configSpec.Variables["m"+c] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:                 "object",
				AdditionalProperties: &ts,
			},
		}
		typeToken := fmt.Sprintf("%s:index:T%s", providerName, c)
		typeRef := fmt.Sprintf("#/types/%s", typeToken)
		configSpec.Variables["o"+c] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Ref: typeRef,
			},
		}
		types[typeToken] = pschema.ComplexTypeSpec{
			ObjectTypeSpec: pschema.ObjectTypeSpec{
				Type: "object",
				Properties: map[string]pschema.PropertySpec{
					"x": pschema.PropertySpec{TypeSpec: ts},
				},
			},
		}
	}

	schema := pschema.PackageSpec{
		Name:    providerName,
		Version: version,
		Config:  configSpec,
		Provider: pschema.ResourceSpec{
			InputProperties: configSpec.Variables,
			ObjectTypeSpec: pschema.ObjectTypeSpec{
				Properties: configSpec.Variables,
			},
		},
		Types:     types,
		Resources: map[string]pschema.ResourceSpec{},
		Functions: map[string]pschema.FunctionSpec{
			fmt.Sprintf("%s:index:getConfig", providerName): pschema.FunctionSpec{
				Outputs: &pschema.ObjectTypeSpec{
					Properties: map[string]pschema.PropertySpec{
						"result": pschema.PropertySpec{
							TypeSpec: pschema.TypeSpec{Type: "string"},
						},
					},
				},
			},
		},
		Language: map[string]pschema.RawMessage{
			"nodejs": []byte(`{"respectSchemaVersion": true}`),
		},
	}

	return schema
}

type configProviderServer struct {
	pulumirpc.UnimplementedResourceProviderServer
	configureRequestJSON string
}

func (c *configProviderServer) Invoke(
	ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	return &pulumirpc.InvokeResponse{Return: &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"result": structpb.NewStringValue(c.configureRequestJSON),
		},
	}}, nil
}

func (c *configProviderServer) GetPluginInfo(
	ctx context.Context,
	req *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version}, nil
}

func (c *configProviderServer) Attach(
	ctx context.Context,
	req *pulumirpc.PluginAttach,
) (*emptypb.Empty, error) {
	return nil, nil
}

func (c *configProviderServer) CheckConfig(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (c *configProviderServer) GetSchema(
	ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	sb, err := json.Marshal(schema())
	if err != nil {
		return nil, err
	}
	return &pulumirpc.GetSchemaResponse{Schema: string(sb)}, nil
}

func (c *configProviderServer) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (resp *pulumirpc.ConfigureResponse, err error) {
	reqJson, err := protojson.Marshal(req)
	if err != nil {
		return nil, err
	}
	c.configureRequestJSON = string(reqJson)
	return &pulumirpc.ConfigureResponse{
		AcceptOutputs:   true,
		AcceptResources: true,
		AcceptSecrets:   true,
		SupportsPreview: true,
	}, nil
}

func makeProvider() (pulumirpc.ResourceProviderServer, error) {
	return &configProviderServer{}, nil
}

//nolint:unused,deadcode
func main() {
	if err := provider.Main(providerName, func(*provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return makeProvider()
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
