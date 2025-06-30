// Copyright 2020-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package lifecycletest

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/blang/semver"
	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// We should be able to create a `A > B > C > D` hierarchy and target `B` with
// `--target-dependents`. When we do, C should be targeted as a dependent, and
// D should also be targeted as a transitive dependent.
func TestRefreshTargetChildren(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()
	var callCount int32

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					count := atomic.AddInt32(&callCount, 1)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Outputs: resource.PropertyMap{
								"count": resource.NewNumberProperty(float64(count)),
							},
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		resC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{Parent: resC.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	var a resource.URN = "urn:pulumi:test::test::pkgA:m:typA::resA"
	var b resource.URN = "urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"
	var c resource.URN = "urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA$pkgA:m:typA::resC"
	var d resource.URN = "urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA$pkgA:m:typA$pkgA:m:typA::resD"
	null := resource.NewPropertyValue(nil)

	require.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[1].URN, a)
	assert.Equal(t, snap.Resources[2].URN, b)
	assert.Equal(t, snap.Resources[3].URN, c)
	assert.Equal(t, snap.Resources[4].URN, d)

	opts = lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{b})
	opts.TargetDependents = true

	snap, err = lt.TestOp(Refresh).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[1].URN, a)
	assert.Equal(t, snap.Resources[1].Outputs["count"], null)
	assert.Equal(t, snap.Resources[2].URN, b)
	assert.NotEqual(t, snap.Resources[2].Outputs["count"], null)
	assert.Equal(t, snap.Resources[3].URN, c)
	assert.NotEqual(t, snap.Resources[3].Outputs["count"], null)
	assert.Equal(t, snap.Resources[4].URN, d)
	assert.NotEqual(t, snap.Resources[4].Outputs["count"], null)

	assert.Equal(t, callCount, int32(3))
}

func TestDestroyTarget(t *testing.T) {
	t.Parallel()

	// Try refreshing a stack with combinations of the above resources as target to destroy.
	subsets := combinations.All(complexTestDependencyGraphNames)

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, subset := range subsets {
		subset := subset
		// limit to up to 3 resources to destroy.  This keeps the test running time under
		// control as it only generates a few hundred combinations instead of several thousand.
		if len(subset) <= 3 {
			t.Run(fmt.Sprintf("%v", subset), func(t *testing.T) {
				t.Parallel()

				destroySpecificTargets(t, subset, true, /*targetDependents*/
					func(urns []resource.URN, deleted map[resource.URN]bool) {})
			})
		}
	}

	t.Run("destroy root", func(t *testing.T) {
		t.Parallel()

		destroySpecificTargets(
			t, []string{"A"}, true, /*targetDependents*/
			func(urns []resource.URN, deleted map[resource.URN]bool) {
				// when deleting 'A' we expect A, B, C, D, E, F, G, H, I, J, K, and L to be deleted
				names := complexTestDependencyGraphNames
				assert.Equal(t, map[resource.URN]bool{
					pickURN(t, urns, names, "A"): true,
					pickURN(t, urns, names, "B"): true,
					pickURN(t, urns, names, "C"): true,
					pickURN(t, urns, names, "D"): true,
					pickURN(t, urns, names, "E"): true,
					pickURN(t, urns, names, "F"): true,
					pickURN(t, urns, names, "G"): true,
					pickURN(t, urns, names, "H"): true,
					pickURN(t, urns, names, "I"): true,
					pickURN(t, urns, names, "J"): true,
					pickURN(t, urns, names, "K"): true,
					pickURN(t, urns, names, "L"): true,
				}, deleted)
			})
	})

	destroySpecificTargets(
		t, []string{"A"}, false, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {})
}

// We should be able to create a simple `A > B > C` hierarchy and exclude the
// `C` descendant.
func TestExcludeTarget(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)

	opts := lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA$pkgA:m:typA::resC",
	})

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
}

// We should be able to create a simple `A > B > C > D` hierarchy and exclude
// `B` with `ExcludeDependents`. This should exclude `C` and `D` as well.
func TestExcludeChildren(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		resC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{Parent: resC.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)

	opts := lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.ExcludeDependents = true
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB",
	})

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

// We should be able to create a simple `A > B > C` hierarchy and destroy
// `C` and by excluding `A` and `B`. We should then be able to exclude the
// provider and destroy everything under it.
func TestDestroyExcludeTarget(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	// Operation 1: deploy the three-item hierarchy
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 4)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")

	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::resA",
		"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB",
	})

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")

	require.NoError(t, err)
	require.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")

	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pulumi:providers:pkgA::default",
	})

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "2")

	require.NoError(t, err)
	require.Len(t, snap.Resources, 1)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
}

// We should be able to create a simple `A > B > C > D` hierarchy, exclude `A`,
// and with `--exclude-dependents` also exclude B, C, and D.
func TestDestroyExcludeChildren(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	// Operation 1: deploy the three-item hierarchy
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 4)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")

	opts.ExcludeDependents = true
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::resA",
	})

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 4)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")
}

// If we're not deleting everything under a provider, we should implicitly
// exclude the provider.
func TestExcludeProviderImplicitly(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")

	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::resA",
	})

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
}

// We should be able to build an `A > B > C > D` and refresh everything other
// than the `B` target.
func TestRefreshExcludeTarget(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			var callCount int32

			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					count := atomic.AddInt32(&callCount, 1)

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Outputs: resource.PropertyMap{
								"count": resource.NewNumberProperty(float64(count)),
							},
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		resC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{Parent: resC.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	null := resource.NewPropertyValue(nil)

	require.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")
	assert.Equal(t, snap.Resources[4].URN.Name(), "resD")

	opts = lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB",
	})

	snap, err = lt.TestOp(Refresh).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[2].Outputs["count"], null)

	// Order isn't guaranteed because of parallelism, so we can't check that
	// these are 1 + 2 + 3
	assert.NotEqual(t, snap.Resources[1].Outputs["count"], null)
	assert.NotEqual(t, snap.Resources[3].Outputs["count"], null)
	assert.NotEqual(t, snap.Resources[4].Outputs["count"], null)
}

// We should be able to build an `A > B > C > D` and refresh the `A` target by
// excluding the `B` target and excluding dependents. `C` should be excluded as
// a dependent, and `D` should be excluded as a transitive dependent.
func TestRefreshExcludeChildren(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			callCount := 0.0

			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					callCount++

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Outputs: resource.PropertyMap{
								"count": resource.NewNumberProperty(callCount),
							},
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{Parent: resA.URN})
		assert.NoError(t, err)

		resC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{Parent: resB.URN})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{Parent: resC.URN})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	null := resource.NewPropertyValue(nil)

	require.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default")
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")
	assert.Equal(t, snap.Resources[4].URN.Name(), "resD")

	opts = lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.ExcludeDependents = true
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB",
	})

	snap, err = lt.TestOp(Refresh).RunStep(project, p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	require.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[1].Outputs["count"], resource.NewNumberProperty(1.0))
	assert.Equal(t, snap.Resources[2].Outputs["count"], null)
	assert.Equal(t, snap.Resources[3].Outputs["count"], null)
	assert.Equal(t, snap.Resources[4].Outputs["count"], null)
}

func destroySpecificTargets(
	t *testing.T, targets []string, targetDependents bool,
	validate func(urns []resource.URN, deleted map[resource.URN]bool),
) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &lt.TestPlan{}

	urns, old, programF := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(
					_ context.Context,
					req plugin.DiffConfigRequest,
				) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(
					_ context.Context,
					req plugin.DiffRequest,
				) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"A"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.TargetDependents = targetDependents

	destroyTargets := []resource.URN{}
	for _, target := range targets {
		destroyTargets = append(destroyTargets, pickURN(t, urns, complexTestDependencyGraphNames, target))
	}

	p.Options.Targets = deploy.NewUrnTargetsFromUrns(destroyTargets)
	p.Options.T = t
	// Skip the display tests, as destroys can happen in different orders, and thus create a flaky test here.
	p.Options.SkipDisplayTests = true
	t.Logf("Destroying targets: %v", destroyTargets)

	// If we're not forcing the targets to be destroyed, then expect to get a failure here as
	// we'll have downstream resources to delete that weren't specified explicitly.
	p.Steps = []lt.TestStep{{
		Op:            Destroy,
		ExpectFailure: !targetDependents,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			deleted := make(map[resource.URN]bool)
			for _, entry := range entries {
				assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				deleted[entry.Step.URN()] = true
			}

			for _, target := range p.Options.Targets.Literals() {
				assert.Contains(t, deleted, target)
			}

			validate(urns, deleted)
			return err
		},
	}}

	p.Run(t, old)
}

