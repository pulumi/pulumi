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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
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

// previewAwareCreate mimics a real provider's create: no id is assigned during previews.
func previewAwareCreate(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	if req.Preview {
		return plugin.CreateResponse{Properties: resource.PropertyMap{}}, nil
	}
	return plugin.CreateResponse{ID: "created-id", Properties: resource.PropertyMap{}}, nil
}

// TestInvokeDependsOnRemoteComponent covers pulumi/pulumi#18299: an output-form
// invoke that depends on a remote component must not execute during a preview
// that has not yet created the component's children.
//
// A caller cannot see a remote component's children: they are registered by
// the component provider directly with the engine. The caller instead declares
// the dependency on ResourceInvokeRequest and the engine expands it -- the
// component aggregates every descendant reachable through component ancestors
// -- and gates the invoke on the created-ness of the expanded resources: the
// invoke must not reach the provider during the initial preview, and must
// reach it during up and steady-state previews.
func TestInvokeDependsOnRemoteComponent(t *testing.T) {
	t.Parallel()

	invoked := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: previewAwareCreate,
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
		comp, err := monitor.RegisterResource("pkgA:m:typComponent", "comp", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)

		// An invoke that depends on the component. The caller cannot see the
		// component's children, so it sends the component's URN and the engine
		// expands it.
		result, err := monitor.InvokeWithResult("pkgA:index:readChild", nil, "", "", "", deploytest.InvokeOptions{
			DependsOn:       []resource.URN{comp.URN},
			AcceptsUnknowns: true,
		})
		require.NoError(t, err)
		require.Empty(t, result.Failures)
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

// TestInvokeDependsOnPendingCustomResource covers the engine's invoke gate for a plain custom resource: an invoke that
// depends on a resource whose creation is pending resolves as unknown without reaching the provider, and runs normally
// during up.
func TestInvokeDependsOnPendingCustomResource(t *testing.T) {
	t.Parallel()

	invoked := false
	expectUnknown := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: previewAwareCreate,
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
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		require.NoError(t, err)

		result, err := monitor.InvokeWithResult("pkgA:index:read", nil, "", "", "", deploytest.InvokeOptions{
			DependsOn:       []resource.URN{dep.URN},
			AcceptsUnknowns: true,
		})
		require.NoError(t, err)
		require.Empty(t, result.Failures)
		assert.Equal(t, expectUnknown, result.Unknown)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()

	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.False(t, invoked, "invoke must not execute while its dependency is pending creation")

	expectUnknown = false
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.True(t, invoked, "invoke must execute during up")
}

// TestInvokeDependsOnStopsAtCustomResources pins the expansion boundary: a custom resource contributes only itself, so
// an invoke depending on it runs even while a custom child of it is pending creation, matching the SDKs' client-side
// expansion of local dependencies.
func TestInvokeDependsOnStopsAtCustomResources(t *testing.T) {
	t.Parallel()

	invoked := false
	registerChild := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: previewAwareCreate,
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
		parent, err := monitor.RegisterResource("pkgA:m:typA", "parent", true)
		require.NoError(t, err)
		if !registerChild {
			return nil
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Parent: parent.URN,
		})
		require.NoError(t, err)

		result, err := monitor.InvokeWithResult("pkgA:index:read", nil, "", "", "", deploytest.InvokeOptions{
			DependsOn:       []resource.URN{parent.URN},
			AcceptsUnknowns: true,
		})
		require.NoError(t, err)
		require.Empty(t, result.Failures)
		assert.False(t, result.Unknown, "a custom dependency's pending children must not defer the invoke")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	registerChild = true
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.True(t, invoked, "invoke must execute: its custom dependency exists and the pending child is not expanded")
}

// TestInvokeDependsOnPreviewCapableProvider: a provider that declared preview invoke semantics in its handshake is
// called despite pending dependencies, with Preview set, and decides for itself.
func TestInvokeDependsOnPreviewCapableProvider(t *testing.T) {
	t.Parallel()

	invoked := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: previewAwareCreate,
				HandshakeF: func(
					_ context.Context, req plugin.ProviderHandshakeRequest,
				) (*plugin.ProviderHandshakeResponse, error) {
					assert.True(t, req.AcceptsInvokeUnknowns, "expected engine to declare it accepts invoke unknowns")
					return &plugin.ProviderHandshakeResponse{InvokeWithPreview: true}, nil
				},
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					invoked = true
					assert.True(t, req.Preview)
					return plugin.InvokeResponse{Properties: resource.PropertyMap{
						"result": resource.NewProperty("read"),
					}}, nil
				},
			}, nil
		}, deploytest.WithGrpc, deploytest.WithHandshake),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		require.NoError(t, err)

		result, err := monitor.InvokeWithResult("pkgA:index:read", nil, "", "", "", deploytest.InvokeOptions{
			DependsOn:       []resource.URN{dep.URN},
			AcceptsUnknowns: true,
		})
		require.NoError(t, err)
		require.Empty(t, result.Failures)
		assert.False(t, result.Unknown)
		assert.Equal(t, resource.PropertyMap{"result": resource.NewProperty("read")}, result.Return)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.True(t, invoked, "a preview-capable provider must be called despite pending dependencies")
}

// TestInvokeUnknownNormalization: a provider with preview invoke semantics may return unknowns. A caller that declared
// acceptsUnknowns receives them; any other caller receives the empty result -- never a partial one it would read as
// complete.
func TestInvokeUnknownNormalization(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				InvokeWithPreviewV: true,
				InvokeF: func(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
					return plugin.InvokeResponse{Properties: resource.PropertyMap{
						"stable":  resource.NewProperty("value"),
						"pending": resource.MakeComputed(resource.NewProperty("")),
					}}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		accepting, err := monitor.InvokeWithResult("pkgA:index:read", nil, "", "", "", deploytest.InvokeOptions{
			AcceptsUnknowns: true,
		})
		require.NoError(t, err)
		require.Empty(t, accepting.Failures)
		assert.Equal(t, resource.PropertyMap{
			"stable":  resource.NewProperty("value"),
			"pending": resource.MakeComputed(resource.NewProperty("")),
		}, accepting.Return)

		plain, failures, err := monitor.Invoke("pkgA:index:read", nil, "", "", "")
		require.NoError(t, err)
		require.Empty(t, failures)
		assert.Equal(t, resource.PropertyMap{}, plain)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, nil, "0")
	require.NoError(t, err)
}

// TestInvokeDependsOnTargetedUp: --target skips the creation of an untargeted dependency, leaving it without an id, so
// an invoke depending on it must resolve as unknown even though the operation is not a preview.
func TestInvokeDependsOnTargetedUp(t *testing.T) {
	t.Parallel()

	invoked := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: previewAwareCreate,
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
		_, err := monitor.RegisterResource("pkgA:m:typA", "keep", true)
		require.NoError(t, err)
		dep, err := monitor.RegisterResource("pkgA:m:typA", "dep", true)
		require.NoError(t, err)

		result, err := monitor.InvokeWithResult("pkgA:index:read", nil, "", "", "", deploytest.InvokeOptions{
			DependsOn:       []resource.URN{dep.URN},
			AcceptsUnknowns: true,
		})
		require.NoError(t, err)
		require.Empty(t, result.Failures)
		assert.True(t, result.Unknown, "invoke must be unknown: its dependency's creation was skipped by --target")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: true,
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{"**keep**"}),
			},
		},
	}
	_, err := lt.TestOp(Update).RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.False(t, invoked, "invoke must not execute against a resource whose creation was skipped")
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
