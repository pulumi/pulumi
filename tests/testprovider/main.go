// Copyright 2016-2023, Pulumi Corporation.
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
//go:build !all
// +build !all

// A provider with resources for use in tests.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	providerName = "testprovider"
	version      = "0.0.1"
)

var providerSchema = pschema.PackageSpec{
	Name:        "testprovider",
	Description: "A test provider.",
	DisplayName: "testprovider",

	Config: pschema.ConfigSpec{},

	Provider: pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "The provider type for the testprovider package.",
			Type:        "object",
		},
		InputProperties: map[string]pschema.PropertySpec{},
	},

	Types:     map[string]pschema.ComplexTypeSpec{},
	Resources: map[string]pschema.ResourceSpec{},
	Functions: map[string]pschema.FunctionSpec{},
	Language:  map[string]pschema.RawMessage{},
}

// Minimal set of methods to implement a basic provider.
type testProvider interface {
	Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error)
	Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error)
	Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error)
	Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error)
	Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error)
	Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error)
	Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error)
	Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error)
}

var testProviders = func() map[string]testProvider {
	ep := &echoProvider{}

	testProviders := map[string]testProvider{
		"testprovider:index:Random":            &randomProvider{},
		"testprovider:index:Echo":              ep,
		"testprovider:index:Echo/doEchoMethod": ep,
		"testprovider:index:doEcho":            ep,
		"testprovider:index:doMultiEcho":       ep,
		"testprovider:index:FailsOnDelete":     &failsOnDeleteProvider{},
		"testprovider:index:FailsOnCreate":     &failsOnCreateProvider{},
		"testprovider:index:Named":             &namedProvider{},
	}
	return testProviders
}()

func providerForURN(urn string) (testProvider, string, bool) {
	ty := string(resource.URN(urn).Type())
	provider, ok := testProviders[ty]
	return provider, ty, ok
}

