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
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/blang/semver"
	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestParallelRefresh(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Create a program that registers four resources, each of which depends on the resource that immediately precedes
	// it.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{respA.URN},
		})
		assert.NoError(t, err)

		respC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{respB.URN},
		})
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{respC.URN},
		})
		assert.NoError(t, err)

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{
			T:             t,
			HostF:         hostF,
			UpdateOptions: UpdateOptions{Parallel: 4},
			// Skip display tests because different ordering makes the colouring different.
			SkipDisplayTests: true,
		},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")
	assert.Equal(t, snap.Resources[4].URN.Name(), "resD")

	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[3].URN.Name(), "resC")
	assert.Equal(t, snap.Resources[4].URN.Name(), "resD")
}

func TestExternalRefresh(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Our program reads a resource and exits.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "", "", "")
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
		Steps:   []TestStep{{Op: Update}},
	}

	// The read should place "resA" in the snapshot with the "External" bit set.
	snap := p.RunWithName(t, nil, "0")
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.True(t, snap.Resources[1].External)

	p = &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
		Steps:   []TestStep{{Op: Refresh}},
	}

	snap = p.RunWithName(t, snap, "1")
	// A refresh should leave "resA" as it is in the snapshot. The External bit should still be set.
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.True(t, snap.Resources[1].External)
}

// External resources should only have their outputs diffed during a refresh, in
// line with the "legacy" implementation. Consequently Diff should not be called
// for them. This test checks that case.
func TestExternalRefreshDoesNotCallDiff(t *testing.T) {
	t.Parallel()

	readCall := 0
	diffCalled := false

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					readCall++
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Outputs: resource.PropertyMap{
								"o1": resource.NewNumberProperty(float64(readCall)),
							},
						},
						Status: resource.StatusOK,
					}, nil
				},

				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					diffCalled = true
					return plugin.DiffResult{}, nil
				},
			}, nil
		}),
	}

	// Our program reads a resource and exits.
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "", "", "")
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
		Steps:   []TestStep{{Op: Update}},
	}

	snap := p.RunWithName(t, nil, "0")

	p = &TestPlan{
		Options: TestUpdateOptions{T: t, HostF: hostF},
		Steps:   []TestStep{{Op: Refresh}},
	}

	p.RunWithName(t, snap, "1")

	assert.False(t, diffCalled, "Refresh should not diff external resources")
}

func TestRefreshInitFailure(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	res2URN := p.NewURN("pkgA:m:typA", "resB", "")

	res2Outputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}

	//
	// Refresh will persist any initialization errors that are returned by `Read`. This provider
	// will error out or not based on the value of `refreshShouldFail`.
	//
	refreshShouldFail := false

	//
	// Set up test environment to use `readFailProvider` as the underlying resource provider.
	//
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if refreshShouldFail && req.URN == resURN {
						err := &plugin.InitError{
							Reasons: []string{"Refresh reports continued to fail to initialize"},
						}
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Outputs: resource.PropertyMap{},
							},
							Status: resource.StatusPartialFailure,
						}, err
					} else if req.URN == res2URN {
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Outputs: res2Outputs,
							},
							Status: resource.StatusOK,
						}, nil
					}
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							Outputs: resource.PropertyMap{},
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options.HostF = hostF
	p.Options.T = t
	//
	// Create an old snapshot with a single initialization failure.
	//
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:       resURN.Type(),
				URN:        resURN,
				Custom:     true,
				ID:         "0",
				Inputs:     resource.PropertyMap{},
				Outputs:    resource.PropertyMap{},
				InitErrors: []string{"Resource failed to initialize"},
			},
			{
				Type:    res2URN.Type(),
				URN:     res2URN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
		},
	}

	//
	// Refresh DOES NOT fail, causing the initialization error to disappear.
	//
	p.Steps = []TestStep{{Op: Refresh}}
	snap := p.Run(t, old)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Empty(t, resource.InitErrors)
		case res2URN:
			assert.Equal(t, res2Outputs, resource.Outputs)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}

	//
	// Refresh again, see the resource is in a partial state of failure, but the refresh operation
	// DOES NOT fail. The initialization error is still persisted.
	//
	refreshShouldFail = true
	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}
	snap = p.Run(t, old)
	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Equal(t, []string{"Refresh reports continued to fail to initialize"}, resource.InitErrors)
		case res2URN:
			assert.Equal(t, res2Outputs, resource.Outputs)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}
}

