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

// This file complements inheritance_test.go: where that file covers base-construction call semantics and feature
// negotiation, this one covers how the inheritance flow interacts with the engine's operations and lifecycle
// (partial failure, destroy/refresh, targeting, refactoring stability, method routing, and cancellation).

package lifecycletest

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// okCreate is the trivial custom-resource creation used by the base children in these tests.
func okCreate(_ context.Context, _ plugin.CreateRequest) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{ID: "id", Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
}

// TestConstructBasePartialFailure verifies failed-construct semantics for inheritance: if the base portion
// succeeds (creating a child) but the derived body then errors, the update fails yet the snapshot stays
// consistent with the base child retained and parented to the component. A subsequent update converges.
func TestConstructBasePartialFailure(t *testing.T) {
	t.Parallel()

	var derivedURN, baseChildURN resource.URN
	var derivedBodyFails atomic.Bool
	derivedBodyFails.Store(true)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: okCreate,
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					child, err := monitor.RegisterResource("pkgA:m:child", "baseChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
					})
					require.NoError(t, err)
					baseChildURN = child.URN
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

					if derivedBodyFails.Load() {
						// The base portion is in place, but the derived component's own body fails.
						return plugin.ConstructResponse{}, errors.New("derived body failed")
					}

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
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()

	// Step 0: the derived body fails after the base child is created. The update fails, but the base child is
	// persisted under the already-registered component and the snapshot stays consistent.
	snap, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.Error(t, err)
	require.NotNil(t, snap)
	require.NoError(t, snap.VerifyIntegrity())

	var sawBaseChild bool
	for _, res := range snap.Resources {
		if res.Type == "pkgA:m:child" {
			sawBaseChild = true
			assert.Equal(t, derivedURN, res.Parent)
		}
	}
	assert.True(t, sawBaseChild, "the base child must be retained after the partial failure")

	// Step 1: the derived body now succeeds; the update converges and the base child is the same resource.
	derivedBodyFails.Store(false)
	snap, err = lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	derivedCount := 0
	sawBaseChild = false
	for _, res := range snap.Resources {
		switch res.Type { //nolint:exhaustive // only the inheritance-relevant types are of interest
		case "pkgB:m:derived":
			derivedCount++
		case "pkgA:m:child":
			sawBaseChild = true
			assert.Equal(t, derivedURN, res.Parent)
			assert.Equal(t, baseChildURN, res.URN)
		}
	}
	assert.Equal(t, 1, derivedCount, "expected exactly one component node after convergence")
	assert.True(t, sawBaseChild, "the base child must survive convergence")
}

// TestConstructBaseDestroyAndRefresh verifies that refresh and destroy treat a base-constructed component the way
// they treat any component: the component itself has no provider involvement, while its base-created custom child
// refreshes and deletes through its provider. Base construction only happens during updates.
func TestConstructBaseDestroyAndRefresh(t *testing.T) {
	t.Parallel()

	var constructBaseCalls, readCalls, deleteCalls atomic.Int32
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: okCreate,
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					readCalls.Add(1)
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{ID: req.ID, Inputs: req.Inputs, Outputs: req.State},
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleteCalls.Add(1)
					return plugin.DeleteResponse{Status: resource.StatusOK}, nil
				},
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					constructBaseCalls.Add(1)
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

	snap, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Equal(t, int32(1), constructBaseCalls.Load())

	// Refresh reads the base-created custom child and leaves the component alone. No base construction happens.
	snap, err = lt.TestOp(engine.Refresh).RunStep(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.Equal(t, int32(1), readCalls.Load(), "only the base-created custom child refreshes")
	assert.Equal(t, int32(1), constructBaseCalls.Load(), "refresh must not construct bases")

	// Destroy deletes the child through its provider; no construct is involved.
	snap, err = lt.TestOp(engine.Destroy).RunStep(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	assert.Equal(t, int32(1), deleteCalls.Load(), "the base child deletes cleanly")
	assert.Equal(t, int32(1), constructBaseCalls.Load(), "destroy must not construct bases")
	assert.Empty(t, snap.Resources, "destroy removes every resource")
}

// TestConstructBaseTargeting verifies that base-constructed components obey ordinary targeting: an update that
// targets only the base-created child leaves the rest of the stack untouched.
func TestConstructBaseTargeting(t *testing.T) {
	t.Parallel()

	var baseChildURN resource.URN
	childInput := "v0"
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: okCreate,
				DiffF: func(_ context.Context, _ plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{Changes: plugin.DiffSome}, nil
				},
				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
				},
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					child, err := monitor.RegisterResource("pkgA:m:child", "baseChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
						Inputs: resource.PropertyMap{"in": resource.NewProperty(childInput)},
					})
					require.NoError(t, err)
					baseChildURN = child.URN
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
					_, _, err = monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
					require.NoError(t, err)
					require.NoError(t, monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{}))
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

	snap, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Change the child's input, but target only the child: it should update, and everything else (the component
	// and providers) should be a no-op.
	childInput = "v1"
	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{baseChildURN})

	validate := func(
		_ workspace.Project, _ deploy.Target, entries engine.JournalEntries, _ []engine.Event, err error,
	) error {
		require.NoError(t, err)
		var childUpdated bool
		for _, entry := range entries {
			op := entry.Step.Op()
			if entry.Step.URN() == baseChildURN {
				childUpdated = true
				assert.Equal(t, deploy.OpUpdate, op, "the targeted base child must update")
			} else {
				assert.Equal(t, deploy.OpSame, op,
					"untargeted resource %s must be a no-op, got %s", entry.Step.URN(), op)
			}
		}
		assert.True(t, childUpdated, "the targeted base child must appear in the plan")
		return err
	}

	_, err = lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	require.NoError(t, err)
}

