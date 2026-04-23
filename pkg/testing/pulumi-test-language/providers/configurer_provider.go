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
	"maps"
	"slices"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// ConfigurerProvider exercises remote component methods that return plain (non-Output) values,
// including plain primitives and plain resource references.
//
// The `Configurer` component accepts a `providerConfig` input, uses it to construct an internal
// provider instance (also from this package), and exposes methods:
//
//   - `plainValue()` returns a plain integer.
//   - `plainProvider()` returns a plain reference to the internal provider.
//   - `nestedPlainProvider()` returns a plain object containing a plain provider reference and a
//     plain integer.
//
// Consumers can use the provider returned from `plainProvider()` (or `nestedPlainProvider().provider`)
// to provision `Custom` resources. The provider echoes its `config` setting onto each resource it
// creates, so consumers can verify that component configuration propagated correctly to the
// constructed provider and through to child resources.
type ConfigurerProvider struct {
	plugin.UnimplementedProvider

	// config is captured from the provider's config in Configure and stamped onto every Custom
	// resource this provider creates, so callers can verify that config propagation worked.
	config string
}

var _ plugin.Provider = (*ConfigurerProvider)(nil)

const (
	configurerPkg     = "configurer"
	configurerVersion = "38.0.0"
)

func (p *ConfigurerProvider) Close() error                             { return nil }
func (p *ConfigurerProvider) SignalCancellation(context.Context) error { return nil }
func (p *ConfigurerProvider) Pkg() tokens.Package                      { return configurerPkg }

func (p *ConfigurerProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	v := semver.MustParse(configurerVersion)
	return plugin.PluginInfo{Version: &v}, nil
}

func (p *ConfigurerProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	primitive := func(t string) schema.PropertySpec {
		return schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: t}}
	}
	ref := func(t string) schema.PropertySpec {
		return schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "ref", Ref: t}}
	}

	resourceSpec := func(isComponent bool, description string,
		inputs, outputs map[string]schema.PropertySpec,
	) schema.ResourceSpec {
		return schema.ResourceSpec{
			IsComponent: isComponent,
			ObjectTypeSpec: schema.ObjectTypeSpec{
				Description: description,
				Type:        "object",
				Properties:  outputs,
				Required:    slices.Sorted(maps.Keys(outputs)),
			},
			InputProperties: inputs,
			RequiredInputs:  slices.Sorted(maps.Keys(inputs)),
		}
	}

	pkg := schema.PackageSpec{
		Name:      configurerPkg,
		Version:   configurerVersion,
		Functions: map[string]schema.FunctionSpec{},
		Resources: map[string]schema.ResourceSpec{},
		Provider: resourceSpec(false,
			"The configurer provider. Its `config` setting is echoed onto each Custom resource it creates.",
			map[string]schema.PropertySpec{"config": primitive("string")},
			map[string]schema.PropertySpec{"config": primitive("string")},
		),
	}

	pkg.Resources["configurer:index:Custom"] = resourceSpec(false,
		"A custom resource whose outputs echo its configured provider's `config` setting.",
		map[string]schema.PropertySpec{
			"value": primitive("string"),
		},
		map[string]schema.PropertySpec{
			"value":  primitive("string"),
			"config": primitive("string"),
		},
	)

	configurer := resourceSpec(true,
		"A component that internally constructs a Provider configured with `providerConfig` and exposes it via methods.",
		map[string]schema.PropertySpec{
			"providerConfig": primitive("string"),
		},
		map[string]schema.PropertySpec{
			"providerConfig": primitive("string"),
		},
	)
	configurer.Methods = map[string]string{
		"plainValue":          "configurer:index:Configurer/plainValue",
		"plainProvider":       "configurer:index:Configurer/plainProvider",
		"nestedPlainProvider": "configurer:index:Configurer/nestedPlainProvider",
	}
	pkg.Resources["configurer:index:Configurer"] = configurer

	selfRef := ref("#/resources/configurer:index:Configurer")

	pkg.Functions["configurer:index:Configurer/plainValue"] = schema.FunctionSpec{
		Description: "Returns a plain integer (42) as a single-value plain return.",
		Inputs: &schema.ObjectTypeSpec{
			Type:       "object",
			Properties: map[string]schema.PropertySpec{"__self__": selfRef},
			Required:   []string{"__self__"},
		},
		ReturnType: &schema.ReturnTypeSpec{
			TypeSpec: &schema.TypeSpec{Type: "integer", Plain: true},
		},
	}

	pkg.Functions["configurer:index:Configurer/plainProvider"] = schema.FunctionSpec{
		Description: "Returns the provider constructed by the component as a single-value plain return.",
		Inputs: &schema.ObjectTypeSpec{
			Type:       "object",
			Properties: map[string]schema.PropertySpec{"__self__": selfRef},
			Required:   []string{"__self__"},
		},
		ReturnType: &schema.ReturnTypeSpec{
			TypeSpec: &schema.TypeSpec{
				Type:  "ref",
				Ref:   "#/resources/pulumi:providers:configurer",
				Plain: true,
			},
		},
	}

	pkg.Functions["configurer:index:Configurer/nestedPlainProvider"] = schema.FunctionSpec{
		Description: "Returns a plain object containing a provider reference and an integer.",
		Inputs: &schema.ObjectTypeSpec{
			Type:       "object",
			Properties: map[string]schema.PropertySpec{"__self__": selfRef},
			Required:   []string{"__self__"},
		},
		ReturnType: &schema.ReturnTypeSpec{
			ObjectTypeSpec: &schema.ObjectTypeSpec{
				Type: "object",
				Properties: map[string]schema.PropertySpec{
					"provider": ref("#/resources/pulumi:providers:configurer"),
					"value":    primitive("integer"),
				},
				Required: []string{"provider", "value"},
			},
			ObjectTypeSpecIsPlain: true,
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	return plugin.GetSchemaResponse{Schema: jsonBytes}, nil
}

func (p *ConfigurerProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ConfigurerProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ConfigurerProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "missing version")}, nil
	}
	if !version.IsString() || version.StringValue() != configurerVersion {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "unexpected version"),
		}, nil
	}
	// Expect version and optionally config.
	if len(req.News) > 2 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ConfigurerProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ConfigurerProvider) Configure(
	_ context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	if cfg, ok := req.Inputs["config"]; ok && cfg.IsString() {
		p.config = cfg.StringValue()
	}
	return plugin.ConfigureResponse{}, nil
}

