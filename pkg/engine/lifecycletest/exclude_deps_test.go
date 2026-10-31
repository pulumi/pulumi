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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Models https://github.com/pulumi/pulumi/issues/22694: an excluded subtree whose
// siblings share old-state dependencies, while other resources those siblings
// depend on are dropped and recreated (new URNs) by the new program. Every
// write of the snapshot must keep dependencies before their dependents.
func excludeReproLoaders() []*deploytest.ProviderLoader {
	return []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
}

// registerExcludeReproTree registers: two shared dependencies, two ref
// components with one child each, and an outer/inner component subtree whose
// "task" and "policy" children both property-depend on the shared and ref
// children. refNames picks the ref component names so a later program can swap
// which refs exist.
func registerExcludeReproTree(t *testing.T, monitor *deploytest.ResourceMonitor, refNames []string) {
	register := func(ty, name string, custom bool, opts deploytest.ResourceOptions) resource.URN {
		res, err := monitor.RegisterResource(tokens.Type(ty), name, custom, opts)
		require.NoError(t, err)
		return res.URN
	}

	sharedA := register("pkgA:m:typA", "sharedA", true, deploytest.ResourceOptions{})
	sharedB := register("pkgA:m:typA", "sharedB", true, deploytest.ResourceOptions{})

	var refData []resource.URN
	for _, n := range refNames {
		comp := register("pkgA:m:typC", n, false, deploytest.ResourceOptions{})
		data := register("pkgA:m:typA", n+"Data", true, deploytest.ResourceOptions{Parent: comp})
		refData = append(refData, data)
	}

	outer := register("pkgA:m:typC", "outer", false, deploytest.ResourceOptions{})
	inner := register("pkgA:m:typC", "inner", false, deploytest.ResourceOptions{Parent: outer})
	local := register("pkgA:m:typA", "local", true, deploytest.ResourceOptions{Parent: inner})

	taskDeps := append(append([]resource.URN{sharedA, sharedB}, refData...), local)
	register("pkgA:m:typA", "task", true, deploytest.ResourceOptions{
		Parent:       inner,
		PropertyDeps: map[resource.PropertyKey][]resource.URN{"combined": taskDeps},
	})
	register("pkgA:m:typA", "policy", true, deploytest.ResourceOptions{
		Parent:       inner,
		PropertyDeps: map[resource.PropertyKey][]resource.URN{"refs": refData},
	})
	register("pkgA:m:typA", "svc", true, deploytest.ResourceOptions{
		Parent:       inner,
		Dependencies: []resource.URN{taskDeps[len(taskDeps)-1]},
	})
}

func TestExcludeSiblingsSharingDepsWithSwappedRefs(t *testing.T) {
	t.Parallel()

	project := (&lt.TestPlan{}).GetProject()

	// Step 1: build the initial state with refs alpha/beta.
	var v1Snap *deploy.Snapshot
	var innerURN resource.URN
	{
		p := &lt.TestPlan{}
		program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			registerExcludeReproTree(t, monitor, []string{"alpha", "beta"})
			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, program, nil, nil, excludeReproLoaders()...)
		opts := lt.TestUpdateOptions{T: t, HostF: hostF}

		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		v1Snap = snap

		for _, r := range snap.Resources {
			if r.URN.Name() == "inner" {
				innerURN = r.URN
			}
		}
		require.NotEmpty(t, innerURN)
	}

	// Step 2: swap the refs for delta/epsilon while excluding inner (and,
	// transitively, its children). Any checkpoint written mid-update must
	// still be a valid topological order.
	for i := 0; i < 25; i++ {
		p := &lt.TestPlan{}
		program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			registerExcludeReproTree(t, monitor, []string{"delta", "epsilon"})
			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, program, nil, nil, excludeReproLoaders()...)
		opts := lt.TestUpdateOptions{T: t, HostF: hostF}
		opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{innerURN})
		opts.ExcludeDependents = true

		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, v1Snap), opts, false, p.BackendClient, nil, "1")
		require.NoErrorf(t, err, "iteration %d", i)

		names := []string{}
		for _, r := range snap.Resources {
			names = append(names, r.URN.Name())
		}
		t.Logf("iteration %d resources: %v", i, names)
		assert.Contains(t, names, "policy")
		assert.Contains(t, names, "alphaData")
	}
}