// TestInsertBaseClassNoReplacement pins the child-naming stability invariant that makes it safe to refactor a
// monolithic component into a base plus a derived component: a child moving from the derived component's own
// constructor into the base's ConstructBase keeps its URN (same type, name, and parent) and is therefore a no-op
// in the plan rather than a replacement.
func TestInsertBaseClassNoReplacement(t *testing.T) {
	t.Parallel()

	var useBase atomic.Bool
	var derivedURN, sharedChildURN resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: okCreate,
				ConstructBaseF: func(
					_ context.Context,
					req plugin.ConstructBaseRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					// The refactored world: the shared child now belongs to the base.
					_, err := monitor.RegisterResource("pkgA:m:child", "sharedChild", true, deploytest.ResourceOptions{
						Parent: req.URN,
					})
					require.NoError(t, err)
					return plugin.ConstructBaseResponse{Outputs: resource.PropertyMap{}}, nil
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

					if useBase.Load() {
						// Refactored: delegate the shared child to the base.
						_, _, err = monitor.ConstructBaseResource(
							resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
						require.NoError(t, err)
					} else {
						// Monolithic: the derived component registers the shared child itself.
						child, err := monitor.RegisterResource("pkgA:m:child", "sharedChild", true,
							deploytest.ResourceOptions{Parent: resp.URN})
						require.NoError(t, err)
						sharedChildURN = child.URN
					}
					require.NoError(t, monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{}))
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

	// Step 0: the monolithic component owns the shared child directly.
	snap, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotEmpty(t, sharedChildURN)
	require.Equal(t, derivedURN, sharedChildParent(t, snap, sharedChildURN))

	// Step 1: refactor so the base owns the shared child. Its URN is unchanged and the plan is all-Same for it.
	useBase.Store(true)
	validate := func(
		_ workspace.Project, _ deploy.Target, entries engine.JournalEntries, _ []engine.Event, err error,
	) error {
		require.NoError(t, err)
		var sawChildSame bool
		for _, entry := range entries {
			if entry.Step.URN() == sharedChildURN {
				sawChildSame = true
				assert.Equal(t, deploy.OpSame, entry.Step.Op(),
					"the shared child must not be replaced when it moves to the base")
			}
		}
		assert.True(t, sawChildSame, "the shared child must be reconciled as a no-op, not dropped")
		return err
	}

	snap, err = lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "1")
	require.NoError(t, err)

	var found bool
	for _, res := range snap.Resources {
		if res.URN == sharedChildURN {
			found = true
			assert.Equal(t, derivedURN, res.Parent, "the shared child stays parented to the component")
		}
	}
	assert.True(t, found, "the shared child's URN must be unchanged after inserting the base class")
}

