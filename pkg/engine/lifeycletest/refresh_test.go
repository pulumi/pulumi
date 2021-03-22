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

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestParallelRefresh(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Create a program that registers four resources, each of which depends on the resource that immediately precedes
	// it.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)

		resB, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resA},
		})
		assert.NoError(t, err)

		resC, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resB},
		})
		assert.NoError(t, err)

		_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resD", true, deploytest.ResourceOptions{
			Dependencies: []resource.URN{resC},
		})
		assert.NoError(t, err)

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Parallel: 4, Host: host},
	}

	p.Steps = []TestStep{{Op: Update}}
	snap := p.Run(t, nil)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.Equal(t, string(snap.Resources[2].URN.Name()), "resB")
	assert.Equal(t, string(snap.Resources[3].URN.Name()), "resC")
	assert.Equal(t, string(snap.Resources[4].URN.Name()), "resD")

	p.Steps = []TestStep{{Op: Refresh}}
	snap = p.Run(t, snap)

	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.Equal(t, string(snap.Resources[2].URN.Name()), "resB")
	assert.Equal(t, string(snap.Resources[3].URN.Name()), "resC")
	assert.Equal(t, string(snap.Resources[4].URN.Name()), "resD")
}

func TestExternalRefresh(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Our program reads a resource and exits.
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, err := monitor.ReadResource("pkgA:m:typA", "resA", "resA-some-id", "", resource.PropertyMap{}, "", "")
		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)
	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Update}},
	}

	// The read should place "resA" in the snapshot with the "External" bit set.
	snap := p.Run(t, nil)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.True(t, snap.Resources[1].External)

	p = &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   []TestStep{{Op: Refresh}},
	}

	snap = p.Run(t, snap)
	// A refresh should leave "resA" as it is in the snapshot. The External bit should still be set.
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, string(snap.Resources[0].URN.Name()), "default") // provider
	assert.Equal(t, string(snap.Resources[1].URN.Name()), "resA")
	assert.True(t, snap.Resources[1].External)
}

func TestRefreshInitFailure(t *testing.T) {
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
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					if refreshShouldFail && urn == resURN {
						err := &plugin.InitError{
							Reasons: []string{"Refresh reports continued to fail to initialize"},
						}
						return plugin.ReadResult{Outputs: resource.PropertyMap{}}, resource.StatusPartialFailure, err
					} else if urn == res2URN {
						return plugin.ReadResult{Outputs: res2Outputs}, resource.StatusOK, nil
					}
					return plugin.ReadResult{Outputs: resource.PropertyMap{}}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.Host = host

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
	for _, parallelFactor := range []int{1, 4} {
		t.Run(fmt.Sprintf("parallel-%d", parallelFactor), func(t *testing.T) {
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						ReadF: func(
							urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
						) (plugin.ReadResult, resource.Status, error) {
							// This thing doesn't exist. Returning nil from Read should trigger
							// the engine to delete it from the snapshot.
							return plugin.ReadResult{}, resource.StatusOK, nil
						},
					}, nil
				}),
			}

			program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				assert.NoError(t, err)
				return err
			})

			host := deploytest.NewPluginHost(nil, nil, program, loaders...)
			p := &TestPlan{Options: UpdateOptions{Host: host, Parallel: parallelFactor}}

			p.Steps = []TestStep{{Op: Update}}
			snap := p.Run(t, nil)

			p.Steps = []TestStep{{Op: Refresh}}
			snap = p.Run(t, snap)

			// Refresh succeeds and records that the resource in the snapshot doesn't exist anymore
			provURN := p.NewProviderURN("pkgA", "default", "")
			assert.Len(t, snap.Resources, 1)
			assert.Equal(t, provURN, snap.Resources[0].URN)
		})
	}
}

// Tests that dependencies are correctly rewritten when refresh removes deleted resources.
func TestRefreshDeleteDependencies(t *testing.T) {
	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshDeleteCombination(t, names, []string{})

	for _, subset := range subsets {
		validateRefreshDeleteCombination(t, names, subset)
	}
}

func validateRefreshDeleteCombination(t *testing.T, names []string, targets []string) {
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

	p.Options.RefreshTargets = refreshTargets

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
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					switch id {
					case "0", "4":
						// We want to delete resources A::0 and A::4.
						return plugin.ReadResult{}, resource.StatusOK, nil
					default:
						return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
					}
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, nil, loaders...)

	p.Steps = []TestStep{
		{
			Op: Refresh,
			Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
				_ []Event, res result.Result) result.Result {

				// Should see only refreshes.
				for _, entry := range entries {
					if len(refreshTargets) > 0 {
						// should only see changes to urns we explicitly asked to change
						assert.Containsf(t, refreshTargets, entry.Step.URN(),
							"Refreshed a resource that wasn't a target: %v", entry.Step.URN())
					}

					assert.Equal(t, deploy.OpRefresh, entry.Step.Op())
				}

				return res
			},
		},
	}

	snap := p.Run(t, old)

	provURN := p.NewProviderURN("pkgA", "default", "")

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
	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshBasicsCombination(t, names, []string{})

	for _, subset := range subsets {
		validateRefreshBasicsCombination(t, names, subset)
	}
}

