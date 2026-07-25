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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// InheritanceProvider is an in-process component provider that exercises the whole component-inheritance stack from a
// single package. It declares a concrete inheritance pair (Base <- Derived), a custom child that base levels register,
// and one method (getStatus) inherited by Derived. The abstract case lives in its own package (see
// InheritanceAbstractProvider) so that a concrete-only package generates and compiles cleanly.
//
// The schema is published in the FLATTENED form: each derived resource carries its full transitive member set in
// inputProperties/properties, with `extends` as an additive annotation. Its Construct implementation is the
// provider-side of the inheritance flow: a derived type registers itself once under its most-derived token and then
// issues ConstructBaseResource for its immediate base; a base type adopts the passed URN, parents a child to it, and
// returns its level's outputs.
type InheritanceProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*InheritanceProvider)(nil)

const inheritanceVersion = "1.0.0"

func (p *InheritanceProvider) Close() error { return nil }

func (p *InheritanceProvider) SignalCancellation(context.Context) error { return nil }

func (p *InheritanceProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	version := semver.MustParse(inheritanceVersion)
	return plugin.PluginInfo{Version: &version}, nil
}

func (p *InheritanceProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	str := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}
	integer := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "integer"}}
	extendsRef := func(token string) *schema.TypeSpec {
		return &schema.TypeSpec{Ref: "#/resources/" + token}
	}

	pkg := schema.PackageSpec{
		Name:      "inherit",
		Version:   inheritanceVersion,
		Functions: map[string]schema.FunctionSpec{},
		Resources: map[string]schema.ResourceSpec{},
	}

	// A custom (non-component) resource that base levels register as a child so the snapshot has a real node parented
	// to the adopted URN.
	pkg.Resources["inherit:index:Custom"] = schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: "A custom resource created as a child by base component levels.",
			Type:        "object",
			Properties:  map[string]schema.PropertySpec{"value": str},
			Required:    []string{"value"},
		},
		InputProperties: map[string]schema.PropertySpec{"value": str},
		RequiredInputs:  []string{"value"},
	}

	// Base: a concrete, directly-constructable component with a single method.
	pkg.Resources["inherit:index:Base"] = schema.ResourceSpec{
		IsComponent: true,
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: "A base component exposing a message input and a computed output.",
			Type:        "object",
			Properties:  map[string]schema.PropertySpec{"baseOutput": str},
			Required:    []string{"baseOutput"},
		},
		InputProperties: map[string]schema.PropertySpec{"message": str},
		RequiredInputs:  []string{"message"},
		Methods:         map[string]string{"getStatus": "inherit:index:Base/getStatus"},
	}
	pkg.Functions["inherit:index:Base/getStatus"] = schema.FunctionSpec{
		Description: "Returns the status of the (possibly derived) component. Inherited, non-overridden, so a Derived " +
			"instance dispatches this call on the base-owned token.",
		Inputs: &schema.ObjectTypeSpec{
			Type: "object",
			Properties: map[string]schema.PropertySpec{
				"__self__": {TypeSpec: schema.TypeSpec{Ref: "#/resources/inherit:index:Base"}},
			},
			Required: []string{"__self__"},
		},
		ReturnType: &schema.ReturnTypeSpec{
			ObjectTypeSpec: &schema.ObjectTypeSpec{
				Type:       "object",
				Properties: map[string]schema.PropertySpec{"status": str},
				Required:   []string{"status"},
			},
		},
	}

	// Derived extends Base. The published spec is flattened: it carries Base's members plus its own.
	pkg.Resources["inherit:index:Derived"] = schema.ResourceSpec{
		IsComponent: true,
		Extends:     extendsRef("inherit:index:Base"),
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Description: "A component that extends Base, adding its own input and output.",
			Type:        "object",
			Properties:  map[string]schema.PropertySpec{"baseOutput": str, "derivedOutput": str},
			Required:    []string{"baseOutput", "derivedOutput"},
		},
		InputProperties: map[string]schema.PropertySpec{"message": str, "scale": integer},
		RequiredInputs:  []string{"message", "scale"},
	}

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	return plugin.GetSchemaResponse{Schema: jsonBytes}, nil
}

func (p *InheritanceProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *InheritanceProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *InheritanceProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *InheritanceProvider) Diff(context.Context, plugin.DiffRequest) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *InheritanceProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "inherit:index:Custom" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid custom resource type: %s", req.URN.Type())
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

// dialMonitor connects to the resource monitor identified in a construct request's info. The caller owns the returned
// connection and must close it.
func dialMonitor(address string) (pulumirpc.ResourceMonitorClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to resource monitor: %w", err)
	}
	return pulumirpc.NewResourceMonitorClient(conn), conn, nil
}

func (p *InheritanceProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	monitor, conn, err := dialMonitor(req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructResponse{}, err
	}
	defer conn.Close()

	switch req.Type { //nolint:exhaustive // only this package's component types are handled
	case "inherit:index:Base":
		// Base is directly constructable; a direct construction has no derived level above it.
		return p.constructConcrete(ctx, req, monitor, "", nil)
	case "inherit:index:Derived":
		return p.constructConcrete(ctx, req, monitor, "inherit:index:Base",
			resource.PropertyMap{"message": req.Inputs["message"]})
	default:
		return plugin.ConstructResponse{}, fmt.Errorf("unknown component type %q", req.Type)
	}
}