func TestUpdateTarget(t *testing.T) {
	t.Parallel()

	// Try refreshing a stack with combinations of the above resources as target to destroy.
	subsets := combinations.All(complexTestDependencyGraphNames)

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, subset := range subsets {
		subset := subset
		// limit to up to 3 resources to destroy.  This keeps the test running time under
		// control as it only generates a few hundred combinations instead of several thousand.
		if len(subset) <= 3 {
			t.Run(fmt.Sprintf("update %v", subset), func(t *testing.T) {
				t.Parallel()

				updateSpecificTargets(t, subset, nil, false /*targetDependents*/, -1)
			})
		}
	}

	updateSpecificTargets(t, []string{"A"}, nil, false /*targetDependents*/, -1)

	// Also update a target that doesn't exist to make sure we don't crash or otherwise go off the rails.
	updateInvalidTarget(t)

	// We want to check that targetDependents is respected
	updateSpecificTargets(t, []string{"C"}, nil, true /*targetDependents*/, -1)

	updateSpecificTargets(t, nil, []string{"**C**"}, false, 3)
	updateSpecificTargets(t, nil, []string{"**providers:pkgA**"}, false, 3)
}

func updateSpecificTargets(t *testing.T, targets, globTargets []string, targetDependents bool, expectedUpdates int) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L
	p := &lt.TestPlan{}

	urns, old, programF := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					outputs := req.OldOutputs.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return plugin.UpdateResponse{
						Properties: outputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.TargetDependents = targetDependents
	p.Options.T = t
	updateTargets := globTargets
	for _, target := range targets {
		updateTargets = append(updateTargets,
			string(pickURN(t, urns, complexTestDependencyGraphNames, target)))
	}

	p.Options.Targets = deploy.NewUrnTargets(updateTargets)
	t.Logf("Updating targets: %v", updateTargets)

	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			updated := make(map[resource.URN]bool)
			sames := make(map[resource.URN]bool)
			for _, entry := range entries {
				if entry.Step.Op() == deploy.OpUpdate {
					updated[entry.Step.URN()] = true
				} else if entry.Step.Op() == deploy.OpSame {
					sames[entry.Step.URN()] = true
				} else {
					assert.FailNowf(t, "", "Got a step that wasn't a same/update: %v", entry.Step.Op())
				}
			}

			for _, target := range p.Options.Targets.Literals() {
				assert.Contains(t, updated, target)
			}

			if !targetDependents {
				// We should only perform updates on the entries we have targeted.
				for _, target := range p.Options.Targets.Literals() {
					assert.Contains(t, targets, target.Name())
				}
			} else {
				// We expect to find at least one other resource updates.

				// NOTE: The test is limited to only passing a subset valid behavior. By specifying
				// a URN with no dependents, no other urns will be updated and the test will fail
				// (incorrectly).
				found := false
				updateList := []string{}
				for target := range updated {
					updateList = append(updateList, target.Name())
					if !contains(targets, target.Name()) {
						found = true
					}
				}
				assert.True(t, found, "Updates: %v", updateList)
			}

			for _, target := range p.Options.Targets.Literals() {
				assert.NotContains(t, sames, target)
			}
			if expectedUpdates > -1 {
				assert.Equal(t, expectedUpdates, len(updated), "Updates = %#v", updated)
			}
			return err
		},
	}}
	p.RunWithName(t, old, strings.Join(updateTargets, ","))
}

func contains(list []string, entry string) bool {
	for _, e := range list {
		if e == entry {
			return true
		}
	}
	return false
}

func updateInvalidTarget(t *testing.T) {
	p := &lt.TestPlan{}

	_, old, programF := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
					outputs := req.OldOutputs.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return plugin.UpdateResponse{
						Properties: outputs,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{"foo"})
	t.Logf("Updating invalid targets: %v", p.Options.Targets)

	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: true,
	}}

	p.Run(t, old)
}

func TestCreateDuringTargetedUpdate_CreateMentionedAsTarget(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: host1F},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		return nil
	})
	host2F := deploytest.NewPluginHostF(nil, nil, program2F, loaders...)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")
	p.Options.HostF = host2F
	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{resA, resB})
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				if entry.Step.URN() == resA {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				} else if entry.Step.URN() == resB {
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				}
			}

			return err
		},
	}}
	p.Run(t, snap1)
}

func TestCreateDuringTargetedUpdate_UntargetedCreateNotReferenced(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: host1F},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		return nil
	})
	host2F := deploytest.NewPluginHostF(nil, nil, program2F, loaders...)

	resA := p.NewURN("pkgA:m:typA", "resA", "")

	p.Options.HostF = host2F
	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{resA})
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				// everything should be a same op here.
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}

			return err
		},
	}}
	p.Run(t, snap1)
}

// Tests that "skipped creates", which are creates that are not performed because they are not targeted, are handled
// correctly when a targeted resource that has changed inputs depends on a resource whose creation was skipped.
func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByChangedTarget(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &lt.TestPlan{}
	project := p.GetProject()

	diffBChanged := func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		if req.URN.Name() == "b" {
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
		}

		return plugin.DiffResponse{}, nil
	}

	// Act.

	// Operation 1 -- create a resource, B.
	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true)
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, beforeLoaders...)

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Operation 2 -- register a resource A, and modify B to depend on it. Target
	// B, but not A. This should fail because A's create will be skipped, meaning
	// that B's dependency cannot be satisfied.
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{DiffF: diffBChanged}, nil
		}),
	}

	afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resA.URN},
		})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**b**"}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "untargeted create")
}

// Tests that "skipped creates", which are creates that are not performed because they are not targeted, are handled
// correctly when a targeted resource that has changed dependencies (but not inputs) depends on a resource whose
// creation was skipped.
func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByUnchangedTarget(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &lt.TestPlan{}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Act.

	// Operation 1 -- create a resource, B.

	beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true)
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Operation 2 -- register a resource A, and modify B to depend on it. Target
	// B, but not A. This should fail because A's create will be skipped, meaning
	// that B's dependency cannot be satisfied.
	afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resA.URN},
		})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**b**"}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "untargeted create")
}

// Tests that "skipped creates", which are creates that are not performed
// because they are not targeted, are handled correctly when a targeted resource
// property-depends on a resource whose creation was skipped.
func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByTargetPropertyDependency(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &lt.TestPlan{}
	project := p.GetProject()

	diffBChanged := func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		if req.URN.Name() == "b" {
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
		}

		return plugin.DiffResponse{}, nil
	}

	// Act.

	// Operation 1 -- create a resource, B.
	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true)
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, beforeLoaders...)

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Operation 2 -- register a resource A, and modify B to have a property that
	// depends on it. Target B, but not A. This should fail because A's create
	// will be skipped, meaning that B's dependency cannot be satisfied.
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{DiffF: diffBChanged}, nil
		}),
	}

	afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"prop": {resA.URN},
			},
		})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**b**"}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "untargeted create")
}

// Tests that "skipped creates", which are creates that are not performed
// because they are not targeted, are handled correctly when a targeted resource
// is deleted with a resource whose creation was skipped.
func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByTargetDeletedWith(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &lt.TestPlan{}
	project := p.GetProject()

	diffBChanged := func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		if req.URN.Name() == "b" {
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
		}

		return plugin.DiffResponse{}, nil
	}

	// Act.

	// Operation 1 -- create a resource, B.
	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true)
		assert.NoError(t, err)

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, beforeLoaders...)

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Operation 2 -- register a resource A, and modify B to be deleted with it.
	// Target B, but not A. This should fail because A's create will be skipped,
	// meaning that B's dependency cannot be satisfied.
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{DiffF: diffBChanged}, nil
		}),
	}

	afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			DeletedWith: resA.URN,
		})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**b**"}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "untargeted create")
}

// Tests that "skipped creates", which are creates that are not performed
// because they are not targeted, are handled correctly when a targeted resource
// is parented to a resource whose creation was skipped.
func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByTargetParent(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &lt.TestPlan{}
	project := p.GetProject()

	diffBChanged := func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		if req.URN.Name() == "b" {
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
		}

		return plugin.DiffResponse{}, nil
	}

	// Act.

	// Operation 1 -- create a resource, B.
	beforeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	var resBOldURN resource.URN
	beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resB, err := monitor.RegisterResource("pkgA:m:typA", "b", true)
		assert.NoError(t, err)
		resBOldURN = resB.URN

		return nil
	})

	beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, beforeLoaders...)

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: beforeHostF,
	}, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)

	// Operation 2 -- register a resource A, and modify B to be parented by it.
	// Target B, but not A. This should fail because A's create will be skipped,
	// meaning that B's dependency cannot be satisfied.
	afterLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{DiffF: diffBChanged}, nil
		}),
	}

	afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
			Parent:    resA.URN,
			AliasURNs: []resource.URN{resBOldURN},
		})
		assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: afterHostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**b**"}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "untargeted create")
}

func TestCreateDuringTargetedUpdate_UntargetedProviderReferencedByTarget(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Create a resource A with --target but don't target its explicit provider.

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		provRef, err := providers.NewReference(resp.URN, resp.ID)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: host1F},
	}

	resA := p.NewURN("pkgA:m:typA", "resA", "")

	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{resA})
	p.Steps = []lt.TestStep{{
		Op: Update,
	}}
	p.Run(t, nil)
}

