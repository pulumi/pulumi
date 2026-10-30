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

package lifecycletest

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// The tests in this file cover the "OutputValues in Check" feature, mirroring the Invoke/Call tests in
// output_invoke_test.go. The axes are:
//
//  1. Whether the SDK sends OutputValues in `RegisterResourceRequest.object` (i.e. the resource inputs).
//  2. Whether the provider advertised `accepts_outputs_in_check=true` on its Handshake response.
//
// The engine should:
//  (a) forward OutputValues in `CheckRequest.news` to providers that negotiated support, else downgrade them,
//  (b) downgrade any OutputValues returned in `CheckResponse.inputs` into the resource's per-property
//      dependency map (and strip the OutputValue wrappers) before the response flows into the rest of the
//      engine.

// checkOutputValuesTestInputs builds an input map with a single OutputValue whose dependency is `dep`.
func checkOutputValuesTestInputs(dep resource.URN) resource.PropertyMap {
	return resource.PropertyMap{
		"in": resource.NewProperty(resource.Output{
			Element:      resource.NewProperty("v"),
			Known:        true,
			Dependencies: []resource.URN{dep},
		}),
	}
}

// TestCheckArgsOutputValuesForwardedToAcceptingProvider verifies that when the provider advertises
// `AcceptsOutputsInCheck` on handshake, an OutputValue the SDK supplied in the resource inputs reaches the
// provider's Check intact (preserving per-value dependencies).
func TestCheckArgsOutputValuesForwardedToAcceptingProvider(t *testing.T) {
	t.Parallel()

	var seenDeps []resource.URN
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					_ context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					assert.True(t, req.AcceptsOutputsInCheck,
						"engine should advertise AcceptsOutputsInCheck on handshake")
					return &plugin.ProviderHandshakeResponse{AcceptsOutputsInCheck: true}, nil
				},
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					if v, ok := req.NewInputs.GetOk("in"); ok {
						seenDeps = v.Dependencies()
					}
					return plugin.CheckResponse{Properties: req.NewInputs}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:B", "b", true, deploytest.ResourceOptions{
			Inputs:           checkOutputValuesTestInputs(respA.URN),
			KeepOutputValues: true,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, seenDeps, 1, "accepting provider should see the OutputValue's dependency on Check inputs")
}

// TestCheckArgsOutputValuesDowngradedForLegacyProvider verifies that when the provider does NOT advertise
// `AcceptsOutputsInCheck`, the engine downgrades any OutputValues in resource inputs to plain
// values (or Computed/Secret) before invoking Check -- legacy providers must never see OutputValues.
func TestCheckArgsOutputValuesDowngradedForLegacyProvider(t *testing.T) {
	t.Parallel()

	var seenDeps []resource.URN
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// No HandshakeF: defaults to AcceptsOutputsInCheck=false.
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					if v, ok := req.NewInputs.GetOk("in"); ok {
						seenDeps = v.Dependencies()
					}
					return plugin.CheckResponse{Properties: req.NewInputs}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:B", "b", true, deploytest.ResourceOptions{
			Inputs:           checkOutputValuesTestInputs(respA.URN),
			KeepOutputValues: true,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	assert.Empty(t, seenDeps, "legacy provider should not see per-value dependencies on Check inputs")
}

// TestCheckRebuildsPropertyDependenciesFromResponse verifies that when a Check-accepting provider moves,
// renames, or synthesizes keys in its response, the resource's `PropertyDependencies` map on state is
// rebuilt from the OutputValues on `CheckResponse.inputs` -- not carried over from the goal.
func TestCheckRebuildsPropertyDependenciesFromResponse(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					context.Context, plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					return &plugin.ProviderHandshakeResponse{AcceptsOutputsInCheck: true}, nil
				},
				CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
					// Move the value under `in` to a new key `renamed`, carrying its deps along.
					v, _ := req.NewInputs.GetOk("in")
					return plugin.CheckResponse{
						Properties: property.NewMap(map[string]property.Value{"renamed": v}),
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:B", "b", true, deploytest.ResourceOptions{
			Inputs:           checkOutputValuesTestInputs(respA.URN),
			KeepOutputValues: true,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	snap, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)

	var b *pkgresource.State
	for _, r := range snap.Resources {
		if r.URN.Name() == "b" {
			b = r
			break
		}
	}
	require.NotNil(t, b, "resource b should be in the snapshot")

	// After Check, `in` should be gone and `renamed` should carry the dependency the provider returned.
	assert.NotContains(t, b.PropertyDependencies, resource.PropertyKey("in"),
		"the pre-Check key should not persist in PropertyDependencies")
	deps := b.PropertyDependencies[resource.PropertyKey("renamed")]
	require.Len(t, deps, 1, "the renamed key should carry the dependency from the response's OutputValue")
}
