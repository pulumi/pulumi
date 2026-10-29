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

// OutputValueInputDepsProvider tests that per-leaf OutputValue dependencies
// on nested resource inputs are preserved end-to-end:
//
//  1. The language SDK registers `comp` with input `refs = {x: a, y: b}`.
//     With the new RESOURCE_MONITOR_FEATURE_REGISTER_RESOURCE_OUTPUT_VALUES
//     flag the SDK sends refs.x as OV[a-URN] and refs.y as OV[b-URN]
//     instead of a flat propertyDependencies map that loses per-leaf info.
//  2. The engine passes the OutputValue-bearing inputs through to this
//     provider's Construct.
//  3. Construct registers a child Custom resource with:
//     foo = inputs.refs["x"]   (OV[a-URN])
//     bar = inputs.refs["y"]   (OV[b-URN])
//     with no propertyDependencies and the new per-request OV-inputs flag
//     set. The engine must harvest per-leaf deps from the OV tree.
//
// The sharp assertion is on the child's PropertyDependencies: it should be
// {foo: [a.URN], bar: [b.URN]} — NOT {foo: [a,b], bar: [a,b]} which is what
// the legacy upgradeOutputValues path produces by uniformly stamping the
// top-level propDeps set onto every leaf.
type OutputValueInputDepsProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*OutputValueInputDepsProvider)(nil)

func (p *OutputValueInputDepsProvider) Close() error                             { return nil }
func (p *OutputValueInputDepsProvider) SignalCancellation(context.Context) error { return nil }

func (p *OutputValueInputDepsProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	v := semver.MustParse("61.0.0")
	return plugin.PluginInfo{Version: &v}, nil
}

func (p *OutputValueInputDepsProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *OutputValueInputDepsProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *OutputValueInputDepsProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    "output-value-input-deps",
		Version: "61.0.0",
		Types: map[string]schema.ComplexTypeSpec{
			"output-value-input-deps:index:Refs": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"x": {TypeSpec: schema.TypeSpec{Type: "string"}},
						"y": {TypeSpec: schema.TypeSpec{Type: "string"}},
					},
					Required: []string{"x", "y"},
				},
			},
		},
		Resources: map[string]schema.ResourceSpec{
			"output-value-input-deps:index:Custom": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: map[string]schema.PropertySpec{"value": {TypeSpec: schema.TypeSpec{Type: "string"}}},
					Required:   []string{"value"},
				},
				InputProperties: map[string]schema.PropertySpec{"value": {TypeSpec: schema.TypeSpec{Type: "string"}}},
				RequiredInputs:  []string{"value"},
			},
			"output-value-input-deps:index:Child": {
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"},
				InputProperties: map[string]schema.PropertySpec{
					"foo": {TypeSpec: schema.TypeSpec{Type: "string"}},
					"bar": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
				RequiredInputs: []string{"foo", "bar"},
			},
			"output-value-input-deps:index:Component": {
				IsComponent:    true,
				ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"},
				InputProperties: map[string]schema.PropertySpec{
					"refs": {TypeSpec: schema.TypeSpec{
						Ref: "#/types/output-value-input-deps:index:Refs",
					}},
				},
				RequiredInputs: []string{"refs"},
			},
		},
	}

	b, err := json.Marshal(pkg)
	if err != nil {
		return plugin.GetSchemaResponse{}, err
	}
	return plugin.GetSchemaResponse{Schema: b}, nil
}

