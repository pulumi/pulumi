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

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type ConformanceComponentProvider struct {
	plugin.UnimplementedProvider
	Version *semver.Version
}

var _ plugin.Provider = (*ConformanceComponentProvider)(nil)

func (p *ConformanceComponentProvider) Close() error {
	return nil
}

func (p *ConformanceComponentProvider) Pkg() tokens.Package {
	return "conformance-component"
}

func (p *ConformanceComponentProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *ConformanceComponentProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.Version{Major: 22}
	if p.Version != nil {
		version = *p.Version
	}
	return plugin.PluginInfo{Version: &version}, nil
}

func (p *ConformanceComponentProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	version := semver.Version{Major: 22}
	if p.Version != nil {
		version = *p.Version
	}
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "conformance-component",
		Version: version.String(),
		Resources: map[string]schema.ResourceSpec{
			"conformance-component:index:Simple": {
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

func marshalReplacementTrigger(pv resource.PropertyValue) *structpb.Value {
	if pv.IsNull() {
		return nil
	}
	v, err := plugin.MarshalPropertyValue("replacementTrigger", pv, plugin.MarshalOptions{})
	if err != nil {
		return nil
	}
	return v
}

func marshalInputs(inputs resource.PropertyMap) *structpb.Struct {
	if len(inputs) == 0 {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	s, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{})
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return s
}

func (p *ConformanceComponentProvider) Construct(
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

	if req.Type != "conformance-component:index:Simple" {
		return plugin.ConstructResponse{}, fmt.Errorf("unknown type %v", req.Type)
	}

	// Register the parent component. Include Object so the engine can build the goal's
	// properties, and ensure ReplaceOnChanges is a non-nil slice for proper serialization.
	replaceOnChanges := req.Options.ReplaceOnChanges
	if replaceOnChanges == nil {
		replaceOnChanges = []string{}
	}
	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:               "conformance-component:index:Simple",
		Name:               req.Name,
		Provider:           req.Options.Providers["conformance-component"],
		Object:             marshalInputs(req.Inputs),
		IgnoreChanges:      req.Options.IgnoreChanges,
		ReplaceOnChanges:   replaceOnChanges,
		ReplacementTrigger: marshalReplacementTrigger(req.Options.ReplacementTrigger),
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register parent component: %w", err)
	}

	// Register a child resource, parented to the component we just created.
	// Use "res-child" when the component is named "res" (provider_resource_component,
	// provider_alias_component, provider_replacement_trigger_component) so those tests
	// can RequireSingleNamedResource(l, resources, "res-child"). Otherwise use a unique
	// name per component to avoid duplicate URNs (e.g. provider_version_component,
	// provider_ignore_changes_component).
	childName := "res-child"
	if req.Name != "res" {
		childName = req.Name + "-res-child"
	}
	child, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "simple:index:Resource",
		Custom:   true,
		Name:     childName,
		Parent:   parent.Urn,
		Provider: req.Options.Providers["simple"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(req.Inputs["value"].BoolValue()),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child resource: %w", err)
	}

	// Register the component's outputs and finish up.
	value := child.Object.Fields["value"].GetBoolValue()
	_, err = monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn: parent.Urn,
		Outputs: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"value": structpb.NewBoolValue(value),
			},
		},
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	return plugin.ConstructResponse{
		URN: resource.URN(parent.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]any{
			"value": value,
		}),
	}, nil
}
