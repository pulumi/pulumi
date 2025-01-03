// Copyright 2024, Pulumi Corporation.
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
//   - A simple custom resource type that is also created by this provider's
//     component resource types.
//   - Construct calls for creating component resources
//   - Component resources that deal with resource references and their hydration.
type ComponentProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*ComponentProvider)(nil)

func (p *ComponentProvider) Close() error {
	return nil
}

func (p *ComponentProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *ComponentProvider) Pkg() tokens.Package {
	return "component"
}

func (p *ComponentProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	version := semver.MustParse("13.3.7")
	info := workspace.PluginInfo{Version: &version}
	return info, nil
}

func (p *ComponentProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	primitiveType := func(t string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type: t,
			},
		}
	}

	refType := func(t string) schema.PropertySpec {
		return schema.PropertySpec{
			TypeSpec: schema.TypeSpec{
				Type: "ref",
				Ref:  t,
			},
		}
	}

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

	pkg := schema.PackageSpec{
		Name:      "component",
		Version:   "13.3.7",
		Resources: map[string]schema.ResourceSpec{},
	}

	pkg.Resources["component:index:Custom"] = customResource(
		"A custom resource with a single string input and output",
		map[string]schema.PropertySpec{
			"value": primitiveType("string"),
		},
		map[string]schema.PropertySpec{
			"value": primitiveType("string"),
		},
	)

	pkg.Resources["component:index:ComponentCustomRefOutput"] = componentResource(
		"A component resource that accepts an input that is used to create a child custom resource. "+
			"A reference to this child custom resource is returned.",
		map[string]schema.PropertySpec{
			"value": primitiveType("string"),
		},
		map[string]schema.PropertySpec{
			"value": primitiveType("string"),
			"ref":   refType("#/resources/component:index:Custom"),
		},
	)

	pkg.Resources["component:index:ComponentCustomRefInputOutput"] = componentResource(
		"A component resource that accepts a reference to a custom resource. "+
			"The input resource's `value` is used to create a child custom resource inside the component, "+
			"before a reference to this child is returned.",
		map[string]schema.PropertySpec{
			"inputRef": refType("#/resources/component:index:Custom"),
		},
		map[string]schema.PropertySpec{
			"inputRef":  refType("#/resources/component:index:Custom"),
			"outputRef": refType("#/resources/component:index:Custom"),
		},
	)

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	res := plugin.GetSchemaResponse{Schema: jsonBytes}
	return res, nil
}

func (p *ComponentProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *ComponentProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *ComponentProvider) CheckConfig(
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

	if version.StringValue() != "13.3.7" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 13.3.7"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *ComponentProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ComponentProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ComponentProvider) Check(
	_ context.Context,
	req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() == "component:index:Custom" {
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

func (p *ComponentProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *ComponentProvider) Create(
	_ context.Context,
	req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() == "component:index:Custom" {
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

func (p *ComponentProvider) Construct(
	ctx context.Context,
	req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "component:index:ComponentCustomRefInputOutput" &&
		req.Type != "component:index:ComponentCustomRefOutput" {
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

	if req.Type == "component:index:ComponentCustomRefOutput" {
		return p.constructComponentCustomRefOutput(ctx, req, monitor)
	}

	if req.Type == "component:index:ComponentCustomRefInputOutput" {
		return p.constructComponentCustomRefInputOutput(ctx, req, monitor)
	}

	return plugin.ConstructResponse{}, fmt.Errorf("unknown type %v", req.Type)
}

func (p *ComponentProvider) constructComponentCustomRefOutput(
	ctx context.Context,
	req plugin.ConstructRequest,
	monitor pulumirpc.ResourceMonitorClient,
) (plugin.ConstructResponse, error) {
	// Register the parent component.
	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: "component:index:ComponentCustomRefOutput",
		Name: req.Name,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	// Register the child resource, parented to the component we just created.
	child, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:    "component:index:Custom",
		Custom:  true,
		Name:    req.Name + "-child",
		Parent:  parent.Urn,
		Version: "13.3.7",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewStringValue(req.Inputs["value"].StringValue()),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child resource: %w", err)
	}

	// Create a resource reference to the child, that we'll register as an output of the component and return as part of
	// our ConstructResponse.
	refPropVal := resource.NewResourceReferenceProperty(resource.ResourceReference{
		URN: resource.URN(child.Urn),
		ID:  resource.NewStringProperty(child.Id),
	})
	refStruct, err := plugin.MarshalPropertyValue("ref", refPropVal, plugin.MarshalOptions{
		KeepResources: true,
		KeepSecrets:   true,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal ref: %w", err)
	}

	// Register the component's outputs and finish up.
	value := child.Object.Fields["value"].GetStringValue()
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: parent.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewStringValue(value),
				"ref":   refStruct,
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(parent.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
			"value": value,
			"ref":   refPropVal,
		}),
	}, nil
}

func (p *ComponentProvider) constructComponentCustomRefInputOutput(
	ctx context.Context,
	req plugin.ConstructRequest,
	monitor pulumirpc.ResourceMonitorClient,
) (plugin.ConstructResponse, error) {
	// Register the parent component.
	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: "component:index:ComponentCustomRefInputOutput",
		Name: req.Name,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	// Hydrate the input resource reference, whether it's a plain value or an output (that should be known and resolved).
	var inputRef resource.ResourceReference
	if req.Inputs["inputRef"].IsOutput() {
		inputRef = req.Inputs["inputRef"].OutputValue().Element.ResourceReferenceValue()
	} else {
		inputRef = req.Inputs["inputRef"].ResourceReferenceValue()
	}

	getRes, err := monitor.Invoke(ctx, &pulumirpc.ResourceInvokeRequest{
		Tok: "pulumi:pulumi:getResource",
		Args: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"urn": structpb.NewStringValue(string(inputRef.URN)),
			},
		},
		AcceptResources: true,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("hydrating input resource reference: %w", err)
	}

	// Register the child resource, parented to the component we just created.
	child, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:    "component:index:Custom",
		Custom:  true,
		Name:    req.Name + "-child",
		Parent:  parent.Urn,
		Version: "13.3.7",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": getRes.Return.Fields["state"].GetStructValue().Fields["value"],
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child resource: %w", err)
	}

	// Create resource references for the inputRef and outputRef component outputs.
	inputRefPropVal := resource.NewResourceReferenceProperty(inputRef)
	inputRefStruct, err := plugin.MarshalPropertyValue("inputRef", inputRefPropVal, plugin.MarshalOptions{
		KeepResources: true,
		KeepSecrets:   true,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal input ref: %w", err)
	}

	outputRefPropVal := resource.NewResourceReferenceProperty(resource.ResourceReference{
		URN: resource.URN(child.Urn),
		ID:  resource.NewStringProperty(child.Id),
	})
	outputRefStruct, err := plugin.MarshalPropertyValue("outputRef", outputRefPropVal, plugin.MarshalOptions{
		KeepResources: true,
		KeepSecrets:   true,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal output ref: %w", err)
	}

	// Register the component's outputs and finish up.
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: parent.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"inputRef":  inputRefStruct,
				"outputRef": outputRefStruct,
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(parent.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
			"inputRef":  inputRefPropVal,
			"outputRef": outputRefPropVal,
		}),
	}, nil
}
