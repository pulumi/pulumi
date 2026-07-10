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
	f func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error),
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

		newResources, forgotten, err := f(resource.URN(migrationRequest.Urn), resources)
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
		for _, f := range forgotten {
			response.Forgotten = append(response.Forgotten, string(f))
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

// renameByName renames the resources with the given name in a serialized resource list. It returns the renamed
// list; the caller is responsible for acknowledging the old URNs as forgotten.
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

// renameMigration returns a migration callback that renames the child resource and acknowledges the old URN as
// forgotten.
func renameMigration(t *testing.T, callbacks *deploytest.CallbackServer, oldName, newName string) *pulumirpc.Callback {
	callback, err := callbacks.Allocate(
		StateMigrationFunction(func(
			urn resource.URN, resources []apitype.ResourceV3,
		) ([]apitype.ResourceV3, []resource.URN, error) {
			var forgotten []resource.URN
			for _, res := range resources {
				if strings.HasSuffix(string(res.URN), "::"+oldName) {
					forgotten = append(forgotten, res.URN)
				}
			}
			if len(forgotten) == 0 {
				// Already migrated: return the state unchanged.
				return nil, nil, nil
			}
			return renameByName(resources, oldName, newName), forgotten, nil
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
			var migrationEvents []StateMigrationEventPayload
			for _, e := range events {
				if e.Type == DiagEvent {
					payload := e.Payload().(DiagEventPayload)
					// A rename keeps the resource's identity, so no unmanaged-resource warning is emitted.
					assert.NotContains(t, payload.Message, "will NOT be deleted")
				}
				if e.Type == StateMigrationEvent {
					migrationEvents = append(migrationEvents, e.Payload().(StateMigrationEventPayload))
				}
			}
			// The applied migration is reported through a dedicated engine event.
			require.Len(t, migrationEvents, 1)
			{
				payload := migrationEvents[0]
				assert.Equal(t, compURN, payload.URN)
				assert.Equal(t, 2, payload.Migrated)
				assert.Equal(t, []resource.URN{childBURN}, payload.Added)
				assert.Equal(t, []resource.URN{childAURN}, payload.Removed)
				assert.Empty(t, payload.Unmanaged)
			}
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

// TestStateMigrationForget tests that a migration can remove a resource from state by acknowledging it as
// forgotten: no delete step runs, the resource disappears from the snapshot, and a warning is emitted.
func TestStateMigrationForget(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), childAURN)

	// Version 2 no longer has the child; the migration forgets it.
	env.registerChild = false
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, []resource.URN, error) {
				var kept []apitype.ResourceV3
				var forgotten []resource.URN
				for _, res := range resources {
					if res.URN == childAURN {
						forgotten = append(forgotten, res.URN)
						continue
					}
					kept = append(kept, res)
				}
				if len(forgotten) == 0 {
					return nil, nil, nil
				}
				return kept, forgotten, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	var sawForgetWarning bool
	var migrationEvents []StateMigrationEventPayload
	snap, err = runUpdate(t, env.plan, snap,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			// No delete step may run for the forgotten resource. (The now-unused default provider is
			// legitimately deleted.)
			for _, entry := range entries {
				if entry.Kind == TestJournalEntrySuccess && entry.Step.Op() == deploy.OpDelete {
					assert.NotEqual(t, childAURN, entry.Step.URN())
				}
			}
			for _, e := range events {
				if e.Type == DiagEvent {
					payload := e.Payload().(DiagEventPayload)
					if payload.Severity == "warning" && strings.Contains(payload.Message, "will NOT be deleted") {
						sawForgetWarning = true
					}
				}
				if e.Type == StateMigrationEvent {
					migrationEvents = append(migrationEvents, e.Payload().(StateMigrationEventPayload))
				}
			}
			return err
		})
	require.NoError(t, err)
	assert.NotContains(t, snapURNs(snap), childAURN)
	assert.True(t, sawForgetWarning, "expected a warning diagnostic about the forgotten resource")
	// The forgotten resource's identity left the state entirely, so the event reports it as unmanaged.
	require.Len(t, migrationEvents, 1)
	{
		assert.Equal(t, []resource.URN{childAURN}, migrationEvents[0].Removed)
		assert.Equal(t, []resource.URN{childAURN}, migrationEvents[0].Unmanaged)
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
			) ([]apitype.ResourceV3, []resource.URN, error) {
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
		migration func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error),
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
			func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error) {
				return nil, nil, errors.New("bad migration")
			}, nil)
	})

	t.Run("unaccounted resource", func(t *testing.T) {
		t.Parallel()
		run(t, "did not account for resource "+string(childAURN),
			func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error) {
				// Drop the child without acknowledging it.
				return resources[:1], nil, nil
			}, nil)
	})

	t.Run("forgotten resource not in state", func(t *testing.T) {
		t.Parallel()
		run(t, "which is not part of the migrated state",
			func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error) {
				return resources, []resource.URN{"urn:pulumi:test::test::pkgA:m:typA::other"}, nil
			}, nil)
	})

	t.Run("forget protected resource", func(t *testing.T) {
		t.Parallel()
		run(t, "cannot forget protected resource",
			func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error) {
				return resources[:1], []resource.URN{childAURN}, nil
			}, func(env *stateMigrationEnv) {
				env.childProtect = true
			})
	})

	t.Run("custom resource without ID", func(t *testing.T) {
		t.Parallel()
		run(t, "has no ID",
			func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error) {
				for i := range resources {
					resources[i].ID = ""
				}
				return resources, nil, nil
			}, nil)
	})

	t.Run("forgotten without new state", func(t *testing.T) {
		t.Parallel()
		run(t, "returned forgotten resources without returning a new state",
			func(urn resource.URN, resources []apitype.ResourceV3) ([]apitype.ResourceV3, []resource.URN, error) {
				return nil, []resource.URN{childAURN}, nil
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

	var child *resource.State
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
			) ([]apitype.ResourceV3, []resource.URN, error) {
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
				split.ID = child.ID + "-split"
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
				) ([]apitype.ResourceV3, []resource.URN, error) {
					if resources[0].URN == newCompURN {
						// Already migrated.
						return nil, nil, nil
					}
					var forgotten []resource.URN
					for i, res := range resources {
						forgotten = append(forgotten, res.URN)
						resources[i].URN = resource.URN(
							strings.Replace(string(res.URN), "my:module:Comp", "my:module:CompV2", 1))
						if res.Parent != "" {
							resources[i].Parent = resource.URN(
								strings.Replace(string(res.Parent), "my:module:Comp", "my:module:CompV2", 1))
						}
					}
					return resources, forgotten, nil
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

// TestStateMigrationDisplay is a display test for state migrations: step 1 applies a rename
// migration (row annotation, no warning) and step 2 applies a forget migration (row annotation
// with an unmanaged-resource warning).
func TestStateMigrationDisplay(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	env.plan.Options.SkipDisplayTests = false
	project := env.plan.GetProject()

	// Step 0: initial install of the component with childA.
	snap, err := lt.TestOp(Update).RunStep(
		project, env.plan.GetTarget(t, nil), env.plan.Options, false, env.plan.BackendClient, nil, "0")
	require.NoError(t, err)

	// Step 1: childA is renamed to childB by a migration; the resource keeps its identity.
	env.childName = "childB"
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
	}
	snap, err = lt.TestOp(Update).RunStep(
		project, env.plan.GetTarget(t, snap), env.plan.Options, false, env.plan.BackendClient, nil, "1")
	require.NoError(t, err)

	// Step 2: childB is forgotten by a migration; the resource becomes unmanaged.
	env.registerChild = false
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, []resource.URN, error) {
				var kept []apitype.ResourceV3
				var forgotten []resource.URN
				for _, res := range resources {
					if res.URN == childBURN {
						forgotten = append(forgotten, res.URN)
						continue
					}
					kept = append(kept, res)
				}
				if len(forgotten) == 0 {
					return nil, nil, nil
				}
				return kept, forgotten, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}
	_, err = lt.TestOp(Update).RunStep(
		project, env.plan.GetTarget(t, snap), env.plan.Options, false, env.plan.BackendClient, nil, "2")
	require.NoError(t, err)
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
			) ([]apitype.ResourceV3, []resource.URN, error) {
				return resources, nil, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	_, err = lt.TestOp(Update).Run(project, env.plan.GetTarget(t, snap), env.plan.Options, false, env.plan.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assert.Equal(t, map[display.StepOp]int{deploy.OpSame: 3}, successOps(entries))
			for _, e := range events {
				assert.NotEqual(t, StateMigrationEvent, e.Type,
					"an echo migration must not emit a state-migration event")
			}
			return err
		})
	require.NoError(t, err)
}

// TestStateMigrationSkippedDuringDestroy tests that migrations do not run during destroy --run-program, so a
// forget migration cannot exempt a resource from deletion.
func TestStateMigrationSkippedDuringDestroy(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	project := env.plan.GetProject()

	snap, err := lt.TestOp(Update).Run(
		project, env.plan.GetTarget(t, nil), env.plan.Options, false, env.plan.BackendClient, nil)
	require.NoError(t, err)

	// The component now ships a migration that forgets its child. Under `up` this would unmanage the child;
	// under destroy the migration must be skipped so the child is actually deleted.
	env.registerChild = false
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, []resource.URN, error) {
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
				) ([]apitype.ResourceV3, []resource.URN, error) {
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

// TestStateMigrationTargeted tests that migrations do not run for resources that are not targeted.
func TestStateMigrationTargeted(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)

	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	env.childName = "childB"
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			StateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, []resource.URN, error) {
				return nil, nil, errors.New("the migration must not run for untargeted resources")
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	// Target only the (renamed) child: the component is untargeted, so its migration must not run.
	options := env.plan.Options
	options.UpdateOptions = UpdateOptions{
		Targets: deploy.NewUrnTargets([]string{string(childBURN)}),
	}
	snap, err = lt.TestOp(Update).Run(
		env.plan.GetProject(), env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
	require.NoError(t, err)

	// Without the migration, the rename is a create of the new URN; the old resource stays in state.
	urns := snapURNs(snap)
	assert.Contains(t, urns, childAURN)
	assert.Contains(t, urns, childBURN)
}
