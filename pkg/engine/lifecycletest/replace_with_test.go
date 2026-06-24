// Copyright 2025, Pulumi Corporation.
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
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceWith(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},

				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	// Any old value that we can change later to trigger a replace.
	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		// When we replace A, we should also replace B.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			ReplaceWith: []resource.URN{respA.URN},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)

	// We create 2 resources, A and B. After the first deploy, we shouldn't have deleted anything.
	require.Len(t, created, 2)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String())

	require.Len(t, deleted, 0)

	// Change the property on A, trigger a replacement of A and B.
	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// We should have replaced A and therefore B, which means two new creates and two deletes.
	// The two new creates will have the same URNs as the old ones, but different IDs. The two
	// deletes will be the original two objects.
	require.Len(t, created, 4)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())

	require.Len(t, deleted, 2)
	require.Contains(t, deleted, created[0])
	require.Contains(t, deleted, created[1])
}

func TestReplaceWithAndDeletedWith(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},

				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},

				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})
	numberOfRuns := 0

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var urnB, urnC resource.URN

		if numberOfRuns == 0 {
			respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs: resource.NewPropertyMapFromMap(map[string]any{}),
			})

			require.NoError(t, err)
			urnB = respB.URN
		}

		respC, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: inputs,
		})

		require.NoError(t, err)
		urnC = respC.URN

		opts := deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{}),
			ReplaceWith: []resource.URN{urnC},
		}

		if numberOfRuns == 0 {
			opts.DeletedWith = urnB
		}

		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, opts)
		require.NoError(t, err)

		numberOfRuns++
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")

	require.NoError(t, err)
	require.NotNil(t, snap)

	require.Len(t, snap.Resources, 4)
	require.Equal(t, "created-id-1", snap.Resources[1].ID.String()) // B
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String()) // C
	require.Equal(t, "created-id-3", snap.Resources[3].ID.String()) // A (deletes with B, replaces with C)

	require.Equal(t, snap.Resources[3].DeletedWith, snap.Resources[1].URN)
	require.Contains(t, snap.Resources[3].ReplaceWith, snap.Resources[2].URN)

	inputs["foo"] = resource.NewProperty("baz") // Trigger replacement of C
	// B doesn't exist for the second update, so deletedWith should be triggered.

	created = []resource.URN{}
	deleted = []resource.URN{}

	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")

	require.NoError(t, err)
	require.NotNil(t, snap)

	require.Len(t, snap.Resources, 3)
	require.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String())

	require.Len(t, created, 2)
	require.ElementsMatch(t, []string{created[0].Name(), created[1].Name()}, []string{"resC", "resA"})

	require.Len(t, deleted, 2)
	require.ElementsMatch(t, []string{deleted[0].Name(), deleted[1].Name()}, []string{"resB", "resC"})
}

