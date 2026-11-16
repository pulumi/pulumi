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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// A provider for testing remote components whose outputs depend on a child resource that fails to create.
type FailingComponentProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*FailingComponentProvider)(nil)

func (p *FailingComponentProvider) Close() error {
	return nil
}

func (p *FailingComponentProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *FailingComponentProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.Version{Major: 59}
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *FailingComponentProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	valueProperties := map[string]schema.PropertySpec{
		"value": {TypeSpec: schema.TypeSpec{Type: "string"}},
	}
	pkg := schema.PackageSpec{
		Name:    "failingcomponent",
		Version: "59.0.0",
		Resources: map[string]schema.ResourceSpec{
			"failingcomponent:index:Custom": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: valueProperties,
					Required:   []string{"value"},
				},
				InputProperties: valueProperties,
				RequiredInputs:  []string{"value"},
			},
			"failingcomponent:index:Component": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: valueProperties,
					Required:   []string{"value"},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *FailingComponentProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *FailingComponentProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *FailingComponentProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	version, ok := req.News.GetOk("version")
	if !ok {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "missing version")}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not a string")}, nil
	}
	if version.AsString() != "59.0.0" {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not 59.0.0")}, nil
	}
	if req.News.Len() != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *FailingComponentProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *FailingComponentProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *FailingComponentProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "failingcomponent:index:Custom" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *FailingComponentProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *FailingComponentProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.Preview {
		return plugin.CreateResponse{Properties: req.Properties, Status: resource.StatusOK}, nil
	}
	return plugin.CreateResponse{}, errors.New("failed create")
}

func (p *FailingComponentProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *FailingComponentProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "failingcomponent:index:Component" {
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
		Type:     "failingcomponent:index:Component",
		Name:     req.Name,
		Provider: req.Options.Providers["failingcomponent"],
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	child, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:                    "failingcomponent:index:Custom",
		Custom:                  true,
		Name:                    req.Name + "-child",
		Parent:                  parent.Urn,
		Version:                 "59.0.0",
		Provider:                req.Options.Providers["failingcomponent"],
		SupportsResultReporting: true,
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewStringValue("hello"),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child resource: %w", err)
	}

	// The component's value comes from its child, which is unknown if the child failed.
	value := property.New(property.Computed)
	if child.GetResult() == pulumirpc.Result_SUCCESS {
		value = property.New(child.GetObject().GetFields()["value"].GetStringValue())
	}

	outputs := property.NewMap(map[string]property.Value{"value": value})
	rpcOutputs, err := plugin.MarshalProperties(resource.ToResourcePropertyMap(outputs), plugin.MarshalOptions{
		KeepUnknowns: true,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal outputs: %w", err)
	}
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     parent.Urn,
		Outputs: rpcOutputs,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN:     resource.URN(parent.Urn),
		Outputs: outputs,
		OutputDependencies: map[resource.PropertyKey][]resource.URN{
			"value": {resource.URN(child.Urn)},
		},
	}, nil
}