func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByUntargetedCreate(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: host1F},
	}

	p.Steps = []lt.TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true,
			deploytest.ResourceOptions{
				Dependencies: []resource.URN{resB},
			})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		return nil
	})
	host2F := deploytest.NewPluginHostF(nil, nil, program2F, loaders...)

	p.Options.HostF = host2F
	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{resA})
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}

			return err
		},
	}}
	p.Run(t, snap1)
}

func TestReplaceSpecificTargets(t *testing.T) {
	t.Parallel()

	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &lt.TestPlan{}

	urns, old, programF := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					// No resources will change.
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},

				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.T = t
	getURN := func(name string) resource.URN {
		return pickURN(t, urns, complexTestDependencyGraphNames, name)
	}

	p.Options.ReplaceTargets = deploy.NewUrnTargetsFromUrns([]resource.URN{
		getURN("F"),
		getURN("B"),
		getURN("G"),
	})

	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			replaced := make(map[resource.URN]bool)
			sames := make(map[resource.URN]bool)
			for _, entry := range entries {
				if entry.Step.Op() == deploy.OpReplace {
					replaced[entry.Step.URN()] = true
				} else if entry.Step.Op() == deploy.OpSame {
					sames[entry.Step.URN()] = true
				}
			}

			for _, target := range p.Options.ReplaceTargets.Literals() {
				assert.Contains(t, replaced, target)
			}

			for _, target := range p.Options.ReplaceTargets.Literals() {
				assert.NotContains(t, sames, target)
			}

			return err
		},
	}}

	p.Run(t, old)
}

var componentBasedTestDependencyGraphNames = []string{
	"A", "B", "C", "D", "E", "F", "G", "H",
	"I", "J", "K", "L", "M", "N",
}

func generateParentedTestDependencyGraph(t *testing.T, p *lt.TestPlan) (
	// Parent-child graph
	//      A               B
	//    __|__         ____|____
	//    D   I         E       F
	//  __|__         __|__   __|__
	//  G   H         J   K   L   M
	//
	// A has children D, I
	// D has children G, H
	// B has children E, F
	// E has children J, K
	// F has children L, M
	//
	// Dependency graph
	//  G        H
	//  |      __|__
	//  I      K   N
	//
	// I depends on G
	// K depends on H
	// N depends on H

	[]resource.URN, *deploy.Snapshot, deploytest.LanguageRuntimeFactory,
) {
	resTypeComponent := tokens.Type("pkgA:index:Component")
	resTypeResource := tokens.Type("pkgA:index:Resource")

	names := componentBasedTestDependencyGraphNames

	urnA := p.NewURN(resTypeComponent, names[0], "")
	urnB := p.NewURN(resTypeComponent, names[1], "")
	urnC := p.NewURN(resTypeResource, names[2], "")
	urnD := p.NewURN(resTypeComponent, names[3], urnA)
	urnE := p.NewURN(resTypeComponent, names[4], urnB)
	urnF := p.NewURN(resTypeComponent, names[5], urnB)
	urnG := p.NewURN(resTypeResource, names[6], urnD)
	urnH := p.NewURN(resTypeResource, names[7], urnD)
	urnI := p.NewURN(resTypeResource, names[8], urnA)
	urnJ := p.NewURN(resTypeResource, names[9], urnE)
	urnK := p.NewURN(resTypeResource, names[10], urnE)
	urnL := p.NewURN(resTypeResource, names[11], urnF)
	urnM := p.NewURN(resTypeResource, names[12], urnF)
	urnN := p.NewURN(resTypeResource, names[13], "")

	urns := []resource.URN{urnA, urnB, urnC, urnD, urnE, urnF, urnG, urnH, urnI, urnJ, urnK, urnL, urnM, urnN}

	newResource := func(urn, parent resource.URN, id resource.ID,
		dependencies []resource.URN, propertyDeps propertyDependencies,
	) *resource.State {
		return newResource(urn, parent, id, "", dependencies, propertyDeps,
			nil, urn.Type() != resTypeComponent)
	}

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			newResource(urnA, "", "0", nil, nil),
			newResource(urnB, "", "1", nil, nil),
			newResource(urnC, "", "2", nil, nil),
			newResource(urnD, urnA, "3", nil, nil),
			newResource(urnE, urnB, "4", nil, nil),
			newResource(urnF, urnB, "5", nil, nil),
			newResource(urnG, urnD, "6", nil, nil),
			newResource(urnH, urnD, "7", nil, nil),
			newResource(urnI, urnA, "8", []resource.URN{urnG},
				propertyDependencies{"A": []resource.URN{urnG}}),
			newResource(urnJ, urnE, "9", nil, nil),
			newResource(urnK, urnE, "10", []resource.URN{urnH},
				propertyDependencies{"A": []resource.URN{urnH}}),
			newResource(urnL, urnF, "11", nil, nil),
			newResource(urnM, urnF, "12", nil, nil),
			newResource(urnN, "", "13", []resource.URN{urnH},
				propertyDependencies{"A": []resource.URN{urnH}}),
		},
	}

	programF := deploytest.NewLanguageRuntimeF(
		func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			register := func(urn, parent resource.URN) resource.ID {
				resp, err := monitor.RegisterResource(
					urn.Type(),
					urn.Name(),
					urn.Type() != resTypeComponent,
					deploytest.ResourceOptions{
						Inputs: nil,
						Parent: parent,
					})
				assert.NoError(t, err)
				return resp.ID
			}

			register(urnA, "")
			register(urnB, "")
			register(urnC, "")
			register(urnD, urnA)
			register(urnE, urnB)
			register(urnF, urnB)
			register(urnG, urnD)
			register(urnH, urnD)
			register(urnI, urnA)
			register(urnJ, urnE)
			register(urnK, urnE)
			register(urnL, urnF)
			register(urnM, urnF)
			register(urnN, "")

			return nil
		})

	return urns, old, programF
}

func TestDestroyTargetWithChildren(t *testing.T) {
	t.Parallel()

	// when deleting 'A' with targetDependents specified we expect A, D, G, H, I, K and N to be deleted.
	destroySpecificTargetsWithChildren(
		t, []string{"A"}, true, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {
			names := componentBasedTestDependencyGraphNames
			assert.Equal(t, map[resource.URN]bool{
				pickURN(t, urns, names, "A"): true,
				pickURN(t, urns, names, "D"): true,
				pickURN(t, urns, names, "G"): true,
				pickURN(t, urns, names, "H"): true,
				pickURN(t, urns, names, "I"): true,
				pickURN(t, urns, names, "K"): true,
				pickURN(t, urns, names, "N"): true,
			}, deleted)
		})

	// when deleting 'A' with targetDependents not specified, we expect an error.
	destroySpecificTargetsWithChildren(
		t, []string{"A"}, false, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {})

	// when deleting 'B' we expect B, E, F, J, K, L, M to be deleted.
	destroySpecificTargetsWithChildren(
		t, []string{"B"}, false, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {
			names := componentBasedTestDependencyGraphNames
			assert.Equal(t, map[resource.URN]bool{
				pickURN(t, urns, names, "B"): true,
				pickURN(t, urns, names, "E"): true,
				pickURN(t, urns, names, "F"): true,
				pickURN(t, urns, names, "J"): true,
				pickURN(t, urns, names, "K"): true,
				pickURN(t, urns, names, "L"): true,
				pickURN(t, urns, names, "M"): true,
			}, deleted)
		})
}

func destroySpecificTargetsWithChildren(
	t *testing.T, targets []string, targetDependents bool,
	validate func(urns []resource.URN, deleted map[resource.URN]bool),
) {
	p := &lt.TestPlan{}

	urns, old, programF := generateParentedTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(
					_ context.Context,
					req plugin.DiffConfigRequest,
				) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"A"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.TargetDependents = targetDependents

	destroyTargets := []resource.URN{}
	for _, target := range targets {
		destroyTargets = append(destroyTargets, pickURN(t, urns, componentBasedTestDependencyGraphNames, target))
	}

	p.Options.Targets = deploy.NewUrnTargetsFromUrns(destroyTargets)
	p.Options.T = t
	t.Logf("Destroying targets: %v", destroyTargets)

	// If we're not forcing the targets to be destroyed, then expect to get a failure here as
	// we'll have downstream resources to delete that weren't specified explicitly.
	p.Steps = []lt.TestStep{{
		Op:            Destroy,
		ExpectFailure: !targetDependents,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)
			assert.True(t, len(entries) > 0)

			deleted := make(map[resource.URN]bool)
			for _, entry := range entries {
				assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				deleted[entry.Step.URN()] = true
			}

			for _, target := range p.Options.Targets.Literals() {
				assert.Contains(t, deleted, target)
			}

			validate(urns, deleted)
			return err
		},
	}}

	p.RunWithName(t, old, strings.Join(targets, ","))
}

