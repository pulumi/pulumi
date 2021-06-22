//nolint:goconst
package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var complexTestDependencyGraphNames = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}

func generateComplexTestDependencyGraph(
	t *testing.T, p *TestPlan) ([]resource.URN, *deploy.Snapshot, plugin.LanguageRuntime) {

	resType := tokens.Type("pkgA:m:typA")
	type propertyDependencies map[resource.PropertyKey][]resource.URN

	names := complexTestDependencyGraphNames

	urnA := p.NewProviderURN("pkgA", names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewProviderURN("pkgA", names[2], "")
	urnD := p.NewProviderURN("pkgA", names[3], "")
	urnE := p.NewURN(resType, names[4], "")
	urnF := p.NewURN(resType, names[5], "")
	urnG := p.NewURN(resType, names[6], "")
	urnH := p.NewURN(resType, names[7], "")
	urnI := p.NewURN(resType, names[8], "")
	urnJ := p.NewURN(resType, names[9], "")
	urnK := p.NewURN(resType, names[10], "")
	urnL := p.NewURN(resType, names[11], "")

	urns := []resource.URN{
		urnA, urnB, urnC, urnD, urnE, urnF,
		urnG, urnH, urnI, urnJ, urnK, urnL,
	}

	newResource := func(urn resource.URN, id resource.ID, provider string, dependencies []resource.URN,
		propertyDeps propertyDependencies, outputs resource.PropertyMap) *resource.State {

		inputs := resource.PropertyMap{}
		for k := range propertyDeps {
			inputs[k] = resource.NewStringProperty("foo")
		}

		return &resource.State{
			Type:                 urn.Type(),
			URN:                  urn,
			Custom:               true,
			Delete:               false,
			ID:                   id,
			Inputs:               inputs,
			Outputs:              outputs,
			Dependencies:         dependencies,
			Provider:             provider,
			PropertyDependencies: propertyDeps,
		}
	}

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			newResource(urnA, "0", "", nil, nil, resource.PropertyMap{"A": resource.NewStringProperty("foo")}),
			newResource(urnB, "1", string(urnA)+"::0", nil, nil, nil),
			newResource(urnC, "2", "",
				[]resource.URN{urnA},
				propertyDependencies{"A": []resource.URN{urnA}},
				resource.PropertyMap{"A": resource.NewStringProperty("bar")}),
			newResource(urnD, "3", "",
				[]resource.URN{urnA},
				propertyDependencies{"B": []resource.URN{urnA}}, nil),
			newResource(urnE, "4", string(urnC)+"::2", nil, nil, nil),
			newResource(urnF, "5", "",
				[]resource.URN{urnC},
				propertyDependencies{"A": []resource.URN{urnC}}, nil),
			newResource(urnG, "6", "",
				[]resource.URN{urnC},
				propertyDependencies{"B": []resource.URN{urnC}}, nil),
			newResource(urnH, "4", string(urnD)+"::3", nil, nil, nil),
			newResource(urnI, "5", "",
				[]resource.URN{urnD},
				propertyDependencies{"A": []resource.URN{urnD}}, nil),
			newResource(urnJ, "6", "",
				[]resource.URN{urnD},
				propertyDependencies{"B": []resource.URN{urnD}}, nil),
			newResource(urnK, "7", "",
				[]resource.URN{urnF, urnG},
				propertyDependencies{"A": []resource.URN{urnF, urnG}}, nil),
			newResource(urnL, "8", "",
				[]resource.URN{urnF, urnG},
				propertyDependencies{"B": []resource.URN{urnF, urnG}}, nil),
		},
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		register := func(urn resource.URN, provider string, inputs resource.PropertyMap) resource.ID {
			_, id, _, err := monitor.RegisterResource(urn.Type(), string(urn.Name()), true, deploytest.ResourceOptions{
				Provider: provider,
				Inputs:   inputs,
			})
			assert.NoError(t, err)
			return id
		}

		idA := register(urnA, "", resource.PropertyMap{"A": resource.NewStringProperty("bar")})
		register(urnB, string(urnA)+"::"+string(idA), nil)
		idC := register(urnC, "", nil)
		idD := register(urnD, "", nil)
		register(urnE, string(urnC)+"::"+string(idC), nil)
		register(urnF, "", nil)
		register(urnG, "", nil)
		register(urnH, string(urnD)+"::"+string(idD), nil)
		register(urnI, "", nil)
		register(urnJ, "", nil)
		register(urnK, "", nil)
		register(urnL, "", nil)

		return nil
	})

	return urns, old, program
}