// Test that tests that Refresh can detect that resources have been deleted and removes them
// from the snapshot.
func TestRefreshWithDelete(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, parallelFactor := range []int32{1, 4} {
		parallelFactor := parallelFactor
		t.Run(fmt.Sprintf("parallel-%d", parallelFactor), func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
							// This thing doesn't exist. Returning nil from Read should trigger
							// the engine to delete it from the snapshot.
							return plugin.ReadResponse{}, nil
						},
					}, nil
				}),
			}

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				assert.NoError(t, err)
				return err
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
			p := &TestPlan{
				Options: TestUpdateOptions{
					T: t,
					// Skip display tests because different ordering makes the colouring different.
					SkipDisplayTests: true,
					HostF:            hostF,
					UpdateOptions:    UpdateOptions{Parallel: parallelFactor},
				},
			}

			p.Steps = []TestStep{{Op: Update}}
			snap := p.RunWithName(t, nil, "0")

			p.Steps = []TestStep{{Op: Refresh}}
			snap = p.RunWithName(t, snap, "1")

			// Refresh succeeds and records that the resource in the snapshot doesn't exist anymore
			provURN := p.NewProviderURN("pkgA", "default", "")
			assert.Len(t, snap.Resources, 1)
			assert.Equal(t, provURN, snap.Resources[0].URN)
		})
	}
}

// Tests that dependencies are correctly rewritten when refresh removes deleted resources.
func TestRefreshDeleteDependencies(t *testing.T) {
	t.Parallel()

	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshDeleteCombination(t, names, []string{}, "pre")

	for i, subset := range subsets {
		validateRefreshDeleteCombination(t, names, subset, strconv.Itoa(i))
	}
}

func TestRefreshDeletePropertyDependencies(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if req.URN.Name() == "resA" {
						return plugin.ReadResponse{}, nil
					}

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"propB1": {resA.URN},
			},
		})

		assert.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{Options: TestUpdateOptions{T: t, HostF: hostF}}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[2].PropertyDependencies["propB1"][0].Name(), "resA")

	err := snap.VerifyIntegrity()
	assert.NoError(t, err)

	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resB")
	assert.Empty(t, snap.Resources[1].PropertyDependencies)

	err = snap.VerifyIntegrity()
	assert.NoError(t, err)
}

func TestRefreshDeleteDeletedWith(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					if req.URN.Name() == "resA" {
						return plugin.ReadResponse{}, nil
					}

					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			DeletedWith: resA.URN,
		})

		assert.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{Options: TestUpdateOptions{T: t, HostF: hostF}}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, snap.Resources[2].URN.Name(), "resB")
	assert.Equal(t, snap.Resources[2].DeletedWith.Name(), "resA")

	err := snap.VerifyIntegrity()
	assert.NoError(t, err)

	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, snap.Resources[0].URN.Name(), "default") // provider
	assert.Equal(t, snap.Resources[1].URN.Name(), "resB")
	assert.Empty(t, snap.Resources[1].DeletedWith)

	err = snap.VerifyIntegrity()
	assert.NoError(t, err)
}

// Looks up the provider ID in newResources and sets "Provider" to reference that in every resource in oldResources.
func setProviderRef(t *testing.T, oldResources, newResources []*resource.State, provURN resource.URN) {
	for _, r := range newResources {
		if r.URN == provURN {
			provRef, err := providers.NewReference(r.URN, r.ID)
			assert.NoError(t, err)
			for i := range oldResources {
				oldResources[i].Provider = provRef.String()
			}
			break
		}
	}
}