//nolint:unused,deadcode
func main() {
	if err := provider.Main(providerName, func(host *provider.HostClient) (rpc.ResourceProviderServer, error) {
		return makeProvider(host, providerName, version)
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type testproviderProvider struct {
	rpc.UnimplementedResourceProviderServer

	parameter string

	host    *provider.HostClient
	name    string
	version string
}

func makeProvider(host *provider.HostClient, name, version string) (rpc.ResourceProviderServer, error) {
	// Return the new provider
	return &testproviderProvider{
		host:    host,
		name:    name,
		version: version,
	}, nil
}

// CheckConfig validates the configuration for this provider.
func (p *testproviderProvider) CheckConfig(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.GetNews()}, nil
}

// DiffConfig diffs the configuration for this provider.
func (p *testproviderProvider) DiffConfig(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return &rpc.DiffResponse{}, nil
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *testproviderProvider) Configure(_ context.Context, req *rpc.ConfigureRequest) (*rpc.ConfigureResponse, error) {
	return &rpc.ConfigureResponse{
		AcceptSecrets:                   true,
		SupportsAutonamingConfiguration: true,
	}, nil
}

func (p *testproviderProvider) Parameterize(_ context.Context, req *rpc.ParameterizeRequest) (*rpc.ParameterizeResponse, error) {
	switch params := req.GetParameters().(type) {
	case *rpc.ParameterizeRequest_Args:
		args := params.Args.Args
		if len(args) != 1 {
			return nil, fmt.Errorf("expected exactly one argument")
		}
		p.parameter = args[0]
	case *rpc.ParameterizeRequest_Value:
		val := string(params.Value.Value)
		if val == "" {
			return nil, fmt.Errorf("expected a non-empty string value")
		}
		p.parameter = val
	default:
		return nil, fmt.Errorf("unexpected parameter type")
	}

	for k, prov := range testProviders {
		testProviders[strings.Replace(k, "testprovider", p.parameter, 1)] = prov
	}

	return &rpc.ParameterizeResponse{
		Name:    p.parameter,
		Version: version,
	}, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (p *testproviderProvider) Invoke(_ context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	if p, ok := testProviders[req.GetTok()]; ok {
		return p.Invoke(context.Background(), req)
	}

	tok := req.GetTok()
	if tok == "testprovider:index:returnArgs" {
		return &rpc.InvokeResponse{
			Return: req.Args,
		}, nil
	}
	return nil, fmt.Errorf("Unknown Invoke token '%s'", tok)
}

func (p *testproviderProvider) Call(_ context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	tok := req.GetTok()

	if p, ok := testProviders[tok]; ok {
		return p.Call(context.Background(), req)
	}

	return nil, fmt.Errorf("Unknown Call token '%s'", tok)
}

func (p *testproviderProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	provider, ty, ok := providerForURN(req.GetUrn())
	if !ok {
		return nil, fmt.Errorf("Unknown resource type '%s'", ty)
	}
	return provider.Check(ctx, req)
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *testproviderProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	provider, ty, ok := providerForURN(req.GetUrn())
	if !ok {
		return nil, fmt.Errorf("Unknown resource type '%s'", ty)
	}
	return provider.Diff(ctx, req)
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.
func (p *testproviderProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	provider, ty, ok := providerForURN(req.GetUrn())
	if !ok {
		return nil, fmt.Errorf("Unknown resource type '%s'", ty)
	}
	return provider.Create(ctx, req)
}

// Read the current live state associated with a resource.
func (p *testproviderProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	provider, ty, ok := providerForURN(req.GetUrn())
	if !ok {
		return nil, fmt.Errorf("Unknown resource type '%s'", ty)
	}
	return provider.Read(ctx, req)
}

// Update updates an existing resource with new values.
func (p *testproviderProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	provider, ty, ok := providerForURN(req.GetUrn())
	if !ok {
		return nil, fmt.Errorf("Unknown resource type '%s'", ty)
	}
	return provider.Update(ctx, req)
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed
// to still exist.
func (p *testproviderProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	provider, ty, ok := providerForURN(req.GetUrn())
	if !ok {
		return nil, fmt.Errorf("Unknown resource type '%s'", ty)
	}
	return provider.Delete(ctx, req)
}

// Construct creates a new component resource.
func (p *testproviderProvider) Construct(ctx context.Context, req *rpc.ConstructRequest) (*rpc.ConstructResponse, error) {
	if req.Type != "testprovider:index:Component" {
		return nil, fmt.Errorf("unknown resource type %s", req.Type)
	}

	return pulumiprovider.Construct(
		ctx, req, p.host.EngineConn(),
		func(ctx *pulumi.Context, typ, name string, inputs pulumiprovider.ConstructInputs,
			options pulumi.ResourceOption,
		) (*pulumiprovider.ConstructResult, error) {
			args := &ComponentArgs{}
			if err := inputs.CopyTo(args); err != nil {
				return nil, fmt.Errorf("setting args: %w", err)
			}

			component, err := NewComponent(ctx, name, args, options)
			if err != nil {
				return nil, err
			}

			return pulumiprovider.NewConstructResult(component)
		})
}

// GetPluginInfo returns generic information about this plugin, like its version.
func (p *testproviderProvider) GetPluginInfo(context.Context, *emptypb.Empty) (*rpc.PluginInfo, error) {
	return &rpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *testproviderProvider) Attach(ctx context.Context, req *rpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// GetSchema returns the JSON-serialized schema for the provider.
func (p *testproviderProvider) GetSchema(ctx context.Context,
	req *rpc.GetSchemaRequest,
) (*rpc.GetSchemaResponse, error) {
	makeJSONString := func(v any) ([]byte, error) {
		var out bytes.Buffer
		encoder := json.NewEncoder(&out)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(v); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	}

	sch := providerSchema
	// If we have a parameter, we'll return a copy of the provider's resources and
	// functions -- this is just enough to test that the engine is calling
	// Parameterize and GetSchema correctly.
	if req.SubpackageName != "" {
		if req.SubpackageName == p.parameter {
			sch = pschema.PackageSpec{
				Name:    p.parameter,
				Version: "1.0.0",
				Parameterization: &pschema.ParameterizationSpec{
					BaseProvider: pschema.BaseProviderSpec{
						Name:    "testprovider",
						Version: version,
					},
					Parameter: []byte(p.parameter),
				},
				Resources: map[string]pschema.ResourceSpec{},
				Functions: map[string]pschema.FunctionSpec{},
			}

			for k, r := range providerSchema.Resources {
				sch.Resources[strings.Replace(k, "testprovider", p.parameter, 1)] = r
				for k, m := range r.Methods {
					r.Methods[k] = strings.Replace(m, "testprovider", p.parameter, 1)
				}
			}
			for k, f := range providerSchema.Functions {
				sch.Functions[strings.Replace(k, "testprovider", p.parameter, 1)] = f
				for k, prop := range f.Inputs.Properties {
					if prop.TypeSpec.Ref != "" {
						prop.TypeSpec.Ref = strings.Replace(prop.TypeSpec.Ref, "testprovider", p.parameter, 1)
						f.Inputs.Properties[k] = prop
					}
				}
			}
		} else {
			return nil, fmt.Errorf("expected subpackage %s", req.SubpackageName)
		}
	}

	schemaJSON, err := makeJSONString(sch)
	if err != nil {
		return nil, err
	}
	return &rpc.GetSchemaResponse{
		Schema: string(schemaJSON),
	}, nil
}

// Cancel signals the provider to gracefully shut down and abort any ongoing resource operations.
// Operations aborted in this way will return an error (e.g., `Update` and `Create` will either a
// creation error or an initialization error). Since Cancel is advisory and non-blocking, it is up
// to the host to decide how long to wait after Cancel is called before (e.g.)
// hard-closing any gRPC connection.
func (p *testproviderProvider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *testproviderProvider) GetMapping(context.Context, *rpc.GetMappingRequest) (*rpc.GetMappingResponse, error) {
	return &rpc.GetMappingResponse{}, nil
}
