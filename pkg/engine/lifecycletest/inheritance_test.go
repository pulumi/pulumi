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
	"slices"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// TestConstructBaseHappyPath exercises the core inheritance flow: a derived component from pkgB registers itself
// once with its own token and then asks the engine to construct its pkgA base portion. The base provider adopts
// the URN, parents a child to it, and returns outputs that the derived component folds into its own state.
func TestConstructBaseHappyPath(t *testing.T) {
	t.Parallel()

	var derivedURN resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					assert.Equal(t, tokens.Type("pkgA:m:base"), req.Type)
					assert.Equal(t, derivedURN, req.URN)
					assert.Equal(t, "res", req.Name)
					assert.Equal(t, tokens.Type("pkgB:m:derived"), req.MostDerivedType)
					assert.Equal(t, resource.NewProperty("hello"), req.Inputs["baseIn"])

					// The base's children parent directly to the adopted URN.
					_, err := monitor.RegisterResource("pkgA:m:child", "baseChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
					})
					require.NoError(t, err)

					return plugin.ConstructBaseResponse{
						Outputs: resource.PropertyMap{
							"baseOut": resource.NewProperty(req.Inputs["baseIn"].StringValue() + "-out"),
						},
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					// The single registration, with the most-derived token.
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)
					derivedURN = resp.URN

					// The generated base stub would issue this after registration completes.
					state, _, err := monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base",
						resource.PropertyMap{"baseIn": resource.NewProperty("hello")},
						nil, "", nil, "1.0.0", "")
					require.NoError(t, err)
					assert.Equal(t, resource.NewProperty("hello-out"), state["baseOut"])

					outputs := resource.PropertyMap{
						"baseOut": state["baseOut"],
						"ownOut":  resource.NewProperty("own"),
					}
					err = monitor.RegisterResourceOutputs(resp.URN, outputs)
					require.NoError(t, err)

					return plugin.ConstructResponse{URN: resp.URN, Outputs: outputs}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.NewProperty("hello-out"), resp.Outputs["baseOut"])
		assert.Equal(t, resource.NewProperty("own"), resp.Outputs["ownOut"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The snapshot must contain exactly one component node for the derived resource, registered under the
	// most-derived token, with the base's child parented to it.
	var derived, baseChild bool
	for _, res := range snap.Resources {
		switch res.Type { //nolint:exhaustive // only the inheritance-relevant types are of interest
		case "pkgB:m:derived":
			derived = true
			assert.Equal(t, resource.NewProperty("hello-out"), res.Outputs["baseOut"])
			assert.Equal(t, resource.NewProperty("own"), res.Outputs["ownOut"])
		case "pkgA:m:base":
			assert.Fail(t, "no resource may be registered under the base type")
		case "pkgA:m:child":
			baseChild = true
			assert.Equal(t, derivedURN, res.Parent)
		}
	}
	assert.True(t, derived, "expected the derived component in the snapshot")
	assert.True(t, baseChild, "expected the base-registered child in the snapshot")
}

// TestConstructBaseProviderUnsupported verifies the negotiation failure mode: constructing atop a base package
// whose provider predates inheritance support fails with an actionable error and registers nothing for the base.
func TestConstructBaseProviderUnsupported(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			// No ConstructBaseF: behaves like a provider built before component inheritance.
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false)
					require.NoError(t, err)

					_, _, err = monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
					require.Error(t, err)
					assert.ErrorContains(t, err, "does not support acting as a base class")
					assert.ErrorContains(t, err, "pkgA")
					return plugin.ConstructResponse{}, err
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.Error(t, err)
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.Error(t, err)
}

// TestConstructBaseCycleGuard verifies that an algorithmic base-construction cycle is a deterministic error
// rather than a hang: a base implementation that (incorrectly) base-constructs its own type is rejected.
func TestConstructBaseCycleGuard(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					// A schema-invalid self-extension: the runtime guard must catch the repeat.
					_, _, err := monitor.ConstructBaseResource(
						req.URN, req.Type, resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
					require.Error(t, err)
					assert.ErrorContains(t, err, "base construction cycle detected")
					return plugin.ConstructBaseResponse{}, err
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false)
					require.NoError(t, err)

					_, _, err = monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
					require.Error(t, err)
					assert.ErrorContains(t, err, "base construction cycle detected")
					return plugin.ConstructResponse{}, err
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		assert.Error(t, err)
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.Error(t, err)
}