func validateRefreshBasicsCombination(t *testing.T, names []string, targets []string) {
	p := &TestPlan{}

	const resType = "pkgA:m:typA"

	urnA := p.NewURN(resType, names[0], "")
	urnB := p.NewURN(resType, names[1], "")
	urnC := p.NewURN(resType, names[2], "")
	urns := []resource.URN{urnA, urnB, urnC}

	refreshTargets := []resource.URN{}

	for _, target := range targets {
		refreshTargets = append(p.Options.RefreshTargets, pickURN(t, urns, names, target))
	}

	p.Options.RefreshTargets = refreshTargets

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

		// B::1 and A::4 will have changes. The latter will also have input changes.
		"1": {Outputs: resource.PropertyMap{"foo": resource.NewStringProperty("bar")}, Inputs: resource.PropertyMap{}},
		"4": {
			Outputs: resource.PropertyMap{"baz": resource.NewStringProperty("qux")},
			Inputs:  resource.PropertyMap{"oof": resource.NewStringProperty("zab")},
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
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					new, hasNewState := newStates[id]
					assert.True(t, hasNewState)
					return new, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	p.Options.Host = deploytest.NewPluginHost(nil, nil, nil, loaders...)

	p.Steps = []TestStep{{
		Op: Refresh,
		Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

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
					// If there were changes to the outputs, we want the result op to be an OpUpdate. Otherwise we want
					// an OpSame.
					if reflect.DeepEqual(old.Outputs, expected.Outputs) {
						assert.Equal(t, deploy.OpSame, resultOp)
					} else {
						assert.Equal(t, deploy.OpUpdate, resultOp)
					}

					// Only the inputs and outputs should have changed (if anything changed).
					old.Inputs = expected.Inputs
					old.Outputs = expected.Outputs
					assert.Equal(t, old, new)
				}
			}
			return res
		},
	}}
	snap := p.Run(t, old)

	provURN := p.NewProviderURN("pkgA", "default", "")

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

		// The new resources should be equal to the old resources + the new inputs and outputs.
		old := oldResources[int(idx)]
		old.Inputs = expected.Inputs
		old.Outputs = expected.Outputs
		assert.Equal(t, old, r)
	}
}

// Tests that an interrupted refresh leaves behind an expected state.
func TestCanceledRefresh(t *testing.T) {
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

	newStates := map[resource.ID]resource.PropertyMap{
		// A::0 and B::1 will have changes; D::3 will be deleted.
		"0": {"foo": resource.NewStringProperty("bar")},
		"1": {"baz": resource.NewStringProperty("qux")},
		"2": nil,
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
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

					refreshes <- id
					<-cancelled

					new, hasNewState := newStates[id]
					assert.True(t, hasNewState)
					return plugin.ReadResult{Outputs: new}, resource.StatusOK, nil
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
	options := UpdateOptions{
		Parallel: 1,
		Host:     deploytest.NewPluginHost(nil, nil, nil, loaders...),
	}
	project, target := p.GetProject(), p.GetTarget(old)
	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		_ []Event, res result.Result) result.Result {

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
			if expected == nil {
				// If the resource was deleted, we want the result op to be an OpDelete.
				assert.Nil(t, new)
				assert.Equal(t, deploy.OpDelete, resultOp)
			} else {
				// If there were changes to the outputs, we want the result op to be an OpUpdate. Otherwise we want
				// an OpSame.
				if reflect.DeepEqual(old.Outputs, expected) {
					assert.Equal(t, deploy.OpSame, resultOp)
				} else {
					assert.Equal(t, deploy.OpUpdate, resultOp)
				}

				// Only the outputs should have changed (if anything changed).
				old.Outputs = expected
				assert.Equal(t, old, new)
			}
		}
		return res
	}

	snap, res := op.RunWithContext(ctx, project, target, options, false, nil, validate)
	assertIsErrorOrBailResult(t, res)
	assert.Equal(t, 1, len(refreshed))

	provURN := p.NewProviderURN("pkgA", "default", "")

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
			if expected == nil {
				assert.Fail(t, "refreshed resource was not deleted")
			} else {
				old := oldResources[int(idx)]
				old.Outputs = expected
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
	p := &TestPlan{}

	provURN := p.NewProviderURN("pkgA", "default", "")
	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	idBefore := resource.ID("myid")
	idAfter := resource.ID("mynewid")
	outputs := resource.PropertyMap{"foo": resource.NewStringProperty("bar")}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{ID: idAfter, Outputs: outputs, Inputs: resource.PropertyMap{}}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.Host = host

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
	p := &TestPlan{}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")
	idBefore := resource.ID("myid")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ReadF: func(
					urn resource.URN, id resource.ID, inputs, state resource.PropertyMap,
				) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{}, resource.StatusOK, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p.Options.Host = host
	p.Options.Refresh = true

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