// TestDeletedWithDuringReplacement verifies that when a resource is replaced,
// dependents with deleted_with pointing to it skip their provider Delete call.
// This covers various combinations of create-before-replace, delete-before-replace,
// and DependsOn to confirm deleted_with always fires during replacement.
func TestDeletedWithDuringReplacement(t *testing.T) {
	t.Parallel()

	type testCase struct {
		aDeleteBeforeReplace bool
		bDeleteBeforeReplace bool
		bDependsOnA          bool
		addResC              bool     // add a third resource C
		transitiveChain      bool     // C has deletedWith:B (transitive) vs depends on A (independent)
		targets              []string // resource names to pass to p.Options.Targets (nil = no --target)
	}

	run := func(t *testing.T, tc testCase, expectedOps []string) {
		var steps []string
		var createCount int

		loaders := []*deploytest.ProviderLoader{
			deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
				return &deploytest.Provider{
					DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
						if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
							return plugin.DiffResult{
								Changes:     plugin.DiffSome,
								ReplaceKeys: []resource.PropertyKey{"foo"},
							}, nil
						}
						return plugin.DiffResult{}, nil
					},
					CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
						createCount++
						id := fmt.Sprintf("%s-%d", req.Name, createCount)
						steps = append(steps, "create("+req.Name+", "+id+")")
						return plugin.CreateResponse{
							ID:         resource.ID(id),
							Properties: req.Properties,
							Status:     resource.StatusOK,
						}, nil
					},
					DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
						steps = append(steps, "delete("+req.Name+", "+string(req.ID)+")")
						return plugin.DeleteResponse{}, nil
					},
				}, nil
			}, deploytest.WithoutGrpc),
		}

		inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})

		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:              inputs,
				DeleteBeforeReplace: &tc.aDeleteBeforeReplace,
			})
			require.NoError(t, err)

			respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:              resource.NewPropertyMapFromMap(map[string]any{}),
				ReplaceWith:         []resource.URN{respA.URN},
				DeletedWith:         respA.URN,
				DeleteBeforeReplace: &tc.bDeleteBeforeReplace,
				Dependencies: slices.DeleteFunc([]resource.URN{respA.URN},
					func(_ resource.URN) bool { return !tc.bDependsOnA }),
			})
			require.NoError(t, err)

			if tc.addResC {
				cOpts := deploytest.ResourceOptions{
					Inputs:       resource.NewPropertyMapFromMap(map[string]any{}),
					Dependencies: []resource.URN{respA.URN},
					ReplaceWith:  []resource.URN{respA.URN},
				}
				if tc.transitiveChain {
					// C's deletedWith points to B (which is itself being replaced via A).
					cOpts.ReplaceWith = []resource.URN{respB.URN}
					cOpts.DeletedWith = respB.URN
					cOpts.Dependencies = nil
				}
				_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, cOpts)
				require.NoError(t, err)
			}

			return nil
		})

		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
		project := p.GetProject()

		// Step 0: initial deployment.
		snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
		require.NoError(t, err)
		require.NotNil(t, snap)

		// Step 1: change resA's input to trigger replacement.
		inputs["foo"] = resource.NewProperty("baz")
		steps = nil

		if len(tc.targets) > 0 {
			urns := make([]string, len(tc.targets))
			for i, name := range tc.targets {
				urns[i] = string(p.NewURN("pkgA:m:typA", name, ""))
			}
			p.Options.Targets = deploy.NewUrnTargets(urns)
		}

		snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
		require.NoError(t, err)
		require.NotNil(t, snap)

		assert.Equal(t, expectedOps, steps)
	}

	// A{}, B{DeleteWith: A}
	t.Run("A_CBR_B_deletedWith", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{}, []string{
			"create(resA, resA-3)",
			"create(resB, resB-4)",
			"delete(resA, resA-1)",
		})
	})

	// A{DeleteBeforeReplace: true}, B{DeleteWith: A}
	t.Run("A_DBR_B_deletedWith", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{aDeleteBeforeReplace: true}, []string{
			"delete(resA, resA-1)",
			"create(resA, resA-3)",
			"create(resB, resB-4)",
		})
	})

	// A{DeleteBeforeReplace: true}, B{DeleteWith: A, DeleteBeforeReplace: true}
	t.Run("A_DBR_B_deletedWith_DBR", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{aDeleteBeforeReplace: true, bDeleteBeforeReplace: true}, []string{
			"delete(resA, resA-1)",
			"create(resA, resA-3)",
			"create(resB, resB-4)",
		})
	})

	// A{}, B{DeleteWith: A, DeleteBeforeReplace: true}
	t.Run("A_CBR_B_deletedWith_DBR", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{bDeleteBeforeReplace: true}, []string{
			"create(resA, resA-3)",
			"delete(resA, resA-1)",
			"create(resB, resB-4)",
		})
	})

	// A{}, B{DeleteWith: A, DeleteBeforeReplace: true, DependsOn: A}
	t.Run("A_CBR_B_deletedWith_DBR_dependsOn", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{bDeleteBeforeReplace: true, bDependsOnA: true}, []string{
			"create(resA, resA-3)",
			"delete(resA, resA-1)",
			"create(resB, resB-4)",
		})
	})

	// A{}, B{DeleteWith: A, DeleteBeforeReplace: true, DependsOn: A}, C{DependsOn: A}
	t.Run("A_CBR_B_deletedWith_DBR_dependsOn_C_dependsOn", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{bDeleteBeforeReplace: true, bDependsOnA: true, addResC: true}, []string{
			"create(resA, resA-4)",
			"create(resC, resC-5)",
			"delete(resC, resC-3)",
			"delete(resA, resA-1)",
			"create(resB, resB-6)",
		})
	})

	// A{}, B{DeleteWith: A}, C{DeleteWith: B}
	t.Run("transitive_deletedWith_chain", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{addResC: true, transitiveChain: true}, []string{
			"create(resA, resA-4)",
			"create(resB, resB-5)",
			"create(resC, resC-6)",
			"delete(resA, resA-1)",
		})
	})

	// A{}, B{DeleteWith: A, DeleteBeforeReplace: true} with --target A,B — defer path must
	// still produce the correct order when running under a targeted update.
	t.Run("A_CBR_B_deletedWith_DBR_targeted_both", func(t *testing.T) {
		t.Parallel()
		run(t, testCase{
			bDeleteBeforeReplace: true,
			targets:              []string{"resA", "resB"},
		}, []string{
			"create(resA, resA-3)",
			"delete(resA, resA-1)",
			"create(resB, resB-4)",
		})
	})
}