func validateRefreshDeleteCombination(t *testing.T, names []string, targets []string, name string) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewURN(resType, names[2], "")
	urns := []resource.URN{urnA, urnB, urnC}

	refreshTargets := []resource.URN{}

	t.Logf("Refreshing targets: %v", targets)
	for _, target := range targets {
		refreshTargets = append(refreshTargets, pickURN(t, urns, names, target))
	}

	p.Options.Targets = deploy.NewUrnTargetsFromUrns(refreshTargets)

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	oldResources := []*resource.State{
		newResource(urnA, "0", false),
		newResource(urnB, "1", false, urnA),
		newResource(urnC, "2", false, urnA, urnB),
		newResource(urnA, "3", true),
		newResource(urnA, "4", true),
		newResource(urnC, "5", true, urnA, urnB),
	}

	old := &deploy.Snapshot{
		Resources: oldResources,
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					switch req.ID {
					case "0", "4":
						// We want to delete resources A::0 and A::4.
						return plugin.ReadResponse{}, nil
					default:
						return plugin.ReadResponse{
							ReadResult: plugin.ReadResult{
								Inputs:  req.Inputs,
								Outputs: req.State,
							},
							Status: resource.StatusOK,
						}, nil
					}
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, nil, loaders...)
	p.Options.T = t

	p.Steps = []TestStep{
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, err error,
			) error {
				// Should see only refreshes.
				for _, entry := range entries {
					if len(refreshTargets) > 0 {
						// should only see changes to urns we explicitly asked to change
						assert.Containsf(t, refreshTargets, entry.Step.URN(),
							"Refreshed a resource that wasn't a target: %v", entry.Step.URN())
					}

					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
				}

				return err
			},
		},
	}

	snap := p.RunWithName(t, old, name)

	provURN := p.NewProviderURN("pkgA", "default", "")

	// The new resources will have had their default provider urn filled in. We fill this in on
	// the old resources here as well so that the equal checks below pass
	setProviderRef(t, oldResources, snap.Resources, provURN)

	for _, r := range snap.Resources {
		switch urn := r.URN; urn {
		case provURN:
			continue
		case urnA, urnB, urnC:
			// break
		default:
			t.Fatalf("unexpected resource %v", urn)
		}

		if len(refreshTargets) == 0 || containsURN(refreshTargets, urnA) {
			// 'A' was deleted, so we should see the impact downstream.

			switch r.ID {
			case "1":
				// A::0 was deleted, so B's dependency list should be empty.
				assert.Equal(t, urnB, r.URN)
				assert.Empty(t, r.Dependencies)
			case "2":
				// A::0 was deleted, so C's dependency list should only contain B.
				assert.Equal(t, urnC, r.URN)
				assert.Equal(t, []resource.URN{urnB}, r.Dependencies)
			case "3":
				// A::3 should not have changed.
				assert.Equal(t, oldResources[3], r)
			case "5":
				// A::4 was deleted but A::3 was still refernceable by C, so C should not have changed.
				assert.Equal(t, oldResources[5], r)
			default:
				t.Fatalf("Unexpected changed resource when refreshing %v: %v::%v", refreshTargets, r.URN, r.ID)
			}
		} else {
			// A was not deleted. So nothing should be impacted.
			id, err := strconv.Atoi(r.ID.String())
			assert.NoError(t, err)
			assert.Equal(t, oldResources[id], r)
		}
	}
}

func containsURN(urns []resource.URN, urn resource.URN) bool {
	for _, val := range urns {
		if val == urn {
			return true
		}
	}

	return false
}

// Tests basic refresh functionality.
func TestRefreshBasics(t *testing.T) {
	t.Parallel()

	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshBasicsCombination(t, names, []string{}, "all")

	for i, subset := range subsets {
		validateRefreshBasicsCombination(t, names, subset, strconv.Itoa(i))
	}
}

