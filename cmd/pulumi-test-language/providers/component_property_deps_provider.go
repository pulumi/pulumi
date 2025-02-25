// Copyright 2025, Pulumi Corporation.
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
	"slices"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// A (component) provider for testing remote component resources (also known as
// "multi-language components", or MLCs). Supports:
//
//  - A simple custom resource type
//  - A simple component resource that returns the property dependencies from
//    the construct request as an output.
//  - A method on the component that similarly returns the arg dependencies
//    from the call request as the result.

type ComponentPropertyDepsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ComponentPropertyDepsProvider)(nil)

func (p *ComponentPropertyDepsProvider) Close() error {
	return nil
}

func (p *ComponentPropertyDepsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ComponentPropertyDepsProvider) Pkg() tokens.Package {
	return "component-property-deps"
}

func (p *ComponentPropertyDepsProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	version := semver.MustParse("1.33.7")
	info := workspace.PluginInfo{Version: &version}
	return info, nil
}

func (p *ComponentPropertyDepsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resource := func(isComponent bool) func(
		description string,
		inputs map[string]schema.PropertySpec,
		outputs map[string]schema.PropertySpec,
	) schema.ResourceSpec {
		return func(
			description string,
			inputs map[string]schema.PropertySpec,
			outputs map[string]schema.PropertySpec,
		) schema.ResourceSpec {
			requiredInputs := maps.Keys(inputs)
			slices.Sort(requiredInputs)

			requiredOutputs := maps.Keys(outputs)
			slices.Sort(requiredOutputs)

			return schema.ResourceSpec{
				IsComponent: isComponent,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: description,
					Type:        "object",
					Properties:  outputs,
					Required:    requiredOutputs,
				},
				InputProperties: inputs,
				RequiredInputs:  requiredInputs,
			}
		}
	}

	customResource := resource(false)
	componentResource := resource(true)

	function := func(
		description string,
		inputs map[string]schema.PropertySpec,
		returnType schema.ReturnTypeSpec,
	) schema.FunctionSpec {
		requiredInputs := maps.Keys(inputs)
		slices.Sort(requiredInputs)

		return schema.FunctionSpec{
			Description: description,
			Inputs: &schema.ObjectTypeSpec{
				Type:       "object",
				Properties: inputs,
				Required:   requiredInputs,
			},
			ReturnType: &returnType,
		}
	}

	pkg := schema.PackageSpec{
		Name:      "component-property-deps",
		Version:   "1.33.7",
		Functions: map[string]schema.FunctionSpec{},
		Resources: map[string]schema.ResourceSpec{},
		Types:     map[string]schema.ComplexTypeSpec{},
	}

	pkg.Resources["component-property-deps:index:Custom"] = customResource(
		"A custom resource with a single string input and output",
		map[string]schema.PropertySpec{
			"value": {
				TypeSpec: schema.TypeSpec{
					Type: "string",
				},
			},
		},
		map[string]schema.PropertySpec{
			"value": {
				TypeSpec: schema.TypeSpec{
					Type: "string",
				},
			},
		},
	)

	properties := func() map[string]schema.PropertySpec {
		return map[string]schema.PropertySpec{
			"resource": {
				TypeSpec: schema.TypeSpec{
					Plain: true,
					Ref:   "#/resources/component-property-deps:index:Custom",
				},
			},
			"resourceList": {
				TypeSpec: schema.TypeSpec{
					Plain: true,
					Type:  "array",
					Items: &schema.TypeSpec{
						Plain: true,
						Ref:   "#/resources/component-property-deps:index:Custom",
					},
				},
			},
			"resourceMap": {
				TypeSpec: schema.TypeSpec{
					Plain: true,
					Type:  "object",
					AdditionalProperties: &schema.TypeSpec{
						Plain: true,
						Ref:   "#/resources/component-property-deps:index:Custom",
					},
				},
			},
		}
	}

	component := componentResource(
		"A component resource that accepts a list of resources. "+
			"The construct request's property dependencies are returned as an output.",
		properties(),
		map[string]schema.PropertySpec{
			"propertyDeps": {
				TypeSpec: schema.TypeSpec{
					Type: "object",
					AdditionalProperties: &schema.TypeSpec{
						Type:  "array",
						Items: &schema.TypeSpec{Type: "string"},
					},
				},
			},
		},
	)

	funcProps := properties()
	funcProps["__self__"] = schema.PropertySpec{
		TypeSpec: schema.TypeSpec{
			Ref: "#/resources/component-property-deps:index:Component",
		},
	}

	pkg.Functions["component-property-deps:index:Component/refs"] = function(
		"The `refs` method of the `Component` component resource. "+
			"Returns the call request's property dependencies.",
		funcProps,
		schema.ReturnTypeSpec{
			ObjectTypeSpec: &schema.ObjectTypeSpec{
				Type: "object",
				Properties: map[string]schema.PropertySpec{
					"result": {
						TypeSpec: schema.TypeSpec{
							Type: "object",
							AdditionalProperties: &schema.TypeSpec{
								Type:  "array",
								Items: &schema.TypeSpec{Type: "string"},
							},
						},
					},
				},
				Required: []string{"result"},
			},
		},
	)
	component.Methods = map[string]string{
		"refs": "component-property-deps:index:Component/refs",
	}

	pkg.Resources["component-property-deps:index:Component"] = component

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	res := plugin.GetSchemaResponse{Schema: jsonBytes}
	return res, nil
}

