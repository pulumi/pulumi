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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// StateMigrationFunction adapts a typed Go function into the raw callback shape expected by
// deploytest.CallbackServer.Allocate for state migration callbacks.
func StateMigrationFunction(
	f func(
		urn resource.URN, resources []apitype.ResourceV3,
	) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error),
) func([]byte) (proto.Message, error) {
	return func(request []byte) (proto.Message, error) {
		var migrationRequest pulumirpc.StateMigrationRequest
		if err := proto.Unmarshal(request, &migrationRequest); err != nil {
			return nil, fmt.Errorf("unmarshaling request: %w", err)
		}

		var resources []apitype.ResourceV3
		if err := json.Unmarshal(migrationRequest.OldState, &resources); err != nil {
			return nil, fmt.Errorf("unmarshaling old state: %w", err)
		}

		newResources, successors, err := f(resource.URN(migrationRequest.Urn), resources)
		if err != nil {
			return nil, err
		}

		response := &pulumirpc.StateMigrationResponse{}
		if newResources != nil {
			newState, err := json.Marshal(newResources)
			if err != nil {
				return nil, fmt.Errorf("marshaling new state: %w", err)
			}
			response.NewState = newState
		}
		if len(successors) > 0 {
			response.Successors = make(map[string]string, len(successors))
			for source, target := range successors {
				response.Successors[string(source)] = string(target)
			}
		}
		return response, nil
	}
}

// successOps counts the step operations of all successful journal entries.
func successOps(entries JournalEntries) map[display.StepOp]int {
	ops := map[display.StepOp]int{}
	for _, entry := range entries {
		if entry.Kind == TestJournalEntrySuccess {
			ops[entry.Step.Op()]++
		}
	}
	return ops
}

// validateOps returns a lifecycle test validation function asserting the given step operation counts.
func validateOps(t *testing.T, expected map[display.StepOp]int) lt.ValidateFunc {
	return func(
		project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error,
	) error {
		assert.Equal(t, expected, successOps(entries))
		return err
	}
}

// runUpdate runs a single update of the given plan against the given prior snapshot.
func runUpdate(
	t *testing.T, p *lt.TestPlan, snap *deploy.Snapshot, validate lt.ValidateFunc,
) (*deploy.Snapshot, error) {
	return lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, snap), p.Options, false, p.BackendClient, validate)
}

// renameByName renames the resources with the given name in a serialized resource list.
func renameByName(resources []apitype.ResourceV3, oldName, newName string) []apitype.ResourceV3 {
	oldSuffix, newSuffix := "::"+oldName, "::"+newName
	for i, res := range resources {
		if strings.HasSuffix(string(res.URN), oldSuffix) {
			resources[i].URN = resource.URN(strings.TrimSuffix(string(res.URN), oldSuffix) + newSuffix)
		}
	}
	return resources
}

// stateMigrationEnv wires up the shared scaffolding for state migration lifecycle tests: a pkgA provider and a
// program consisting of a component "comp" with a custom child resource parented to it. The child's name and the
// migrations attached to the component are controlled per-update via the returned struct's fields.
type stateMigrationEnv struct {
	plan *lt.TestPlan

	// childName is the name the program registers the child resource under.
	childName string
	// childInputs are the inputs the program registers the child resource with.
	childInputs resource.PropertyMap
	// childProtect marks the child resource as protected.
	childProtect bool
	// registerChild controls whether the child resource is registered at all.
	registerChild bool
	// migrations builds the migration callbacks to attach to the component, given a callback server.
	migrations func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback
}

func newStateMigrationEnv(t *testing.T) *stateMigrationEnv {
	env := &stateMigrationEnv{
		childName:     "childA",
		childInputs:   resource.PropertyMap{"foo": resource.NewProperty("bar")},
		registerChild: true,
	}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		var migrations []*pulumirpc.Callback
		if env.migrations != nil {
			migrations = env.migrations(t, callbacks)
		}

		resp, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
			StateMigrations: migrations,
		})
		if err != nil {
			return err
		}

		if env.registerChild {
			_, err = monitor.RegisterResource("pkgA:m:typA", env.childName, true, deploytest.ResourceOptions{
				Parent:  resp.URN,
				Inputs:  env.childInputs,
				Protect: &env.childProtect,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	env.plan = &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}
	return env
}

const (
	compURN   = resource.URN("urn:pulumi:test::test::my:module:Comp::comp")
	childAURN = resource.URN("urn:pulumi:test::test::my:module:Comp$pkgA:m:typA::childA")
	childBURN = resource.URN("urn:pulumi:test::test::my:module:Comp$pkgA:m:typA::childB")
	childCURN = resource.URN("urn:pulumi:test::test::my:module:Comp$pkgA:m:typA::childC")
)

// snapURNs returns the URNs of all resources in the snapshot.
func snapURNs(snap *deploy.Snapshot) []resource.URN {
	urns := make([]resource.URN, len(snap.Resources))
	for i, res := range snap.Resources {
		urns[i] = res.URN
	}
	return urns
}

// renameMigration returns a migration callback that renames the child resource and maps its old URN to the new one.
func renameMigration(t *testing.T, callbacks *deploytest.CallbackServer, oldName, newName string) *pulumirpc.Callback {
	callback, err := callbacks.Allocate(
		StateMigrationFunction(func(
			urn resource.URN, resources []apitype.ResourceV3,
		) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
			successors := make(map[resource.URN]resource.URN)
			for _, res := range resources {
				if strings.HasSuffix(string(res.URN), "::"+oldName) {
					renamed := renameByName([]apitype.ResourceV3{res}, oldName, newName)
					successors[res.URN] = renamed[0].URN
				}
			}
			if len(successors) == 0 {
				// Already migrated: return the state unchanged.
				return nil, nil, nil
			}
			return renameByName(resources, oldName, newName), successors, nil
		}))
	require.NoError(t, err)
	return callback
}

