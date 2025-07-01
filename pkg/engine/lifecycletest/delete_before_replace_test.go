// Copyright 2020-2024, Pulumi Corporation.
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
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type propertyDependencies map[resource.PropertyKey][]resource.URN

var complexTestDependencyGraphNames = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"}

func generateComplexTestDependencyGraph(
	t *testing.T, p *lt.TestPlan,
) ([]resource.URN, *deploy.Snapshot, deploytest.LanguageRuntimeFactory) {
	resType := tokens.Type("pkgA:m:typA")

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
		propertyDeps propertyDependencies, outputs resource.PropertyMap,
	) *resource.State {
		return newResource(urn, "", id, provider, dependencies, propertyDeps, outputs, true)
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
	assert.NoError(t, old.VerifyIntegrity())

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		register := func(urn resource.URN, provider string, inputs resource.PropertyMap) resource.ID {
			resp, err := monitor.RegisterResource(urn.Type(), urn.Name(), true, deploytest.ResourceOptions{
				Provider: provider,
				Inputs:   inputs,
			})
			assert.NoError(t, err)
			return resp.ID
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

	return urns, old, programF
}

func TestDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

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

	p := &lt.TestPlan{}

	urns, old, programF := generateComplexTestDependencyGraph(t, p)
	names := complexTestDependencyGraphNames

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
	p.Options.T = t
	p.Steps = []lt.TestStep{{
		Op:            Update,
		ExpectFailure: false,
		SkipPreview:   true,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			assert.NoError(t, err)

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

			return err
		},
	}}

	p.Run(t, old)
}

func TestPropertyDependenciesAdapter(t *testing.T) {
	t.Parallel()
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		register := func(name string, inputs resource.PropertyMap, inputDeps propertyDependencies,
			dependencies []resource.URN,
		) resource.URN {
			resp, err := monitor.RegisterResource(resType, name, true, deploytest.ResourceOptions{
				Inputs:       inputs,
				Dependencies: dependencies,
				PropertyDeps: inputDeps,
			})
			assert.NoError(t, err)

			return resp.URN
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

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
		Steps:   []lt.TestStep{{Op: Update}},
	}
	snap := p.Run(t, nil)
	for _, res := range snap.Resources {
		switch res.URN {
		case urnA, urnB:
			assert.Empty(t, res.Dependencies)
			assert.Empty(t, res.PropertyDependencies)
		case urnC:
			assert.ElementsMatch(t, []resource.URN{urnA, urnB}, res.Dependencies)
			assert.ElementsMatch(t, []resource.PropertyKey{"A", "B"}, maps.Keys(res.PropertyDependencies))
			assert.ElementsMatch(t, res.Dependencies, res.PropertyDependencies["A"])
			assert.ElementsMatch(t, res.Dependencies, res.PropertyDependencies["B"])
		case urnD:
			assert.ElementsMatch(t, []resource.URN{urnA, urnB, urnC}, res.Dependencies)
			assert.ElementsMatch(t, []resource.PropertyKey{"A", "B"}, maps.Keys(res.PropertyDependencies))
			assert.ElementsMatch(t, []resource.URN{urnB}, res.PropertyDependencies["A"])
			assert.ElementsMatch(t, []resource.URN{urnA, urnC}, res.PropertyDependencies["B"])
		}
	}
}

func TestExplicitDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	dbrDiff := false
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
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
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(providers.MakeProviderType("pkgA"), "provA", true)
		assert.NoError(t, err)
		provURN = resp.URN

		provRef, err := providers.NewReference(provURN, resp.ID)
		assert.NoError(t, err)
		provA := provRef.String()

		respA, err := monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Provider:            provA,
			Inputs:              inputsA,
			DeleteBeforeReplace: dbrA,
		})
		assert.NoError(t, err)
		urnA = respA.URN

		inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {urnA}}
		respB, err := monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Provider:     provA,
			Inputs:       inputsB,
			Dependencies: []resource.URN{urnA},
			PropertyDeps: inputDepsB,
		})
		assert.NoError(t, err)
		urnB = respB.URN

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.T = t
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	// Change the value of resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	inputsA["A"] = resource.NewStringProperty("bar")
	p.Steps = []lt.TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			require.NoError(t, err)

			AssertSameStepsUnordered(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it requires delete-before-replace and change the value of resA.A. Both
	// resA and resB should be replaced, and the replacements should be delete-before-replace.
	dbrA, inputsA["A"] = &dbrValue, resource.NewStringProperty("baz")
	p.Steps = []lt.TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			require.NoError(t, err)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpCreateReplacement, URN: urnB},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Change the value of resB.A. Only resB should be replaced, and the replacement should be create-before-delete.
	inputsB["A"] = resource.NewStringProperty("qux")
	p.Steps = []lt.TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			require.NoError(t, err)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpSame, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnB},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it no longer requires delete-before-replace and change the value of
	// resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	dbrA, inputsA["A"] = nil, resource.NewStringProperty("zam")
	p.Steps = []lt.TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			require.NoError(t, err)

			AssertSameStepsUnordered(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Change the diff of resA such that it requires delete-before-replace and change the value of resA.A. Both
	// resA and resB should be replaced, and the replacements should be delete-before-replace.
	dbrDiff, inputsA["A"] = true, resource.NewStringProperty("foo")
	p.Steps = []lt.TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			require.NoError(t, err)

			AssertSameSteps(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpDeleteReplaced, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnB},
				{Op: deploy.OpCreateReplacement, URN: urnB},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	snap = p.Run(t, snap)

	// Change the registration of resA such that it disables delete-before-replace and change the value of
	// resA.A. Only resA should be replaced, and the replacement should be create-before-delete.
	dbrA, dbrValue, inputsA["A"] = &dbrValue, false, resource.NewStringProperty("bar")
	p.Steps = []lt.TestStep{{
		Op: Update,

		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			evts []Event, err error,
		) error {
			require.NoError(t, err)

			AssertSameStepsUnordered(t, []StepSummary{
				{Op: deploy.OpSame, URN: provURN},
				{Op: deploy.OpCreateReplacement, URN: urnA},
				{Op: deploy.OpReplace, URN: urnA},
				{Op: deploy.OpSame, URN: urnB},
				{Op: deploy.OpDeleteReplaced, URN: urnA},
			}, SuccessfulSteps(entries))

			return err
		},
	}}
	p.Run(t, snap)
}

func TestDependencyChangeDBR(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					if !req.OldOutputs["B"].DeepEquals(req.NewInputs["B"]) {
						return plugin.DiffResult{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResult{}, nil
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

	const resType = "pkgA:index:typ"

	inputsA := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})
	inputsB := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})

	var urnA, urnB resource.URN
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)
		urnA = resp.URN

		inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {urnA}}
		resp, err = monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Inputs:       inputsB,
			Dependencies: []resource.URN{urnA},
			PropertyDeps: inputDepsB,
		})
		assert.NoError(t, err)
		urnB = resp.URN

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Options.T = t
	p.Steps = []lt.TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	inputsA["A"] = resource.NewStringProperty("bar")
	programF = deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resp, err := monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
			Inputs: inputsB,
		})
		assert.NoError(t, err)
		urnB = resp.URN

		resp, err = monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)
		urnA = resp.URN

		return nil
	})

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p.Steps = []lt.TestStep{
		{
			Op: Update,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				evts []Event, err error,
			) error {
				assert.NoError(t, err)
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

				return err
			},
		},
	}
	p.Run(t, snap)
}