// constructConcrete performs the single most-derived registration for a concrete component and, when it has a base,
// issues ConstructBaseResource for that base and folds the base's outputs into its own. baseType is empty for a
// directly-constructed base component (Base itself), which registers its own child inline.
func (p *InheritanceProvider) constructConcrete(
	ctx context.Context,
	req plugin.ConstructRequest,
	monitor pulumirpc.ResourceMonitorClient,
	baseType string,
	baseInputs resource.PropertyMap,
) (plugin.ConstructResponse, error) {
	// The single registration, carrying the most-derived token.
	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     string(req.Type),
		Name:     req.Name,
		Parent:   string(req.Parent),
		Provider: req.Options.Providers["inherit"],
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	outputs := map[string]*structpb.Value{}

	switch req.Type { //nolint:exhaustive // only this package's component types are handled
	case "inherit:index:Base":
		// A directly constructed Base registers its own child and computes its own output.
		if err := p.registerBaseChild(ctx, req, monitor, resource.URN(parent.Urn), "base"); err != nil {
			return plugin.ConstructResponse{}, err
		}
		outputs["baseOutput"] = structpb.NewStringValue("base-" + req.Inputs["message"].StringValue())
	case "inherit:index:Derived":
		baseState, err := p.constructBaseResource(ctx, monitor, req, parent.Urn, baseType, baseInputs)
		if err != nil {
			return plugin.ConstructResponse{}, err
		}
		outputs["baseOutput"] = baseState.Fields["baseOutput"]
		outputs["derivedOutput"] = structpb.NewStringValue(
			fmt.Sprintf("derived-%d", int(req.Inputs["scale"].NumberValue())))
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

// constructBaseResource issues the ConstructBaseResource monitor call that constructs the portion of the resource
// corresponding to its immediate base. The base is served in the same package (version pinning selects this provider).
func (p *InheritanceProvider) constructBaseResource(
	ctx context.Context,
	monitor pulumirpc.ResourceMonitorClient,
	req plugin.ConstructRequest,
	urn, baseType string,
	baseInputs resource.PropertyMap,
) (*structpb.Struct, error) {
	inputs, err := plugin.MarshalProperties(baseInputs, plugin.MarshalOptions{KeepResources: true, KeepSecrets: true})
	if err != nil {
		return nil, fmt.Errorf("marshal base inputs: %w", err)
	}
	resp, err := monitor.ConstructBaseResource(ctx, &pulumirpc.ConstructBaseResourceRequest{
		Urn:       urn,
		BaseType:  baseType,
		Inputs:    inputs,
		Version:   inheritanceVersion,
		Providers: req.Options.Providers,
	})
	if err != nil {
		return nil, fmt.Errorf("construct base %q: %w", baseType, err)
	}
	return resp.State, nil
}

func (p *InheritanceProvider) ConstructBase(
	ctx context.Context, req plugin.ConstructBaseRequest,
) (plugin.ConstructBaseResponse, error) {
	monitor, conn, err := dialMonitor(req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructBaseResponse{}, err
	}
	defer conn.Close()

	// Base levels adopt the passed URN: they register no new resource, only children parented to it.
	creq := plugin.ConstructRequest{Name: req.Name, Options: plugin.ConstructOptions{Providers: req.Providers}}

	switch req.Type { //nolint:exhaustive // only this package's base component types are handled
	case "inherit:index:Base":
		if err := p.registerBaseChild(ctx, creq, monitor, req.URN, "base"); err != nil {
			return plugin.ConstructBaseResponse{}, err
		}
		return plugin.ConstructBaseResponse{
			Outputs: resource.PropertyMap{
				"baseOutput": resource.NewProperty("base-" + req.Inputs["message"].StringValue()),
			},
		}, nil
	default:
		return plugin.ConstructBaseResponse{}, fmt.Errorf("unknown base component type %q", req.Type)
	}
}

// registerBaseChild registers a custom child parented directly to the adopted URN, with no level-specific naming: its
// URN derives from the parent exactly as if the most-derived class had registered it.
func (p *InheritanceProvider) registerBaseChild(
	ctx context.Context,
	req plugin.ConstructRequest,
	monitor pulumirpc.ResourceMonitorClient,
	parent resource.URN,
	label string,
) error {
	_, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:     "inherit:index:Custom",
		Custom:   true,
		Name:     req.Name + "-" + label + "-child",
		Parent:   string(parent),
		Version:  inheritanceVersion,
		Provider: req.Options.Providers["inherit"],
		Object: &structpb.Struct{
			Fields: map[string]*structpb.Value{"value": structpb.NewStringValue(label)},
		},
	})
	if err != nil {
		return fmt.Errorf("register %s child: %w", label, err)
	}
	return nil
}

func (p *InheritanceProvider) Call(
	ctx context.Context, req plugin.CallRequest,
) (plugin.CallResponse, error) {
	if req.Tok != "inherit:index:Base/getStatus" {
		return plugin.CallResponse{}, fmt.Errorf("unknown method %q", req.Tok)
	}
	// getStatus is inherited and non-overridden, so a call on a Derived receiver dispatches on the base-owned token.
	// Echoing the token that arrived lets the conformance assertion verify exactly that.
	return plugin.CallResponse{
		Return: resource.NewPropertyMapFromMap(map[string]any{"status": string(req.Tok)}),
	}, nil
}