func newResource(urn, parent resource.URN, id resource.ID, provider string, dependencies []resource.URN,
	propertyDeps propertyDependencies, outputs resource.PropertyMap, custom bool,
) *resource.State {
	inputs := resource.PropertyMap{}
	for k := range propertyDeps {
		inputs[k] = resource.NewStringProperty("foo")
	}

	return &resource.State{
		Type:                 urn.Type(),
		URN:                  urn,
		Custom:               custom,
		Delete:               false,
		ID:                   id,
		Inputs:               inputs,
		Outputs:              outputs,
		Dependencies:         dependencies,
		PropertyDependencies: propertyDeps,
		Provider:             provider,
		Parent:               parent,
	}
}

// TestTargetedCreateDefaultProvider checks that an update that targets a resource still creates the default
// provider if not targeted.
func TestTargetedCreateDefaultProvider(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{}

	project := p.GetProject()

	// Check that update succeeds despite the default provider not being targeted.
	options := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil)
	assert.NoError(t, err)

	// Check that the default provider was created.
	var foundDefaultProvider bool
	for _, res := range snap.Resources {
		if res.URN == "urn:pulumi:test::test::pulumi:providers:pkgA::default" {
			foundDefaultProvider = true
		}
	}
	assert.True(t, foundDefaultProvider)
}

// Returns the resource with the matching URN, or nil.
func findResourceByURN(rs []*resource.State, urn resource.URN) *resource.State {
	for _, r := range rs {
		if r.URN == urn {
			return r
		}
	}
	return nil
}

// TestEnsureUntargetedSame checks that an untargeted resource retains the prior state after an update when the provider
// alters the inputs. This is a regression test for pulumi/pulumi#12964.
func TestEnsureUntargetedSame(t *testing.T) {
	t.Parallel()

	// Provider that alters inputs during Check.
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CheckF: func(
					_ context.Context,
					req plugin.CheckRequest,
				) (plugin.CheckResponse, error) {
					// Pulumi GCP provider alters inputs during Check.
					req.News["__defaults"] = resource.NewStringProperty("exists")
					return plugin.CheckResponse{Properties: req.News}, nil
				},
			}, nil
		}),
	}

	// Program that creates 2 resources.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test-test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("foo"),
			},
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("bar"),
			},
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Set up stack with initial two resources.
	options := lt.TestUpdateOptions{T: t, HostF: hostF}
	origSnap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Target only `resA` and run a targeted update.
	options = lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}
	finalSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, origSnap), options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// Check that `resB` (untargeted) is the same between the two snapshots.
	{
		initialState := findResourceByURN(origSnap.Resources, "urn:pulumi:test::test::pkgA:m:typA::resB")
		assert.NotNil(t, initialState, "initial `resB` state not found")

		finalState := findResourceByURN(finalSnap.Resources, "urn:pulumi:test::test::pkgA:m:typA::resB")
		assert.NotNil(t, finalState, "final `resB` state not found")

		assert.Equal(t, initialState, finalState)
	}
}

// TestReplaceSpecificTargetsPlan checks combinations of --target and --replace for expected behavior.
func TestReplaceSpecificTargetsPlan(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Initial state
	fooVal := "bar"

	// Don't try to create resB yet.
	createResB := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test-test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty(fooVal),
			},
			ReplaceOnChanges: []string{"foo"},
		})
		assert.NoError(t, err)

		if createResB {
			// Now try to create resB which is not targeted and should show up in the plan.
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"foo": resource.NewStringProperty(fooVal),
				},
			})
			assert.NoError(t, err)
		}

		err = monitor.RegisterResourceOutputs(resp.URN, resource.PropertyMap{
			"foo": resource.NewStringProperty(fooVal),
		})

		assert.NoError(t, err)

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	project := p.GetProject()

	old, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: p.Options.HostF,
	}, false, p.BackendClient, nil)
	assert.NoError(t, err)

	// Configure next update.
	fooVal = "changed-from-bar" // This triggers a replace

	// Now try to create resB.
	createResB = true

	urnA := resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA")
	urnB := resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB")

	// `--target-replace a`
	t.Run("EnsureUntargetedIsSame", func(t *testing.T) {
		t.Parallel()
		// Create the update plan with only targeted resources.
		plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, old), lt.TestUpdateOptions{
			T:     t,
			HostF: p.Options.HostF,
			UpdateOptions: UpdateOptions{
				Experimental: true,
				GeneratePlan: true,

				// `--target-replace a` means ReplaceTargets and UpdateTargets are both set for a.
				Targets: deploy.NewUrnTargetsFromUrns([]resource.URN{
					urnA,
				}),
				ReplaceTargets: deploy.NewUrnTargetsFromUrns([]resource.URN{
					urnA,
				}),
			},
		}, p.BackendClient, nil)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		// Ensure resB is in the plan.
		foundResB := false
		for _, r := range plan.ResourcePlans {
			if r.Goal == nil {
				continue
			}
			switch r.Goal.Name {
			case "resB":
				foundResB = true
				// Ensure resB is created in the plan.
				assert.Equal(t, []display.StepOp{
					deploy.OpSame,
				}, r.Ops)
			}
		}
		assert.True(t, foundResB, "resB should be in the plan")
	})

	// `--replace a`
	t.Run("EnsureReplaceTargetIsReplacedAndNotTargeted", func(t *testing.T) {
		t.Parallel()
		// Create the update plan with only targeted resources.
		plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, old), lt.TestUpdateOptions{
			T:     t,
			HostF: p.Options.HostF,
			UpdateOptions: UpdateOptions{
				Experimental: true,
				GeneratePlan: true,

				// `--replace a` means ReplaceTargets is set. It is not a targeted update.
				// Both a and b should be changed.
				ReplaceTargets: deploy.NewUrnTargetsFromUrns([]resource.URN{
					urnA,
				}),
			},
		}, p.BackendClient, nil)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		foundResA := false
		foundResB := false
		for _, r := range plan.ResourcePlans {
			if r.Goal == nil {
				continue
			}
			switch r.Goal.Name {
			case "resA":
				foundResA = true
				assert.Equal(t, []display.StepOp{
					deploy.OpCreateReplacement,
					deploy.OpReplace,
					deploy.OpDeleteReplaced,
				}, r.Ops)
			case "resB":
				foundResB = true
				assert.Equal(t, []display.StepOp{
					deploy.OpCreate,
				}, r.Ops)
			}
		}
		assert.True(t, foundResA, "resA should be in the plan")
		assert.True(t, foundResB, "resB should be in the plan")
	})

	// `--replace a --target b`
	// This is a targeted update where the `--replace a` is irrelevant as a is not targeted.
	t.Run("EnsureUntargetedReplaceTargetIsNotReplaced", func(t *testing.T) {
		t.Parallel()
		// Create the update plan with only targeted resources.
		plan, err := lt.TestOp(Update).Plan(project, p.GetTarget(t, old), lt.TestUpdateOptions{
			T:     t,
			HostF: p.Options.HostF,
			UpdateOptions: UpdateOptions{
				Experimental: true,
				GeneratePlan: true,

				Targets: deploy.NewUrnTargetsFromUrns([]resource.URN{
					urnB,
				}),
				ReplaceTargets: deploy.NewUrnTargetsFromUrns([]resource.URN{
					urnA,
				}),
			},
		}, p.BackendClient, nil)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		foundResA := false
		foundResB := false
		for _, r := range plan.ResourcePlans {
			if r.Goal == nil {
				continue
			}
			switch r.Goal.Name {
			case "resA":
				foundResA = true
				assert.Equal(t, []display.StepOp{
					deploy.OpSame,
				}, r.Ops)
			case "resB":
				foundResB = true
				assert.Equal(t, []display.StepOp{
					deploy.OpCreate,
				}, r.Ops)
			}
		}
		assert.True(t, foundResA, "resA should be in the plan")
		assert.True(t, foundResB, "resB should be in the plan")
	})
}

func TestTargetDependents(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/pull/13560. This test ensures that when
	// --target-dependents is set we don't start creating untargted resources.
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Target only resA and check only A is created
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pkgA:m:typA::resA"}),
			TargetDependents: false,
		},
	}, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Check we only have three resources, stack, provider, and resA
	require.Equal(t, 3, len(snap.Resources))

	// Run another fresh update (note we're starting from a nil snapshot again), and target only resA and check
	// only A is created but also turn on --target-dependents.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pkgA:m:typA::resA"}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	// Check we still only have three resources, stack, provider, and resA
	require.Equal(t, 3, len(snap.Resources))
}

func TestTargetDependentsExplicitProvider(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/pull/13560. This test ensures that when
	// --target-dependents is set we still target explicit providers resources.
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resp, err := monitor.RegisterResource(
			providers.MakeProviderType("pkgA"), "provider", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		provRef, err := providers.NewReference(resp.URN, resp.ID)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Target only the explicit provider and check that only the provider is created
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pulumi:providers:pkgA::provider"}),
			TargetDependents: false,
		},
	}, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we only have two resources, stack, and provider
	require.Equal(t, 2, len(snap.Resources))

	// Run another fresh update (note we're starting from a nil snapshot again), and target only the provider
	// but turn on  --target-dependents and check the provider, A, and B are created
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pulumi:providers:pkgA::provider"}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// Check we still only have four resources, stack, provider, resA, and resB.
	require.Equal(t, 4, len(snap.Resources))
}