// sharedChildParent returns the parent URN of the resource with the given URN in the snapshot.
func sharedChildParent(t *testing.T, snap *deploy.Snapshot, urn resource.URN) resource.URN {
	t.Helper()
	for _, res := range snap.Resources {
		if res.URN == urn {
			return res.Parent
		}
	}
	require.Failf(t, "resource not found", "no resource %s in snapshot", urn)
	return ""
}

// TestConstructBaseInheritedMethodCall verifies method routing for inherited components: a Call whose token names
// the base package is routed to the base's provider even though __self__ is a derived-typed component whose goal
// records the derived package's provider, while a Call on the derived component's own method routes to the derived
// provider.
func TestConstructBaseInheritedMethodCall(t *testing.T) {
	t.Parallel()

	var pkgACall, pkgBCall atomic.Bool
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CallF: func(
					_ context.Context, req plugin.CallRequest, _ *deploytest.ResourceMonitor,
				) (plugin.CallResponse, error) {
					assert.Equal(t, "pkgA:m:base/method", string(req.Tok))
					pkgACall.Store(true)
					return plugin.CallResponse{}, nil
				},
			}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CallF: func(
					_ context.Context, req plugin.CallRequest, _ *deploytest.ResourceMonitor,
				) (plugin.CallResponse, error) {
					assert.Equal(t, "pkgB:m:derived/method", string(req.Tok))
					pkgBCall.Store(true)
					return plugin.CallResponse{}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// Register an explicit pkgB provider and give it to the derived component, so its goal records a pkgB
		// provider reference -- exactly the case the routing guard must handle for base-package method calls.
		provB, err := monitor.RegisterResource("pulumi:providers:pkgB", "provB", true)
		require.NoError(t, err)
		provBRef, err := providers.NewReference(provB.URN, provB.ID)
		require.NoError(t, err)

		derived, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Provider: provBRef.String(),
		})
		require.NoError(t, err)

		args := resource.PropertyMap{
			"__self__": resource.NewProperty(resource.ResourceReference{URN: derived.URN}),
		}

		// The inherited method is owned by pkgA: despite the pkgB goal provider, it routes to pkgA.
		_, _, _, err = monitor.Call("pkgA:m:base/method", args, nil, "", "", "", "", nil, "")
		require.NoError(t, err)
		assert.True(t, pkgACall.Load(), "the inherited method must route to pkgA")
		assert.False(t, pkgBCall.Load(), "the inherited method must not route to pkgB")

		// The derived component's own method routes to pkgB.
		_, _, _, err = monitor.Call("pkgB:m:derived/method", args, nil, "", "", "", "", nil, "")
		require.NoError(t, err)
		assert.True(t, pkgBCall.Load(), "the derived method must route to pkgB")
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
}

// TestConstructBaseCancellation verifies that a base construction blocked in the provider unwinds cleanly when the
// deployment is cancelled, rather than hanging: the monitor abandons the in-flight base construct and the update
// returns an error.
func TestConstructBaseCancellation(t *testing.T) {
	t.Parallel()

	//nolint:usetesting // the test controls cancellation; t.Context would add unintended cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// An independent context used to release the blocked base construct, driven by the provider's Cancel.
	//nolint:usetesting // released by the provider's Cancel, not by the test's lifetime
	provCtx, provCancel := context.WithCancel(context.Background())
	// Safety net so the provider never blocks forever even if Cancel is not delivered.
	time.AfterFunc(30*time.Second, provCancel)

	entered := make(chan struct{})
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructBaseF: func(
					_ context.Context,
					_ plugin.ConstructBaseRequest,
					_ *deploytest.ResourceMonitor,
				) (plugin.ConstructBaseResponse, error) {
					close(entered)
					<-provCtx.Done()
					return plugin.ConstructBaseResponse{}, nil
				},
				CancelF: func() error {
					provCancel()
					return nil
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
					_, _, err = monitor.ConstructBaseResource(
						resp.URN, "pkgA:m:base", resource.PropertyMap{}, nil, "", nil, "1.0.0", "")
					// The base construct is abandoned on cancellation; surface the error to fail the construct.
					return plugin.ConstructResponse{}, err
				},
			}, nil
		}),
	}

	// Cancel the deployment once the base construct is blocked in the provider.
	go func() {
		<-entered
		cancel()
	}()

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgB:m:derived", "res", false, deploytest.ResourceOptions{
			Remote: true,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	project := p.GetProject()
	_, err := lt.TestOp(engine.Update).RunWithContext(
		ctx, project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
}