func TestDeleteBeforeReplace(t *testing.T) {
	//             A
	//    _________|_________
	//    B        C        D
	//          ___|___  ___|___
	//          E  F  G  H  I  J
	//             |__|
	//             K  L
	//
	// For a given resource R in (A, C, D):
	// - R will be the provider for its first dependent
	// - A change to R will require that its second dependent be replaced
	// - A change to R will not require that its third dependent be replaced
	//
	// In addition, K will have a requires-replacement property that depends on both F and G, and
	// L will have a normal property that depends on both F and G.
	//
	// With that in mind, the following resources should require replacement: A, B, C, E, F, and K

	p := &TestPlan{}

	urns, old, program := generateComplexTestDependencyGraph(t, p)
	names := complexTestDependencyGraphNames

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

	p.Steps = []TestStep{{
		Op:            Update,
		ExpectFailure: false,
		SkipPreview:   true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			replaced := make(map[resource.URN]bool)
			for _, entry := range entries {
				if entry.Step.Op() == deploy.OpReplace {
					replaced[entry.Step.URN()] = true
				}
			}

			assert.Equal(t, map[resource.URN]bool{
				pickURN(t, urns, names, "A"): true,
				pickURN(t, urns, names, "B"): true,
				pickURN(t, urns, names, "C"): true,
				pickURN(t, urns, names, "E"): true,
				pickURN(t, urns, names, "F"): true,
				pickURN(t, urns, names, "K"): true,
			}, replaced)

			return res
		},
	}}

	p.Run(t, old)
}

func TestPropertyDependenciesAdapter(t *testing.T) {
	// Ensure that the eval source properly shims in property dependencies if none were reported (and does not if
	// any were reported).

	type propertyDependencies map[resource.PropertyKey][]resource.URN

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	const resType = "pkgA:m:typA"
	var urnA, urnB, urnC, urnD resource.URN
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {

		register := func(name string, inputs resource.PropertyMap, inputDeps propertyDependencies,
			dependencies []resource.URN) resource.URN {

			urn, _, _, err := monitor.RegisterResource(resType, name, true, deploytest.ResourceOptions{
				Inputs:       inputs,
				Dependencies: dependencies,
				PropertyDeps: inputDeps,
			})
			assert.NoError(t, err)

			return urn
		}

		urnA = register("A", nil, nil, nil)
		urnB = register("B", nil, nil, nil)
		urnC = register("C", resource.PropertyMap{
			"A": resource.NewStringProperty("foo"),
			"B": resource.NewStringProperty("bar"),
		}, nil, []resource.URN{urnA, urnB})
		urnD = register("D", resource.PropertyMap{
			"A": resource.NewStringProperty("foo"),
			"B": resource.NewStringProperty("bar"),
		}, propertyDependencies{
			"A": []resource.URN{urnB},
			"B": []resource.URN{urnA, urnC},
		}, []resource.URN{urnA, urnB, urnC})

		return nil
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update}},
	}
	snap := p.Run(t, nil)
	for _, res := range snap.Resources {
		switch res.URN {
		case urnA, urnB:
			assert.Empty(t, res.Dependencies)
			assert.Empty(t, res.PropertyDependencies)
		case urnC:
			assert.Equal(t, []resource.URN{urnA, urnB}, res.Dependencies)
			assert.EqualValues(t, propertyDependencies{
				"A": res.Dependencies,
				"B": res.Dependencies,
			}, res.PropertyDependencies)
		case urnD:
			assert.Equal(t, []resource.URN{urnA, urnB, urnC}, res.Dependencies)
			assert.EqualValues(t, propertyDependencies{
				"A": []resource.URN{urnB},
				"B": []resource.URN{urnA, urnC},
			}, res.PropertyDependencies)
		}
	}
}