// TestStateMigrationRenameChild tests the basic state migration scenario: version 2 of a component renames its
// child resource and ships a migration that renames the prior state to match. The second update produces only
// same steps: no create, no delete, and the child keeps its ID.
func TestStateMigrationRenameChild(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	// Version 1: component with child "childA".
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), childAURN)
	var childID resource.ID
	for _, res := range snap.Resources {
		if res.URN == childAURN {
			childID = res.ID
		}
	}
	require.NotEmpty(t, childID)

	// Version 2: the child is renamed to "childB" and a migration renames the prior state.
	env.childName = "childB"
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
	}

	// A preview must behave the same as an update and not persist anything.
	_, err = lt.TestOp(Update).Run(
		env.plan.GetProject(), env.plan.GetTarget(t, snap), env.plan.Options, true, env.plan.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			// The migrated state matches the program, so the preview plans no changes.
			for _, e := range events {
				if e.Type == ResourcePreEvent {
					payload := e.Payload().(ResourcePreEventPayload)
					assert.Equal(t, deploy.OpSame, payload.Metadata.Op, "unexpected %v for %v",
						payload.Metadata.Op, payload.Metadata.URN)
				}
			}
			return err
		})
	require.NoError(t, err)

	snap, err = runUpdate(t, env.plan, snap,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assert.Equal(t, map[display.StepOp]int{deploy.OpSame: 3}, successOps(entries))
			return err
		})
	require.NoError(t, err)

	urns := snapURNs(snap)
	assert.Contains(t, urns, childBURN)
	assert.NotContains(t, urns, childAURN)
	for _, res := range snap.Resources {
		if res.URN == childBURN {
			// The renamed resource keeps its identity.
			assert.Equal(t, childID, res.ID)
			assert.Equal(t, compURN, res.Parent)
		}
	}

	// A third update is a steady-state no-op: the migration sees the new shape and returns nil.
	snap, err = runUpdate(t, env.plan, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	assert.Contains(t, snapURNs(snap), childBURN)
}

// TestStateMigrationProtectedRename tests that protection follows state through an explicit successor. The old
// resource continues through the successor, so protection does not need to be cleared first.
func TestStateMigrationProtectedRename(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	env.childProtect = true

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), childAURN)
	var childID resource.ID
	for _, state := range snap.Resources {
		if state.URN == childAURN {
			childID = state.ID
			require.True(t, state.Protect)
		}
	}

	env.childName = "childB"
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
	}

	snap, err = runUpdate(t, env.plan, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	assert.NotContains(t, snapURNs(snap), childAURN)
	for _, state := range snap.Resources {
		if state.URN == childBURN {
			assert.Equal(t, childID, state.ID)
			assert.True(t, state.Protect)
		}
	}
}

// TestStateMigrationChained tests that multiple migrations compose: each receives the previous one's output.
func TestStateMigrationChained(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	env.childName = "childC"
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{
			renameMigration(t, callbacks, "childA", "childB"),
			renameMigration(t, callbacks, "childB", "childC"),
		}
	}

	snap, err = runUpdate(t, env.plan, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, childCURN)
	assert.NotContains(t, urns, childAURN)
	assert.NotContains(t, urns, childBURN)
}