// Regression test for https://github.com/pulumi/pulumi/issues/15763. Check that if a resource gets implicated in a
// replacement chain that it fails if the resource is protected.
func TestDBRProtect(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"A"},
							DeleteBeforeReplace: true,
						}, nil
					}
					if !req.OldOutputs["B"].DeepEquals(req.NewInputs["B"]) {
						return plugin.DiffResult{
							Changes: plugin.DiffSome,
						}, nil
					}
					return plugin.DiffResult{}, nil
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

	const resType = "pkgA:index:typ"

	inputsA := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})
	inputsB := resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource(resType, "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})

		if err == nil {
			inputDepsB := map[resource.PropertyKey][]resource.URN{"A": {respA.URN}}
			protect := true
			_, err := monitor.RegisterResource(resType, "resB", true, deploytest.ResourceOptions{
				Inputs:       inputsB,
				Dependencies: []resource.URN{respA.URN},
				PropertyDeps: inputDepsB,
				Protect:      &protect,
			})
			assert.NoError(t, err)
		} else {
			assert.ErrorContains(t, err, "resource monitor shut down while waiting on step's done channel")
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{T: t, HostF: hostF}
	p := &lt.TestPlan{}

	project := p.GetProject()

	// First update just create the two resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Len(t, snap.Resources, 3)

	// Update A to trigger a replace this should error because of the protect flag on B.
	inputsA["A"] = resource.NewStringProperty("bar")
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "unable to replace resource \"urn:pulumi:test::test::pkgA:index:typ::resB\""+
		" as part of replacing \"urn:pulumi:test::test::pkgA:index:typ::resA\" as it is currently marked for protection.")

	// Remove the protect flag and try again
	assert.Equal(t, snap.Resources[2].Protect, true)
	snap.Resources[2].Protect = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
}

// Regression test for https://github.com/pulumi/pulumi/issues/19056. If a resource has "replaceOnChanges" set and a
// DBR changes that property we should trigger a replace.
func TestDBRReplaceOnChanges(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if req.Name == "resA" {
						return plugin.DiffResult{
							ReplaceKeys:         []resource.PropertyKey{"value"},
							DeleteBeforeReplace: true,
						}, nil
					}

					return plugin.DiffResult{}, nil
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

	inputsA := resource.PropertyMap{
		"value": resource.NewStringProperty("foo"),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:index:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: inputsA,
		})
		assert.NoError(t, err)

		inputDepsB := map[resource.PropertyKey][]resource.URN{"value": {respA.URN}}
		_, err = monitor.RegisterResource("pkgA:index:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"value": respA.Outputs["value"],
			},
			Dependencies:     []resource.URN{respA.URN},
			PropertyDeps:     inputDepsB,
			ReplaceOnChanges: []string{"value"},
		})
		assert.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	options := lt.TestUpdateOptions{T: t, HostF: hostF}
	p := &lt.TestPlan{}

	project := p.GetProject()

	// First update just create the two resources.
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.Len(t, snap.Resources, 3)

	// Update value to trigger a replace this should also replace resB because of the replaceOnChanges.
	inputsA["value"] = resource.NewStringProperty("bar")
	validate := func(
		project workspace.Project, target deploy.Target, entries engine.JournalEntries,
		events []engine.Event, err error,
	) error {
		// resB should have replaced before resA was created. The ReplaceOnChanges kicks in (even before the
		// fixes for #19056) so we will see a replace op, the
		resADeleted := false
		resBDeleted := false
		for _, entry := range entries {
			if entry.Kind == engine.JournalEntrySuccess && entry.Step.URN().Name() == "resA" {
				switch entry.Step.Op() {
				case deploy.OpDeleteReplaced:
					resADeleted = true
				}
			}

			if entry.Kind == engine.JournalEntrySuccess && entry.Step.URN().Name() == "resB" {
				switch entry.Step.Op() {
				case deploy.OpDeleteReplaced:
					assert.False(t, resADeleted, "resA should not have been deleted yet")
					resBDeleted = true
				}
			}
		}
		assert.True(t, resADeleted, "resA should have been deleted")
		assert.True(t, resBDeleted, "resB should have been deleted")

		return err
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, validate, "1")
	require.NoError(t, err)
	assert.Len(t, snap.Resources, 3)
}