func (p *ConfigurerProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "configurer:index:Custom" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	value, ok := req.News["value"]
	if !ok || !value.IsString() {
		return plugin.CheckResponse{Failures: makeCheckFailure("value", "missing or non-string value")}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *ConfigurerProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

// Create handles Custom resources. Outputs include the provider's captured `config` so callers
// can verify configuration propagation from parent → component → constructed provider → resource.
func (p *ConfigurerProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	switch req.URN.Type() { //nolint:exhaustive //  Default covers the other case
	case "configurer:index:Custom":
		outs := resource.PropertyMap{
			"value":  req.Properties["value"],
			"config": resource.NewProperty(p.config),
		}
		id := "id-" + req.URN.Name()
		if req.Preview {
			id = ""
		}
		return plugin.CreateResponse{
			ID:         resource.ID(id),
			Properties: outs,
			Status:     resource.StatusOK,
		}, nil
	default:
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
}

// Construct handles the Configurer component. It registers the component, constructs an internal
// provider configured with the component's `providerConfig` input, and stores the provider
// reference on the component so that methods can return references to it.
func (p *ConfigurerProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "configurer:index:Configurer" {
		return plugin.ConstructResponse{}, fmt.Errorf("unknown type %v", req.Type)
	}

	conn, err := grpc.NewClient(
		req.Info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("connect to resource monitor: %w", err)
	}
	defer conn.Close()

	monitor := pulumirpc.NewResourceMonitorClient(conn)

	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "configurer:index:Configurer",
		Name:     req.Name,
		Provider: req.Options.Providers[configurerPkg],
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	providerConfig := req.Inputs["providerConfig"].StringValue()

	innerProv, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:    "pulumi:providers:configurer",
		Custom:  true,
		Name:    req.Name + "-inner-provider",
		Parent:  parent.Urn,
		Version: configurerVersion,
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"config": structpb.NewStringValue(providerConfig),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register inner provider: %w", err)
	}

	// Store the inner provider reference on the component so methods can retrieve it later.
	innerRef := resource.MakeCustomResourceReference(
		resource.URN(innerProv.Urn), resource.ID(innerProv.Id), "")
	innerRefStruct, err := plugin.MarshalPropertyValue(
		"innerProviderRef", innerRef,
		plugin.MarshalOptions{KeepResources: true, KeepSecrets: true})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal inner provider ref: %w", err)
	}

	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: parent.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"providerConfig":   structpb.NewStringValue(providerConfig),
				"innerProviderRef": innerRefStruct,
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(parent.Urn),
		Outputs: resource.PropertyMap{
			"providerConfig":   resource.NewProperty(providerConfig),
			"innerProviderRef": innerRef,
		},
	}, nil
}

// Call implements the three methods of Configurer. Each method hydrates __self__ to retrieve the
// inner provider reference that Construct stored, then returns the appropriate plain result.
func (p *ConfigurerProvider) Call(
	ctx context.Context, req plugin.CallRequest,
) (plugin.CallResponse, error) {
	conn, err := grpc.NewClient(
		req.Info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return plugin.CallResponse{}, fmt.Errorf("connect to resource monitor: %w", err)
	}
	defer conn.Close()
	monitor := pulumirpc.NewResourceMonitorClient(conn)

	selfRef := req.Args["__self__"].ResourceReferenceValue()
	self, err := monitor.Invoke(ctx, &pulumirpc.ResourceInvokeRequest{
		Tok: "pulumi:pulumi:getResource",
		Args: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"urn": structpb.NewStringValue(string(selfRef.URN)),
			},
		},
		AcceptResources: true,
	})
	if err != nil {
		return plugin.CallResponse{}, fmt.Errorf("hydrating __self__: %w", err)
	}

	innerRefValue, err := plugin.UnmarshalPropertyValue(
		"innerProviderRef",
		self.Return.Fields["state"].GetStructValue().Fields["innerProviderRef"],
		plugin.MarshalOptions{KeepResources: true, KeepSecrets: true})
	if err != nil {
		return plugin.CallResponse{}, fmt.Errorf("unmarshal inner provider ref: %w", err)
	}

	switch req.Tok {
	case "configurer:index:Configurer/plainValue":
		// Single-value plain returns use the magic key "res" on the wire.
		return plugin.CallResponse{
			Return: resource.PropertyMap{"res": resource.NewProperty(42.0)},
		}, nil
	case "configurer:index:Configurer/plainProvider":
		return plugin.CallResponse{
			Return: resource.PropertyMap{"res": *innerRefValue},
		}, nil
	case "configurer:index:Configurer/nestedPlainProvider":
		return plugin.CallResponse{
			Return: resource.PropertyMap{
				"provider": *innerRefValue,
				"value":    resource.NewProperty(42.0),
			},
		}, nil
	}
	return plugin.CallResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}