// TestStateMigrationNoOp tests that a migration that returns no new state leaves everything untouched.
func TestStateMigrationNoOp(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				assert.Equal(t, compURN, urn)
				// The subtree is handed over root-first.
				require.Len(t, resources, 2)
				assert.Equal(t, compURN, resources[0].URN)
				assert.Equal(t, childAURN, resources[1].URN)
				return nil, nil, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	snap, err = runUpdate(t, env.plan, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	assert.Contains(t, snapURNs(snap), childAURN)
}

// TestStateMigrationErrors tests the failure semantics of state migrations: any callback error or validation
// failure fails the update and leaves the prior state untouched.
func TestStateMigrationErrors(t *testing.T) {
	t.Parallel()

	run := func(
		t *testing.T, expectedError string,
		migration func(
			urn resource.URN, resources []apitype.ResourceV3,
		) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error),
		prepare func(env *stateMigrationEnv),
	) {
		env := newStateMigrationEnv(t)
		if prepare != nil {
			prepare(env)
		}

		snap, err := lt.TestOp(Update).Run(
			env.plan.GetProject(), env.plan.GetTarget(t, nil), env.plan.Options, false, env.plan.BackendClient, nil)
		require.NoError(t, err)
		before := snapURNs(snap)

		env.childName = "childB"
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			callback, err := callbacks.Allocate(StateMigrationFunction(migration))
			require.NoError(t, err)
			return []*pulumirpc.Callback{callback}
		}

		snap, err = lt.TestOp(Update).Run(
			env.plan.GetProject(), env.plan.GetTarget(t, snap), env.plan.Options, false, env.plan.BackendClient, nil)
		require.ErrorContains(t, err, expectedError)
		// The prior state is untouched by the failed migration.
		assert.Equal(t, before, snapURNs(snap))
	}

	t.Run("callback error", func(t *testing.T) {
		t.Parallel()
		run(t, "state migration 1 of 1 for "+string(compURN),
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				return nil, nil, errors.New("bad migration")
			}, nil)
	})

	t.Run("unaccounted resource", func(t *testing.T) {
		t.Parallel()
		run(t, "did not account for resource "+string(childAURN),
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				// Drop the child without identifying a successor.
				return resources[:1], nil, nil
			}, nil)
	})

	t.Run("successor source not in state", func(t *testing.T) {
		t.Parallel()
		run(t, "returned successor for resource",
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				other := resource.URN("urn:pulumi:test::test::pkgA:m:typA::other")
				return resources, map[resource.URN]resource.URN{other: childAURN}, nil
			}, nil)
	})

	t.Run("successor target not returned", func(t *testing.T) {
		t.Parallel()
		run(t, "is not present in the returned state",
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				return resources[:1], map[resource.URN]resource.URN{childAURN: childBURN}, nil
			}, nil)
	})

	t.Run("custom resource without ID", func(t *testing.T) {
		t.Parallel()
		run(t, "changes the physical ID",
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				for i := range resources {
					if resources[i].URN == childAURN {
						resources[i].URN = childBURN
						resources[i].ID = ""
					}
				}
				return resources, map[resource.URN]resource.URN{childAURN: childBURN}, nil
			}, nil)
	})

	t.Run("custom resource changes ID", func(t *testing.T) {
		t.Parallel()
		run(t, "changes the physical ID",
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				for i := range resources {
					if resources[i].URN == childAURN {
						resources[i].URN = childBURN
						resources[i].ID = "different-id"
					}
				}
				return resources, map[resource.URN]resource.URN{childAURN: childBURN}, nil
			}, nil)
	})

	t.Run("successors without new state", func(t *testing.T) {
		t.Parallel()
		run(t, "returned successors without returning a new state",
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				return nil, map[resource.URN]resource.URN{childAURN: childBURN}, nil
			}, nil)
	})
}

// TestStateMigrationDeleteViaLifecycle tests that migrated resources that are no longer registered by the
// program are deleted by the normal engine lifecycle: a migration keeps the state, and the delete scan reaps it.
func TestStateMigrationDeleteViaLifecycle(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	// Version 2 renames the child in state but does not register it: the renamed resource must be deleted by
	// the provider, going through the normal lifecycle.
	env.registerChild = false
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
	}

	var deleted []resource.URN
	snap, err = runUpdate(t, env.plan, snap,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			for _, entry := range entries {
				if entry.Kind == TestJournalEntrySuccess && entry.Step.Op() == deploy.OpDelete {
					deleted = append(deleted, entry.Step.URN())
				}
			}
			return err
		})
	require.NoError(t, err)
	// The renamed resource is deleted through the normal lifecycle. (The now-unused default provider is
	// deleted as well.)
	assert.Contains(t, deleted, childBURN)
	assert.NotContains(t, deleted, childAURN)
	urns := snapURNs(snap)
	assert.NotContains(t, urns, childAURN)
	assert.NotContains(t, urns, childBURN)
}

// TestStateMigrationSecrets tests that secret values in the prior state survive the migration round-trip.
func TestStateMigrationSecrets(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	env.childInputs = resource.PropertyMap{
		"foo":    resource.NewProperty("bar"),
		"secret": resource.MakeSecret(resource.NewProperty("shh")),
	}

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	env.childName = "childB"
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
	}

	snap, err = runUpdate(t, env.plan, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)

	var child *pkgresource.State
	for _, res := range snap.Resources {
		if res.URN == childBURN {
			child = res
		}
	}
	require.NotNil(t, child)
	assert.True(t, child.Inputs["secret"].IsSecret(), "expected the secret input to stay secret")
	assert.Equal(t, "shh", child.Inputs["secret"].SecretValue().Element.StringValue())
}

// TestStateMigrationSplit tests a migration that introduces an additional resource state that was not present
// before (a one-to-many migration). The new state is effectively an unverified import, so a warning is emitted.
func TestStateMigrationSplit(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	// Version 2 registers a second child, whose state is split out of the first child by the migration.
	registerExtra := true
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				for _, res := range resources {
					if strings.HasSuffix(string(res.URN), "::childB") {
						// Already migrated.
						return nil, nil, nil
					}
				}
				var child apitype.ResourceV3
				for _, res := range resources {
					if res.URN == childAURN {
						child = res
					}
				}
				split := child
				split.URN = childBURN
				// Reuse the old ID deliberately: a new URN without an explicit predecessor is still an
				// unverified state entry even when its type and provider-assigned ID match prior state.
				split.ID = child.ID
				split.Inputs = map[string]any{"foo": "bar"}
				split.Outputs = map[string]any{"foo": "bar"}
				return append(resources, split), nil, nil
			}))
		require.NoError(t, err)

		resp, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
			StateMigrations: []*pulumirpc.Callback{callback},
		})
		if err != nil {
			return err
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
			Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
		})
		if err != nil {
			return err
		}
		if registerExtra {
			_, err = monitor.RegisterResource("pkgA:m:typA", "childB", true, deploytest.ResourceOptions{
				Parent: resp.URN,
				Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	var sawImportWarning bool
	snap, err = runUpdate(t, p, snap,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			// Everything is a same: the split state matches the new registration.
			assert.Equal(t, map[display.StepOp]int{deploy.OpSame: 4}, successOps(entries))
			for _, e := range events {
				if e.Type == DiagEvent {
					payload := e.Payload().(DiagEventPayload)
					if payload.Severity == "warning" && strings.Contains(payload.Message, "cannot verify") {
						sawImportWarning = true
					}
				}
			}
			return err
		})
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, childAURN)
	assert.Contains(t, urns, childBURN)
	assert.True(t, sawImportWarning, "expected a warning about the unverified new resource state")
}

