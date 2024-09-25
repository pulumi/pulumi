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
	"strings"
	"testing"

	"github.com/blang/semver"
	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

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

	p := &TestPlan{}

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
	p.Steps = []TestStep{{
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
	p := &TestPlan{}

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

	p.Steps = []TestStep{{
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
	p := &TestPlan{}

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

	p.Steps = []TestStep{{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
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
	p.Steps = []TestStep{{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
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
	p.Steps = []TestStep{{
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

// Tests that "skipped creates", which are creates that are not performed
// because they are not targeted, are handled correctly when a targeted resource
// depends on a resource whose creation was skipped.
func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByTarget(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &TestPlan{}
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

	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		assert.NoError(t, err)

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	p := &TestPlan{}
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

	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		assert.NoError(t, err)

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	p := &TestPlan{}
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

	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		assert.NoError(t, err)

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	p := &TestPlan{}
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

	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		assert.NoError(t, err)

		return nil
	})

	afterHostF := deploytest.NewPluginHostF(nil, nil, afterF, afterLoaders...)

	_, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

		provID := resp.ID
		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(resp.URN, provID)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: host1F},
	}

	resA := p.NewURN("pkgA:m:typA", "resA", "")

	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{resA})
	p.Steps = []TestStep{{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
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
	p.Steps = []TestStep{{
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

	p := &TestPlan{}

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
	p.Options.SkipDisplayTests = true
	getURN := func(name string) resource.URN {
		return pickURN(t, urns, complexTestDependencyGraphNames, name)
	}

	p.Options.ReplaceTargets = deploy.NewUrnTargetsFromUrns([]resource.URN{
		getURN("F"),
		getURN("B"),
		getURN("G"),
	})

	p.Steps = []TestStep{{
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

func generateParentedTestDependencyGraph(t *testing.T, p *TestPlan) (
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
	p := &TestPlan{}

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
	p.Options.SkipDisplayTests = true
	t.Logf("Destroying targets: %v", destroyTargets)

	// If we're not forcing the targets to be destroyed, then expect to get a failure here as
	// we'll have downstream resources to delete that weren't specified explicitly.
	p.Steps = []TestStep{{
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

	p := &TestPlan{}

	project := p.GetProject()

	// Check that update succeeds despite the default provider not being targeted.
	options := TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil)
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
	p := &TestPlan{}

	project := p.GetProject()

	// Set up stack with initial two resources.
	options := TestUpdateOptions{T: t, HostF: hostF}
	origSnap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Target only `resA` and run a targeted update.
	options = TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}
	finalSnap, err := TestOp(Update).RunStep(project, p.GetTarget(t, origSnap), options, false, p.BackendClient, nil, "1")
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

	p := &TestPlan{}

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

	old, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		plan, err := TestOp(Update).Plan(project, p.GetTarget(t, old), TestUpdateOptions{
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
		plan, err := TestOp(Update).Plan(project, p.GetTarget(t, old), TestUpdateOptions{
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
		plan, err := TestOp(Update).Plan(project, p.GetTarget(t, old), TestUpdateOptions{
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
	p := &TestPlan{}

	project := p.GetProject()

	// Target only resA and check only A is created
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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

		provID := resp.ID
		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(resp.URN, provID)
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
	p := &TestPlan{}

	project := p.GetProject()

	// Target only the explicit provider and check that only the provider is created
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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

		provID := resp.ID
		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(resp.URN, provID)
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
	p := &TestPlan{}

	project := p.GetProject()

	// Target implicitX and explicitX and ensure that those, their children and the providers are created.
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		resp, err := monitor.RegisterResource("component", "parent", false)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Parent: resp.URN,
			Inputs: inputs,
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{}

	project := p.GetProject()

	//nolint:paralleltest // Requires serial access to TestPlan
	t.Run("target update", func(t *testing.T) {
		// Create all resources.
		snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
			T:     t,
			HostF: hostF,
		}, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		// Check we have 4 resources in the stack (stack, parent, provider, child)
		require.Equal(t, 4, len(snap.Resources))

		// Run an update to target the child. This works because we don't need to create the parent so can just
		// SameStep it using the data currently in state.
		inputs = resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}
		snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
			T:     t,
			HostF: hostF,
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
		snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
			T:     t,
			HostF: hostF,
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
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
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(snap)

	snap, err = TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
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
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(snap)

	snap, err = TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	validateSnap := func(snap *deploy.Snapshot) {
		assert.NotNil(t, snap)
		assert.Nil(t, snap.VerifyIntegrity())
		assert.Len(t, snap.Resources, 2)
		assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	}

	// Run an update for initial state.
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(snap)

	// Now run the targeted destroy. We expect an error because the resA errored on delete.
	// The state should still contain resA.
	snap, err = TestOp(Destroy).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
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
	originalSnap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(originalSnap)

	// Now run the targeted destroy specifying TargetDependents.
	// We expect an error because resB errored on delete.
	// The state should still contain resA and resB.
	snap, err := TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), TestUpdateOptions{
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
	snap, err = TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), TestUpdateOptions{
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

	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
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
	originalSnap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	validateSnap(originalSnap)

	// Now run the targeted destroy specifying TargetDependents.
	// We expect an error because resB errored on delete.
	// The state should still contain resA and resB.
	snap, err := TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), TestUpdateOptions{
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
	snap, err = TestOp(Destroy).RunStep(project, p.GetTarget(t, originalSnap), TestUpdateOptions{
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
	p := &TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
		assert.NoError(t, err)

		return nil
	})

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

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	hostF2 := deploytest.NewPluginHostF(nil, nil, programF2, loaders...)
	p := &TestPlan{}

	project := p.GetProject()

	//nolint:paralleltest // Requires serial access to TestPlan
	t.Run("target update", func(t *testing.T) {
		// Create all resources.
		snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
			T:     t,
			HostF: hostF,
		}, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		// Check we have 5 resources in the stack (stack, newResource, parent, provider, child)
		require.Equal(t, 5, len(snap.Resources))

		// Run an update to target the child. This works because we don't need to create the parent so can just
		// SameStep it using the data currently in state.
		inputs = resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}
		snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
		snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
			T:     t,
			HostF: hostF,
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
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := TestUpdateOptions{T: t, HostF: hostF}
	p := &TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we have 5 resources in the stack (stack, provider A, target, provider B, unrelated)
	require.Equal(t, 5, len(snap.Resources))

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
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
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

		_, err = monitor.RegisterResource("pkgB:index:typA", "unrelated1", true, deploytest.ResourceOptions{
			Inputs:  inputs,
			Version: providerVersion,
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgB:index:typA", "unrelated2", true, deploytest.ResourceOptions{
			Inputs: inputs,
			// This one always uses 1.0.0
			Version: "1.0.0",
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := TestUpdateOptions{T: t, HostF: hostF}
	p := &TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
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
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
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

		p := &TestPlan{}
		project := p.GetProject()

		snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
		t.Run("deleting the bottom of a dependency chain", func(t *testing.T) {
			// Arrange.
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			// Arrange.
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			// Arrange.
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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
			p := &TestPlan{}
			project := p.GetProject()

			snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), TestUpdateOptions{
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
			snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), TestUpdateOptions{
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

			provID := resp.ID

			if provID == "" {
				provID = providers.UnknownID
			}
			provider, err = providers.NewReference(resp.URN, provID)
			assert.NoError(t, err)
		}

		_, err = monitor.RegisterResource("pkgB:index:typA", "unrelated", true, deploytest.ResourceOptions{
			Inputs:   inputs,
			Provider: provider.String(),
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}
	p := &TestPlan{}

	project := p.GetProject()

	// Create all resources.
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	// Check we have 5 resources in the stack (stack, provider A, target, provider B, unrelated)
	require.Equal(t, 5, len(snap.Resources))
	unrelated := snap.Resources[4]
	assert.Equal(t, "unrelated", unrelated.URN.Name())
	providerRef := unrelated.Provider

	// Run an update to target the target, that also happens to change the unrelated provider.
	explicitProvider = false
	inputs = resource.PropertyMap{
		"foo": resource.NewStringProperty("bar"),
	}
	options.UpdateOptions = UpdateOptions{
		Targets: deploy.NewUrnTargets([]string{
			"**target**",
		}),
	}
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err,
		"for resource urn:pulumi:test::test::pkgB:index:typA::unrelated has not been registered yet")
	// 6 because we have the stack, provider A, target, provider B, unrelated, and the new provider B
	assert.Equal(t, 6, len(snap.Resources))
	// unrelated shouldn't have had its provider changed
	unrelated = snap.Resources[5]
	assert.Equal(t, "unrelated", unrelated.URN.Name())
	assert.Equal(t, providerRef, unrelated.Provider)
}