func (p *OutputValueInputDepsProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *OutputValueInputDepsProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *OutputValueInputDepsProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *OutputValueInputDepsProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *OutputValueInputDepsProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *OutputValueInputDepsProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	id := resource.ID("id-" + req.URN.Name())
	if req.Preview {
		id = ""
	}
	return plugin.CreateResponse{
		ID:         id,
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *OutputValueInputDepsProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Type != "output-value-input-deps:index:Component" {
		return plugin.ConstructResponse{}, fmt.Errorf("unknown type %v", req.Type)
	}

	conn, err := grpc.NewClient(
		req.Info.MonitorAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("connect monitor: %w", err)
	}
	defer conn.Close()
	monitor := pulumirpc.NewResourceMonitorClient(conn)

	supportsOV, err := monitor.SupportsFeature(ctx, &pulumirpc.SupportsFeatureRequest{
		Id: "registerResourceOutputValues",
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("probe feature: %w", err)
	}
	useOutputValues := supportsOV.GetHasSupport()

	parent, err := monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type: string(req.Type),
		Name: req.Name,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register component: %w", err)
	}

	// PCL sends refs.x and refs.y as per-leaf OutputValues carrying their own URN dependencies. Pull
	// each OutputValue out and extract its (element, deps) pair.
	legacyInputs := resource.ToResourcePropertyMap(req.Inputs)
	refs, ok := legacyInputs["refs"]
	if !ok || !refs.IsObject() {
		return plugin.ConstructResponse{}, fmt.Errorf("refs missing or not an object: %v", legacyInputs)
	}
	refsMap := refs.ObjectValue()
	leaf := func(key resource.PropertyKey) (resource.PropertyValue, []resource.URN, error) {
		v, ok := refsMap[key]
		if !ok {
			return resource.PropertyValue{}, nil, fmt.Errorf("refs missing %s: %v", key, refsMap)
		}
		if !v.IsOutput() {
			return resource.PropertyValue{}, nil, fmt.Errorf("refs.%s is not an OutputValue: %v", key, v)
		}
		out := v.OutputValue()
		return out.Element, out.Dependencies, nil
	}
	xVal, xDeps, err := leaf("x")
	if err != nil {
		return plugin.ConstructResponse{}, err
	}
	yVal, yDeps, err := leaf("y")
	if err != nil {
		return plugin.ConstructResponse{}, err
	}

	// Build the child register request. Two shapes, same observable outcome — the engine should accept
	// either wire format and record the same per-leaf propertyDependencies on the child:
	//
	//   useOutputValues=true  (new path): forward each OutputValue to the child with no
	//       propertyDependencies map and uses_output_values=true. The engine harvests from the OV tree.
	//
	//   useOutputValues=false (legacy path): strip OutputValues to their plain elements and send an
	//       explicit propertyDependencies map built from each OV's deps. The engine stores it verbatim.
	var childInputs resource.PropertyMap
	var propDeps map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies
	marshalOpts := plugin.MarshalOptions{KeepResources: true, KeepSecrets: true, KeepUnknowns: true}
	if useOutputValues {
		childInputs = resource.PropertyMap{
			"foo": resource.NewProperty(resource.Output{Element: xVal, Known: true, Dependencies: xDeps}),
			"bar": resource.NewProperty(resource.Output{Element: yVal, Known: true, Dependencies: yDeps}),
		}
		marshalOpts.KeepOutputValues = true
	} else {
		childInputs = resource.PropertyMap{"foo": xVal, "bar": yVal}
		urnStrs := func(urns []resource.URN) []string {
			s := make([]string, len(urns))
			for i, u := range urns {
				s[i] = string(u)
			}
			return s
		}
		propDeps = map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies{
			"foo": {Urns: urnStrs(xDeps)},
			"bar": {Urns: urnStrs(yDeps)},
		}
	}
	marshaled, err := plugin.MarshalProperties(childInputs, marshalOpts)
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("marshal child inputs: %w", err)
	}

	_, err = monitor.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
		Type:                 "output-value-input-deps:index:Child",
		Custom:               true,
		Name:                 req.Name + "-child",
		Parent:               parent.Urn,
		Version:              "61.0.0",
		Object:               marshaled,
		PropertyDependencies: propDeps,
		UsesOutputValues:     useOutputValues,
	})
	if err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register child: %w", err)
	}

	if _, err := monitor.RegisterResourceOutputs(ctx, &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     parent.Urn,
		Outputs: &structpb.Struct{Fields: map[string]*structpb.Value{}},
	}); err != nil {
		return plugin.ConstructResponse{}, fmt.Errorf("register outputs: %w", err)
	}

	return plugin.ConstructResponse{URN: resource.URN(parent.Urn)}, nil
}