// TestStateMigrationFold exercises an N-to-M migration by folding an old component child into the renamed managed
// child. Component states carry no physical identity and may fold freely; the managed child keeps its ID. References
// from a resource outside the component subtree are rewritten instead of being dropped.
func TestStateMigrationFold(t *testing.T) {
	t.Parallel()

	const consumerURN = resource.URN("urn:pulumi:test::test::pkgA:m:consumer::consumer")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	var migrate bool
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		options := deploytest.ResourceOptions{}
		if migrate {
			callback, err := callbacks.Allocate(
				StateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					for _, state := range resources {
						if state.URN == childCURN {
							return nil, nil, nil
						}
					}

					folded := make([]apitype.ResourceV3, 0, len(resources)-1)
					successors := make(map[resource.URN]resource.URN)
					for _, state := range resources {
						switch state.URN {
						case childAURN:
							successors[state.URN] = childCURN
							state.URN = childCURN
							folded = append(folded, state)
						case childBURN:
							successors[state.URN] = childCURN
						default:
							folded = append(folded, state)
						}
					}
					return folded, successors, nil
				}))
			require.NoError(t, err)
			options.StateMigrations = []*pulumirpc.Callback{callback}
		}

		comp, err := monitor.RegisterResource("my:module:Comp", "comp", false, options)
		if err != nil {
			return err
		}

		var canonicalURN resource.URN
		var canonicalID resource.ID
		var dependencies []resource.URN
		if migrate {
			child, err := monitor.RegisterResource("pkgA:m:typA", "childC", true, deploytest.ResourceOptions{
				Parent: comp.URN,
				Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
			})
			if err != nil {
				return err
			}
			canonicalURN, canonicalID = child.URN, child.ID
			dependencies = []resource.URN{child.URN}
		} else {
			childA, err := monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
				Parent: comp.URN,
				Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
			})
			if err != nil {
				return err
			}
			childB, err := monitor.RegisterResource("pkgA:m:typA", "childB", false, deploytest.ResourceOptions{
				Parent: comp.URN,
				Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
			})
			if err != nil {
				return err
			}
			canonicalURN, canonicalID = childA.URN, childA.ID
			dependencies = []resource.URN{childA.URN, childB.URN}
		}

		_, err = monitor.RegisterResource("pkgA:m:consumer", "consumer", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"ref": resource.MakeCustomResourceReference(canonicalURN, canonicalID, ""),
			},
			Dependencies: dependencies,
			PropertyDeps: map[resource.PropertyKey][]resource.URN{
				"ref": dependencies,
			},
			DeletedWith: canonicalURN,
			ReplaceWith: dependencies,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	snap, err := runUpdate(t, p, nil, nil)
	require.NoError(t, err)
	var canonicalID resource.ID
	for _, state := range snap.Resources {
		if state.URN == childAURN {
			canonicalID = state.ID
		}
	}
	require.NotEmpty(t, canonicalID)

	migrate = true
	snap, err = runUpdate(t, p, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 4}))
	require.NoError(t, err)
	assert.NotContains(t, snapURNs(snap), childAURN)
	assert.NotContains(t, snapURNs(snap), childBURN)
	assert.Contains(t, snapURNs(snap), childCURN)

	var consumer *pkgresource.State
	for _, state := range snap.Resources {
		if state.URN == childCURN {
			assert.Equal(t, canonicalID, state.ID)
		}
		if state.URN == consumerURN {
			consumer = state
		}
	}
	require.NotNil(t, consumer)
	assert.Equal(t, []resource.URN{childCURN}, consumer.Dependencies)
	assert.Equal(t, []resource.URN{childCURN}, consumer.PropertyDependencies["ref"])
	assert.Equal(t, childCURN, consumer.DeletedWith)
	assert.Equal(t, []resource.URN{childCURN}, consumer.ReplaceWith)
	require.True(t, consumer.Inputs["ref"].IsResourceReference())
	ref := consumer.Inputs["ref"].ResourceReferenceValue()
	assert.Equal(t, childCURN, ref.URN)
	assert.Equal(t, canonicalID, resource.ID(ref.ID.StringValue()))
}

