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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// PickCallProvider is a test-only provider for exercising OutputValues on Call. It exposes a Picker
// resource with a `getBoth(a, b)` method that returns `{wrapper: {a, b}}` with `wrapper` plain. The
// provider annotates `ReturnDependencies["wrapper"]` as the coarse union of a's and b's deps —
// ReturnDependencies cannot address nested leaves. Per-leaf precision only survives via OutputValues
// wrapping on `a` and `b` individually. The l2-call-per-value-deps test uses this to differentiate an
// OutputValues-aware caller from one that only reads ReturnDependencies.
type PickCallProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PickCallProvider)(nil)

func (p *PickCallProvider) Close() error { return nil }

func (p *PickCallProvider) Handshake(
	_ context.Context, req plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	// Only opt in when the caller advertised it — schema-loading tools and older engines call Handshake too
	// and need the negotiation to succeed. When both sides opt in the engine will hand us OutputValues in
	// Call args and honour OutputValues we return.
	return &plugin.ProviderHandshakeResponse{
		AcceptsOutputsInCall: req.AcceptsOutputsInCall,
		// Advertise AcceptSecrets so the engine's provider wrapper doesn't union-wrap every return
		// value in a Secret when any arg was secret — that would mask the per-value secretness we're
		// trying to exercise here.
		AcceptSecrets: true,
	}, nil
}

func (p *PickCallProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PickCallProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("56.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *PickCallProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	stringProp := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}
	refSelf := schema.PropertySpec{TypeSpec: schema.TypeSpec{
		Type: "ref",
		Ref:  "#/resources/pick-call:index:Picker",
	}}
	picker := schema.ResourceSpec{
		ObjectTypeSpec: schema.ObjectTypeSpec{
			Type:       "object",
			Properties: map[string]schema.PropertySpec{"value": stringProp},
			Required:   []string{"value"},
		},
		InputProperties: map[string]schema.PropertySpec{"value": stringProp},
		RequiredInputs:  []string{"value"},
		Methods: map[string]string{
			"getBoth": "pick-call:index:Picker/getBoth",
		},
	}
	// BothWrapper is a named object type with non-plain string props `a` and `b`. It is returned from
	// `getBoth` wrapped as a *plain* field so codegen SDKs surface `a` and `b` individually as
	// `Output<string>` rather than wrapping the whole object as `Output<BothWrapper>`. This matters
	// for the per-leaf vs coarse-top-level-key differential test.
	bothWrapperPlain := schema.PropertySpec{TypeSpec: schema.TypeSpec{
		Type:  "ref",
		Ref:   "#/types/pick-call:index:BothWrapper",
		Plain: true,
	}}
	pkg := schema.PackageSpec{
		Name:    "pick-call",
		Version: "56.0.0",
		Resources: map[string]schema.ResourceSpec{
			"pick-call:index:Picker": picker,
		},
		Types: map[string]schema.ComplexTypeSpec{
			"pick-call:index:BothWrapper": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"a": stringProp,
						"b": stringProp,
					},
					Required: []string{"a", "b"},
				},
			},
		},
		Functions: map[string]schema.FunctionSpec{
			// getBoth returns `{wrapper: {a, b}}` where `wrapper` is a *plain* named-object field and
			// `a`, `b` are non-plain strings. ReturnDependencies can only annotate at the top-level
			// return key `wrapper` (coarse union of a's and b's deps) — the OutputValue wrapping on
			// `a` and `b` individually carries per-leaf deps. See the test for the differential.
			"pick-call:index:Picker/getBoth": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"__self__": refSelf,
						"a":        stringProp,
						"b":        stringProp,
					},
					Required: []string{"__self__", "a", "b"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type:       "object",
						Properties: map[string]schema.PropertySpec{"wrapper": bothWrapperPlain},
						Required:   []string{"wrapper"},
					},
				},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *PickCallProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PickCallProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PickCallProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	if req.URN.Type() != "pick-call:index:Picker" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}
	return plugin.CheckResponse{Properties: req.NewInputs}, nil
}

func (p *PickCallProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PickCallProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	if req.URN.Type() != "pick-call:index:Picker" {
		return plugin.CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}
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

func (p *PickCallProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *PickCallProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *PickCallProvider) Call(
	_ context.Context, req plugin.CallRequest,
) (plugin.CallResponse, error) {
	switch req.Tok {
	case "pick-call:index:Picker/getBoth":
		a, aOk := req.Args.GetOk("a")
		b, bOk := req.Args.GetOk("b")
		if !aOk {
			return plugin.CallResponse{Failures: makeCheckFailure("a", "missing a")}, nil
		}
		if !bOk {
			return plugin.CallResponse{Failures: makeCheckFailure("b", "missing b")}, nil
		}
		// Union of the two args' deps, preserving order and dedup. This is the coarse top-level
		// annotation — ReturnDependencies cannot address nested leaves like `wrapper.a`. In the
		// OutputValues path per-leaf deps still ride on `a` and `b` themselves, so a downstream
		// consumer of only `wrapper.a` will depend only on a's dep set. In the legacy path PCL falls
		// back to this coarse union attached to the whole `wrapper`.
		aDeps := req.Options.ArgDependencies["a"]
		bDeps := req.Options.ArgDependencies["b"]
		union := append([]resource.URN(nil), aDeps...)
		seen := map[resource.URN]bool{}
		for _, d := range aDeps {
			seen[d] = true
		}
		for _, d := range bDeps {
			if !seen[d] {
				seen[d] = true
				union = append(union, d)
			}
		}
		// Return `a` and `b` verbatim so any OutputValue wrapping is preserved on the leaves.
		wrapper := property.NewMap(map[string]property.Value{"a": a, "b": b})
		return plugin.CallResponse{
			Return: property.NewMap(map[string]property.Value{"wrapper": property.New(wrapper)}),
			ReturnDependencies: map[resource.PropertyKey][]resource.URN{
				"wrapper": union,
			},
		}, nil
	default:
		return plugin.CallResponse{
			Failures: makeCheckFailure("", "unknown function "+string(req.Tok)),
		}, nil
	}
}
