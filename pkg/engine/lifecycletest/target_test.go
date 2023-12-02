package lifecycletest

import (
	"fmt"
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
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["A"].DeepEquals(newInputs["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["A"].DeepEquals(newInputs["A"]) {
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
				DiffF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					timeout float64, ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					outputs := oldOutputs.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return outputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.TargetDependents = targetDependents

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
	p.Run(t, old)
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
				DiffF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					timeout float64, ignoreChanges []string, preview bool,
				) (resource.PropertyMap, resource.Status, error) {
					outputs := oldOutputs.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return outputs, resource.StatusOK, nil
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
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

func TestCreateDuringTargetedUpdate_UntargetedCreateReferencedByTarget(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program1F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
	p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true,
			deploytest.ResourceOptions{
				Dependencies: []resource.URN{resB},
			})
		assert.NoError(t, err)

		return nil
	})
	host2F := deploytest.NewPluginHostF(nil, nil, program2F, loaders...)

	p.Options.HostF = host2F
	p.Options.Targets = deploy.NewUrnTargetsFromUrns([]resource.URN{resA})
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
	}}
	p.Run(t, nil)
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
		provURN, provID, _, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: host1F},
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1F := deploytest.NewPluginHostF(nil, nil, program1F, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: host1F},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2F := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true,
			deploytest.ResourceOptions{
				Dependencies: []resource.URN{resB},
			})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true)
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
				DiffF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					// No resources will change.
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},

				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)

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
				_, id, _, err := monitor.RegisterResource(
					urn.Type(),
					urn.Name(),
					urn.Type() != resTypeComponent,
					deploytest.ResourceOptions{
						Inputs: nil,
						Parent: parent,
					})
				assert.NoError(t, err)
				return id
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
				DiffConfigF: func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["A"].DeepEquals(newInputs["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["A"].DeepEquals(newInputs["A"]) {
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
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{}

	project := p.GetProject()

	// Check that update succeeds despite the default provider not being targeted.
	options := TestUpdateOptions{
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
				CheckF: func(urn resource.URN,
					olds, news resource.PropertyMap, _ []byte,
				) (resource.PropertyMap, []plugin.CheckFailure, error) {
					// Pulumi GCP provider alters inputs during Check.
					news["__defaults"] = resource.NewStringProperty("exists")
					return news, nil, nil
				},
			}, nil
		}),
	}

	// Program that creates 2 resources.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test-test", false)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty("foo"),
			},
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
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
	options := TestUpdateOptions{HostF: hostF}
	origSnap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil)
	require.NoError(t, err)

	// Target only `resA` and run a targeted update.
	options = TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::resA",
			}),
		},
	}
	finalSnap, err := TestOp(Update).Run(project, p.GetTarget(t, origSnap), options, false, p.BackendClient, nil)
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
		stackURN, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test-test", false)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"foo": resource.NewStringProperty(fooVal),
			},
			ReplaceOnChanges: []string{"foo"},
		})
		assert.NoError(t, err)

		if createResB {
			// Now try to create resB which is not targeted and should show up in the plan.
			_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{
					"foo": resource.NewStringProperty(fooVal),
				},
			})
			assert.NoError(t, err)
		}

		err = monitor.RegisterResourceOutputs(stackURN, resource.PropertyMap{
			"foo": resource.NewStringProperty(fooVal),
		})

		assert.NoError(t, err)

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	project := p.GetProject()

	old, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
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
		_, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{}

	project := p.GetProject()

	// Target only resA and check only A is created
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pkgA:m:typA::resA"}),
			TargetDependents: false,
		},
	}, false, p.BackendClient, nil)
	require.NoError(t, err)
	// Check we only have three resources, stack, provider, and resA
	require.Equal(t, 3, len(snap.Resources))

	// Run another fresh update (note we're starting from a nil snapshot again), and target only resA and check
	// only A is created but also turn on --target-dependents.
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pkgA:m:typA::resA"}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil)
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
		_, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		provURN, provID, _, err := monitor.RegisterResource(
			providers.MakeProviderType("pkgA"), "provider", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{}

	project := p.GetProject()

	// Target only the explicit provider and check that only the provider is created
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pulumi:providers:pkgA::provider"}),
			TargetDependents: false,
		},
	}, false, p.BackendClient, nil)
	require.NoError(t, err)
	// Check we only have two resources, stack, and provider
	require.Equal(t, 2, len(snap.Resources))

	// Run another fresh update (note we're starting from a nil snapshot again), and target only the provider
	// but turn on  --target-dependents and check the provider, A, and B are created
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets:          deploy.NewUrnTargets([]string{"urn:pulumi:test::test::pulumi:providers:pkgA::provider"}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil)
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
		_, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		// We're creating 8 resources here (one the implicit default provider). First we create three
		// pkgA:m:typA resources called "implicitX", "implicitY", and "implicitZ" (which will trigger the
		// creation of the default provider for pkgA). Second we create an explicit provider for pkgA and then
		// create three resources using that ("explicitX", "explicitY", and "explicitZ"). We want to check
		// that if we target the X resources, the Y resources aren't created, but the providers are, and the Z
		// resources are if --target-dependents is on.

		implicitX, _, _, err := monitor.RegisterResource("pkgA:m:typA", "implicitX", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "implicitY", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "implicitZ", true, deploytest.ResourceOptions{
			Parent: implicitX,
		})
		assert.NoError(t, err)

		provURN, provID, _, err := monitor.RegisterResource(
			providers.MakeProviderType("pkgA"), "provider", true, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}

		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)

		explicitX, _, _, err := monitor.RegisterResource("pkgA:m:typA", "explicitX", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "explicitY", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "explicitZ", true, deploytest.ResourceOptions{
			Parent: explicitX,
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{}

	project := p.GetProject()

	// Target implicitX and explicitX and ensure that those, their children and the providers are created.
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::implicitX",
				"urn:pulumi:test::test::pkgA:m:typA::explicitX",
			}),
			TargetDependents: false,
		},
	}, false, p.BackendClient, nil)
	require.NoError(t, err)
	// Check we only have the 5 resources expected, the stack, the two providers and the two X resources.
	require.Equal(t, 5, len(snap.Resources))

	// Run another fresh update (note we're starting from a nil snapshot again) but turn on
	// --target-dependents and check we get 7 resources, the same set as above plus the two Z resources.
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
		HostF: hostF,
		UpdateOptions: UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test::test::pkgA:m:typA::implicitX",
				"urn:pulumi:test::test::pkgA:m:typA::explicitX",
			}),
			TargetDependents: true,
		},
	}, false, p.BackendClient, nil)
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
		_, _, _, err := monitor.RegisterResource("pulumi:pulumi:Stack", "test", false)
		assert.NoError(t, err)

		parent, _, _, err := monitor.RegisterResource("component", "parent", false)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "child", true, deploytest.ResourceOptions{
			Parent: parent,
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
		snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), TestUpdateOptions{
			HostF: hostF,
		}, false, p.BackendClient, nil)
		require.NoError(t, err)
		// Check we have 4 resources in the stack (stack, parent, provider, child)
		require.Equal(t, 4, len(snap.Resources))

		// Run an update to target the child. This works because we don't need to create the parent so can just
		// SameStep it using the data currently in state.
		inputs = resource.PropertyMap{
			"foo": resource.NewStringProperty("bar"),
		}
		snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), TestUpdateOptions{
			HostF: hostF,
			UpdateOptions: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{
					"**child**",
				}),
			},
		}, false, p.BackendClient, nil)
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