func (p *ComponentPropertyDepsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ComponentPropertyDepsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ComponentPropertyDepsProvider) CheckConfig(
	_ context.Context,
	req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "missing version"),
		}, nil
	}

	if !version.IsString() {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not a string"),
		}, nil
	}

	if version.StringValue() != "1.33.7" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 1.33.7"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ComponentPropertyDepsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ComponentPropertyDepsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ComponentPropertyDepsProvider) Check(
	_ context.Context,
	req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() == "component-property-deps:index:Custom" {
		value, ok := req.News["value"]
		if !ok {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("value", "missing value"),
			}, nil
		}

		if !value.IsString() {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("value", "value is not a string"),
			}, nil
		}

		if len(req.News) != 1 {
			return plugin.CheckResponse{
				Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
			}, nil
		}

		return plugin.CheckResponse{Properties: req.News}, nil
	}

	return plugin.CheckResponse{
		Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
	}, nil
}

func (p *ComponentPropertyDepsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ComponentPropertyDepsProvider) Create(
	_ context.Context,
	req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() == "component-property-deps:index:Custom" {
		id := "id-" + req.Properties["value"].StringValue()
		if req.Preview {
			id = ""
		}

		return plugin.CreateResponse{
			ID:         resource.ID(id),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	return plugin.CreateResponse{Status: resource.StatusUnknown}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}

func (p *ComponentPropertyDepsProvider) Construct(
	ctx context.Context,
	req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
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

	if req.Type != "component-property-deps:index:Component" {
		return plugin.ConstructResponse{}, fmt.Errorf("unknown type %v", req.Type)
	}

	// Register the parent component.
	component, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: "component-property-deps:index:Component",
		Name: req.Name,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	propertyDeps, err := p.convertMapToStruct(req.Options.PropertyDependencies)
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("convert property dependencies: %w", err)
	}

	// Register the component's outputs and finish up.
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: component.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"propertyDeps": structpb.NewStructValue(propertyDeps),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(component.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
			"propertyDeps": p.convertMapToObjectProperty(req.Options.PropertyDependencies),
		}),
	}, nil
}

func (p *ComponentPropertyDepsProvider) Call(
	ctx context.Context,
	req plugin.CallRequest,
) (plugin.CallResponse, error) {
	if req.Tok != "component-property-deps:index:Component/refs" {
		return plugin.CallResponse{}, fmt.Errorf("unknown function %v", req.Tok)
	}

	return plugin.CallResponse{
		Return: resource.PropertyMap{
			"result": p.convertMapToObjectProperty(req.Options.ArgDependencies),
		},
	}, nil
}

func (p *ComponentPropertyDepsProvider) convertMapToObjectProperty(
	m map[resource.PropertyKey][]resource.URN,
) resource.PropertyValue {
	fields := make(map[string]any)
	for key, urns := range m {
		fields[string(key)] = urns
	}
	return resource.NewObjectProperty(resource.NewPropertyMapFromMap(fields))
}

func (p *ComponentPropertyDepsProvider) convertMapToStruct(
	m map[resource.PropertyKey][]resource.URN,
) (*structpb.Struct, error) {
	fields := make(map[string]*structpb.Value)

	for key, urns := range m {
		// Convert []resource.URN to []any
		anyURNs := make([]any, len(urns))
		for i, urn := range urns {
			anyURNs[i] = string(urn)
		}

		// Create ListValue from the interface slice
		listValue, err := structpb.NewList(anyURNs)
		if err != nil {
			return nil, err
		}

		// Add to fields map using the string key
		fields[string(key)] = structpb.NewListValue(listValue)
	}

	return &structpb.Struct{
		Fields: fields,
	}, nil
}
