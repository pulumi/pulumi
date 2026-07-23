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

package lifecycletest

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestPreviewInvoke(t *testing.T) {
	t.Parallel()

	expectPreview := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					ctx context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					assert.True(t, req.InvokeWithPreview, "expected engine to advertise invoke_with_preview support")
					return &plugin.ProviderHandshakeResponse{}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, expectPreview, req.Preview)
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("invoked"),
						},
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, _, err := monitor.Invoke("pkgA:index:myFunc", nil, "", "", "")
		require.NoError(t, err)
		assert.Equal(t, resource.NewProperty("invoked"), resp["result"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)

	expectPreview = false
	_, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

// TestInvokeDependsOnRemoteComponent demonstrates pulumi/pulumi#18299: an
// output-form invoke that depends on a remote component executes during a
// preview that has not yet created the component's children.
//
// SDKs gate output-form invokes client-side: they expand the dependency set
// into the transitively reachable custom resources and defer the invoke while
// any of their ids is unknown (sdk/nodejs/runtime/invoke.ts,
// sdk/python/lib/pulumi/runtime/invoke.py). That expansion works for local
// components, whose children live in the caller's process, but terminates at
// remote components: their children are registered by the component provider
// directly with the engine, and no monitor RPC exposes them -- or their
// created-ness -- to the caller. RegisterResourceResponse carries only the
// component's URN, outputs and the outputs' dependency URNs; a component
// without outputs, like the one here, reports nothing at all. So the gate
// passes vacuously and the invoke fires.
//
// Only the engine knows both the component's children and whether the current
// operation still has to create them, but ResourceInvokeRequest gives it no
// dependency information to act on. Until it does (pulumi/pulumi#24021), no
// client can implement the behavior this test asserts: the invoke must not
// reach the provider during the initial preview, and must reach it during up
// and steady-state previews.
func TestInvokeDependsOnRemoteComponent(t *testing.T) {
	t.Parallel()

	invoked := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context, req plugin.ConstructRequest, monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)

					_, err = monitor.RegisterResource("pkgA:m:typChild", req.Name+"-child", true, deploytest.ResourceOptions{
						Parent: resp.URN,
					})
					require.NoError(t, err)

					return plugin.ConstructResponse{URN: resp.URN}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					invoked = true
					return plugin.InvokeResponse{Properties: resource.PropertyMap{
						"result": resource.NewProperty("read"),
					}}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register the remote component. The engine calls Construct, which
		// registers the child, before this returns.
		_, err := monitor.RegisterResource("pkgA:m:typComponent", "comp", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		// An invoke with dependsOn on the component registered above.
		// Everything a client can do with that dependency has already
		// happened: the component is remote, so there are no children to
		// expand into and no ids to await, and its own registration resolved
		// above. There is nowhere to put the dependency on the wire, so the
		// invoke just fires.
		_, _, err = monitor.Invoke("pkgA:index:readChild", nil, "", "", "")
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()

	// Initial preview: the child's creation is still pending, so the invoke
	// must not reach the provider.
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.False(t, invoked, "invoke must not execute while the remote component's child is pending creation")

	// Up: Construct returns only after the child is created, so the invoke
	// runs after it.
	invoked = false
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.True(t, invoked, "invoke must execute during up")

	// Steady-state preview: every dependency already exists, so the invoke
	// must run to keep the preview accurate. This is why a client cannot
	// simply always defer on remote components.
	invoked = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient, nil, "2")
	require.NoError(t, err)
	assert.True(t, invoked, "invoke must execute during a steady-state preview")
}

func TestSecretsInvoke(t *testing.T) {
	t.Parallel()

	expectPreview := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				HandshakeF: func(
					ctx context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					return &plugin.ProviderHandshakeResponse{AcceptSecrets: false}, nil
				},
				InvokeF: func(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					assert.Equal(t, expectPreview, req.Preview)
					return plugin.InvokeResponse{
						Properties: resource.PropertyMap{
							"result": resource.NewProperty("invoked"),
						},
					}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, _, err := monitor.Invoke("pkgA:index:myFunc", resource.PropertyMap{
			"secret": resource.MakeSecret(resource.NewProperty("my-secret")),
		}, "", "", "")
		require.NoError(t, err)
		assert.Equalf(t, resource.MakeSecret(resource.NewProperty("invoked")), resp["result"], "Returned: %#v", resp)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)

	expectPreview = false
	_, err = lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