func TestExplicitDeleteBeforeReplace(t *testing.T) {
	p := &TestPlan{}

	dbrDiff := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: dbrDiff,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	const resType = "pkgA:index:typ"

	inputsA := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})
	dbrValue, dbrA := true, (*bool)(nil)
	inputsB := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})

	var provURN, urnA, urnB resource.URN
	var provID resource.ID
	var err error
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		provURN, provID, _, err = monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)

		if provID == "" {
			provID = providers.UnknownID
		}
		provRef, err := providers.NewReference(provURN, provID)
		assert.NoError(t, err)
		provA := provRef.String()

		urnA, _, _, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Provider:            provA,
			Inputs:              inputsA,
			DeleteBeforeReplace: dbrA,
		})
		assert.NoError(t, err)

		inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {urnA}}
		urnB, _, _, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Provider:     provA,
			Inputs:       inputsB,
			Dependencies: []resource.URN{urnA},
			PropertyDeps: inputDepsB,
		})
		assert.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	// Change the value of resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	inputsA["A"] = resource.NewStringProperty("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it requires delete-before-replace and change the value of resA.A. Both
	// resA and resB should be replaced, and the replacements should be delete-before-replace.
	dbrA, inputsA["A"] = &dbrValue, resource.NewStringProperty("baz")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpCreateReplacement, URN: urnB},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the value of resB.A. Only resB should be replaced, and the replacement should be create-before-delete.
	inputsB["A"] = resource.NewStringProperty("qux")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpSame, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnB},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it no longer requires delete-before-replace and change the value of
	// resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	dbrA, inputsA["A"] = nil, resource.NewStringProperty("zam")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the diff of resA such that it requires delete-before-replace and change the value of resA.A. Both
	// resA and resB should be replaced, and the replacements should be delete-before-replace.
	dbrDiff, inputsA["A"] = true, resource.NewStringProperty("foo")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpCreateReplacement, URN: urnB},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it disables delete-before-replace and change the value of
	// resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	dbrA, dbrValue, inputsA["A"] = &dbrValue, false, resource.NewStringProperty("bar")
	p.Steps = []TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, res result.Result) result.Result {

			assert.Nil(t, res)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return res
		},
	}}
	p.Run(t, snap)
}

func TestDependencyChangeDBR(t *testing.T) {
	p := &TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(urn resource.URN, id resource.ID,
					olds, news resource.PropertyMap, ignoreChanges []string) (plugin.DiffResult, error) {

					if !olds["A"].DeepEquals(news["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					if !olds["B"].DeepEquals(news["B"]) {
						return plugin.DiffResult{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	const resType = "pkgA:index:typ"

	inputsA := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})
	inputsB := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})

	var urnA, urnB resource.URN
	var err error
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnA, _, _, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)

		inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {urnA}}
		urnB, _, _, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Inputs:       inputsB,
			Dependencies: []resource.URN{urnA},
			PropertyDeps: inputDepsB,
		})
		assert.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	inputsA["A"] = resource.NewStringProperty("bar")
	program = deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnB, _, _, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Inputs: inputsB,
		})
		assert.NoError(t, err)

		urnA, _, _, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)

		return nil
	})

	p.Options.Host = deploytest.NewPluginHost(nil, nil, program, loaders...)
	p.Steps = []TestStep{
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, res result.Result) result.Result {

				assert.Nil(t, res)
				assert.True(t, len(entries) > 0)

				resBDeleted, resBSame := false, false
				for _, entry := range entries {
					if entry.Step.URN() == urnB {
						switch entry.Step.Op() {
						case deploy.OpDelete, deploy.OpDeleteReplaced:
							resBDeleted = true
						case deploy.OpSame:
							resBSame = true
						}
					}
				}
				assert.True(t, resBSame)
				assert.False(t, resBDeleted)

				return res
			},
		},
	}
	p.Run(t, snap)
}
