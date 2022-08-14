package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDestroyTarget(t *testing.T) {
	t.Parallel()

	// Try refreshing a stack with combinations of the above resources as target to destroy.
	subsets := combinations.All(complexTestDependencyGraphNames)

	for _, subset := range subsets {
		// limit to up to 3 resources to destroy.  This keeps the test running time under
		// control as it only generates a few hundred combinations instead of several thousand.
		if len(subset) <= 3 {
			destroySpecificTargets(t, subset, true, /*targetDependents*/
				func(urns []resource.URN, deleted map[resource.URN]bool) {})
		}
	}

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

	destroySpecificTargets(
		t, []string{"A"}, false, /*targetDependents*/
		func(urns []resource.URN, deleted map[resource.URN]bool) {})
}

func destroySpecificTargets(
	t *testing.T, targets []string, targetDependents bool,
	validate func(urns []resource.URN, deleted map[resource.URN]bool)) {

	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"A"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Options.TargetDependents = targetDependents

	destroyTargets := []resource.URN{}
	for _, target := range targets {
		destroyTargets = append(destroyTargets, pickURN(t, urns, complexTestDependencyGraphNames, target))
	}

	p.Options.DestroyTargets = destroyTargets
	t.Logf("Destroying targets: %v", destroyTargets)

	// If we're not forcing the targets to be destroyed, then expect to get a failure here as
	// we'll have downstream resources to delete that weren't specified explicitly.
	p.Steps = []TestStep{{
		Op:            Destroy,
		ExpectFailure: !targetDependents,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			deleted := make(map[resource.URN]bool)
			for _, entry := range entries {
				assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				deleted[entry.Step.URN()] = true
			}

			for _, target := range p.Options.DestroyTargets {
				assert.Contains(t, deleted, target)
			}

			validate(urns, deleted)
			return res
		},
	}}

	p.Run(t, old)
}

func TestUpdateTarget(t *testing.T) {
	t.Parallel()

	// Try refreshing a stack with combinations of the above resources as target to destroy.
	subsets := combinations.All(complexTestDependencyGraphNames)

	for _, subset := range subsets {
		// limit to up to 3 resources to destroy.  This keeps the test running time under
		// control as it only generates a few hundred combinations instead of several thousand.
		if len(subset) <= 3 {
			updateSpecificTargets(t, subset, false /*targetDependents*/)
		}
	}

	updateSpecificTargets(t, []string{"A"}, false /*targetDependents*/)

	// Also update a target that doesn't exist to make sure we don't crash or otherwise go off the rails.
	updateInvalidTarget(t)

	// We want to check that targetDependents is respected
	updateSpecificTargets(t, []string{"C"}, true /*targetDependents*/)
}