func validateRefreshBasicsCombination(t *testing.T, names []string, targets []string, name string) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewURN(resType, names[2], "")
	urns := []resource.URN{urnA, urnB, urnC}

	refreshTargets := []resource.URN{}

	for _, target := range targets {
		refreshTargets = append(p.Options.Targets.Literals(), pickURN(t, urns, names, target))
	}

	p.Options.Targets = deploy.NewUrnTargetsFromUrns(refreshTargets)

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	oldResources := []*resource.State{
		newResource(urnA, "0", false),
		newResource(urnB, "1", false, urnA),
		newResource(urnC, "2", false, urnA, urnB),
		newResource(urnA, "3", true),
		newResource(urnA, "4", true),
		newResource(urnC, "5", true, urnA, urnB),
	}

	newStates := map[resource.ID]plugin.ReadResult{
		// A::0 and A::3 will have no changes.
		"0": {Outputs: resource.PropertyMap{}, Inputs: resource.PropertyMap{}},
		"3": {Outputs: resource.PropertyMap{}, Inputs: resource.PropertyMap{}},

		// B::1 has output-only changes which will not be reported as a refresh diff.
		"1": {Outputs: resource.PropertyMap{"foo": resource.NewStringProperty("bar")}, Inputs: resource.PropertyMap{}},

		// A::4 will have input and output changes. The changes that impact the inputs will be reported
		// as a refresh diff.
		"4": {
			Outputs: resource.PropertyMap{
				"baz": resource.NewStringProperty("qux"),
				"oof": resource.NewStringProperty("zab"),
			},
			Inputs: resource.PropertyMap{"oof": resource.NewStringProperty("zab")},
		},

		// C::2 and C::5 will be deleted.
		"2": {},
		"5": {},
	}

	old := &deploy.Snapshot{
		Resources: oldResources,
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					new, hasNewState := newStates[req.ID]
					assert.True(t, hasNewState)
					return plugin.ReadResponse{
						ReadResult: new,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	p.Options.HostF = deploytest.NewPluginHostF(nil, nil, nil, loaders...)
	p.Options.T = t

	p.Steps = []TestStep{{
		Op: Refresh,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, err error,
		) error {
			// Should see only refreshes.
			for _, entry := range entries {
				if len(refreshTargets) > 0 {
					// should only see changes to urns we explicitly asked to change
					assert.Containsf(t, refreshTargets, entry.Step.URN(),
						"Refreshed a resource that wasn't a target: %v", entry.Step.URN())
				}

				assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
				resultOp := entry.Step.(*deploy.RefreshStep).ResultOp()

				old := entry.Step.Old()
				if !old.Custom || providers.IsProviderType(old.Type) {
					// Component and provider resources should never change.
					assert.Equal(t, deploy.OpSame, resultOp)
					continue
				}

				expected, new := newStates[old.ID], entry.Step.New()
				if expected.Outputs == nil {
					// If the resource was deleted, we want the result op to be an OpDelete.
					assert.Nil(t, new)
					assert.Equal(t, deploy.OpDelete, resultOp)
				} else {
					// If there were changes to the inputs, we want the result op to be an
					// OpUpdate. Otherwise we want an OpSame.
					if reflect.DeepEqual(old.Inputs, expected.Inputs) {
						assert.Equal(t, deploy.OpSame, resultOp)
					} else {
						assert.Equal(t, deploy.OpUpdate, resultOp)
					}

					old = old.Copy()
					new = new.Copy()

					// Only the inputs and outputs should have changed (if anything changed).
					old.Inputs = expected.Inputs
					old.Outputs = expected.Outputs

					// Discard timestamps for refresh test.
					new.Modified = nil
					old.Modified = nil

					assert.Equal(t, old, new)
				}
			}
			return err
		},
	}}
	snap := p.RunWithName(t, old, name)

	provURN := p.NewProviderURN("pkgA", "default", "")

	// The new resources will have had their default provider urn filled in. We fill this in on
	// the old resources here as well so that the equal checks below pass
	setProviderRef(t, oldResources, snap.Resources, provURN)

	for _, r := range snap.Resources {
		switch urn := r.URN; urn {
		case provURN:
			continue
		case urnA, urnB, urnC:
			// break
		default:
			t.Fatalf("unexpected resource %v", urn)
		}

		// The only resources left in the checkpoint should be those that were not deleted by the refresh.
		expected := newStates[r.ID]
		assert.NotNil(t, expected)

		idx, err := strconv.ParseInt(string(r.ID), 0, 0)
		assert.NoError(t, err)

		targetedForRefresh := len(refreshTargets) == 0
		for _, targetUrn := range refreshTargets {
			if targetUrn == r.URN {
				targetedForRefresh = true
			}
		}

		// If targeted for refresh the new resources should be equal to the old resources + the new inputs and outputs
		// and timestamp.
		old := oldResources[int(idx)]
		if targetedForRefresh {
			old.Inputs = expected.Inputs
			old.Outputs = expected.Outputs
			old.Modified = r.Modified
		}

		assert.Equal(t, old, r)
	}
}