// TestConstructBaseThreeLevel exercises a base-construction chain that is more than one level deep and spans
// three distinct provider packages: pkgB:m:derived extends pkgC:m:mid extends pkgA:m:base. Each level folds the
// outputs of the level beneath it into its own, and each level parents a child to the single adopted URN.
func TestConstructBaseThreeLevel(t *testing.T) {
	t.Parallel()

	var derivedURN resource.URN

	loaders := []*deploytest.ProviderLoader{
		// pkgA is the root base: it parents a child to the URN and returns baseOut derived from its input.
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					assert.Equal(t, tokens.Type("pkgA:m:base"), req.Type)
					assert.Equal(t, derivedURN, req.URN)
					assert.Equal(t, tokens.Type("pkgB:m:derived"), req.MostDerivedType)

					_, err := monitor.RegisterResource("pkgA:m:child", "baseChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
					})
					require.NoError(t, err)

					return plugin.ConstructBaseResponse{
						Outputs: resource.PropertyMap{
							"baseOut": resource.NewProperty(req.Inputs["in"].StringValue() + "-base"),
						},
					}, nil
				},
			}, nil
		}),
		// pkgC is the middle base: it parents its own child, base-constructs pkgA, and folds baseOut into its
		// response alongside its own midOut.
		deploytest.NewProviderLoader("pkgC", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					assert.Equal(t, tokens.Type("pkgC:m:mid"), req.Type)
					assert.Equal(t, derivedURN, req.URN)

					_, err := monitor.RegisterResource("pkgC:m:child", "midChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
					})
					require.NoError(t, err)

					state, _, err := monitor.ConstructBaseResource(
						req.URN, "pkgA:m:base",
						resource.PropertyMap{"in": req.Inputs["in"]},
						nil, "", nil, "1.0.0", "")
					require.NoError(t, err)

					return plugin.ConstructBaseResponse{
						Outputs: resource.PropertyMap{
							"baseOut": state["baseOut"],
							"midOut":  resource.NewProperty(req.Inputs["in"].StringValue() + "-mid"),
						},
					}, nil
				},
			}, nil
		}),
		// pkgB is the most-derived: it registers itself once, base-constructs pkgC, and folds the whole chain's
		// outputs into its own state.
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)
					derivedURN = resp.URN

					state, _, err := monitor.ConstructBaseResource(
						resp.URN, "pkgC:m:mid",
						resource.PropertyMap{"in": resource.NewProperty("x")},
						nil, "", nil, "1.0.0", "")
					require.NoError(t, err)

					outputs := resource.PropertyMap{
						"baseOut": state["baseOut"],
						"midOut":  state["midOut"],
						"ownOut":  resource.NewProperty("own"),
					}
					err = monitor.RegisterResourceOutputs(resp.URN, outputs)
					require.NoError(t, err)

					return plugin.ConstructResponse{URN: resp.URN, Outputs: outputs}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)
		assert.Equal(t, resource.NewProperty("x-base"), resp.Outputs["baseOut"])
		assert.Equal(t, resource.NewProperty("x-mid"), resp.Outputs["midOut"])
		assert.Equal(t, resource.NewProperty("own"), resp.Outputs["ownOut"])
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Exactly one component node, both bases' children parented to it, and nothing registered under a base type.
	derivedCount := 0
	var sawBaseChild, sawMidChild bool
	for _, res := range snap.Resources {
		switch res.Type { //nolint:exhaustive // only the inheritance-relevant types are of interest
		case "pkgB:m:derived":
			derivedCount++
			assert.Equal(t, resource.NewProperty("x-base"), res.Outputs["baseOut"])
			assert.Equal(t, resource.NewProperty("x-mid"), res.Outputs["midOut"])
		case "pkgA:m:base", "pkgC:m:mid":
			assert.Failf(t, "base type leaked", "no resource may be registered under base type %s", res.Type)
		case "pkgA:m:child":
			sawBaseChild = true
			assert.Equal(t, derivedURN, res.Parent)
		case "pkgC:m:child":
			sawMidChild = true
			assert.Equal(t, derivedURN, res.Parent)
		}
	}
	assert.Equal(t, 1, derivedCount, "expected exactly one component node for the derived resource")
	assert.True(t, sawBaseChild, "expected the pkgA base child in the snapshot")
	assert.True(t, sawMidChild, "expected the pkgC mid child in the snapshot")
}

// hasConstructBaseFeature reports whether the monitor advertises base-class construction via GetDeploymentInfo.
func hasConstructBaseFeature(t *testing.T, monitor *deploytest.ResourceMonitor) bool {
	t.Helper()
	info, err := monitor.Client().GetDeploymentInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	return slices.Contains(info.GetSupportedFeatures(),
		pulumirpc.ResourceMonitorFeature_RESOURCE_MONITOR_FEATURE_CONSTRUCT_BASE)
}

// TestConstructBaseFeatureAdvertised verifies that, by default, the engine advertises base-class construction so
// that SDKs know they may issue ConstructBaseResource.
func TestConstructBaseFeatureAdvertised(t *testing.T) {
	t.Parallel()

	var advertised bool
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		advertised = hasConstructBaseFeature(t, monitor)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.True(t, advertised, "the engine must advertise the construct-base feature by default")
}