// Regression test for a DBR issue with parallel diff. Given two resources A and B where B depends on A, if we
// re-register them such that they no longer depend on each other but A is deleteBeforeReplace, we need to
// guarantee that delete(B) happens before delete(A).
func TestDBRParallel(t *testing.T) {
	t.Parallel()

	// We're going to run this test twice, first time we'll ensure diff(A) returns first, second time we'll ensure
	// diff(B) returns first.
	for _, first := range []string{"resA", "resB"} {
		t.Run(first, func(t *testing.T) {
			t.Parallel()

			// We're going to assert that we always delete B before A
			seenDeleteB := false
			diffCalled := false
			var waitForDiff sync.WaitGroup
			waitForDiff.Add(1)

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
							if req.URN.Name() == "resB" {
								seenDeleteB = true
							}
							if req.URN.Name() == "resA" {
								assert.True(t, seenDeleteB)
							}
							return plugin.DeleteResponse{
								Status: resource.StatusOK,
							}, nil
						},
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							// If we're re-creating B ensure it was deleted first
							if diffCalled && req.URN.Name() == "resB" {
								assert.True(t, seenDeleteB)
							}
							return plugin.CreateResponse{
								ID:         "created-id",
								Properties: req.Properties,
								Status:     resource.StatusOK,
							}, nil
						},
						DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
							diffCalled = true
							// Make sure we always return diff for the first resource first
							defer func() {
								if req.URN.Name() == first {
									waitForDiff.Done()
								} else {
									// Wait for the other diff to be done then wait a second more to ensure
									// it triggers the event first.
									waitForDiff.Wait()
									time.Sleep(time.Second)
								}
							}()

							if !req.OldInputs["A"].DeepEquals(req.NewInputs["A"]) {
								return plugin.DiffResult{
									ReplaceKeys:         []resource.PropertyKey{"A"},
									DeleteBeforeReplace: true,
								}, nil
							}
							return plugin.DiffResult{}, nil
						},
					}, nil
				}),
			}

			var program func(monitor *deploytest.ResourceMonitor) error
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				return program(monitor)
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			options := lt.TestUpdateOptions{
				UpdateOptions: UpdateOptions{
					ParallelDiff: true,
					Parallel:     2,
				},
				T:                t,
				HostF:            hostF,
				SkipDisplayTests: true,
			}
			p := &lt.TestPlan{}

			project := p.GetProject()

			// First update just create the two resources.
			program = func(monitor *deploytest.ResourceMonitor) error {
				respA, err := monitor.RegisterResource("pkgA:index:typ", "resA", true, deploytest.ResourceOptions{
					Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"}),
				})
				assert.NoError(t, err)

				_, err = monitor.RegisterResource("pkgA:index:typ", "resB", true, deploytest.ResourceOptions{
					Inputs:       resource.NewPropertyMapFromMap(map[string]interface{}{"A": "foo"}),
					Dependencies: []resource.URN{respA.URN},
					PropertyDeps: map[resource.PropertyKey][]resource.URN{"A": {respA.URN}},
				})
				assert.NoError(t, err)

				return nil
			}
			snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), options, false, p.BackendClient, nil, "0")
			require.NoError(t, err)
			assert.Len(t, snap.Resources, 3)

			// Update A to trigger a replace with deleteBeforeReplace set, register B in parallel with no dependencies on A.
			program = func(monitor *deploytest.ResourceMonitor) error {
				var wg sync.WaitGroup
				wg.Add(2)

				go func() {
					_, err := monitor.RegisterResource("pkgA:index:typ", "resA", true, deploytest.ResourceOptions{
						Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"A": "bar"}),
					})
					assert.NoError(t, err)
					wg.Done()
				}()

				go func() {
					_, err = monitor.RegisterResource("pkgA:index:typ", "resB", true, deploytest.ResourceOptions{
						Inputs: resource.NewPropertyMapFromMap(map[string]interface{}{"A": "bar"}),
					})
					assert.NoError(t, err)
					wg.Done()
				}()

				wg.Wait()
				return nil
			}

			snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), options, false, p.BackendClient, nil, "1")
			require.NoError(t, err)
			assert.Len(t, snap.Resources, 3)
		})
	}
}