// Tests that an interrupted refresh leaves behind an expected state.
func TestCanceledRefresh(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, "resA", "")
	urnB := p.NewURN(resType, "resB", "")
	urnC := p.NewURN(resType, "resC", "")

	newResource := func(urn resource.URN, id resource.ID, delete bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       delete,
			ID:           id,
			Inputs:       resource.PropertyMap{},
			Outputs:      resource.PropertyMap{},
			Dependencies: dependencies,
		}
	}

	oldResources := []*resource.State{
		newResource(urnA, "0", false),
		newResource(urnB, "1", false),
		newResource(urnC, "2", false),
	}

	newStates := map[resource.ID]plugin.ReadResult{
		// A::0 will have input and output changes. The changes that impact the inputs will be reported
		// as a refresh diff.
		"0": {
			Outputs: resource.PropertyMap{"foo": resource.NewStringProperty("bar")},
			Inputs:  resource.PropertyMap{"oof": resource.NewStringProperty("rab")},
		},
		// B::1 will have output changes.
		"1": {
			Outputs: resource.PropertyMap{"baz": resource.NewStringProperty("qux")},
		},
		// C::2 will be deleted.
		"2": {},
	}

	old := &deploy.Snapshot{
		Resources: oldResources,
	}

	// Set up a cancelable context for the refresh operation.
	ctx, cancel := context.WithCancel(context.Background())

	// Serialize all refreshes s.t. we can cancel after the first is issued.
	refreshes, cancelled := make(chan resource.ID), make(chan bool)
	go func() {
		<-refreshes
		cancel()
	}()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					refreshes <- req.ID
					<-cancelled

					new, hasNewState := newStates[req.ID]
					assert.True(t, hasNewState)
					return plugin.ReadResponse{
						ReadResult: new,
						Status:     resource.StatusOK,
					}, nil
				},
				CancelF: func() error {
					close(cancelled)
					return nil
				},
			}, nil
		}),
	}

	refreshed := make(map[resource.ID]bool)
	op := TestOp(Refresh)
	options := TestUpdateOptions{
		T:     t,
		HostF: deploytest.NewPluginHostF(nil, nil, nil, loaders...),
		UpdateOptions: UpdateOptions{
			Parallel: 1,
		},
	}
	project, target := p.GetProject(), p.GetTarget(t, old)
	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		_ []Event, err error,
	) error {
		for _, entry := range entries {
			assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
			resultOp := entry.Step.(*deploy.RefreshStep).ResultOp()

			old := entry.Step.Old()
			if !old.Custom || providers.IsProviderType(old.Type) {
				// Component and provider resources should never change.
				assert.Equal(t, deploy.OpSame, resultOp)
				continue
			}

			refreshed[old.ID] = true

			expected, new := newStates[old.ID], entry.Step.New()
			if expected.Outputs == nil {
				// If the resource was deleted, we want the result op to be an OpDelete.
				assert.Nil(t, new)
				assert.Equal(t, deploy.OpDelete, resultOp)
			} else {
				// If there were changes to the inputs, we want the result op to be an
				// OpUpdate. Otherwise we want an OpSame.
				if reflect.DeepEqual(old.Inputs, expected.Inputs) {
					assert.Equal(t, deploy.OpSame, resultOp)
				} else {
					assert.Equal(t, deploy.OpUpdate, resultOp)
				}

				// The inputs, outputs and modified timestamps should have changed (if
				// anything changed at all).
				old = old.Copy()
				old.Inputs = expected.Inputs
				old.Outputs = expected.Outputs
				old.Modified = new.Modified

				assert.Equal(t, old, new)
			}
		}
		return err
	}

	snap, err := op.RunWithContext(ctx, project, target, options, false, nil, validate)
	assert.ErrorContains(t, err, "BAIL: canceled")
	assert.Equal(t, 1, len(refreshed))

	provURN := p.NewProviderURN("pkgA", "default", "")

	// The new resources will have had their default provider urn filled in. We fill this in on
	// the old resources here as well so that the equal checks below pass
	setProviderRef(t, oldResources, snap.Resources, provURN)

	for _, r := range snap.Resources {
		switch urn := r.URN; urn {
		case provURN:
			continue
		case urnA, urnB, urnC:
			// break
		default:
			t.Fatalf("unexpected resource %v", urn)
		}

		idx, err := strconv.ParseInt(string(r.ID), 0, 0)
		assert.NoError(t, err)

		if refreshed[r.ID] {
			// The refreshed resource should have its new state.
			expected := newStates[r.ID]
			if expected.Outputs == nil {
				assert.Fail(t, "refreshed resource was not deleted")
			} else {
				old := oldResources[int(idx)]

				// The inputs, outputs and modified timestamps should have changed (if
				// anything changed at all).
				old.Inputs = expected.Inputs
				old.Outputs = expected.Outputs
				old.Modified = r.Modified

				assert.Equal(t, old, r)
			}
		} else {
			// Any resources that were not refreshed should retain their original state.
			old := oldResources[int(idx)]
			assert.Equal(t, old, r)
		}
	}
}

