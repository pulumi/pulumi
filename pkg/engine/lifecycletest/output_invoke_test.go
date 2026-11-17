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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// The tests in this file cover the compatibility matrix for the "OutputValues in Invoke" feature negotiated on
// three axes:
//
//  1. Whether the SDK sends OutputValues in `ResourceInvokeRequest.args`.
//  2. Whether the SDK sets `accept_output_values=true` on the request.
//  3. Whether the provider advertised `accepts_outputs_in_invoke=true` on its Handshake response.
//
// The engine's role is to (a) harvest dependencies from any OutputValues in args into the invoke's wait-set,
// (b) forward args to the provider as OutputValues when the provider negotiated support (else downgrade), and
// (c) marshal the response with OutputValues when the SDK asked for them (else downgrade).

// makeArgsWithOutputValue builds an args map containing a single OutputValue with the given dependencies and
// underlying value. Used to simulate an SDK sending OutputValues in Invoke args.
func makeArgsWithOutputValue(deps []resource.URN, element property.Value, known bool) property.Map {
	v := element.WithDependencies(deps)
	if !known {
		v = property.New(property.Computed).WithDependencies(deps)
	}
	return property.NewMap(map[string]property.Value{
		"in": v,
	})
}

// TestInvokeArgsOutputValuesForwardedToAcceptingProvider verifies that when the provider advertises
// `AcceptsOutputsInInvoke` on its Handshake response, an OutputValue that the SDK put in `args` reaches the
// provider intact (as an OutputValue) rather than being downgraded to Computed/Secret.
func TestInvokeArgsOutputValuesForwardedToAcceptingProvider(t *testing.T) {
	t.Parallel()

	var (
		seenArg property.Value
		invoked bool
	)
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					_ context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					assert.True(t, req.AcceptsOutputsInInvoke,
						"engine should advertise AcceptsOutputsInInvoke on handshake")
					return &plugin.ProviderHandshakeResponse{AcceptsOutputsInInvoke: true}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					v, ok := req.Args.GetOk("in")
					require.True(t, ok, "expected `in` in invoke args")
					seenArg = v
					invoked = true
					return plugin.InvokeResponse{
						Properties: property.NewMap(map[string]property.Value{"out": property.New("ok")}),
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register a resource first so its URN is known and resolved by the tracker.
		resp, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		args := makeArgsWithOutputValue([]resource.URN{resp.URN}, property.New("v"), true /*known*/)
		_, _, err = monitor.Invoke("pkgA:index:f", args, "", "", "",
			deploytest.InvokeOptions{KeepArgOutputValues: true})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The provider should have observed an OutputValue in the args, carrying the dependency.
	require.True(t, invoked, "InvokeF was not called")
	require.NotEmpty(t, seenArg.Dependencies(), "expected the OutputValue's dependency to be preserved")
}

// TestInvokeArgsOutputValuesDowngradedForLegacyProvider verifies that when the provider does not advertise
// `AcceptsOutputsInInvoke`, the engine downgrades any OutputValues in args to plain/Computed/Secret before
// calling the provider.
func TestInvokeArgsOutputValuesDowngradedForLegacyProvider(t *testing.T) {
	t.Parallel()

	var (
		seenArg property.Value
		invoked bool
	)
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// No HandshakeF override: default returns an empty response, so AcceptsOutputsInInvoke is false.
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					v, ok := req.Args.GetOk("in")
					require.True(t, ok)
					seenArg = v
					invoked = true
					return plugin.InvokeResponse{
						Properties: property.NewMap(map[string]property.Value{"out": property.New("ok")}),
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		args := makeArgsWithOutputValue([]resource.URN{resp.URN}, property.New("v"), true)
		_, _, err = monitor.Invoke("pkgA:index:f", args, "", "", "",
			deploytest.InvokeOptions{KeepArgOutputValues: true})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The legacy provider should see a plain value, no OutputValue wrapper, no dependencies.
	require.True(t, invoked, "InvokeF was not called")
	assert.Empty(t, seenArg.Dependencies(),
		"legacy provider should not receive dependencies via an OutputValue wrapper")
	assert.True(t, seenArg.IsString(), "engine should have downgraded the known OutputValue to its element")
}

// TestInvokeReturnOutputValuesToAcceptingSDK verifies that when the SDK sets `accept_output_values=true` on
// its request and the provider returns an OutputValue, the engine marshals the OutputValue through to the SDK.
func TestInvokeReturnOutputValuesToAcceptingSDK(t *testing.T) {
	t.Parallel()

	var providerURN resource.URN
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					context.Context, plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					return &plugin.ProviderHandshakeResponse{AcceptsOutputsInInvoke: true}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					out := property.New("hello").WithDependencies([]resource.URN{providerURN})
					return plugin.InvokeResponse{
						Properties: property.NewMap(map[string]property.Value{"out": out}),
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)
		providerURN = resp.URN

		ret, _, err := monitor.Invoke("pkgA:index:f", property.Map{}, "", "", "",
			deploytest.InvokeOptions{AcceptOutputValues: true})
		require.NoError(t, err)

		got, ok := ret.GetOk("out")
		require.True(t, ok)
		assert.Equal(t, []resource.URN{providerURN}, got.Dependencies())
		assert.True(t, got.IsString())
		assert.Equal(t, "hello", got.AsString())
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// TestInvokeReturnOutputValuesDowngradedForLegacySDK verifies that when the SDK does not set
// `accept_output_values`, the engine downgrades any OutputValues the provider returned into plain values so the
// SDK sees them in the legacy shape (with secrecy/computed-ness preserved via the existing Secret/Computed wrappers).
func TestInvokeReturnOutputValuesDowngradedForLegacySDK(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					context.Context, plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					return &plugin.ProviderHandshakeResponse{AcceptsOutputsInInvoke: true}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					out := property.New("hello").WithDependencies([]resource.URN{"urn:pulumi:stack::proj::pkgA:m:A::a"})
					return plugin.InvokeResponse{
						Properties: property.NewMap(map[string]property.Value{"out": out}),
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// no InvokeOptions => AcceptOutputValues=false
		ret, _, err := monitor.Invoke("pkgA:index:f", property.Map{}, "", "", "")
		require.NoError(t, err)

		got, ok := ret.GetOk("out")
		require.True(t, ok)
		assert.Empty(t, got.Dependencies(),
			"the SDK should not receive an OutputValue when AcceptOutputValues is false")
		require.True(t, got.IsString())
		assert.Equal(t, "hello", got.AsString())
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// TestInvokeLegacyProviderRehydratesReturnDependencies verifies that when the SDK sends OutputValues in args
// but the provider does NOT advertise `AcceptsOutputsInInvoke`, the engine drops per-value dependencies on the
// way to the provider (as expected) and then re-hydrates the return by wrapping each returned property in an
// OutputValue whose dependencies are the union of the arg OutputValues' dependencies. Without this the SDK
// would receive a plain value with no dependency information at all.
func TestInvokeLegacyProviderRehydratesReturnDependencies(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// No HandshakeF override: provider does NOT opt into AcceptsOutputsInInvoke.
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					// Confirm the provider sees no OutputValues (the engine downgraded them).
					aArg, _ := req.Args.GetOk("a")
					bArg, _ := req.Args.GetOk("b")
					assert.Empty(t, aArg.Dependencies(), "legacy provider must not see per-value dependencies")
					assert.Empty(t, bArg.Dependencies(), "legacy provider must not see per-value dependencies")
					return plugin.InvokeResponse{
						Properties: property.NewMap(map[string]property.Value{"out": property.New("computed")}),
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:A", "a", true, deploytest.ResourceOptions{})
		require.NoError(t, err)
		respB, err := monitor.RegisterResource("pkgA:m:B", "b", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		args := property.NewMap(map[string]property.Value{
			"a": property.New("a-val").WithDependencies([]resource.URN{respA.URN}),
			"b": property.New("b-val").WithDependencies([]resource.URN{respB.URN}),
		})
		ret, _, err := monitor.Invoke("pkgA:index:f", args, "", "", "",
			deploytest.InvokeOptions{KeepArgOutputValues: true, AcceptOutputValues: true})
		require.NoError(t, err)

		got, ok := ret.GetOk("out")
		require.True(t, ok)
		require.NotEmpty(t, got.Dependencies(),
			"engine should have re-hydrated the return as an OutputValue because the provider did not preserve deps")
		require.ElementsMatch(t, []resource.URN{respA.URN, respB.URN}, got.Dependencies(),
			"re-hydrated OutputValue should carry the union of arg dependencies")
		assert.Equal(t, "computed", got.AsString())
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF}}
	_, err := lt.TestOp(Update).RunStep(
		p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
}