// TestStateMigrationConstruct tests state migrations on remote components: the migration callbacks attached to
// the program's remote registration are applied when the component provider constructs the component.
func TestStateMigrationConstruct(t *testing.T) {
	t.Parallel()

	childName := "resA"
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				ConstructF: func(
					_ context.Context,
					req plugin.ConstructRequest,
					monitor *deploytest.ResourceMonitor,
				) (plugin.ConstructResponse, error) {
					resp, err := monitor.RegisterResource(req.Type, req.Name, false, deploytest.ResourceOptions{})
					require.NoError(t, err)

					_, err = monitor.RegisterResource("pkgA:m:typA", childName, true, deploytest.ResourceOptions{
						Parent: resp.URN,
						Inputs: resource.PropertyMap{"foo": resource.NewProperty(1.0)},
					})
					require.NoError(t, err)

					return plugin.ConstructResponse{URN: resp.URN}, nil
				},
			}, nil
		}),
	}

	var migrate bool
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		var migrations []*pulumirpc.Callback
		if migrate {
			migrations = []*pulumirpc.Callback{renameMigration(t, callbacks, "resA", "resB")}
		}
		_, err = monitor.RegisterResource("pkgA:m:typC", "comp", false, deploytest.ResourceOptions{
			Remote:          true,
			StateMigrations: migrations,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	snap, err := runUpdate(t, p, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resA"))

	childName, migrate = "resB", true
	snap, err = runUpdate(t, p, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resB"))
	assert.NotContains(t, urns, resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resA"))
}

// TestStateMigrationAliasedRootRename tests the component-type-change scenario: version 2 changes the component's
// type (aliased to the old type) and ships a migration that rewrites the prior URNs of the component and its
// child to the new type.
func TestStateMigrationAliasedRootRename(t *testing.T) {
	t.Parallel()

	newCompURN := resource.URN("urn:pulumi:test::test::my:module:CompV2::comp")
	newChildURN := resource.URN("urn:pulumi:test::test::my:module:CompV2$pkgA:m:typA::childA")

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	compType := "my:module:Comp"
	var migrate bool
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		options := deploytest.ResourceOptions{}
		if migrate {
			callback, err := callbacks.Allocate(
				StateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					if resources[0].URN == newCompURN {
						// Already migrated.
						return nil, nil, nil
					}
					successors := make(map[resource.URN]resource.URN)
					for i, res := range resources {
						oldURN := res.URN
						resources[i].URN = resource.URN(
							strings.Replace(string(res.URN), "my:module:Comp", "my:module:CompV2", 1))
						resources[i].Type = resources[i].URN.Type()
						successors[oldURN] = resources[i].URN
						if res.Parent != "" {
							resources[i].Parent = resource.URN(
								strings.Replace(string(res.Parent), "my:module:Comp", "my:module:CompV2", 1))
						}
					}
					return resources, successors, nil
				}))
			require.NoError(t, err)
			options.StateMigrations = []*pulumirpc.Callback{callback}
			options.Aliases = []*pulumirpc.Alias{{
				Alias: &pulumirpc.Alias_Spec_{Spec: &pulumirpc.Alias_Spec{Type: "my:module:Comp"}},
			}}
		}

		resp, err := monitor.RegisterResource(tokens.Type(compType), "comp", false, options)
		if err != nil {
			return err
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
			Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	snap, err := runUpdate(t, p, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), childAURN)

	compType, migrate = "my:module:CompV2", true
	snap, err = runUpdate(t, p, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, newCompURN)
	assert.Contains(t, urns, newChildURN)
	assert.NotContains(t, urns, compURN)
	assert.NotContains(t, urns, childAURN)
}

// TestStateMigrationEchoNoOp tests that a migration which returns its input unchanged (rather than nil) — the
// common "check whether already migrated, otherwise return the input" idiom — is treated as a no-op, even when
// the state contains secrets that serialize asymmetrically across the JSON round-trip.
func TestStateMigrationEchoNoOp(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	env.childInputs = resource.PropertyMap{
		"foo":    resource.NewProperty("bar"),
		"secret": resource.MakeSecret(resource.NewProperty("shh")),
	}
	project := env.plan.GetProject()

	snap, err := lt.TestOp(Update).Run(
		project, env.plan.GetTarget(t, nil), env.plan.Options, false, env.plan.BackendClient, nil)
	require.NoError(t, err)

	// A migration that echoes its input back verbatim, on every run.
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				return resources, nil, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	_, err = lt.TestOp(Update).Run(project, env.plan.GetTarget(t, snap), env.plan.Options, false, env.plan.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assert.Equal(t, map[display.StepOp]int{deploy.OpSame: 3}, successOps(entries))
			return err
		})
	require.NoError(t, err)
}

// TestStateMigrationSkippedDuringDestroy tests that migrations do not run during destroy --run-program.
func TestStateMigrationSkippedDuringDestroy(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	project := env.plan.GetProject()

	snap, err := lt.TestOp(Update).Run(
		project, env.plan.GetTarget(t, nil), env.plan.Options, false, env.plan.BackendClient, nil)
	require.NoError(t, err)

	// The callback fails if invoked. Under destroy it must be skipped and the child must be deleted normally.
	env.registerChild = false
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				return nil, nil, errors.New("migrations must not run during destroy")
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	var deleted []resource.URN
	snap, err = lt.TestOp(DestroyV2).Run(project, env.plan.GetTarget(t, snap), env.plan.Options, false,
		env.plan.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			for _, entry := range entries {
				if entry.Kind == TestJournalEntrySuccess && entry.Step.Op() == deploy.OpDelete {
					deleted = append(deleted, entry.Step.URN())
				}
			}
			return err
		})
	require.NoError(t, err)
	assert.Contains(t, deleted, childAURN, "the child must be deleted by destroy, not skipped by a migration")
	assert.Empty(t, snap.Resources)
}