func updateSpecificTargets(t *testing.T, targets []string, targetDependents bool) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					outputs := olds.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return outputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Options.TargetDependents = targetDependents

	updateTargets := []resource.URN{}
	for _, target := range targets {
		updateTargets = append(updateTargets,
			pickURN(t, urns, complexTestDependencyGraphNames, target))
	}

	p.Options.UpdateTargets = updateTargets
	t.Logf("Updating targets: %v", updateTargets)

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
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

			for _, target := range p.Options.UpdateTargets {
				assert.Contains(t, updated, target)
			}

			if !targetDependents {
				// We should only perform updates on the entries we have targeted.
				for _, target := range p.Options.UpdateTargets {
					assert.Contains(t, targets, target.Name().String())
				}
			} else {
				// We expect to find at least one other resource updates.

				// NOTE: The test is limited to only passing a subset valid behavior. By specifying
				// a URN with no dependents, no other urns will be updated and the test will fail
				// (incorrectly).
				found := false
				updateList := []string{}
				for target := range updated {
					updateList = append(updateList, target.Name().String())
					if !contains(targets, target.Name().String()) {
						found = true
					}
				}
				assert.True(t, found, "Updates: %v", updateList)
			}

			for _, target := range p.Options.UpdateTargets {
				assert.NotContains(t, sames, target)
			}

			return res
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

	_, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// all resources will change.
					return plugin.DiffResult{
						Changes: plugin.DiffSome,
					}, nil
				},

				UpdateF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap, timeout float64,
					ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

					outputs := olds.Copy()

					outputs["output_prop"] = resource.NewPropertyValue(42)
					return outputs, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.UpdateTargets = []resource.URN{"foo"}
	t.Logf("Updating invalid targets: %v", p.Options.UpdateTargets)

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

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")
	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA, resB}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				if entry.Step.URN() == resA {
					assert.Equal(t, deploy.OpSame, entry.Step.Op())
				} else if entry.Step.URN() == resB {
					assert.Equal(t, deploy.OpCreate, entry.Step.Op())
				}
			}

			return res
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

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	// Now, create a resource resB.  This shouldn't be a problem since resB isn't referenced by anything.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	resA := p.NewURN("pkgA:m:typA", "resA", "")

	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				// everything should be a same op here.
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}

			return res
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

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resA", true,
			deploytest.ResourceOptions{
				Dependencies: []resource.URN{resB},
			})
		assert.NoError(t, err)

		return nil
	})
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: true,
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

	program1 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host1 := deploytest.NewPluginHost(nil, nil, program1, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host1},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap1 := p.Run(t, nil)

	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	// Now, create a resource resB.  But reference it from A. This will cause a dependency we can't
	// satisfy.
	program2 := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
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
	host2 := deploytest.NewPluginHost(nil, nil, program2, loaders...)

	p.Options.Host = host2
	p.Options.UpdateTargets = []resource.URN{resA}
	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			for _, entry := range entries {
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}

			return res
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

	urns, old, program := generateComplexTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {

					// No resources will change.
					return plugin.DiffResult{Changes: plugin.DiffNone}, nil
				},

				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)

	getURN := func(name string) resource.URN {
		return pickURN(t, urns, complexTestDependencyGraphNames, name)
	}

	p.Options.ReplaceTargets = []resource.URN{
		getURN("F"),
		getURN("B"),
		getURN("G"),
	}

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
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

			for _, target := range p.Options.ReplaceTargets {
				assert.Contains(t, replaced, target)
			}

			for _, target := range p.Options.ReplaceTargets {
				assert.NotContains(t, sames, target)
			}

			return res
		},
	}}

	p.Run(t, old)
}

var componentBasedTestDependencyGraphNames = []string{"A", "B", "C", "D", "E", "F", "G", "H",
	"I", "J", "K", "L", "M", "N"}

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

	[]resource.URN, *deploy.Snapshot, plugin.LanguageRuntime) {
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
		dependencies []resource.URN, propertyDeps propertyDependencies) *resource.State {
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

	program := deploytest.NewLanguageRuntime(
		func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			register := func(urn, parent resource.URN) resource.ID {
				_, id, _, err := monitor.RegisterResource(
					urn.Type(),
					string(urn.Name()),
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

	return urns, old, program
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
	validate func(urns []resource.URN, deleted map[resource.URN]bool)) {

	p := &TestPlan{}

	urns, old, program := generateParentedTestDependencyGraph(t, p)

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffConfigF: func(urn resource.URN, olds, news resource.PropertyMap,
					ignoreChanges []string) (plugin.DiffResult, error) {
					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{ReplaceKeys: []resource.PropertyKey{"A"}}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Options.TargetDependents = targetDependents

	destroyTargets := []resource.URN{}
	for _, target := range targets {
		destroyTargets = append(destroyTargets, pickURN(t, urns, componentBasedTestDependencyGraphNames, target))
	}

	p.Options.DestroyTargets = destroyTargets
	t.Logf("Destroying targets: %v", destroyTargets)

	// If we're not forcing the targets to be destroyed, then expect to get a failure here as
	// we'll have downstream resources to delete that weren't specified explicitly.
	p.Steps = []TestStep{{
		Op:            Destroy,
		ExpectFailure: !targetDependents,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)
			assert.True(t, len(entries) > 0)

			deleted := make(map[resource.URN]bool)
			for _, entry := range entries {
				assert.Equal(t, deploy.OpDelete, entry.Step.Op())
				deleted[entry.Step.URN()] = true
			}

			for _, target := range p.Options.DestroyTargets {
				assert.Contains(t, deleted, target)
			}

			validate(urns, deleted)
			return res
		},
	}}

	p.Run(t, old)
}

func newResource(urn, parent resource.URN, id resource.ID, provider string, dependencies []resource.URN,
	propertyDeps propertyDependencies, outputs resource.PropertyMap, custom bool) *resource.State {

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
