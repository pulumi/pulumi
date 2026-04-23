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

// A provider for testing component resources that accept plain inputs and return
// both plain outputs and references to child custom resources.
type PlainComponentProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PlainComponentProvider)(nil)

func (p *PlainComponentProvider) Close() error {
	return nil
}

func (p *PlainComponentProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *PlainComponentProvider) Pkg() tokens.Package {
	return "plaincomponent"
}

func (p *PlainComponentProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.Version{Major: 36}
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *PlainComponentProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "plaincomponent",
		Version: "36.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"plaincomponent:index:Settings": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"tags": {
							TypeSpec: schema.TypeSpec{
								Type: "object",
								AdditionalProperties: &schema.TypeSpec{
									Type:  "string",
									Plain: true,
								},
								Plain: true,
							},
						},
						"enabled": {
							TypeSpec: schema.TypeSpec{
								Type:  "boolean",
								Plain: true,
							},
						},
					},
					Required: []string{"enabled", "tags"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"plaincomponent:index:Custom": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"value": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"value"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"value": {
						TypeSpec: schema.TypeSpec{Type: "string"},
					},
				},
				RequiredInputs: []string{"value"},
			},
			"plaincomponent:index:Component": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"label": {
							TypeSpec: schema.TypeSpec{Type: "string"},
						},
					},
					Required: []string{"label"},
				},
				InputProperties: map[string]schema.PropertySpec{
					"name": {
						TypeSpec: schema.TypeSpec{
							Type:  "string",
							Plain: true,
						},
					},
					"settings": {
						TypeSpec: schema.TypeSpec{
							Type:  "ref",
							Ref:   "#/types/plaincomponent:index:Settings",
							Plain: true,
						},
					},
				},
				RequiredInputs: []string{"name", "settings"},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *PlainComponentProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PlainComponentProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PlainComponentProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
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
	if version.StringValue() != "36.0.0" {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("version", "version is not 36.0.0"),
		}, nil
	}
	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PlainComponentProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PlainComponentProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PlainComponentProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "plaincomponent:index:Custom" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

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

func (p *PlainComponentProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PlainComponentProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "plaincomponent:index:Custom" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

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

func (p *PlainComponentProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}

func (p *PlainComponentProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "plaincomponent:index:Component" {
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

	// Register the parent component.
	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "plaincomponent:index:Component",
		Name:     req.Name,
		Provider: req.Options.Providers["plaincomponent"],
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	// Build the label from the plain inputs.
	name := req.Inputs["name"].StringValue()
	settings := req.Inputs["settings"].ObjectValue()
	enabled := settings["enabled"].BoolValue()
	label := name
	if !enabled {
		label = name + " (disabled)"
	}

	// Register a child custom resource.
	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "plaincomponent:index:Custom",
		Custom:   true,
		Name:     req.Name + "-child",
		Parent:   parent.Urn,
		Version:  "36.0.0",
		Provider: req.Options.Providers["plaincomponent"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewStringValue(label),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child resource: %w", err)
	}

	// Register outputs.
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: parent.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"label": structpb.NewStringValue(label),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(parent.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]any{
			"label": label,
		}),
	}, nil
}