// TestStateMigrationRejectsClaimedState tests that a migration cannot rewrite prior state that an earlier
// registration in the same deployment already claimed via an alias (a resource hoisted out of the component).
func TestStateMigrationRejectsClaimedState(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// v1: a component with a child.
	hoist := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		if hoist {
			// v2: register a top-level resource aliased to the old child URN BEFORE the component, then the
			// component with a no-op migration whose subtree still includes the (now-claimed) child.
			_, err := monitor.RegisterResource("pkgA:m:typA", "hoisted", true, deploytest.ResourceOptions{
				Inputs:    resource.PropertyMap{"foo": resource.NewProperty("bar")},
				AliasURNs: []resource.URN{childAURN},
			})
			require.NoError(t, err)

			callback, err := callbacks.Allocate(
				StateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					return resources, nil, nil
				}))
			require.NoError(t, err)
			_, err = monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
				StateMigrations: []*pulumirpc.Callback{callback},
			})
			return err
		}

		resp, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
			Parent: resp.URN,
			Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)

	hoist = true
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, "was already claimed by")
}

// TestStateMigrationRejectsClaimedRoot verifies that a registration cannot migrate root state already claimed by
// an earlier registration, whether it resolves the prior root through an alias or its own URN was used as an alias.
func TestStateMigrationRejectsClaimedRoot(t *testing.T) {
	tests := []struct {
		name     string
		conflict func(*deploytest.ResourceMonitor, *pulumirpc.Callback) error
		want     string
	}{
		{
			name: "aliased prior root was registered",
			conflict: func(monitor *deploytest.ResourceMonitor, callback *pulumirpc.Callback) error {
				_, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{})
				if err != nil {
					return err
				}
				_, err = monitor.RegisterResource("my:module:CompV2", "comp", false, deploytest.ResourceOptions{
					AliasURNs:       []resource.URN{compURN},
					StateMigrations: []*pulumirpc.Callback{callback},
				})
				return err
			},
			want: "prior state of " + string(compURN) + " was already registered earlier",
		},
		{
			name: "registration URN was claimed as alias",
			conflict: func(monitor *deploytest.ResourceMonitor, callback *pulumirpc.Callback) error {
				_, err := monitor.RegisterResource("my:module:Claimant", "claimant", false, deploytest.ResourceOptions{
					AliasURNs: []resource.URN{compURN},
				})
				if err != nil {
					return err
				}
				_, err = monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
					StateMigrations: []*pulumirpc.Callback{callback},
				})
				return err
			},
			want: "registration URN was already claimed as an alias by",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var conflict bool
			programF := deploytest.NewLanguageRuntimeF(func(
				_ plugin.RunInfo, monitor *deploytest.ResourceMonitor,
			) error {
				if !conflict {
					_, err := monitor.RegisterResource(
						"my:module:Comp", "comp", false, deploytest.ResourceOptions{})
					return err
				}

				callbacks, err := deploytest.NewCallbacksServer()
				require.NoError(t, err)
				defer func() { require.NoError(t, callbacks.Close()) }()
				callback, err := callbacks.Allocate(
					StateMigrationFunction(func(
						urn resource.URN, resources []apitype.ResourceV3,
					) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
						return resources, nil, nil
					}))
				require.NoError(t, err)
				return tt.conflict(monitor, callback)
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
			p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
			project := p.GetProject()

			snap, err := lt.TestOp(Update).Run(
				project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
			require.NoError(t, err)

			conflict = true
			_, err = lt.TestOp(Update).Run(
				project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// TestStateMigrationTargeted verifies that a state-changing migration requires a full update. No-op migrations
// remain safe during targeted updates so callbacks can stay attached after every stack has migrated.
func TestStateMigrationTargeted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options UpdateOptions
	}{
		{
			name: "target",
			options: UpdateOptions{
				Targets: deploy.NewUrnTargets([]string{string(childBURN)}),
			},
		},
		{
			name: "exclude",
			options: UpdateOptions{
				Excludes: deploy.NewUrnTargets([]string{string(compURN)}),
			},
		},
		{
			name: "target snippet",
			options: UpdateOptions{
				TargetSnippets: []string{"snippet-id"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newStateMigrationEnv(t)
			snap, err := runUpdate(t, env.plan, nil, nil)
			require.NoError(t, err)
			before := snapURNs(snap)

			env.childName = "childB"
			env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
				return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
			}

			options := env.plan.Options
			options.UpdateOptions = tt.options
			_, err = lt.TestOp(Update).Run(
				env.plan.GetProject(), env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
			require.ErrorContains(t, err, "cannot change state during a targeted or excluded update")
			assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")
		})
	}

	t.Run("no-op migration proceeds", func(t *testing.T) {
		t.Parallel()

		env := newStateMigrationEnv(t)
		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			return []*pulumirpc.Callback{renameMigration(t, callbacks, "childZ", "childA")}
		}

		options := env.plan.Options
		options.UpdateOptions = UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{string(compURN)}),
		}
		_, err = lt.TestOp(Update).Run(
			env.plan.GetProject(), env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
		require.NoError(t, err)
	})
}