func TestTargetDependentsSiblingResources(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/pull/13591. This test ensures that when
	// --target-dependents is set we don't target sibling resources (that is resources created by the same
	// provider as the one being targeted).
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		// We're creating 8 resources here (one the implicit default provider). First we create three
		// pkgA:m:typA resources called "implicitX", "implicitY", and "implicitZ" (which will trigger the
		// creation of the default provider for pkgA). Second we create an explicit provider for pkgA and then
		// create three resources using that ("explicitX", "explicitY", and "explicitZ"). We want to check
		// that if we target the X resources, the Y resources aren't created, but the providers are, and the Z
		// resources are if --target-dependents is on.

		resp, err := monitor.RegisterResource("pkgA:m:typA", "implicitX", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "implicitY", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "implicitZ", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		assert.NoError(t, err)

		resp, err = monitor.RegisterResource(
			providers.MakeProviderType("pkgA"), "provider", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		provRef, err := providers.NewReference(resp.URN, resp.ID)
		assert.NoError(t, err)

		resp, err = monitor.RegisterResource("pkgA:m:typA", "explicitX", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "explicitY", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "explicitZ", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Target implicitX and explicitX and ensure that those, their children and the providers are created.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::implicitX",
				"urn:pulumi:test::test::pkgA:m:typA::explicitX",
			}),
			TargetDependents: false,
		},
	}, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we only have the 5 resources expected, the stack, the two providers and the two X resources.
	require.Equal(t, 5, len(snap.Resources))

	// Run another fresh update (note we're starting from a nil snapshot again) but turn on
	// --target-dependents and check we get 7 resources, the same set as above plus the two Z resources.
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::implicitX",
				"urn:pulumi:test::test::pkgA:m:typA::explicitX",
			}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.Equal(t, 7, len(snap.Resources))
}

// Regression test for https://github.com/pulumi/pulumi/issues/14531. This test ensures that when
// --targets is set non-targeted parents in creates trigger an error.
func TestTargetUntargetedParent(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{}

	programFF := func(expectError bool) deploytest.LanguageRuntimeFactory {
		return deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			resp, err := monitor.RegisterResource("component", "parent", false)
			require.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
				Parent: resp.URN,
				Inputs: inputs,
			})
			if expectError {
				assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			} else {
				assert.NoError(t, err)
			}

			return nil
		})
	}

	hostFF := func(expectError bool) deploytest.PluginHostFactory {
		return deploytest.NewPluginHostF(nil, nil, programFF(expectError), loaders...)
	}
	p := &lt.TestPlan{}

	project := p.GetProject()

	//nolint:paralleltest // Requires serial access to TestPlan
	t.Run("target update", func(t *testing.T) {
		// Create all resources.
		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
			T:     t,
			HostF: hostFF(false),
		}, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		// Check we have 4 resources in the stack (stack, parent, provider, child)
		require.Equal(t, 4, len(snap.Resources))

		// Run an update to target the child. This works because we don't need to create the parent so can just
		// SameStep it using the data currently in state.
		inputs = resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}
		snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
			T:     t,
			HostF: hostFF(false),
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"**child**",
				}),
			},
		}, false, p.BackendClient, nil, "1")
		require.NoError(t, err)
		assert.Equal(t, 4, len(snap.Resources))
		parentURN := snap.Resources[1].URN
		assert.Equal(t, "parent", parentURN.Name())
		assert.Equal(t, parentURN, snap.Resources[3].Parent)
	})

	//nolint:paralleltest // Requires serial access to TestPlan
	t.Run("target create", func(t *testing.T) {
		// Create all resources from scratch (nil snapshot) but only target the child. This should error that the parent
		// needs to be created.
		snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
			T:     t,
			HostF: hostFF(true),
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"**child**",
				}),
			},
		}, false, p.BackendClient, nil)
		assert.ErrorContains(t, err, "untargeted create")
		// We should have two resources the stack and the default provider we made for the child.
		assert.Equal(t, 2, len(snap.Resources))
		assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
		assert.Equal(t, tokens.Type("pulumi:providers:pkgA"), snap.Resources[1].URN.Type())
	})
}

// TestTargetDestroyDependencyErrors ensures we get an error when doing a targeted destroy of a resource that has a
// dependency and the dependency isn't specified as a target and TargetDependents isn't set.
func TestTargetDestroyDependencyErrors(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resp.URN},
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSnap := func(snap *deploy.Snapshot) {
		assert.NotNil(t, snap)
		assert.Nil(t, snap.VerifyIntegrity())
		assert.Len(t, snap.Resources, 3)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[2].URN)
	}

	// Run an update for initial state.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(snap)

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.Error(t, err) // Expect error because we didn't specify the dependency as a target or TargetDependents
	validateSnap(snap)
}

// TestTargetDestroyChildErrors ensures we get an error when doing a targeted destroy of a resource that has a
// child, and the child isn't specified as a target and TargetDependents isn't set.
func TestTargetDestroyChildErrors(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSnap := func(snap *deploy.Snapshot) {
		assert.NotNil(t, snap)
		assert.Nil(t, snap.VerifyIntegrity())
		assert.Len(t, snap.Resources, 3)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"), snap.Resources[2].URN)
	}

	// Run an update for initial state.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(snap)

	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.Error(t, err) // Expect error because we didn't specify the child as a target or TargetDependents
	validateSnap(snap)
}

// TestTargetDestroyDeleteFails ensures a resource that is part of a targeted destroy that fails to delete still
// remains in the snapshot.
func TestTargetDestroyDeleteFails(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{Status: resource.StatusUnknown}, errors.New("can't delete")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSnap := func(snap *deploy.Snapshot) {
		assert.NotNil(t, snap)
		assert.Nil(t, snap.VerifyIntegrity())
		assert.Len(t, snap.Resources, 2)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	}

	// Run an update for initial state.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(snap)

	// Now run the targeted destroy. We expect an error because the resA errored on delete.
	// The state should still contain resA.
	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
	validateSnap(snap)
}

// TestTargetDestroyDependencyDeleteFails ensures a resource that is part of a targeted destroy that fails to delete
// still remains in the snapshot.
func TestTargetDestroyDependencyDeleteFails(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					assert.Equal(t, "urn:pulumi:test::test::pkgA:m:typA::resB", string(req.URN))
					return plugin.DeleteResponse{Status: resource.StatusUnknown}, errors.New("can't delete")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resp.URN},
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSnap := func(snap *deploy.Snapshot) {
		assert.NotNil(t, snap)
		assert.Nil(t, snap.VerifyIntegrity())
		assert.Len(t, snap.Resources, 3)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[2].URN)
	}

	// Run an update for initial state.
	originalSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(originalSnap)

	// Now run the targeted destroy specifying TargetDependents.
	// We expect an error because resB errored on delete.
	// The state should still contain resA and resB.
	snap, err := lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
	validateSnap(snap)

	// Run the targeted destroy again against the original snapshot, this time explicitly specifying the targets.
	// We expect an error because resB errored on delete.
	// The state should still contain resA and resB.
	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
				"urn:pulumi:test::test::pkgA:m:typA::resB",
			}),
		},
	}, false, p.BackendClient, nil, "2")
	assert.Error(t, err)
	validateSnap(snap)
}

// TestTargetDestroyChildDeleteFails ensures a resource that is part of a targeted destroy that fails to delete
// still remains in the snapshot.
func TestTargetDestroyChildDeleteFails(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					assert.Equal(t, "urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB", string(req.URN))
					return plugin.DeleteResponse{Status: resource.StatusUnknown}, errors.New("can't delete")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSnap := func(snap *deploy.Snapshot) {
		assert.NotNil(t, snap)
		assert.Nil(t, snap.VerifyIntegrity())
		assert.Len(t, snap.Resources, 3)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"), snap.Resources[2].URN)
	}

	// Run an update for initial state.
	originalSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(originalSnap)

	// Now run the targeted destroy specifying TargetDependents.
	// We expect an error because resB errored on delete.
	// The state should still contain resA and resB.
	snap, err := lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
	validateSnap(snap)

	// Run the targeted destroy again against the original snapshot, this time explicitly specifying the targets.
	// We expect an error because resB errored on delete.
	// The state should still contain resA and resB.
	snap, err = lt.TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
				"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB",
			}),
		},
	}, false, p.BackendClient, nil, "2")
	assert.Error(t, err)
	validateSnap(snap)
}

func TestDependencyUnreleatedToTargetUpdatedSucceeds(t *testing.T) {
	// This test is a regression test for https://github.com/pulumi/pulumi/issues/12096
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "target", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "unrelated", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)

		return nil
	})

	programF2 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "target", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		resp, err := monitor.RegisterResource("pkgA:m:typA", "dep", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "unrelated", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{
				resp.URN,
			},
			Inputs: inputs,
		})
		assert.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	hostF2 := deploytest.NewPluginHostF(nil, nil, programF2, loaders...)
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
	}, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we have 4 resources in the stack (stack, parent, provider, child)
	require.Equal(t, 4, len(snap.Resources))

	// Run an update to target the target, and make sure the unrelated dependency isn't changed
	inputs = resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
		T:     t,
		HostF: hostF2,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"**target**",
			}),
		},
	}, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	assert.Equal(t, 4, len(snap.Resources))
	unrelatedURN := snap.Resources[3].URN
	assert.Equal(t, "unrelated", unrelatedURN.Name())
	assert.Equal(t, 0, len(snap.Resources[2].Dependencies))
}