// In the shape above the SDK happens to register the shared dependencies before
// the excluded tree, so the excluded tree's dependency walk only has to *wait*
// for them. When a dependency edge is removed in the program (here: the
// excluded children no longer reference the shared resources), the SDK no
// longer sequences the registrations, and the excluded tree can register
// *first*. The dependency walk then emits speculative same steps for the
// shared resources — and the program registers them for real right after.
func TestExcludeRegistersBeforeTargetedDeps(t *testing.T) {
	t.Parallel()

	project := (&lt.TestPlan{}).GetProject()

	urn := func(name string) resource.URN {
		return resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name)
	}
	compURN := func(parent, name string) resource.URN {
		if parent == "" {
			return resource.URN("urn:pulumi:test::test::pkgA:m:typC::" + name)
		}
		return resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typC::" + name)
	}

	var v1Snap *deploy.Snapshot
	{
		p := &lt.TestPlan{}
		program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			registerExcludeReproTree(t, monitor, []string{"alpha", "beta"})
			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, program, nil, nil, excludeReproLoaders()...)
		opts := lt.TestUpdateOptions{T: t, HostF: hostF}

		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		v1Snap = snap
	}

	// Register the excluded subtree *before* the shared resources and refs it
	// (transitively) depends on in the old state.
	for i := 0; i < 25; i++ {
		p := &lt.TestPlan{}
		program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			register := func(ty, name string, custom bool, opts deploytest.ResourceOptions) resource.URN {
				res, err := monitor.RegisterResource(tokens.Type(ty), name, custom, opts)
				require.NoError(t, err)
				return res.URN
			}

			sharedA := urn("sharedA")
			sharedB := urn("sharedB")
			alphaData := urn("alphaData")
			betaData := urn("betaData")
			inner := compURN("outer", "inner")
			local := urn("local")

			register("pkgA:m:typC", "outer", false, deploytest.ResourceOptions{})
			register("pkgA:m:typC", "inner", false, deploytest.ResourceOptions{Parent: compURN("", "outer")})
			register("pkgA:m:typA", "local", true, deploytest.ResourceOptions{Parent: inner})

			taskDeps := []resource.URN{sharedA, sharedB, alphaData, betaData, local}
			register("pkgA:m:typA", "task", true, deploytest.ResourceOptions{
				Parent:       inner,
				PropertyDeps: map[resource.PropertyKey][]resource.URN{"combined": taskDeps},
			})
			register("pkgA:m:typA", "policy", true, deploytest.ResourceOptions{
				Parent:       inner,
				PropertyDeps: map[resource.PropertyKey][]resource.URN{"refs": {alphaData, betaData}},
			})
			register("pkgA:m:typA", "svc", true, deploytest.ResourceOptions{
				Parent:       inner,
				Dependencies: []resource.URN{local},
			})

			// The targeted resources register only now, after the excluded tree.
			register("pkgA:m:typA", "sharedA", true, deploytest.ResourceOptions{})
			register("pkgA:m:typA", "sharedB", true, deploytest.ResourceOptions{})
			delta := register("pkgA:m:typC", "delta", false, deploytest.ResourceOptions{})
			register("pkgA:m:typA", "deltaData", true, deploytest.ResourceOptions{Parent: delta})
			epsilon := register("pkgA:m:typC", "epsilon", false, deploytest.ResourceOptions{})
			register("pkgA:m:typA", "epsilonData", true, deploytest.ResourceOptions{Parent: epsilon})
			return nil
		})
		hostF := deploytest.NewPluginHostF(nil, nil, program, nil, nil, excludeReproLoaders()...)
		opts := lt.TestUpdateOptions{T: t, HostF: hostF}
		opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{compURN("outer", "inner")})
		opts.ExcludeDependents = true

		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, v1Snap), opts, false, p.BackendClient, nil, "1")
		require.NoErrorf(t, err, "iteration %d", i)
		_ = snap
	}
}