// TestStateMigrationPendingOperation verifies that any recovery state must be resolved before a migration changes
// the checkpoint. No-op migrations still proceed so a permanently attached callback does not block recovery.
func TestStateMigrationPendingOperation(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*stateMigrationEnv, *deploy.Snapshot) {
		env := newStateMigrationEnv(t)
		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)

		pending := &pkgresource.State{
			URN:    "urn:pulumi:test::test::pkgA:m:consumer::pending",
			Type:   "pkgA:m:consumer",
			Custom: true,
		}
		snap.PendingOperations = append(snap.PendingOperations,
			pkgresource.NewOperation(pending, pkgresource.OperationTypeCreating))
		require.NoError(t, snap.VerifyIntegrity())
		return env, snap
	}

	t.Run("changing migration is rejected", func(t *testing.T) {
		t.Parallel()

		env, snap := setup(t)
		before := snapURNs(snap)
		env.childName = "childB"
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
		}

		_, err := runUpdate(t, env.plan, snap, nil)
		require.ErrorContains(t, err, "snapshot has 1 pending operation")
		assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")
		require.Len(t, snap.PendingOperations, 1)
	})

	t.Run("no-op migration proceeds", func(t *testing.T) {
		t.Parallel()

		env, snap := setup(t)
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			return []*pulumirpc.Callback{renameMigration(t, callbacks, "childZ", "childA")}
		}

		_, err := runUpdate(t, env.plan, snap, nil)
		require.NoError(t, err)
		require.Len(t, snap.PendingOperations, 1)
	})
}

// TestStateMigrationPendingDelete tests migrations against prior state containing a pending-delete resource
// under the migrated component (as left behind by an interrupted replacement): a migration that changes the
// state fails with an explicit error and leaves the state untouched, while an update whose migrations make no
// changes proceeds as usual and reaps the pending deletion.
func TestStateMigrationPendingDelete(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*stateMigrationEnv, *deploy.Snapshot) {
		env := newStateMigrationEnv(t)
		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)

		// Seed a pending-delete copy of the child, as an interrupted create-before-delete replacement would
		// leave behind.
		var live *pkgresource.State
		for _, res := range snap.Resources {
			if res.URN == childAURN {
				live = res
			}
		}
		require.NotNil(t, live)
		pending := live.Copy()
		pending.ID = live.ID + "-old"
		pending.Delete = true
		snap.Resources = append(snap.Resources, pending)
		require.NoError(t, snap.VerifyIntegrity())
		return env, snap
	}

	t.Run("migration with changes fails explicitly", func(t *testing.T) {
		t.Parallel()

		env, snap := setup(t)
		before := snapURNs(snap)

		env.childName = "childB"
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
		}

		snap, err := runUpdate(t, env.plan, snap, nil)
		require.ErrorContains(t, err, "pending deletion from a previous update")
		// The prior state is untouched by the rejected migration.
		assert.Equal(t, before, snapURNs(snap))
	})

	t.Run("no-op migration proceeds and reaps the pending deletion", func(t *testing.T) {
		t.Parallel()

		env, snap := setup(t)

		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			// The rename has nothing to do ("childZ" does not exist), so the migration returns no new state.
			return []*pulumirpc.Callback{renameMigration(t, callbacks, "childZ", "childA")}
		}

		snap, err := runUpdate(t, env.plan, snap,
			validateOps(t, map[display.StepOp]int{deploy.OpSame: 3, deploy.OpDeleteReplaced: 1}))
		require.NoError(t, err)
		for _, res := range snap.Resources {
			assert.False(t, res.Delete, "expected the update to reap the pending deletion")
		}
	})
}

const (
	nestedAURN = resource.URN("urn:pulumi:test::test::my:module:Comp$my:module:Nested::nestedA")
	nestedBURN = resource.URN("urn:pulumi:test::test::my:module:Comp$my:module:Nested::nestedB")
	leafAURN   = resource.URN("urn:pulumi:test::test::my:module:Comp$my:module:Nested$pkgA:m:typA::leafA")
	leafBURN   = resource.URN("urn:pulumi:test::test::my:module:Comp$my:module:Nested$pkgA:m:typA::leafB")
)

// nestedEnv is the deeper-subtree sibling of stateMigrationEnv: a component "comp" containing a nested component
// containing a custom leaf resource, with the nested component's and leaf's names controlled per-update.
type nestedEnv struct {
	plan *lt.TestPlan

	nestedName string
	leafName   string
	migrations func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback
}

func newNestedEnv(t *testing.T) *nestedEnv {
	env := &nestedEnv{nestedName: "nestedA", leafName: "leafA"}

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		var migrations []*pulumirpc.Callback
		if env.migrations != nil {
			migrations = env.migrations(t, callbacks)
		}

		comp, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
			StateMigrations: migrations,
		})
		if err != nil {
			return err
		}
		nested, err := monitor.RegisterResource("my:module:Nested", env.nestedName, false, deploytest.ResourceOptions{
			Parent: comp.URN,
		})
		if err != nil {
			return err
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", env.leafName, true, deploytest.ResourceOptions{
			Parent: nested.URN,
			Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	env.plan = &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	return env
}

