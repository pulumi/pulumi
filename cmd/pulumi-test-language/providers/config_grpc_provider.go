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
	"fmt"
	"sync"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/types"
	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// ConfigGrpcProvider helps testing gRPC-level payloads over CheckConfig, Configure, and DiffConfig methods.
//
// In particular this provider is used to verify that all languages have parity in how the payloads that get sent on the
// wire to the provider.
//
// The schema for configuration is intentionally interesting and has the following properties:
//
//	"s", "b", "i", "n": string, boolean, integer and number-typed properties.
//	"ls", "lb", "li", "ln": primitives wrapped in a list
//	"ms", "mb", "mi", "mn": primitives wrapped in a map
//	"os", "ob", "oi", "on": primitives wrapped in an object type {"x": t}
//
// Each property is suffixed by 1,2,3 to allow more simultaneous tests.
//
// A special resource ConfigGetter can be invoked to retrieve a JSON-encoded representation of what the provider
// received over the wire for configuration. It exposes this as the "config" output property.
type ConfigGrpcProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ConfigGrpcProvider)(nil)
var _ types.ProviderWithCustomServer = (*ConfigGrpcProvider)(nil)

func (*ConfigGrpcProvider) Pkg() tokens.Package {
	return "testconfigprovider"
}

func (*ConfigGrpcProvider) version() string {
	return "0.0.1"
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
	for _, t := range []string{"string", "boolean", "integer", "number"} {
		ts := pschema.TypeSpec{Type: t}
		c := fmt.Sprintf("%s", t[0:1])
		if n != 0 {
			c = fmt.Sprintf("%s%d", t[0:1], n)
		}
		spec[c] = pschema.PropertySpec{TypeSpec: ts}
		spec["l"+c] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:  "array",
				Items: &ts,
			},
		}
		spec["m"+c] = pschema.PropertySpec{
			TypeSpec: pschema.TypeSpec{
				Type:                 "object",
				AdditionalProperties: &ts,
			},
		}
		typeToken := fmt.Sprintf("%s:index:T%s", p.Pkg(), c)
		typeRef := "#/types/" + typeToken
		spec["o"+c] = pschema.PropertySpec{
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
			fmt.Sprintf("%s:index:ConfigGetter", p.Pkg()): pschema.ResourceSpec{
				ObjectTypeSpec: pschema.ObjectTypeSpec{
					Properties: map[string]pschema.PropertySpec{
						"config": pschema.PropertySpec{
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

	toSecretSchema := p.generateSchema(types, 0, 0)
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
	return &configGrpcProviderServer{
		ResourceProviderServer: plugin.NewProviderServer(p),
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
		return plugin.InvokeResponse{}, fmt.Errorf("Unknown function")
	}
}

// This lower level implementation should be used at runtime specifically to test at the lower level, since not all
// actual providers use plugin.Provider consistently but some are using older or modified versions.
// pulumirpc.UnimplementedResourceProviderServer
type configGrpcProviderServer struct {
	pulumirpc.ResourceProviderServer

	// Guard the state.
	sync.Mutex

	// State to capture of configuration-related requests.
	configRequests []json.RawMessage
}

func (p *configGrpcProviderServer) logMessage(msg proto.Message) error {
	req, err := protojson.Marshal(msg)
	if err != nil {
		return err
	}
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	type tagged struct {
		Method  string          `json:"method"`
		Message json.RawMessage `json:"message"`
	}
	bytes, err := json.Marshal(tagged{
		Method:  string(msg.ProtoReflect().Descriptor().FullName()),
		Message: req,
	})
	if err != nil {
		return err
	}
	p.configRequests = append(p.configRequests, bytes)
	return nil
}

func (p *configGrpcProviderServer) formatLoggedMessages() string {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	bytes, err := json.Marshal(p.configRequests)
	contract.AssertNoErrorf(err, "json.Marshal failed")
	return string(bytes)
}

func (p *configGrpcProviderServer) CheckConfig(
	ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	if err := p.logMessage(req); err != nil {
		return nil, err
	}
	return p.ResourceProviderServer.CheckConfig(ctx, req)
}

func (p *configGrpcProviderServer) Configure(
	ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (resp *pulumirpc.ConfigureResponse, err error) {
	if err := p.logMessage(req); err != nil {
		return nil, err
	}
	return p.ResourceProviderServer.Configure(ctx, req)
}

func (p *configGrpcProviderServer) DiffConfig(
	ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	if err := p.logMessage(req); err != nil {
		return nil, err
	}
	return p.ResourceProviderServer.DiffConfig(ctx, req)
}

func (p *configGrpcProviderServer) Create(
	ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	id := ""
	if !req.Preview {
		id = "id0"
	}
	return &pulumirpc.CreateResponse{
		Id: id,
		Properties: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"config": structpb.NewStringValue(p.formatLoggedMessages()),
			},
		},
	}, nil
}
