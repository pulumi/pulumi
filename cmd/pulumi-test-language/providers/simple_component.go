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

package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blang/semver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type SimpleComponentProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SimpleComponentProvider)(nil)

func (p *SimpleComponentProvider) Close() error {
	return nil
}

func (p *SimpleComponentProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SimpleComponentProvider) Pkg() tokens.Package {
	return "simple-component"
}

func (p *SimpleComponentProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "simple-component",
		Version: "15.0.0",
		Resources: map[string]schema.ResourceSpec{
			"simple-component:index:Resource": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SimpleComponentProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
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
	if version.StringValue() != "15.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 15.0.0"),
		}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SimpleComponentProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{}, errors.New("Check not implemented")
}

func (p *SimpleComponentProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{}, errors.New("Create not implemented")
}

func (p *SimpleComponentProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("15.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SimpleComponentProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SimpleComponentProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SimpleComponentProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SimpleComponentProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleComponentProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleComponentProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *SimpleComponentProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	// Type should be of the form "simpleComponent:index:Resource"
	if req.Type != "simple-component:index:Resource" {
		return plugin.ConstructResponse{}, fmt.Errorf("invalid URN type: %s", req.Type)
	}

	// Connect to the monitor and register the component and a simple resource.
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

	// Register the component.
	res, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: "simple-component:index:Resource",
		Name: req.Name,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	// Register a simple resource.
	value := req.Inputs["value"].BoolValue()

	inner, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:    "simple:index:Resource",
		Custom:  true,
		Name:    req.Name + "-simple",
		Parent:  res.Urn,
		Version: "2.0.0",
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(value),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource: %w", err)
	}

	// Register the resource outputs
	value = inner.Object.Fields["value"].GetBoolValue()
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: res.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(value),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register output: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(res.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]interface{}{
			"value": value,
		}),
	}, nil
}