func TestRefreshStepWillPersistUpdatedIDs(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	idBefore := resource.ID("myid")
	idAfter := resource.ID("mynewid")
	outputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{
						ReadResult: plugin.ReadResult{
							ID:      idAfter,
							Inputs:  resource.PropertyMap{},
							Outputs: outputs,
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options.HostF = hostF
	p.Options.T = t

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:       resURN.Type(),
				URN:        resURN,
				Custom:     true,
				ID:         idBefore,
				Inputs:     resource.PropertyMap{},
				Outputs:    outputs,
				InitErrors: []string{"Resource failed to initialize"},
			},
		},
	}

	p.Steps = []TestStep{{Op: Refresh, SkipPreview: true}}
	snap := p.Run(t, old)

	for _, resource := range snap.Resources {
		switch urn := resource.URN; urn {
		case provURN:
			// break
		case resURN:
			assert.Empty(t, resource.InitErrors)
			assert.Equal(t, idAfter, resource.ID)
		default:
			t.Fatalf("unexpected resource %v", urn)
		}
	}
}

// TestRefreshUpdateWithDeletedResource validates that the engine handles a deleted resource without error on an
// update with refresh.
func TestRefreshUpdateWithDeletedResource(t *testing.T) {
	t.Parallel()

	p := &TestPlan{}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	idBefore := resource.ID("myid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{}, nil
				},
			}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p.Options.HostF = hostF
	p.Options.Refresh = true
	p.Options.T = t

	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      idBefore,
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
		},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, old)
	assert.Equal(t, 0, len(snap.Resources))
}
