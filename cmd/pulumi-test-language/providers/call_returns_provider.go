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
	"maps"
	"slices"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// Copy of ComponentProvider but the return types are scalars instead of objects.
type CallReturnsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*CallReturnsProvider)(nil)

func (p *CallReturnsProvider) Close() error {
	return nil
}

func (p *CallReturnsProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *CallReturnsProvider) Pkg() tokens.Package {
	return "callreturnsprovider"
}

func (p *CallReturnsProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	version := semver.MustParse("18.0.0")
	info := workspace.PluginInfo{Version: &version}
	return info, nil
}

func (p *CallReturnsProvider) GetSchema(
	context.Context,
	plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
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
			requiredInputs := slices.Sorted(maps.Keys(inputs))
			requiredOutputs := slices.Sorted(maps.Keys(outputs))

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

	componentResource := resource(true)

	pkg := schema.PackageSpec{
		Name:      "callreturnsprovider",
		Version:   "18.0.0",
		Functions: map[string]schema.FunctionSpec{},
		Resources: map[string]schema.ResourceSpec{},
		Config:    schema.ConfigSpec{},
	}

	callableResource := componentResource(
		"A component resource that has callable methods.",
		map[string]schema.PropertySpec{},
		map[string]schema.PropertySpec{},
	)

	stringReturnType := &schema.ReturnTypeSpec{
		TypeSpec: &schema.TypeSpec{
			Type: "string",
		},
	}
	pkg.Functions["callreturnsprovider:index:ComponentCallable/identity"] = schema.FunctionSpec{
		Description: "The `identity` method of the `ComponentCallable` component resource. " +
			"Returns the component's `value` unaltered.",
		Inputs: &schema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]schema.PropertySpec{
				"__self__": refType("#/resources/callreturnsprovider:index:ComponentCallable"),
				"value":    primitiveType("string"),
			},
			Required: []string{"__self__"},
		},
		ReturnType: stringReturnType,
	}
	callableResource.Methods = map[string]string{
		"identity": "callreturnsprovider:index:ComponentCallable/identity",
	}

	pkg.Resources["callreturnsprovider:index:ComponentCallable"] = callableResource

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}

	res := plugin.GetSchemaResponse{Schema: jsonBytes}
	return res, nil
}

func (p *CallReturnsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *CallReturnsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *CallReturnsProvider) CheckConfig(
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

	if version.StringValue() != "18.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 18.0.0"),
		}, nil
	}

	if len(req.News) > 2 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *CallReturnsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *CallReturnsProvider) Configure(
	_ context.Context,
	req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *CallReturnsProvider) Check(
	_ context.Context,
	req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{
		Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
	}, nil
}

func (p *CallReturnsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *CallReturnsProvider) Create(
	_ context.Context,
	req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{Status: resource.StatusUnknown}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
}

func (p *CallReturnsProvider) Construct(
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

	if req.Type == "callreturnsprovider:index:ComponentCallable" {
		return p.constructComponentCallable(ctx, req, monitor)
	}

	return plugin.ConstructResponse{}, fmt.Errorf("unknown type %v", req.Type)
}

func (p *CallReturnsProvider) constructComponentCallable(
	ctx context.Context,
	req plugin.ConstructRequest,
	monitor pulumirpc.ResourceMonitorClient,
) (plugin.ConstructResponse, error) {
	comp, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: "callreturnsprovider:index:ComponentCallable",
		Name: req.Name,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(comp.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
			"value": structpb.NewStringValue(""),
		}),
	}, nil
}

func (p *CallReturnsProvider) Call(
	ctx context.Context,
	req plugin.CallRequest,
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
	switch req.Tok {
	case "callreturnsprovider:index:ComponentCallable/identity":
		return p.callComponentCallableIdentity(ctx, req, monitor)
	}

	return plugin.CallResponse{}, fmt.Errorf("unknown function %v", req.Tok)
}

func (p *CallReturnsProvider) callComponentCallableIdentity(
	ctx context.Context,
	req plugin.CallRequest,
	monitor pulumirpc.ResourceMonitorClient,
) (plugin.CallResponse, error) {
	result := req.Args["value"]

	return plugin.CallResponse{
		Return: resource.NewPropertyMapFromMap(map[string]interface{}{
			"value": result,
		}),
	}, nil
}
