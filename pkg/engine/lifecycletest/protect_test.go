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
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// TestMultipleProtectedDeletes tests that a preview and up can see multiple protect delete errors.
func TestMultipleProtectedDeletes(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{ID: "id", Status: resource.StatusOK}, nil
				},
			}, nil
		}),
	}

	creating := true
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource(resource.RootStackType, "test", false)
		require.NoError(t, err)

		if creating {
			protect := true
			_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Protect: &protect,
			})
			require.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Protect: &protect,
			})
			require.NoError(t, err)
		}

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the initial update which sets some stack outputs.
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, snap.Resources, 4)
	assert.Equal(t, resource.RootStackType, snap.Resources[0].Type)

	// Run a preview that will try and delete both resA and resB.
	creating = false
	validate := func(
		project workspace.Project, target deploy.Target, entries engine.JournalEntries,
		events []engine.Event, err error,
	) error {
		// Check that we saw events for both resA and resB
		var sawA, sawB bool
		for _, e := range events {
			if e.Type == DiagEvent {
				payload := e.Payload().(engine.DiagEventPayload)
				if payload.URN != "" {
					if payload.URN.Name() == "resA" {
						assert.Contains(t, payload.Message, "resA\" cannot be deleted")
						sawA = true
					}
					if payload.URN.Name() == "resB" {
						assert.Contains(t, payload.Message, "resB\" cannot be deleted")
						sawB = true
					}
				}
			}
		}

		assert.True(t, sawA, "did not see resA")
		assert.True(t, sawB, "did not see resB")

		return err
	}
	_, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, true, p.BackendClient, validate, "1")
	assert.Error(t, err)

	// Run an update that will try and delete both resA and resB.
	_, err = lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate, "2")
	assert.Error(t, err)
}

// TestProtectInheritance tests that the protect option is inherited from parent resources but can be overridden.
func TestProtectInheritance(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		protect := true
		resp, err := monitor.RegisterResource("my_component", "parent", false, deploytest.ResourceOptions{
			Protect: &protect,
		})
		require.NoError(t, err)

		// Inherit protect true from parent
		_, err = monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
		})
		require.NoError(t, err)

		// Override protect true from parent
		protectB := false
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Parent:  resp.URN,
			Protect: &protectB,
		})
		require.NoError(t, err)

		return err
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	// Run the update
	snap, err := lt.TestOp(Update).
		RunStep(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, snap.Resources, 4)
	// Assert that parent and resA are protected and resB is not
	assert.Equal(t, "parent", snap.Resources[0].URN.Name())
	assert.Equal(t, "resA", snap.Resources[2].URN.Name())
	assert.Equal(t, "resB", snap.Resources[3].URN.Name())
	assert.True(t, snap.Resources[0].Protect)
	assert.True(t, snap.Resources[2].Protect)
	assert.False(t, snap.Resources[3].Protect)
	// Assert that resA and resB have the correct parent
	assert.Equal(t, snap.Resources[0].URN, snap.Resources[2].Parent)
	assert.Equal(t, snap.Resources[0].URN, snap.Resources[3].Parent)
}

// TestProtectedDeleteChainsWithDuplicateDeletedResources tests that delete chains that error out due to protected
// resources correctly abandon resources left in the chain, even in the presence of other resources with the same URN
// (such as can occur when a replace is interrupted leaving both old and new copies of a resource in the snapshot).
func TestProtectedDeleteChainsWithDuplicateDeletedResources(t *testing.T) {
	t.Parallel()

	// Arrange.

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	// Set up the initial snapshot. We want to end up with the following starting state:
	//
	// * A
	// * B, which depends on A through a dependency, and has Protect: true
	// * A copy of A, which has Delete: true (that is, it's an old copy of A that should be deleted as part of the next
	//   operation).
	//
	// To do this, we first run an update that sets up A and B, with B depending on A and protected as described. We then
	// run a second operation where a Diff indicates that A should be replaced. However, we set the deletion of A up to
	// fail. Since by default the replace is create-then-delete (delete-after-replace), we'll end up with the requisite
	// two copies of A.
	//
	// Why do we want this starting state? Well, once we've got this set up, we're going to run a destroy, which will
	// attempt to delete all three resources (A, B, and the old copy of A). Pulumi will determine that it needs to delete
	// B first, since that is at the top of the dependency chain. However, since B is protected, the delete will fail.
	// We'd expect Pulumi then to abandon the deletes of B's dependencies (A), but this will only work if Pulumi correctly
	// sees that it's the new copy of A whose deletion should be skipped, and not the old copy of A.

	// Step 1 of the setup -- create A and B.
	setupLoaders1 := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	setupProgramF1 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:modA:typA", "resA", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:typA", "resB", true, deploytest.ResourceOptions{
			Protect:      ptr(true),
			Dependencies: []resource.URN{resA.URN},
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF1 := deploytest.NewPluginHostF(nil, nil, setupProgramF1, setupLoaders1...)
	setupOpts1 := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF1,
	}
	setupSnap1, err := lt.TestOp(engine.Update).
		RunStep(project, p.GetTarget(t, nil), setupOpts1, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Step 2 of the setup -- create a second copy of A with Delete: true by triggering a replace which we'll interrupt
	// with a failed Delete.
	setupLoaders2 := []*deploytest.ProviderLoader{
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
						return plugin.DeleteResponse{}, errors.New("delete error")
					}

					return plugin.DeleteResponse{}, nil
				},
			}, nil
		}),
	}

	setupProgramF2 := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:modA:typA", "resA", true)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:typA", "resB", true, deploytest.ResourceOptions{
			Protect:      ptr(true),
			Dependencies: []resource.URN{resA.URN},
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF2 := deploytest.NewPluginHostF(nil, nil, setupProgramF2, setupLoaders2...)
	setupOpts2 := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF2,
	}
	setupSnap2, err := lt.TestOp(engine.Update).
		RunStep(project, p.GetTarget(t, setupSnap1), setupOpts2, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "delete error")

	// Act.

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewProviderLoader("pkg-gWXu", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
	opts := lt.TestUpdateOptions{
		T:     t,
		HostF: hostF,
		UpdateOptions: engine.UpdateOptions{
			ContinueOnError: false,
		},
	}

	snap, err := lt.TestOp(engine.Destroy).
		RunStep(project, p.GetTarget(t, setupSnap2), opts, false, p.BackendClient, nil, "2")
	require.ErrorContains(
		t, err,
		`resource "urn:pulumi:test-stack::test-project::pkgA:modA:typA::resB" cannot be deleted`,
	)

	// Assert.

	// The default provider for A and B, and A and B (since B's being protected will halt the destroy).
	require.Len(t, snap.Resources, 3)

	require.Equal(t, "resA", snap.Resources[1].URN.Name())
	require.Equal(t, "resB", snap.Resources[2].URN.Name())
}

func ptr[T any](v T) *T {
	return &v
}
