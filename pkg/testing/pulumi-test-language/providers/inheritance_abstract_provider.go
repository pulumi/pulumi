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

// InheritanceAbstractProvider is the "inheritabstract" package: an abstract base (AbstractBase) and a concrete
// component that extends it (ConcreteChild). It exercises the two abstract-specific rules:
//
//   - A direct Construct of the abstract type is rejected with the pinned host error (the host check is the source of
//     truth, since a language guard can be bypassed).
//   - ConstructBase of the abstract type is always permitted: constructing ConcreteChild base-constructs AbstractBase.
type InheritanceAbstractProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*InheritanceAbstractProvider)(nil)

const inheritAbstractVersion = "1.0.0"

func (p *InheritanceAbstractProvider) Close() error { return nil }

func (p *InheritanceAbstractProvider) SignalCancellation(context.Context) error { return nil }

func (p *InheritanceAbstractProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.MustParse(inheritAbstractVersion)
	return plugin.PluginInfo{Version: &version}, nil
}

func (p *InheritanceAbstractProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	str := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}
	pkg := schema.PackageSpec{
		Name:    "inheritabstract",
		Version: inheritAbstractVersion,
		Resources: map[string]schema.ResourceSpec{
			"inheritabstract:index:Custom": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "A custom resource created as a child by the abstract base.",
					Type:        "object",
					Properties:  map[string]schema.PropertySpec{"value": str},
					Required:    []string{"value"},
				},
				InputProperties: map[string]schema.PropertySpec{"value": str},
				RequiredInputs:  []string{"value"},
			},
			"inheritabstract:index:AbstractBase": {
				IsComponent: true,
				Abstract:    true,
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "An abstract base component that cannot be instantiated directly.",
					Type:        "object",
					Properties:  map[string]schema.PropertySpec{"abstractOutput": str},
					Required:    []string{"abstractOutput"},
				},
				InputProperties: map[string]schema.PropertySpec{"seed": str},
				RequiredInputs:  []string{"seed"},
			},
			"inheritabstract:index:ConcreteChild": {
				IsComponent: true,
				Extends:     &schema.TypeSpec{Ref: "#/resources/inheritabstract:index:AbstractBase"},
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "A concrete component that extends AbstractBase.",
					Type:        "object",
					Properties:  map[string]schema.PropertySpec{"abstractOutput": str, "concreteOutput": str},
					Required:    []string{"abstractOutput", "concreteOutput"},
				},
				InputProperties: map[string]schema.PropertySpec{"seed": str, "extra": str},
				RequiredInputs:  []string{"seed", "extra"},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *InheritanceAbstractProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *InheritanceAbstractProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceAbstractProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *InheritanceAbstractProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *InheritanceAbstractProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceAbstractProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "inheritabstract:index:Custom" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid custom resource type: %s", req.URN.Type())
	}
	id := "id-" + req.Properties["value"].StringValue()
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{ID: resource.ID(id), Properties: req.Properties, Status: resource.StatusOK}, nil
}

func (p *InheritanceAbstractProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	// The host check is the source of truth: even if a language-level guard is bypassed, a direct Construct of an
	// abstract type must fail with this pinned error. This is checked before connecting to the monitor, since no
	// resource is ever registered for a rejected construction.
	if req.Type == "inheritabstract:index:AbstractBase" {
		return plugin.ConstructResponse{},
			fmt.Errorf("type '%s' is abstract and cannot be instantiated directly", req.Type)
	}

	monitor, conn, err := dialMonitor(req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructResponse{}, err
	}
	defer conn.Close()

	switch req.Type { //nolint:exhaustive // only this package's component types are handled
	case "inheritabstract:index:ConcreteChild":
		parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:     string(req.Type),
			Name:     req.Name,
			Parent:   string(req.Parent),
			Provider: req.Options.Providers["inheritabstract"],
		})
		if err != nil {
			return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
		}

		baseInputs, err := plugin.MarshalProperties(
			resource.PropertyMap{"seed": req.Inputs["seed"]},
			plugin.MarshalOptions{KeepResources: true, KeepSecrets: true})
		if err != nil {
			return plugin.ConstructResponse{}, fmt.Errorf("marshal base inputs: %w", err)
		}
		// ConstructBase of an abstract type is always permitted.
		baseResp, err := monitor.ConstructBaseResource(ctx, &pulumirpc.ConstructBaseResourceRequest{
			Urn:       parent.Urn,
			BaseType:  "inheritabstract:index:AbstractBase",
			Inputs:    baseInputs,
			Version:   inheritAbstractVersion,
			Providers: req.Options.Providers,
		})
		if err != nil {
			return plugin.ConstructResponse{}, fmt.Errorf("construct abstract base: %w", err)
		}

		outputs := map[string]*structpb.Value{
			"abstractOutput": baseResp.State.Fields["abstractOutput"],
			"concreteOutput": structpb.NewStringValue("concrete-" + req.Inputs["extra"].StringValue()),
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
	default:
		return plugin.ConstructResponse{}, fmt.Errorf("unknown component type %q", req.Type)
	}
}

func (p *InheritanceAbstractProvider) ConstructBase(
	ctx context.Context, req plugin.ConstructBaseRequest,
) (plugin.ConstructBaseResponse, error) {
	if req.Type != "inheritabstract:index:AbstractBase" {
		return plugin.ConstructBaseResponse{}, fmt.Errorf("unknown base component type %q", req.Type)
	}
	monitor, conn, err := dialMonitor(req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructBaseResponse{}, err
	}
	defer conn.Close()

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "inheritabstract:index:Custom",
		Custom:   true,
		Name:     req.Name + "-abstract-child",
		Parent:   string(req.URN),
		Version:  inheritAbstractVersion,
		Provider: req.Providers["inheritabstract"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{"value": structpb.NewStringValue("abstract")},
		},
	})
	if err != nil {
		return plugin.ConstructBaseResponse{}, fmt.Errorf("register abstract child: %w", err)
	}
	return plugin.ConstructBaseResponse{
		Outputs: resource.PropertyMap{
			"abstractOutput": resource.NewProperty("abstract-" + req.Inputs["seed"].StringValue()),
		},
	}, nil
}