func TestTargetUntargetedParentWithUpdatedDependency(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{}

	programFF := func(expectError bool) deploytest.LanguageRuntimeFactory {
		return deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "newResource", true)
			assert.NoError(t, err)
			resp, err := monitor.RegisterResource("component", "parent", false)
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
				Parent: resp.URN,
				Inputs: inputs,
			})
			if expectError {
				assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
			} else {
				assert.NoError(t, err)
			}

			return nil
		})
	}

	programF2 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resp, err := monitor.RegisterResource("pkgA:m:typA", "newResource", true)
		assert.NoError(t, err)

		respParent, err := monitor.RegisterResource("component", "parent", false, deploytest.ResourceOptions{
			Dependencies: []resource.URN{
				resp.URN,
			},
			Inputs: inputs,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Parent: respParent.URN,
			Inputs: inputs,
		})
		assert.NoError(t, err)

		return nil
	})

	hostFF := func(expectError bool) deploytest.PluginHostFactory {
		return deploytest.NewPluginHostF(nil, nil, programFF(expectError), loaders...)
	}
	hostF2 := deploytest.NewPluginHostF(nil, nil, programF2, loaders...)
	p := &lt.TestPlan{}

	project := p.GetProject()

	//nolint:paralleltest // Requires serial access to TestPlan
	t.Run("target update", func(t *testing.T) {
		// Create all resources.
		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
			T:     t,
			HostF: hostFF(false),
		}, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		// Check we have 5 resources in the stack (stack, newResource, parent, provider, child)
		require.Equal(t, 5, len(snap.Resources))

		// Run an update to target the child. This works because we don't need to create the parent so can just
		// SameStep it using the data currently in state.
		inputs = resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}
		snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
			T:     t,
			HostF: hostF2,
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"**child**",
				}),
			},
		}, false, p.BackendClient, nil, "1")
		require.NoError(t, err)
		assert.Equal(t, 5, len(snap.Resources))
		parentURN := snap.Resources[3].URN
		assert.Equal(t, "parent", parentURN.Name())
		assert.Equal(t, parentURN, snap.Resources[4].Parent)
		parentDeps := snap.Resources[3].Dependencies
		assert.Equal(t, 0, len(parentDeps))
	})

	//nolint:paralleltest // Requires serial access to TestPlan
	t.Run("target create", func(t *testing.T) {
		// Create all resources from scratch (nil snapshot) but only target the child. This should error that the parent
		// needs to be created.
		snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
			T:     t,
			HostF: hostFF(true),
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"**child**",
				}),
			},
		}, false, p.BackendClient, nil)
		assert.ErrorContains(t, err, "untargeted create")
		// We should have two resources the stack and the default provider we made for the child.
		assert.Equal(t, 2, len(snap.Resources))
		assert.Equal(t, tokens.Type("pulumi:pulumi:Stack"), snap.Resources[0].URN.Type())
		assert.Equal(t, tokens.Type("pulumi:providers:pkgA"), snap.Resources[1].URN.Type())
	})
}

func TestTargetChangeProviderVersion(t *testing.T) {
	// This test is a regression test for https://github.com/pulumi/pulumi/issues/15704
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{}

	providerVersion := "1.0.0"
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:typA", "target", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgB:index:typA", "unrelated", true, deploytest.ResourceOptions{
			Inputs:  inputs,
			Version: providerVersion,
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{T: t, HostF: hostF}
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we have 5 resources in the stack (stack, provider A, target, provider B, unrelated)
	require.Equal(t, 5, len(snap.Resources))

	// Run an update to target the target, that also happens to change the unrelated provider version.
	providerVersion = "2.0.0"
	expectError = true
	inputs = resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	options.UpdateOptions = UpdateOptions{
		Targets: deploy.NewUrnTargets([]string{
			"**target**",
		}),
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err,
		"for resource urn:pulumi:test::test::pkgB:index:typA::unrelated has not been registered yet")
	// 6 because we have the stack, provider A, target, provider B, unrelated, and the new provider B
	assert.Equal(t, 6, len(snap.Resources))
}

func TestTargetChangeAndSameProviderVersion(t *testing.T) {
	// This test is a regression test for https://github.com/pulumi/pulumi/issues/15704
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{}

	providerVersion := "1.0.0"
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:typA", "target", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)

		_, _ = monitor.RegisterResource("pkgB:index:typA", "unrelated1", true, deploytest.ResourceOptions{
			Inputs:  inputs,
			Version: providerVersion,
		})

		_, _ = monitor.RegisterResource("pkgB:index:typA", "unrelated2", true, deploytest.ResourceOptions{
			Inputs: inputs,
			// This one always uses 1.0.0
			Version: "1.0.0",
		})

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{T: t, HostF: hostF}
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we have 6 resources in the stack (stack, provider A, target, provider B, unrelated1, unrelated2)
	require.Equal(t, 6, len(snap.Resources))

	// Run an update to target the target, that also happens to change the unrelated provider version.
	providerVersion = "2.0.0"
	inputs = resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	options.UpdateOptions = UpdateOptions{
		Targets: deploy.NewUrnTargets([]string{
			"**target**",
		}),
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err,
		"for resource urn:pulumi:test::test::pkgB:index:typA::unrelated1 has not been registered yet")
	// Check we have 7 resources in the stack (stack, provider A, target, provider B, unrelated1, unrelated2, new
	// provider B)
	assert.Equal(t, 7, len(snap.Resources))
}

// Tests that resources which are modified (e.g. omitted from a program) but not
// targeted are preserved correctly during targeted operations. Specifically, if
// a resource is removed from the program but not targeted, resources which
// depend on that resource should not break. This includes checking parents,
// dependencies, property dependencies, deleted-with relationships and aliases.
// Parents and aliases are of particular interest because they result in URN
// changes.
func TestUntargetedDependencyChainsArePreserved(t *testing.T) {
	t.Parallel()

	// Arrange.
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	targetName := "target"

	// Dependencies in the presence of renames and aliases
	// ---------------------------------------------------
	//
	// Setup:
	//
	// * A
	// * B depends on A
	// * C depends on B
	// * TARGET is unrelated to all other resources
	//
	// Actions:
	//
	// * A is removed from the program
	// * B is renamed, changing its URN, but aliased to its previous URN
	// * An update targeting TARGET is performed
	t.Run("aliases", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		var bBeforeURN resource.URN

		beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			a, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{a.URN},
			})
			assert.NoError(t, err)
			bBeforeURN = b.URN

			_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{b.URN},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

		p := &lt.TestPlan{}
		project := p.GetProject()

		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
			T:     t,
			HostF: beforeHostF,
		}, false, p.BackendClient, nil, "0")
		assert.NoError(t, err)

		afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "not-b", true, deploytest.ResourceOptions{
				Aliases: []*pulumirpc.Alias{
					{
						Alias: &pulumirpc.Alias_Urn{
							Urn: string(bBeforeURN),
						},
					},
				},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{b.URN},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

		// Act.
		snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
			T:     t,
			HostF: afterHostF,
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
			},
		}, false, p.BackendClient, nil, "1")

		// Assert.
		assert.NoError(t, err)
		assert.NoError(t, snap.VerifyIntegrity())
	})

	// Chains caused by parent-child relationships
	// -------------------------------------------
	//
	// Setup:
	//
	// * A
	// * B is a child of A
	// * C is a child of B
	// * TARGET is unrelated to all other resources
	//
	t.Run("parents", func(t *testing.T) {
		t.Parallel()

		beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			a, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
				Parent: a.URN,
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				Parent: b.URN,
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

		// Actions:
		//
		// * A is removed from the program
		// * An update targeting TARGET is performed
		//nolint:paralleltest // golangci-lint v2 upgrade
		t.Run("deleting the bottom of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				b, err := monitor.RegisterResource("pkgA:m:typA", "b", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					Parent: b.URN,
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * B is removed from the program
		// * An update targeting TARGET is performed
		//nolint:paralleltest // golangci-lint v2 upgrade
		t.Run("deleting the middle of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "a", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * A is removed from the program
		// * B is removed from the program
		// * An update targeting TARGET is performed
		//nolint:paralleltest // golangci-lint v2 upgrade
		t.Run("deleting the entirety of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	})

	// Chains caused by parent-child relationships and aliasing
	// --------------------------------------------------------
	//
	// Setup:
	//
	// * A
	// * B is a child of A
	// * C is a child of B
	// * TARGET is unrelated to all other resources
	//
	//nolint:paralleltest // Not parallel since bBeforeURN and cBeforeURN are shared between tests.
	t.Run("parents/aliasing", func(t *testing.T) {
		var bBeforeURN, cBeforeURN resource.URN

		beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			a, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
				Parent: a.URN,
			})
			assert.NoError(t, err)
			bBeforeURN = b.URN

			c, err := monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				Parent: b.URN,
			})
			assert.NoError(t, err)
			cBeforeURN = c.URN

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

		// Actions:
		//
		// * A is removed from the program
		// * B is aliased to its previous URN (since the change of parent would
		//   otherwise change it)
		// * An update targeting TARGET is performed
		t.Run("deleting the bottom of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						{
							Alias: &pulumirpc.Alias_Urn{
								Urn: string(bBeforeURN),
							},
						},
					},
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					Parent: b.URN,
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * B is removed from the program
		// * C is aliased to its previous URN (since the change of parent would
		//   otherwise change it)
		// * An update targeting TARGET is performed
		t.Run("deleting the middle of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "a", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						{
							Alias: &pulumirpc.Alias_Urn{
								Urn: string(cBeforeURN),
							},
						},
					},
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * A is removed from the program
		// * B is removed from the program
		// * C is aliased to its previous URN (since the change of parent would
		//   otherwise change it)
		// * An update targeting TARGET is performed
		t.Run("deleting the entirety of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						{
							Alias: &pulumirpc.Alias_Urn{
								Urn: string(cBeforeURN),
							},
						},
					},
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	})

	// Chains caused by dependencies
	// -----------------------------
	//
	// Setup:
	//
	// * A
	// * B depends on A
	// * C depends on B
	// * TARGET is unrelated to all other resources
	t.Run("dependencies", func(t *testing.T) {
		beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			a, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{a.URN},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{b.URN},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

		// Actions:
		//
		// * A is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the bottom of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				b, err := monitor.RegisterResource("pkgA:m:typA", "b", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					Dependencies: []resource.URN{b.URN},
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * B is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the middle of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "a", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * A is removed from the program
		// * B is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the entirety of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	})

	// Chains caused by property dependencies
	// --------------------------------------
	//
	// Setup:
	//
	// * A
	// * B depends on A through property "prop"
	// * C depends on B through property "prop"
	// * TARGET is unrelated to all other resources
	t.Run("property dependencies", func(t *testing.T) {
		beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			a, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
				PropertyDeps: map[resource.PropertyKey][]resource.URN{
					"prop": {a.URN},
				},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				PropertyDeps: map[resource.PropertyKey][]resource.URN{
					"prop": {b.URN},
				},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

		// Actions:
		//
		// * A is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the bottom of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				b, err := monitor.RegisterResource("pkgA:m:typA", "b", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					PropertyDeps: map[resource.PropertyKey][]resource.URN{
						"prop": {b.URN},
					},
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * B is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the middle of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "a", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * A is removed from the program
		// * B is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the entirety of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	})

	// Chains caused by deleted-with relationships
	// -------------------------------------------
	//
	// Setup:
	//
	// * A
	// * B is deleted with A
	// * C is deleted with B
	// * TARGET is unrelated to all other resources
	t.Run("deleted with", func(t *testing.T) {
		beforeF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
			assert.NoError(t, err)

			a, err := monitor.RegisterResource("pkgA:m:typA", "a", true)
			assert.NoError(t, err)

			b, err := monitor.RegisterResource("pkgA:m:typA", "b", true, deploytest.ResourceOptions{
				DeletedWith: a.URN,
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
				DeletedWith: b.URN,
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
			assert.NoError(t, err)

			return nil
		})

		beforeHostF := deploytest.NewPluginHostF(nil, nil, beforeF, loaders...)

		// Actions:
		//
		// * A is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the bottom of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				b, err := monitor.RegisterResource("pkgA:m:typA", "b", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true, deploytest.ResourceOptions{
					DeletedWith: b.URN,
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * B is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the middle of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "a", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})

		// Actions:
		//
		// * A is removed from the program
		// * B is removed from the program
		// * An update targeting TARGET is performed
		t.Run("deleting the entirety of a dependency chain", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			p := &lt.TestPlan{}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), lt.TestUpdateOptions{
				T:     t,
				HostF: beforeHostF,
			}, false, p.BackendClient, nil, "0")
			assert.NoError(t, err)

			afterF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", "c", true)
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:m:typA", targetName, true)
				assert.NoError(t, err)

				return nil
			})

			afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, loaders...)

			// Act.
			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), lt.TestUpdateOptions{
				T:     t,
				HostF: afterHostF,
				UpdateOptions: UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{fmt.Sprintf("**%s**", targetName)}),
				},
			}, false, p.BackendClient, nil, "1")

			// Assert.
			assert.NoError(t, err)
			assert.NoError(t, snap.VerifyIntegrity())
		})
	})
}