// newDeferredTestProvider returns a pkgA provider loader used by the TestDeferredCreate_* tests. The
// shared DiffF triggers a replacement when the "foo" input changes; createF and deleteF may be nil to use
// defaults (createF echoes inputs back as outputs; deleteF is a no-op).
func newDeferredTestProvider(
	createF func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error),
	deleteF func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error),
) []*deploytest.ProviderLoader {
	if createF == nil {
		createF = func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
			return plugin.CreateResponse{
				ID:         resource.ID(req.Name + "-id"),
				Properties: req.Properties,
				Status:     resource.StatusOK,
			}, nil
		}
	}
	return []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: createF,
				DeleteF: deleteF,
			}, nil
		}, deploytest.WithoutGrpc),
	}
}

// TestDeferredCreate_Preview pins that preview mode still emits a create-replacement event for the
// deferred resource — a regression in preview semantics would surface here.
func TestDeferredCreate_Preview(t *testing.T) {
	t.Parallel()

	loaders := newDeferredTestProvider(nil, nil)

	inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})
	bDBR := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: inputs})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:              resource.NewPropertyMapFromMap(map[string]any{}),
			ReplaceWith:         []resource.URN{respA.URN},
			DeletedWith:         respA.URN,
			DeleteBeforeReplace: &bDBR,
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)

	// Preview doesn't populate JournalEntries, so we inspect the fired events to check the plan shape.
	inputs["foo"] = resource.NewProperty("baz")
	resBCreateReplacements := 0
	deferredWarnings := 0
	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries,
		events []Event, err error,
	) error {
		require.NoError(t, err)
		for _, event := range events {
			//nolint:exhaustive // Only ResourcePreEvent and DiagEvent are relevant here.
			switch event.Type {
			case ResourcePreEvent:
				meta := event.Payload().(ResourcePreEventPayload).Metadata
				if meta.URN.Name() == "resB" && meta.Op == deploy.OpCreateReplacement {
					resBCreateReplacements++
				}
			case DiagEvent:
				d := event.Payload().(DiagEventPayload)
				if d.Severity == diag.Warning && d.URN.Name() == "resB" &&
					strings.Contains(d.Message, "deferred") {
					deferredWarnings++
				}
			}
		}
		return nil
	}
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, true, p.BackendClient, validate, "1")
	require.NoError(t, err)
	assert.Positive(t, resBCreateReplacements,
		"preview plan should include a create-replacement step for resB")
	assert.Positive(t, deferredWarnings,
		"preview should warn that resB's create is deferred and its outputs will be empty")
}

// TestDeferredCreate_DownstreamDepIsRejected verifies that when a resource depends on a deferred
// replacement, the engine rejects the update with a clear error rather than silently handing back empty
// outputs. The dependent's real outputs aren't available yet, so any downstream consumer would be built
// with empty values.
func TestDeferredCreate_DownstreamDepIsRejected(t *testing.T) {
	t.Parallel()

	loaders := newDeferredTestProvider(nil, nil)

	inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})
	bDBR := true
	registerC := false

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: inputs})
		require.NoError(t, err)

		respB, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:              resource.NewPropertyMapFromMap(map[string]any{}),
			ReplaceWith:         []resource.URN{respA.URN},
			DeletedWith:         respA.URN,
			DeleteBeforeReplace: &bDBR,
		})
		require.NoError(t, err)

		if registerC {
			_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Inputs:       resource.NewPropertyMapFromMap(map[string]any{}),
				Dependencies: []resource.URN{respB.URN},
			})
			require.Error(t, err, "registration of resC should fail: its dep resB is deferred")
		}
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)

	// Flip to a program that also registers resC depending on resB, then trigger the replacement.
	registerC = true
	inputs["foo"] = resource.NewProperty("baz")
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot depend on")
	assert.Contains(t, err.Error(), "deferred until after")
}

