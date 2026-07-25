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

// InheritanceDerivedProvider is the "inheritderived" package of the two-package cross-package inheritance chain. Its
// DerivedComponent extends BaseComponent from the "inheritbase" package (a cross-package `$ref` in the flattened
// schema). Its Construct registers the single most-derived resource and then issues ConstructBaseResource for the
// base type owned by the other package, causing the engine to dispatch ConstructBase to that separate provider
// instance.
type InheritanceDerivedProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*InheritanceDerivedProvider)(nil)

const inheritDerivedVersion = "1.0.0"

func (p *InheritanceDerivedProvider) Close() error { return nil }

func (p *InheritanceDerivedProvider) SignalCancellation(context.Context) error { return nil }

func (p *InheritanceDerivedProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.MustParse(inheritDerivedVersion)
	return plugin.PluginInfo{Version: &version}, nil
}

func (p *InheritanceDerivedProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	str := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}
	integer := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "integer"}}
	baseVersion := semver.MustParse(inheritBaseVersion)

	pkg := schema.PackageSpec{
		Name:    "inheritderived",
		Version: inheritDerivedVersion,
		Dependencies: []schema.PackageDescriptor{
			{Name: "inheritbase", Version: &baseVersion},
		},
		Resources: map[string]schema.ResourceSpec{
			"inheritderived:index:DerivedComponent": {
				IsComponent: true,
				// Cross-package extends: the base lives in the inheritbase package.
				Extends: &schema.TypeSpec{
					Ref: fmt.Sprintf("/inheritbase/v%s/schema.json#/resources/inheritbase:index:BaseComponent",
						inheritBaseVersion),
				},
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "A component in one package that extends a base component in another package.",
					Type:        "object",
					// Flattened: carries the base's baseOutput plus its own derivedOutput.
					Properties: map[string]schema.PropertySpec{"baseOutput": str, "derivedOutput": str},
					Required:   []string{"baseOutput", "derivedOutput"},
				},
				// Flattened: carries the base's message plus its own scale.
				InputProperties: map[string]schema.PropertySpec{"message": str, "scale": integer},
				RequiredInputs:  []string{"message", "scale"},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *InheritanceDerivedProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *InheritanceDerivedProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceDerivedProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *InheritanceDerivedProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *InheritanceDerivedProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceDerivedProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "inheritderived:index:DerivedComponent" {
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
		Provider: req.Options.Providers["inheritderived"],
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	// Construct the base portion owned by the inheritbase package. The version selects that package's provider.
	baseInputs, err := plugin.MarshalProperties(
		resource.PropertyMap{"message": req.Inputs["message"]},
		plugin.MarshalOptions{KeepResources: true, KeepSecrets: true})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal base inputs: %w", err)
	}
	baseResp, err := monitor.ConstructBaseResource(ctx, &pulumirpc.ConstructBaseResourceRequest{
		Urn:       parent.Urn,
		BaseType:  "inheritbase:index:BaseComponent",
		Inputs:    baseInputs,
		Version:   inheritBaseVersion,
		Providers: req.Options.Providers,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("construct base: %w", err)
	}

	outputs := map[string]*structpb.Value{
		"baseOutput":    baseResp.State.Fields["baseOutput"],
		"derivedOutput": structpb.NewStringValue(fmt.Sprintf("derived-%d", int(req.Inputs["scale"].NumberValue()))),
	}
	if _, err := monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     parent.Urn,
		Outputs: &structpb.Struct{Fields: outputs},
	}); err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register resource outputs: %w", err)
	}

	unmarshaled, err := plugin.UnmarshalProperties(
		&structpb.Struct{Fields: outputs}, plugin.MarshalOptions{KeepResources: true, KeepSecrets: true})
	if err != nil {
		return plugin.ConstructResponse{}, err
	}
	return plugin.ConstructResponse{URN: resource.URN(parent.Urn), Outputs: unmarshaled}, nil
}
