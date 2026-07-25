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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"
)

// InheritanceBaseProvider is the "inheritbase" package of the two-package cross-package inheritance chain. It exposes a
// concrete base component that a component in a different package (see InheritanceDerivedProvider) extends, exercising
// engine base-construct dispatch across two distinct provider instances — the closest conformance gets to a genuine
// cross-language inheritance chain.
type InheritanceBaseProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*InheritanceBaseProvider)(nil)

const inheritBaseVersion = "1.0.0"

func (p *InheritanceBaseProvider) Close() error { return nil }

func (p *InheritanceBaseProvider) SignalCancellation(context.Context) error { return nil }

func (p *InheritanceBaseProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.MustParse(inheritBaseVersion)
	return plugin.PluginInfo{Version: &version}, nil
}

func (p *InheritanceBaseProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	str := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}
	pkg := schema.PackageSpec{
		Name:    "inheritbase",
		Version: inheritBaseVersion,
		Resources: map[string]schema.ResourceSpec{
			"inheritbase:index:Custom": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "A custom resource created as a child by the base component.",
					Type:        "object",
					Properties:  map[string]schema.PropertySpec{"value": str},
					Required:    []string{"value"},
				},
				InputProperties: map[string]schema.PropertySpec{"value": str},
				RequiredInputs:  []string{"value"},
			},
			"inheritbase:index:BaseComponent": {
				IsComponent: true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "A base component intended to be extended from another package.",
					Type:        "object",
					Properties:  map[string]schema.PropertySpec{"baseOutput": str},
					Required:    []string{"baseOutput"},
				},
				InputProperties: map[string]schema.PropertySpec{"message": str},
				RequiredInputs:  []string{"message"},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *InheritanceBaseProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *InheritanceBaseProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceBaseProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *InheritanceBaseProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *InheritanceBaseProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceBaseProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "inheritbase:index:Custom" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid custom resource type: %s", req.URN.Type())
	}
	id := "id-" + req.Properties["value"].StringValue()
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{ID: resource.ID(id), Properties: req.Properties, Status: resource.StatusOK}, nil
}

func (p *InheritanceBaseProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "inheritbase:index:BaseComponent" {
		return plugin.ConstructResponse{}, fmt.Errorf("unknown component type %q", req.Type)
	}
	monitor, conn, err := dialMonitor(req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructResponse{}, err
	}
	defer conn.Close()

	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     string(req.Type),
		Name:     req.Name,
		Parent:   string(req.Parent),
		Provider: req.Options.Providers["inheritbase"],
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	if err := p.registerChild(ctx, req, monitor, resource.URN(parent.Urn)); err != nil {
		return plugin.ConstructResponse{}, err
	}

	output := "base-" + req.Inputs["message"].StringValue()
	if _, err := monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     parent.Urn,
		Outputs: &structpb.Struct{Fields: map[string]*structpb.Value{"baseOutput": structpb.NewStringValue(output)}},
	}); err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}
	return plugin.ConstructResponse{
		URN:     resource.URN(parent.Urn),
		Outputs: resource.NewPropertyMapFromMap(map[string]any{"baseOutput": output}),
	}, nil
}

func (p *InheritanceBaseProvider) ConstructBase(
	ctx context.Context, req plugin.ConstructBaseRequest,
) (plugin.ConstructBaseResponse, error) {
	if req.Type != "inheritbase:index:BaseComponent" {
		return plugin.ConstructBaseResponse{}, fmt.Errorf("unknown base component type %q", req.Type)
	}
	monitor, conn, err := dialMonitor(req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructBaseResponse{}, err
	}
	defer conn.Close()

	creq := plugin.ConstructRequest{Name: req.Name, Options: plugin.ConstructOptions{Providers: req.Providers}}
	if err := p.registerChild(ctx, creq, monitor, req.URN); err != nil {
		return plugin.ConstructBaseResponse{}, err
	}
	return plugin.ConstructBaseResponse{
		Outputs: resource.PropertyMap{
			"baseOutput": resource.NewProperty("base-" + req.Inputs["message"].StringValue()),
		},
	}, nil
}

// registerChild registers the base's custom child parented directly to the URN, with no level-specific naming.
func (p *InheritanceBaseProvider) registerChild(
	ctx context.Context,
	req plugin.ConstructRequest,
	monitor pulumirpc.ResourceMonitorClient,
	parent resource.URN,
) error {
	_, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "inheritbase:index:Custom",
		Custom:   true,
		Name:     req.Name + "-base-child",
		Parent:   string(parent),
		Version:  inheritBaseVersion,
		Provider: req.Options.Providers["inheritbase"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{"value": structpb.NewStringValue("base")},
		},
	})
	if err != nil {
		return fmt.Errorf("register base child: %w", err)
	}
	return nil
}