// This test is a regression test for https://github.com/pulumi/pulumi/issues/14254. This was "fixed" by
// https://github.com/pulumi/pulumi/pull/15716 but we didn't notice. This test is to ensure that the issue stays fixed
// because we _almost_ regressed it in https://github.com/pulumi/pulumi/pull/17245.
//
// The test checks that if a resource has an explicit provider and we then run an update that changes the resource to
// use the default provider _but DON'T_ target it that we preserve its explicit provider reference in state. We do NOT
// want to change state to refer to the default provider as that can then cause provider replace diffs in a later
// update.
func TestUntargetedProviderChange(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgB", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	inputs := resource.PropertyMap{}

	explicitProvider := true
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:typA", "target", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})
		assert.NoError(t, err)

		var provider providers.Reference
		if explicitProvider {
			resp, err := monitor.RegisterResource("pulumi:providers:pkgB", "explicit", true)
			assert.NoError(t, err)

			provider, err = providers.NewReference(resp.URN, resp.ID)
			assert.NoError(t, err)
		}

		_, err = monitor.RegisterResource("pkgB:index:typA", "unrelated", true, deploytest.ResourceOptions{
			Inputs:   inputs,
			Provider: provider.String(),
		})
		if expectError {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		} else {
			assert.NoError(t, err)
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{T: t, HostF: hostF}
	p := &lt.TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we have 5 resources in the stack (stack, provider A, target, provider B, unrelated)
	require.Equal(t, 5, len(snap.Resources))
	unrelated := snap.Resources[4]
	assert.Equal(t, "unrelated", unrelated.URN.Name())
	providerRef := unrelated.Provider

	// Run an update to target the target, that also happens to change the unrelated provider.
	expectError = true
	explicitProvider = false
	inputs = resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	options.UpdateOptions = UpdateOptions{
		Targets: deploy.NewUrnTargets([]string{
			"**target**",
		}),
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err,
		"for resource urn:pulumi:test::test::pkgB:index:typA::unrelated has not been registered yet")
	// 6 because we have the stack, provider A, target, provider B, unrelated, and the new provider B
	assert.Equal(t, 6, len(snap.Resources))
	// unrelated shouldn't have had its provider changed
	unrelated = snap.Resources[5]
	assert.Equal(t, "unrelated", unrelated.URN.Name())
	assert.Equal(t, providerRef, unrelated.Provider)
}

// TestUntargetedAliasedProviderChanges tests that a provider can be renamed in a targeting update as long as there is
// an alias that enables Pulumi to spot the rename. In the absence of such an alias, Pulumi would attempt to create a
// new provider, which in the context of a targeted update could lead to an error if untargeted resources depend on the
// old provider (by its old URN) -- see TestTargetChangeProviderVersion for a test case for this.
func TestUntargetedAliasedProviderChanges(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{Project: "test", Stack: "test"}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgParent", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	var setupProvURN resource.URN
	var setupProvID resource.ID

	// Set up the initial program:
	//
	// * Parent, which is a component that will be used a parent in the next step
	// * Prov, an explicit provider instance for pkgA that we will reparent and alias in the next step
	// * Res, a resource which references Prov as its provider
	setupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgParent:pulumi:typParent", "parent", false)
		assert.NoError(t, err)

		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true)
		require.NoError(t, err)

		setupProvURN = prov.URN
		setupProvID = prov.ID

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:typA", "res", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})

	setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, loaders...)
	setupOptions := lt.TestUpdateOptions{T: t, HostF: setupHostF}

	setupSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), setupOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Set up the test program:
	//
	// * Reparent Prov to Parent
	// * Alias Prov to its old URN
	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		parent, err := monitor.RegisterResource("pkgParent:pulumi:typParent", "parent", false)
		assert.NoError(t, err)

		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{
			Parent:    parent.URN,
			AliasURNs: []resource.URN{setupProvURN},
		})
		require.NoError(t, err)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:index:typA", "res", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})

	// Run a targeted update that does not target any resources. Since providers are implicitly targeted, and since we
	// gave Prov an alias, it should be renamed to its new URN (and subsequently, the set of resources that we didn't
	// target should have their provider references updated to reflect the new URN).
	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, loaders...)
	reproOptions := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**non-existing**"}),
		},
	}

	reproSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, setupSnap), reproOptions, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	assert.Equal(t, 3, len(reproSnap.Resources))

	expectedProvRef, err := providers.NewReference(
		resource.NewURN(
			tokens.QName(p.Stack),
			tokens.PackageName(p.Project),
			"pkgParent:pulumi:typParent",
			"pulumi:providers:pkgA",
			"prov",
		),
		setupProvID,
	)
	require.NoError(t, err)
	require.Equal(t, expectedProvRef.String(), reproSnap.Resources[2].Provider)
}