// TestStateMigrationNestedComponents tests migrations over a subtree deeper than one level: a component
// containing a nested component containing a custom leaf. The migration renames both the nested component and
// the leaf, exercising the transitive subtree collection and the multi-hop parent-chain validation.
func TestStateMigrationNestedComponents(t *testing.T) {
	t.Parallel()

	t.Run("rename intermediate and leaf", func(t *testing.T) {
		t.Parallel()

		env := newNestedEnv(t)

		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)
		urns := snapURNs(snap)
		require.Contains(t, urns, nestedAURN)
		require.Contains(t, urns, leafAURN)
		var leafID resource.ID
		for _, res := range snap.Resources {
			if res.URN == leafAURN {
				leafID = res.ID
			}
		}
		require.NotEmpty(t, leafID)

		env.nestedName = "nestedB"
		env.leafName = "leafB"
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			callback, err := callbacks.Allocate(
				StateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					// The whole subtree is handed over root-first, in snapshot order.
					require.Len(t, resources, 3)
					assert.Equal(t, compURN, resources[0].URN)
					assert.Equal(t, nestedAURN, resources[1].URN)
					assert.Equal(t, leafAURN, resources[2].URN)

					renamed := renameByName(renameByName(resources, "nestedA", "nestedB"), "leafA", "leafB")
					for i := range renamed {
						if renamed[i].Parent == nestedAURN {
							renamed[i].Parent = nestedBURN
						}
					}
					return renamed, map[resource.URN]resource.URN{
						nestedAURN: nestedBURN,
						leafAURN:   leafBURN,
					}, nil
				}))
			require.NoError(t, err)
			return []*pulumirpc.Callback{callback}
		}

		snap, err = runUpdate(t, env.plan, snap,
			validateOps(t, map[display.StepOp]int{deploy.OpSame: 4}))
		require.NoError(t, err)

		urns = snapURNs(snap)
		assert.Contains(t, urns, nestedBURN)
		assert.Contains(t, urns, leafBURN)
		assert.NotContains(t, urns, nestedAURN)
		assert.NotContains(t, urns, leafAURN)
		for _, res := range snap.Resources {
			if res.URN == leafBURN {
				// The renamed leaf keeps its identity and is parented to the renamed intermediate.
				assert.Equal(t, leafID, res.ID)
				assert.Equal(t, nestedBURN, res.Parent)
			}
		}
	})

	t.Run("rewrites a child's parent to its successor", func(t *testing.T) {
		t.Parallel()

		env := newNestedEnv(t)

		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)
		// The migration renames the intermediate component and deliberately leaves the leaf parented to the old
		// URN. The engine rewrites that reference from the explicit successor mapping.
		env.nestedName = "nestedB"
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			callback, err := callbacks.Allocate(
				StateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					return renameByName(resources, "nestedA", "nestedB"),
						map[resource.URN]resource.URN{nestedAURN: nestedBURN}, nil
				}))
			require.NoError(t, err)
			return []*pulumirpc.Callback{callback}
		}

		snap, err = runUpdate(t, env.plan, snap,
			validateOps(t, map[display.StepOp]int{deploy.OpSame: 4}))
		require.NoError(t, err)
		for _, state := range snap.Resources {
			if state.URN == leafAURN {
				assert.Equal(t, nestedBURN, state.Parent)
			}
		}
	})
}

// TestStateMigrationAcrossTypes tests that an explicit successor authorizes a type change, so the new state is not
// treated as an unverified import.
func TestStateMigrationAcrossTypes(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	// Version 2 replaces the typA child with a typB child; the migration rewrites the state to the new type,
	// reusing the prior resource's ID.
	const childBTypBURN = resource.URN("urn:pulumi:test::test::my:module:Comp$pkgA:m:typB::childB")
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				successors := make(map[resource.URN]resource.URN)
				for i, res := range resources {
					if res.URN == childAURN {
						successors[res.URN] = childBTypBURN
						resources[i].URN = childBTypBURN
						resources[i].Type = "pkgA:m:typB"
						// The ID is deliberately kept: the new state reuses the old resource's ID under a
						// different type.
					}
				}
				if len(successors) == 0 {
					// Already migrated.
					return nil, nil, nil
				}
				return resources, successors, nil
			}))
		require.NoError(t, err)

		resp, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
			StateMigrations: []*pulumirpc.Callback{callback},
		})
		if err != nil {
			return err
		}
		_, err = monitor.RegisterResource("pkgA:m:typB", "childB", true, deploytest.ResourceOptions{
			Parent: resp.URN,
			Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	var sawImportWarning bool
	snap, err = runUpdate(t, p, snap,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			for _, e := range events {
				if e.Type == DiagEvent {
					payload := e.Payload().(DiagEventPayload)
					if payload.Severity != "warning" {
						continue
					}
					if strings.Contains(payload.Message, "cannot verify") {
						sawImportWarning = true
					}
				}
			}
			return err
		})
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, childBTypBURN)
	assert.NotContains(t, urns, childAURN)
	assert.False(t, sawImportWarning, "an explicit successor authorizes the type-changing continuation")
}
