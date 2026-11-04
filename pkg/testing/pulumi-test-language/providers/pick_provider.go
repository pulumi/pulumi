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

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// PickProvider is a test-only provider that opts into OutputValues on Invoke and returns one of its inputs
// unchanged, preserving that input's per-value dependency information. It exists to exercise the
// engine/provider negotiation of `accepts_outputs_in_invoke`: a downstream resource that consumes the return
// should depend on exactly the resources that fed the picked argument, not the union of every arg.
type PickProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*PickProvider)(nil)

func (p *PickProvider) Close() error { return nil }

func (p *PickProvider) Handshake(
	_ context.Context, req plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	// Only opt in when the caller advertised it — schema-loading tools and older engines call Handshake too
	// and need the negotiation to succeed. When both sides opt in the engine will hand us OutputValues in
	// Invoke args and honour OutputValues we return.
	return &plugin.ProviderHandshakeResponse{
		AcceptsOutputsInInvoke: req.AcceptsOutputsInInvoke,
		// Advertise AcceptSecrets so the engine's provider wrapper doesn't union-wrap every return
		// value in a Secret when any arg was secret — that would mask the per-value secretness we're
		// trying to exercise here.
		AcceptSecrets: true,
	}, nil
}

func (p *PickProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *PickProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	ver := semver.MustParse("55.0.0")
	return plugin.PluginInfo{Version: &ver}, nil
}

func (p *PickProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	stringProp := schema.PropertySpec{TypeSpec: schema.TypeSpec{Type: "string"}}
	pkg := schema.PackageSpec{
		Name:    "pick",
		Version: "55.0.0",
		Functions: map[string]schema.FunctionSpec{
			// pickSecond returns `second` unchanged, discarding `first`. When the caller sends OutputValues
			// the return value carries only `second`'s dependencies.
			"pick:index:pickSecond": {
				Inputs: &schema.ObjectTypeSpec{
					Type: "object",
					Properties: map[string]schema.PropertySpec{
						"first":  stringProp,
						"second": stringProp,
					},
					Required: []string{"first", "second"},
				},
				ReturnType: &schema.ReturnTypeSpec{
					ObjectTypeSpec: &schema.ObjectTypeSpec{
						Type:       "object",
						Properties: map[string]schema.PropertySpec{"result": stringProp},
						Required:   []string{"result"},
					},
				},
			},
		},
	}
	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *PickProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *PickProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *PickProvider) Invoke(
	_ context.Context, req plugin.InvokeRequest,
) (plugin.InvokeResponse, error) {
	if req.Tok != "pick:index:pickSecond" {
		return plugin.InvokeResponse{
			Failures: makeCheckFailure("", "unknown function "+string(req.Tok)),
		}, nil
	}
	second, ok := req.Args.GetOk("second")
	if !ok {
		return plugin.InvokeResponse{Failures: makeCheckFailure("second", "missing second")}, nil
	}
	// Return `second` verbatim — including any dependencies carried on it. The engine and downstream SDK see
	// only `second`'s per-value deps, not `first`'s.
	return plugin.InvokeResponse{
		Properties: property.NewMap(map[string]property.Value{"result": second}),
	}, nil
}