// TestUntargetedSameStepsAcceptDeletedResources tests that if untargeted resources depend on partially-replaced
// resources (that is, those with Delete: true) that those resources are skipped (since creating SameSteps would panic)
// and copied over as usual by the snapshot persistence layer.
func TestUntargetedSameStepsAcceptDeletedResources(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	// Operation 1 -- set up an initial state
	//
	// We initialise the following resources:
	//
	// * P
	// * A, which has P as a parent
	// * B, which has a dependency on A (in this case, a deleted-with relationship)
	loaders1 := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF1 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resP, err := monitor.RegisterResource("pkgA:m:typA", "resP", true)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Parent: resP.URN,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			DeletedWith: resA.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	hostF1 := deploytest.NewPluginHostF(nil, nil, programF1, loaders1...)
	opts1 := lt.TestUpdateOptions{T: t, HostF: hostF1}

	snap1, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), opts1, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The 3 we defined plus the default provider for pkgA.
	require.Len(t, snap1.Resources, 4)

	// Operation 2 -- attempt to reparent A using a targeted update that requires a replace
	//
	// We change the program as follows:
	//
	// * P is unchanged
	// * A no longer has P as a parent, but is *aliased* to its old URN (where P was its parent)
	// * B is unchanged (still depends on A via a deleted-with relationship)
	//
	// We set up a Diff operation that will report A as needing (create-before-)replacement. We set up a Delete operation
	// that will fail for A. This means that if we try to replace A:
	//
	// * We'll create a new A with a new ID with A's new (parentless) URN
	// * We'll marked the old A (with the parented URN) as Delete: true
	// * We'll attempt to delete the old A and fail
	// * Our state will thus contain both As before we continue with the operation
	//
	// With the stage set, we run a targeted update which *only targets A*. Since B is not targeted, we'll copy it over
	// as-is. As part of this, we'll traverse its dependencies to see if any need to be copied over as well (since they
	// may not have been registered or targeted).
	//
	// B's dependencies will reference the old, parented, URN, meaning we'll pluck the old A out of the state with Delete:
	// true on it. Ordinarily this kind of thing wouldn't happen -- Delete: true resources are filtered out before step
	// generation and in an untargeted update, there is no part of step generation that "looks backwards" at resources
	// that have already been processed. As a result, we need to test that this case is handled properly and that we don't
	// e.g. panic.
	loaders2 := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
					if req.URN.Name() == "resA" {
						return plugin.DiffResponse{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"__replace"},
						}, nil
					}

					return plugin.DiffResponse{}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					if req.URN.Name() == "resA" {
						return plugin.DeleteResponse{}, errors.New("failed to delete resA")
					}

					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}),
	}

	programF2 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resP, err := monitor.RegisterResource("pkgA:m:typA", "resP", true)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(resP.URN),
							},
						},
					},
				},
			},
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			DeletedWith: resA.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	hostF2 := deploytest.NewPluginHostF(nil, nil, programF2, loaders2...)
	opts2 := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF2,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{"**resA**"}),
		},
	}

	snap2, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, snap1), opts2, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "failed to delete resA")

	// The 3 we defined plus the deleted A and the default provider for pkgA.
	require.Len(t, snap2.Resources, 5)

	require.Equal(t, snap2.Resources[2].URN.Name(), "resA")
	require.False(t, snap2.Resources[2].Delete, "New A should not be deleted")

	// We expect the old A to be at the end of the snapshot since it is marked for deletion.
	require.Equal(t, snap2.Resources[4].URN.Name(), "resA")
	require.True(t, snap2.Resources[4].Delete, "Old A should be deleted")

	// Operation 3 -- complete the replacement of A
	//
	// This time we arrange for all provider operations to succeed, meaning that the old version of A with Delete: true
	// will be cleaned up correctly.
	loaders3 := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF3 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resP", true)
		assert.NoError(t, err)

		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			DeletedWith: resA.URN,
		})
		assert.NoError(t, err)

		return nil
	})

	hostF3 := deploytest.NewPluginHostF(nil, nil, programF3, loaders3...)
	opts3 := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF3,
	}

	snap3, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, snap2), opts3, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	// The 3 we defined plus the default provider for pkgA.
	require.Len(t, snap3.Resources, 4)

	// There should just be the one A now (the replacement), and it should not be marked for deletion.
	require.Equal(t, snap3.Resources[2].URN.Name(), "resA")
	require.False(t, snap3.Resources[2].Delete, "A should not be deleted")
}

// TestUntargetedResourceAnalyzer tests that if a resource is removed from a program, but is not targeted it
// is still sent for stack analysis.
func TestUntargetedResourceAnalyzer(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	// Operation 1 -- set up an initial state
	//
	// We initialise the following resources:
	//
	// * A and B

	// Analyzer paths are always absolute paths
	analyzerPath, err := filepath.Abs("test")
	require.NoError(t, err)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader(analyzerPath, func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				AnalyzeStackF: func(resources []plugin.AnalyzerStackResource) ([]plugin.AnalyzeDiagnostic, error) {
					// We expect to see resA and resB in the analysis.
					var foundA, foundB bool
					for _, res := range resources {
						if res.URN.Name() == "resA" {
							foundA = true
						} else if res.URN.Name() == "resB" {
							foundB = true
						}
					}
					assert.True(t, foundA, "Expected to find resA in analysis")
					assert.True(t, foundB, "Expected to find resB in analysis")
					return nil, nil
				},
			}, nil
		}),
	}

	skipB := false
	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		if !skipB {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
			assert.NoError(t, err)
		}
		return nil
	})

	host := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: host,
		UpdateOptions: UpdateOptions{
			LocalPolicyPacks: []LocalPolicyPack{
				{
					Path: "test",
				},
			},
		},
	}
	snap1, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The 2 we defined plus the default provider for pkgA.
	require.Len(t, snap1.Resources, 3)

	// Operation 2 -- run again but skip B but only target A, we should still see B in the analysis.
	skipB = true

	opts.Targets = deploy.NewUrnTargets([]string{"**resA**"})

	snap2, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, snap1), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// The 2 we defined plus the default provider for pkgA.
	require.Len(t, snap2.Resources, 3)
}

// TestUntargetedRefreshedProviderUpdate is a regression test for
// https://github.com/pulumi/pulumi/issues/19879. If we refresh a resource that refers to an old provider that
// isn't registered in the current update we then would hit an assert later on in the analysis phase.
func TestUntargetedRefreshedProviderUpdate(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	// Operation 1 -- set up an initial state
	//
	// We initialise the following resources:
	//
	// * A and B

	// Analyzer paths are always absolute paths
	analyzerPath, err := filepath.Abs("test")
	require.NoError(t, err)

	version := "1.0.0"
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkgA", semver.MustParse("2.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader(analyzerPath, func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				AnalyzeStackF: func(resources []plugin.AnalyzerStackResource) ([]plugin.AnalyzeDiagnostic, error) {
					// We expect to see resA and resB in the analysis. resA should have the new provider and
					// resB should have the old provider.
					var foundA, foundB bool
					for _, res := range resources {
						if res.URN.Name() == "resA" {
							foundA = true
							expected := "default_" + strings.ReplaceAll(version, ".", "_")
							assert.Equal(t, expected, res.Provider.Name)
						} else if res.URN.Name() == "resB" {
							foundB = true
							assert.Equal(t, "default_1_0_0", res.Provider.Name)
						}
					}
					assert.True(t, foundA, "Expected to find resA in analysis")
					assert.True(t, foundB, "Expected to find resB in analysis")
					return nil, nil
				},
			}, nil
		}),
	}

	skipB := false
	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Version: version,
		})
		assert.NoError(t, err)

		if !skipB {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Version: version,
			})
			assert.NoError(t, err)
		}
		return nil
	})

	host := deploytest.NewPluginHostF(nil, nil, program, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: host,
		UpdateOptions: UpdateOptions{
			LocalPolicyPacks: []LocalPolicyPack{
				{
					Path: "test",
				},
			},
		},
	}
	snap1, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// The 2 we defined plus the default provider for pkgA.
	require.Len(t, snap1.Resources, 3)

	// Operation 2 -- run again but skip B but only target A, _but_ also do a refresh (which ignores targets)
	// we shouldn't get a panic but instead just see the old provider resource for B.
	skipB = true
	version = "2.0.0"

	opts.Targets = deploy.NewUrnTargets([]string{"**resA**"})
	opts.Refresh = true

	snap2, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, snap1), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	// The 2 we defined plus the new and old default provider for pkgA
	require.Len(t, snap2.Resources, 4)
}