// TestConstructBaseEngineFeatureDisabled verifies the kill switch: when base-class construction is disabled the
// feature is not advertised and any ConstructBaseResource call is rejected as Unimplemented.
func TestConstructBaseEngineFeatureDisabled(t *testing.T) {
	t.Parallel()

	var advertised bool
	var constructErr error
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		advertised = hasConstructBaseFeature(t, monitor)

		// Register a component so the URN is valid, then attempt to construct its base: the engine refuses.
		resp, err := monitor.RegisterResource("pkgB:m:derived", "res", false)
		require.NoError(t, err)
		_, _, constructErr = monitor.ConstructBaseResource(
			resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t, HostF: hostF, SkipDisplayTests: true,
			UpdateOptions: engine.UpdateOptions{DisableConstructBase: true},
		},
	}
	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	assert.False(t, advertised, "a disabled engine must not advertise the construct-base feature")
	require.Error(t, constructErr)
	rpcerr, ok := rpcerror.FromError(constructErr)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, rpcerr.Code())
	assert.Contains(t, rpcerr.Message(), "base-class construction has been disabled")
}

// TestConstructBaseUnregisteredURN verifies the two ways a ConstructBaseResource target can be invalid: a URN that
// was never registered is NotFound, and a custom (non-component) resource's URN is InvalidArgument.
func TestConstructBaseUnregisteredURN(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "id",
						Properties: resource.PropertyMap{},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var ghostErr, customErr error
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// A syntactically valid URN that was never registered.
		ghost := resource.NewURN("test", "test", "", "pkgB:m:derived", "ghost")
		_, _, ghostErr = monitor.ConstructBaseResource(
			ghost, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")

		// A custom resource's URN: base-class construction is only defined for components.
		res, err := monitor.RegisterResource("pkgA:m:res", "custom", true)
		require.NoError(t, err)
		_, _, customErr = monitor.ConstructBaseResource(
			res.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Error(t, ghostErr)
	rpcerr, ok := rpcerror.FromError(ghostErr)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, rpcerr.Code())
	assert.Contains(t, rpcerr.Message(), "has been registered")

	require.Error(t, customErr)
	rpcerr, ok = rpcerror.FromError(customErr)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, rpcerr.Code())
	assert.Contains(t, rpcerr.Message(), "only components support base-class construction")
}

// TestConstructBaseProviderVersionPinning verifies that the version carried on a ConstructBaseResource request
// selects the provider version that serves the base: with two versions of pkgA loaded, pinning 2.0.0 routes the
// base construction there and leaves 1.0.0 uninvolved.
func TestConstructBaseProviderVersionPinning(t *testing.T) {
	t.Parallel()

	var v1Called, v2Called bool
	baseLoader := func(version string, called *bool) func() (plugin.Provider, error) {
		return func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					_ plugin.ConstructBaseRequest,
					_ *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					*called = true
					return plugin.ConstructBaseResponse{
						Outputs: resource.PropertyMap{"v": resource.NewProperty(version)},
					}, nil
				},
			}, nil
		}
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), baseLoader("1.0.0", &v1Called)),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), baseLoader("2.0.0", &v2Called)),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)

					state, _, err := monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "2.0.0", "")
					require.NoError(t, err)
					assert.Equal(t, resource.NewProperty("2.0.0"), state["v"])

					return plugin.ConstructResponse{URN: resp.URN}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	assert.True(t, v2Called, "pkgA@2.0.0 must serve the pinned base construction")
	assert.False(t, v1Called, "pkgA@1.0.0 must not be involved")
}

// TestConstructBaseEventStream asserts event-stream sanity for the inheritance flow: the derived component is
// registered exactly once, and no step event is ever emitted under a base type (bases are not resources).
func TestConstructBaseEventStream(t *testing.T) {
	t.Parallel()

	var derivedURN resource.URN
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					_, err := monitor.RegisterResource("pkgA:m:child", "baseChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
					})
					require.NoError(t, err)
					return plugin.ConstructBaseResponse{
						Outputs: resource.PropertyMap{"baseOut": resource.NewProperty("out")},
					}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{
						Parent: req.Parent,
					})
					require.NoError(t, err)
					derivedURN = resp.URN

					state, _, err := monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
					require.NoError(t, err)

					outputs := resource.PropertyMap{"baseOut": state["baseOut"]}
					require.NoError(t, monitor.RegisterResourceOutputs(resp.URN, outputs))
					return plugin.ConstructResponse{URN: resp.URN, Outputs: outputs}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()

	validate := func(
		_ workspace.Project, _ deploy.Target, _ engine.JournalEntries, events []engine.Event, err error,
	) error {
		derivedRegistrations := 0
		for _, e := range events {
			var typ tokens.Type
			var urn resource.URN
			switch e.Type { //nolint:exhaustive // we only inspect resource step events
			case engine.ResourcePreEvent:
				m := e.Payload().(engine.ResourcePreEventPayload).Metadata
				typ, urn = m.Type, m.URN
			case engine.ResourceOutputsEvent:
				m := e.Payload().(engine.ResourceOutputsEventPayload).Metadata
				typ, urn = m.Type, m.URN
			default:
				continue
			}
			assert.NotEqual(t, tokens.Type("pkgA:m:base"), typ, "no step event may carry a base type")
			if e.Type == engine.ResourcePreEvent && urn == derivedURN {
				derivedRegistrations++
			}
		}
		assert.Equal(t, 1, derivedRegistrations, "the derived component must be registered exactly once")
		return err
	}

	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate, "0")
	require.NoError(t, err)
}