// TestDeferredCreate_ProviderError verifies that when the deferred provider Create fails, the deployment
// reports an error and the resulting snapshot is in a recoverable state (old resources are gone, new is
// absent) so a follow-up update can complete normally.
func TestDeferredCreate_ProviderError(t *testing.T) {
	t.Parallel()

	failCreateB := false

	loaders := newDeferredTestProvider(
		func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
			if req.Name == "resB" && failCreateB {
				return plugin.CreateResponse{Status: resource.StatusOK}, errors.New("boom: resB create failed")
			}
			return plugin.CreateResponse{
				ID:         resource.ID(req.Name + "-id"),
				Properties: req.Properties,
				Status:     resource.StatusOK,
			}, nil
		},
		nil,
	)

	inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})
	bDBR := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: inputs})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:              resource.NewPropertyMapFromMap(map[string]any{}),
			ReplaceWith:         []resource.URN{respA.URN},
			DeletedWith:         respA.URN,
			DeleteBeforeReplace: &bDBR,
		})
		require.NoError(t, err)
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)

	// Flip the provider to fail on resB's deferred create, then run the replacement.
	failCreateB = true
	inputs["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.Error(t, err)

	failCreateB = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	require.NoError(t, err)
	require.NotNil(t, snap)
	names := map[string]bool{}
	for _, r := range snap.Resources {
		names[r.URN.Name()] = true
	}
	assert.True(t, names["resA"], "resA should be present after recovery")
	assert.True(t, names["resB"], "resB should be present after recovery")
}

// TestDeferredCreate_SkipsOnPriorError verifies a prior delete-phase error causes deferred creates to be
// skipped rather than run against broken state.
func TestDeferredCreate_SkipsOnPriorError(t *testing.T) {
	t.Parallel()
	var resBCreates int
	bDBR := true
	inputs := resource.NewPropertyMapFromMap(map[string]any{"foo": "bar"})
	loaders := newDeferredTestProvider(
		func(_ context.Context, r plugin.CreateRequest) (plugin.CreateResponse, error) {
			if r.Name == "resB" {
				resBCreates++
			}
			return plugin.CreateResponse{ID: resource.ID(r.Name + "-id"), Properties: r.Properties}, nil
		},
		func(_ context.Context, r plugin.DeleteRequest) (plugin.DeleteResponse, error) {
			if r.Name == "resA" {
				return plugin.DeleteResponse{}, errors.New("boom")
			}
			return plugin.DeleteResponse{}, nil
		})
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, m *deploytest.ResourceMonitor) error {
		a, err := m.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: inputs})
		require.NoError(t, err)
		_, err = m.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{}, DeletedWith: a.URN, DeleteBeforeReplace: &bDBR,
		})
		require.NoError(t, err)
		return nil
	})
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{
		T: t, HostF: deploytest.NewPluginHostF(nil, nil, programF, loaders...), SkipDisplayTests: true,
	}}
	project := p.GetProject()
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Equal(t, 1, resBCreates)
	inputs["foo"] = resource.NewProperty("baz")
	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.Error(t, err)
	assert.Equal(t, 1, resBCreates, "deferred create must be skipped when a prior delete errored")
}

func TestReplaceWithDeleteBeforeReplace(t *testing.T) {
	t.Parallel()

	created := []resource.URN{}
	deleted := []resource.URN{}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if !req.OldOutputs["foo"].DeepEquals(req.NewInputs["foo"]) {
						return plugin.DiffResult{
							Changes:             plugin.DiffSome,
							ReplaceKeys:         []resource.PropertyKey{"foo"},
							DeleteBeforeReplace: true,
						}, nil
					}
					return plugin.DiffResult{}, nil
				},

				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					created = append(created, req.URN)
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", len(created)))
					return plugin.CreateResponse{
						ID:         resourceID,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					deleted = append(deleted, req.URN)
					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	// Any old value that we can change later to trigger a replace.
	ins := resource.NewPropertyMapFromMap(map[string]any{
		"foo": "bar",
	})

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{Inputs: ins})
		require.NoError(t, err)

		// When we replace A, we should also replace B.
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:      resource.NewPropertyMapFromMap(map[string]any{"fixed": "property"}),
			ReplaceWith: []resource.URN{respA.URN},
		})
		require.NoError(t, err)

		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 3)

	// We create 2 resources, A and B. After the first deploy, we shouldn't have deleted anything.
	require.Len(t, created, 2)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-1", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-2", snap.Resources[2].ID.String())

	require.Len(t, deleted, 0)

	// Change the property on A, trigger a replacement of A and B.
	ins["foo"] = resource.NewProperty("baz")
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	require.NoError(t, err)

	// We should have replaced A and therefore B, which means two new creates and two deletes.
	// The two new creates will have the same URNs as the old ones, but different IDs. The two
	// deletes will be the original two objects.
	require.Len(t, created, 4)
	require.Equal(t, created[0], snap.Resources[1].URN)
	require.Equal(t, created[1], snap.Resources[2].URN)

	require.Equal(t, "created-id-3", snap.Resources[1].ID.String())
	require.Equal(t, "created-id-4", snap.Resources[2].ID.String())

	require.Len(t, deleted, 2)
	require.Contains(t, deleted, created[0])
	require.Contains(t, deleted, created[1])
}