// Two held-back same steps may depend on one another when the program registers the dependent
// before its dependency. The flush that emits them must still order dependencies first.
func TestExcludeDeferredFlushOrdersDependenciesFirst(t *testing.T) {
	t.Parallel()

	project := (&lt.TestPlan{}).GetProject()
	urn := func(name string) resource.URN {
		return resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name)
	}

	v2 := false
	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		// z has no deps, x depends on z, y depends on x. The first deployment registers
		// dependency-first; the second registers dependent-first, which the removal of the
		// dependency edges from the program makes possible.
		if !v2 {
			_, err := monitor.RegisterResource("pkgA:m:typA", "z", true)
			require.NoError(t, err)
			_, err = monitor.RegisterResource("pkgA:m:typA", "x", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{urn("z")},
			})
			require.NoError(t, err)
			_, err = monitor.RegisterResource("pkgA:m:typA", "y", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{urn("x")},
			})
			require.NoError(t, err)
			return nil
		}
		_, err := monitor.RegisterResource("pkgA:m:typA", "y", true)
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "x", true)
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "z", true)
		require.NoError(t, err)
		return nil
	})

	// First deployment: everything registers normally (dependent-first order is fine here
	// because nothing is excluded, so the engine sees the dependency edges).
	hostF := deploytest.NewPluginHostF(nil, nil, program, nil, nil, excludeReproLoaders()...)
	var v1Snap *deploy.Snapshot
	{
		p := &lt.TestPlan{}
		opts := lt.TestUpdateOptions{T: t, HostF: hostF}
		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), opts, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		v1Snap = snap
	}

	// Second deployment: everything is excluded. y registers first and is held back behind
	// unregistered x; x is held back behind unregistered z. The flush must still order z < x < y.
	v2 = true
	p := &lt.TestPlan{}
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{urn("x"), urn("y"), urn("z")})

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, v1Snap), opts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
	require.NoError(t, snap.VerifyIntegrity())
	idx := map[resource.URN]int{}
	for i, r := range snap.Resources {
		idx[r.URN] = i
	}
	require.Less(t, idx[urn("z")], idx[urn("x")])
	require.Less(t, idx[urn("x")], idx[urn("y")])
}

// If a held-back step is released mid-deployment (here because a targeted resource depends on
// it), the release speculatively copies forward any dependency that has not registered yet. When
// such a dependency registers later *targeted*, its old state is already written in the right
// place and cannot be retracted or written twice, so the deployment bails with the duplicate URN
// error instead of corrupting the snapshot.
func TestExcludeTargetedRegistrationOfSpeculativelyCopiedDepBails(t *testing.T) {
	t.Parallel()

	project := (&lt.TestPlan{}).GetProject()
	urn := func(name string) resource.URN {
		return resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name)
	}

	v2 := false
	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if !v2 {
			// Dependency-first: x, then y (depends on x), then t (depends on y).
			_, err := monitor.RegisterResource("pkgA:m:typA", "x", true)
			require.NoError(t, err)
			_, err = monitor.RegisterResource("pkgA:m:typA", "y", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{urn("x")},
			})
			require.NoError(t, err)
			_, err = monitor.RegisterResource("pkgA:m:typA", "t", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{urn("y")},
			})
			require.NoError(t, err)
			return nil
		}
		// y is excluded and depends on x. t is targeted and depends on y, forcing y's held-back
		// step out mid-deployment, which speculatively copies x forward. x registers targeted
		// last and must bail.
		_, err := monitor.RegisterResource("pkgA:m:typA", "y", true)
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "t", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{urn("y")},
		})
		require.NoError(t, err)
		// The deployment bails on this registration; the response may error or never arrive, so
		// nothing after this may touch t.
		_, _ = monitor.RegisterResource("pkgA:m:typA", "x", true)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, program, nil, nil, excludeReproLoaders()...)

	// First deployment: everything registers normally.
	var v1Snap *deploy.Snapshot
	{
		p := &lt.TestPlan{}
		plainOpts := lt.TestUpdateOptions{T: t, HostF: hostF}
		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), plainOpts, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		v1Snap = snap
	}

	v2 = true
	p := &lt.TestPlan{}
	opts := lt.TestUpdateOptions{T: t, HostF: hostF}
	opts.Excludes = deploy.NewUrnTargetsFromUrns([]resource.URN{urn("y")})
	_, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, v1Snap), opts, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "Duplicate resource URN")
}
