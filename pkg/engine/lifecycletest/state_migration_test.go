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
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// stateMigrationFunction adapts a typed Go function into the raw callback shape expected by
// deploytest.CallbackServer.Allocate for state migration callbacks.
func stateMigrationFunction(
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

// countSuccessfulOps counts successful resource steps by operation type. For example, three unchanged resources and
// one deletion produce {same: 3, delete: 1}.
func countSuccessfulOps(entries JournalEntries) map[display.StepOp]int {
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
		assert.Equal(t, expected, countSuccessfulOps(entries))
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
		if prefix, ok := strings.CutSuffix(string(res.URN), oldSuffix); ok {
			resources[i].URN = resource.URN(prefix + newSuffix)
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
	// childRetainOnDelete marks the child resource as retained when deleted.
	childRetainOnDelete bool
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
				Parent:         resp.URN,
				Inputs:         env.childInputs,
				Protect:        &env.childProtect,
				RetainOnDelete: &env.childRetainOnDelete,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	env.plan = &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			HostF:            hostF,
			SkipDisplayTests: true,
			SnapshotManagerCapabilities: SnapshotManagerCapabilities{
				StateMigrations: true,
			},
		},
	}
	return env
}

// TestStateMigrationRejectsSafetyMetadataChanges verifies that a callback cannot clear engine-owned safety metadata
// while renaming a managed resource.
func TestStateMigrationRejectsSafetyMetadataChanges(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		clear func(*apitype.ResourceV3)
		want  string
	}{
		{
			name: "protect",
			clear: func(state *apitype.ResourceV3) {
				state.Protect = false
			},
			want: "changes Protect",
		},
		{
			name: "retain on delete",
			clear: func(state *apitype.ResourceV3) {
				state.RetainOnDelete = false
			},
			want: "changes RetainOnDelete",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := newStateMigrationEnv(t)
			env.childProtect = true
			env.childRetainOnDelete = true
			snap, err := runUpdate(t, env.plan, nil, nil)
			require.NoError(t, err)

			env.childName = "childB"
			env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
				callback, err := callbacks.Allocate(
					stateMigrationFunction(func(
						urn resource.URN, resources []apitype.ResourceV3,
					) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
						for i := range resources {
							if resources[i].URN != childAURN {
								continue
							}
							resources[i].URN = childBURN
							tt.clear(&resources[i])
							return resources, map[resource.URN]resource.URN{childAURN: childBURN}, nil
						}
						return nil, nil, nil
					}))
				require.NoError(t, err)
				return []*pulumirpc.Callback{callback}
			}

			_, err = runUpdate(t, env.plan, snap, nil)
			require.ErrorContains(t, err, tt.want)
			assert.Contains(t, snapURNs(snap), childAURN)
			assert.NotContains(t, snapURNs(snap), childBURN)
		})
	}
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
		stateMigrationFunction(func(
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

// setRenameMigration attaches the common rename migration used by lifecycle tests.
func (env *stateMigrationEnv) setRenameMigration(oldName, newName string) {
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		return []*pulumirpc.Callback{renameMigration(t, callbacks, oldName, newName)}
	}
}

// TestStateMigrationRenameChild exercises the basic migration:
//
//	childA (managed) -> childB (managed, retaining childA's ID and protection)
//
// Version 2 registers childB and ships a migration that renames the prior state to match, producing only same steps.
func TestStateMigrationRenameChild(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	env.childProtect = true

	// Version 1: component with child "childA".
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), childAURN)
	var childID resource.ID
	for _, res := range snap.Resources {
		if res.URN == childAURN {
			childID = res.ID
			require.True(t, res.Protect)
		}
	}
	require.NotEmpty(t, childID)

	// Version 2: the child is renamed to "childB" and a migration renames the prior state.
	env.childName = "childB"
	env.setRenameMigration("childA", "childB")

	// A preview must reject the callback when the backend says the corresponding update could not persist it.
	unsupportedPreviewOpts := env.plan.Options
	unsupportedPreviewOpts.SnapshotManagerCapabilities.StateMigrations = false
	_, err = lt.TestOp(Update).Run(
		env.plan.GetProject(), env.plan.GetTarget(t, snap), unsupportedPreviewOpts, true, env.plan.BackendClient, nil)
	require.ErrorContains(t, err, "state migrations are not supported by this deployment")

	// Lifecycle previews have plan generation enabled. Since a changing migration cannot be represented in a plan,
	// the preview must fail without modifying the prior snapshot.
	_, err = lt.TestOp(Update).Run(
		env.plan.GetProject(), env.plan.GetTarget(t, snap), env.plan.Options, true, env.plan.BackendClient, nil)
	require.ErrorContains(t, err, "cannot change state while generating an update plan")
	assert.Contains(t, snapURNs(snap), childAURN)
	assert.NotContains(t, snapURNs(snap), childBURN)

	snap, err = runUpdate(t, env.plan, snap,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assert.Equal(t, map[display.StepOp]int{deploy.OpSame: 3}, countSuccessfulOps(entries))
			return err
		})
	require.NoError(t, err)

	urns := snapURNs(snap)
	assert.Contains(t, urns, childBURN)
	assert.NotContains(t, urns, childAURN)
	for _, res := range snap.Resources {
		if res.URN == childBURN {
			// The renamed resource keeps its identity and resource options.
			assert.Equal(t, childID, res.ID)
			assert.Equal(t, compURN, res.Parent)
			assert.True(t, res.Protect)
		}
	}

	// A third update is a steady-state no-op: the migration sees the new shape and returns nil.
	snap, err = runUpdate(t, env.plan, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	assert.Contains(t, snapURNs(snap), childBURN)
}

// TestStateMigrationAfterRefresh exercises this refresh-and-migrate sequence:
//
//	childA -> refreshed childA -> childB (retaining the refreshed ID)
//
// `pulumi up --refresh` must rebuild the journal base before applying the migration in the update phase.
func TestStateMigrationAfterRefresh(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	var childID resource.ID
	for _, state := range snap.Resources {
		if state.URN == childAURN {
			childID = state.ID
		}
	}
	require.NotEmpty(t, childID)

	env.childName = "childB"
	env.setRenameMigration("childA", "childB")
	options := env.plan.Options
	options.Refresh = true
	snap, err = lt.TestOp(Update).Run(
		env.plan.GetProject(), env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			require.NoError(t, err)
			lastRefresh, migration := -1, -1
			for i, entry := range entries {
				if entry.Kind == TestJournalEntrySuccess && entry.Step.Op() == deploy.OpRefresh {
					lastRefresh = i
				}
				if entry.Kind == TestJournalEntryStateMigration {
					migration = i
				}
			}
			require.NotEqual(t, -1, lastRefresh, "expected refresh entries")
			require.NotEqual(t, -1, migration, "expected a state migration entry")
			assert.Less(t, lastRefresh, migration, "the refresh must complete before the migration")
			return err
		})
	require.NoError(t, err)

	assert.NotContains(t, snapURNs(snap), childAURN)
	for _, state := range snap.Resources {
		if state.URN == childBURN {
			assert.Equal(t, childID, state.ID)
			return
		}
	}
	t.Fatal("migrated child is missing from the snapshot")
}

// TestStateMigrationChained exercises two callbacks in sequence:
//
//	childA -> childB -> childC
//
// Each migration receives the previous migration's output.
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

// TestStateMigrationErrors tests that any callback error or validation failure fails the update and leaves the prior
// state untouched.
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
			callback, err := callbacks.Allocate(stateMigrationFunction(migration))
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

	t.Run("custom resource changes provider", func(t *testing.T) {
		t.Parallel()
		run(t, "changes the provider reference",
			func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				for i := range resources {
					if resources[i].URN == childAURN {
						resources[i].URN = childBURN
						oldProvider, err := sdkproviders.ParseReference(resources[i].Provider)
						require.NoError(t, err)
						newProvider, err := sdkproviders.NewReference(oldProvider.URN(), "different-provider-id")
						require.NoError(t, err)
						resources[i].Provider = newProvider.String()
					}
				}
				return resources, map[resource.URN]resource.URN{childAURN: childBURN}, nil
			}, nil)
	})
}

// TestStateMigrationSecrets exercises this migration:
//
//	childA (managed, secret input) -> childB (managed, same secret input)
//
// Secret values in the prior state must survive the callback serialization round-trip.
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
	env.setRenameMigration("childA", "childB")

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

// TestStateMigrationRejectsSplit verifies that a one-to-many migration cannot fabricate managed state. A final custom
// resource must have a custom predecessor whose provider-managed identity the engine can verify.
func TestStateMigrationRejectsSplit(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			stateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				var child apitype.ResourceV3
				for _, res := range resources {
					if res.URN == childAURN {
						child = res
					}
				}
				split := child
				split.URN = childBURN
				split.Inputs = map[string]any{"foo": "bar"}
				split.Outputs = map[string]any{"foo": "bar"}
				return append(resources, split), nil, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	_, err = runUpdate(t, env.plan, snap, nil)
	require.ErrorContains(t, err, "without a managed custom predecessor")
	assert.Contains(t, snapURNs(snap), childAURN)
	assert.NotContains(t, snapURNs(snap), childBURN)
}

// TestStateMigrationFold exercises this many-to-one migration:
//
//	childA (managed)   -> childC (managed, retaining childA's ID)
//	childB (component) -> childC
//
// References from a resource outside the component subtree are rewritten to childC and deduplicated.
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
				stateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					for _, state := range resources {
						if state.URN == childCURN {
							// We've already migrated
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

// TestStateMigrationS3BucketFold exercises a component upgrade built around the Pulumi AWS v6-to-v7 S3 bucket
// unification. It replaces BucketV2 and its separately managed versioning sidecar with one Bucket:
//
//	BucketV2 (managed) ───────────┐
//	                              ├─> Bucket (managed, retaining the shared bucket ID)
//	BucketVersioningV2 (managed) ─┘
//
// The migration moves the sidecar's versioning value onto the surviving bucket.
func TestStateMigrationS3BucketFold(t *testing.T) {
	t.Parallel()

	const (
		componentType     = "example:storage:BucketComponent"
		oldBucketType     = "aws:s3/bucketV2:BucketV2"
		oldVersioningType = "aws:s3/bucketVersioningV2:BucketVersioningV2"
		newBucketType     = "aws:s3/bucket:Bucket"

		componentURN = resource.URN("urn:pulumi:test::test::example:storage:BucketComponent::bucket")
		oldBucketURN = resource.URN(
			"urn:pulumi:test::test::example:storage:BucketComponent$aws:s3/bucketV2:BucketV2::bucket")
		oldVersioningURN = resource.URN(
			"urn:pulumi:test::test::example:storage:BucketComponent$" +
				"aws:s3/bucketVersioningV2:BucketVersioningV2::bucket-versioning")
		newBucketURN = resource.URN(
			"urn:pulumi:test::test::example:storage:BucketComponent$aws:s3/bucket:Bucket::bucket")
	)

	oldBucketInputs := resource.NewPropertyMapFromMap(map[string]any{
		"bucket": "bucket-name",
	})
	versioningInputs := resource.NewPropertyMapFromMap(map[string]any{
		"enabled": true,
	})
	newBucketInputs := resource.NewPropertyMapFromMap(map[string]any{
		"bucket":     "bucket-name",
		"versioning": true,
	})

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("aws", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// The bucket and sidecar have the same provider identity, allowing them to share a successor.
					return plugin.CreateResponse{
						ID:         "bucket-name",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var upgrade bool
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		var migrations []*pulumirpc.Callback
		if upgrade {
			callback, err := callbacks.Allocate(
				stateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					var component, oldBucket, oldVersioning *apitype.ResourceV3
					for i := range resources {
						state := &resources[i]
						switch state.URN {
						case componentURN:
							component = state
						case oldBucketURN:
							oldBucket = state
						case oldVersioningURN:
							oldVersioning = state
						}
					}
					if component == nil || oldBucket == nil || oldVersioning == nil {
						return nil, nil, errors.New("unexpected S3 migration subtree")
					}

					migratedBucket := *oldBucket
					migratedBucket.URN = newBucketURN
					migratedBucket.Type = newBucketType
					migratedBucket.Inputs["versioning"] = oldVersioning.Inputs["enabled"]
					migratedBucket.Outputs["versioning"] = oldVersioning.Outputs["enabled"]

					return []apitype.ResourceV3{*component, migratedBucket}, map[resource.URN]resource.URN{
						oldBucketURN:     newBucketURN,
						oldVersioningURN: newBucketURN,
					}, nil
				}))
			require.NoError(t, err)
			migrations = []*pulumirpc.Callback{callback}
		}

		component, err := monitor.RegisterResource(componentType, "bucket", false, deploytest.ResourceOptions{
			StateMigrations: migrations,
		})
		if err != nil {
			return err
		}
		if !upgrade {
			_, err = monitor.RegisterResource(oldBucketType, "bucket", true, deploytest.ResourceOptions{
				Parent: component.URN,
				Inputs: oldBucketInputs,
			})
			if err != nil {
				return err
			}
			_, err = monitor.RegisterResource(
				oldVersioningType, "bucket-versioning", true, deploytest.ResourceOptions{
					Parent: component.URN,
					Inputs: versioningInputs,
				})
			return err
		}

		_, err = monitor.RegisterResource(newBucketType, "bucket", true, deploytest.ResourceOptions{
			Parent: component.URN,
			Inputs: newBucketInputs,
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	plan := &lt.TestPlan{Options: lt.TestUpdateOptions{
		T:                t,
		HostF:            hostF,
		SkipDisplayTests: true,
		SnapshotManagerCapabilities: SnapshotManagerCapabilities{
			StateMigrations: true,
		},
	}}

	snap, err := runUpdate(t, plan, nil, nil)
	require.NoError(t, err)

	upgrade = true
	snap, err = runUpdate(t, plan, snap, validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	assert.NotContains(t, snapURNs(snap), oldBucketURN)
	assert.NotContains(t, snapURNs(snap), oldVersioningURN)

	var migratedBucket *pkgresource.State
	for _, state := range snap.Resources {
		if state.URN == newBucketURN {
			migratedBucket = state
		}
	}
	require.NotNil(t, migratedBucket)
	assert.Equal(t, resource.ID("bucket-name"), migratedBucket.ID)
	for _, properties := range []resource.PropertyMap{migratedBucket.Inputs, migratedBucket.Outputs} {
		versioning, ok := properties["versioning"]
		require.True(t, ok)
		assert.True(t, versioning.IsBool())
		assert.True(t, versioning.BoolValue())
	}
}

// TestStateMigrationConstruct exercises this migration inside a remote component:
//
//	remote component
//	└─ resA (managed) -> resB (managed, retaining resA's ID)
//
// The callback attached to the program's remote registration must run before the component provider constructs it.
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

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		_, err = monitor.RegisterResource("pkgA:m:typC", "comp", false, deploytest.ResourceOptions{
			Remote:          true,
			StateMigrations: []*pulumirpc.Callback{renameMigration(t, callbacks, "resA", "resB")},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	snap, err := runUpdate(t, p, nil, nil)
	require.NoError(t, err)
	require.Contains(t, snapURNs(snap), resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resA"))

	childName = "resB"
	snap, err = runUpdate(t, p, snap,
		validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resB"))
	assert.NotContains(t, urns, resource.URN("urn:pulumi:test::test::pkgA:m:typC$pkgA:m:typA::resA"))
}

// TestStateMigrationAliasedRootRename exercises this component type change:
//
//	Comp::comp                  CompV2::comp
//	└─ typA::childA     ->      └─ typA::childA (URN requalified through CompV2)
//
// Version 2 aliases the old component type and migrates both the root and child URNs to the new qualified type.
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
				stateMigrationFunction(func(
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

// TestStateMigrationEchoNoOp exercises this no-op migration:
//
//	childA (managed, secret input) -> childA (unchanged)
//
// Returning the callback input unchanged must be treated as a no-op. For a secret property, the JSON round trip
// changes the concrete Go value without changing the encoded data:
//
//	before JSON: Inputs["secret"] is *apitype.SecretV1
//	after JSON:  Inputs["secret"] is map[string]any (with the secret signature preserved)
//
// No-op detection must ignore that representation-only difference.
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
			stateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				return resources, nil, nil
			}))
		require.NoError(t, err)
		return []*pulumirpc.Callback{callback}
	}

	_, err = lt.TestOp(Update).Run(project, env.plan.GetTarget(t, snap), env.plan.Options, false, env.plan.BackendClient,
		func(project workspace.Project, target deploy.Target, entries JournalEntries, events []Event, err error) error {
			assert.Equal(t, map[display.StepOp]int{deploy.OpSame: 3}, countSuccessfulOps(entries))
			return err
		})
	require.NoError(t, err)
}

// TestStateMigrationSkippedDuringDestroy exercises this destroy path:
//
//	component + childA -> registered during destroy -> deleted (migration callback skipped)
//
// Destroy --run-program evaluates resource registrations to obtain current provider configuration and hooks, but
// still queues the registered resources for deletion. Migrations must not run while processing those registrations.
func TestStateMigrationSkippedDuringDestroy(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	project := env.plan.GetProject()

	snap, err := lt.TestOp(Update).Run(
		project, env.plan.GetTarget(t, nil), env.plan.Options, false, env.plan.BackendClient, nil)
	require.NoError(t, err)

	// The callback fails if invoked. Destroy must skip it even though the program registers the component and child.
	env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
		callback, err := callbacks.Allocate(
			stateMigrationFunction(func(
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

// TestStateMigrationRewritesLaterAlias verifies alias lookup after a migration has committed earlier in the same
// update. The program makes these registration calls in order:
//
//  1. Register the component. Its callback changes prior state from component$childA to component$childB.
//  2. Register top-level childB, with component$childA as an alias.
//
// By the second call, component$childA no longer exists in prior state. The engine must rewrite the alias to
// component$childB so the top-level registration adopts that state and retains its physical ID.
func TestStateMigrationRewritesLaterAlias(t *testing.T) {
	t.Parallel()

	const topLevelChildURN = resource.URN("urn:pulumi:test::test::pkgA:m:typA::childB")
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	upgrade := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		if upgrade {
			callback := renameMigration(t, callbacks, "childA", "childB")
			_, err = monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
				StateMigrations: []*pulumirpc.Callback{callback},
			})
			if err != nil {
				return err
			}

			// This call happens after the component registration above has run the migration. Omitting Parent makes
			// childB top-level; the alias identifies it as the original nested childA.
			_, err = monitor.RegisterResource("pkgA:m:typA", "childB", true, deploytest.ResourceOptions{
				Inputs:    resource.PropertyMap{"foo": resource.NewProperty("bar")},
				AliasURNs: []resource.URN{childAURN},
			})
			return err
		}

		component, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{})
		if err != nil {
			return err
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
			Parent: component.URN,
			Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	plan := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}

	snap, err := runUpdate(t, plan, nil, nil)
	require.NoError(t, err)
	var childID resource.ID
	for _, state := range snap.Resources {
		if state.URN == childAURN {
			childID = state.ID
		}
	}
	require.NotEmpty(t, childID)

	upgrade = true
	snap, err = runUpdate(t, plan, snap, validateOps(t, map[display.StepOp]int{deploy.OpSame: 3}))
	require.NoError(t, err)
	assert.NotContains(t, snapURNs(snap), childAURN)
	assert.NotContains(t, snapURNs(snap), childBURN)
	require.Contains(t, snapURNs(snap), topLevelChildURN)
	for _, state := range snap.Resources {
		if state.URN == topLevelChildURN {
			assert.Equal(t, childID, state.ID)
			assert.Empty(t, state.Parent)
		}
	}
}

// TestStateMigrationRejectsPredecessorURNReuse verifies that a migration's predecessor URN cannot be assigned to a
// different resource later in the same update. It covers both operations that can introduce a primary resource URN:
// registering a managed resource and reading an existing physical resource by ID.
func TestStateMigrationRejectsPredecessorURNReuse(t *testing.T) {
	t.Parallel()

	for _, useReadResource := range []bool{false, true} {
		name := "register"
		if useReadResource {
			name = "read"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}),
			}

			upgrade := false
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				callbacks, err := deploytest.NewCallbacksServer()
				require.NoError(t, err)
				defer func() { require.NoError(t, callbacks.Close()) }()

				var migrations []*pulumirpc.Callback
				if upgrade {
					migrations = []*pulumirpc.Callback{renameMigration(t, callbacks, "childA", "childB")}
				}
				component, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
					StateMigrations: migrations,
				})
				if err != nil {
					return err
				}

				if !upgrade {
					// The first update creates the predecessor state at component$childA.
					_, err = monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
						Parent: component.URN,
						Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
					})
					return err
				}

				// On the second update, registering the component above has already migrated that state to
				// component$childB. The next operation deliberately tries to reuse component$childA.
				if useReadResource {
					// ReadResource asks the provider to read the existing physical resource "read-id". The childA
					// name and component parent would assign the returned state the consumed childAURN.
					_, _, err = monitor.ReadResource(
						"pkgA:m:typA", "childA", "read-id", component.URN, nil, "", "", "", nil, "", "")
					return err
				}
				// RegisterResource likewise attempts to assign a new managed resource the consumed childAURN.
				_, err = monitor.RegisterResource("pkgA:m:typA", "childA", true, deploytest.ResourceOptions{
					Parent: component.URN,
					Inputs: resource.PropertyMap{"foo": resource.NewProperty("bar")},
				})
				return err
			})
			hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
			plan := &lt.TestPlan{Options: lt.TestUpdateOptions{
				T:                t,
				HostF:            hostF,
				SkipDisplayTests: true,
				SnapshotManagerCapabilities: SnapshotManagerCapabilities{
					StateMigrations: true,
				},
			}}

			snap, err := runUpdate(t, plan, nil, nil)
			require.NoError(t, err)
			upgrade = true

			_, err = runUpdate(t, plan, snap, nil)
			require.ErrorContains(t, err, "resource "+string(childAURN)+" cannot be registered or read")
			assert.ErrorContains(t, err, "replaced it with "+string(childBURN))
		})
	}
}

// TestStateMigrationRejectsClaimedState covers the reverse ordering of TestStateMigrationRewritesLaterAlias. The
// upgrade first registers top-level childB with component$childA as an alias, claiming childA's prior state. When the
// component registers afterward, its childA -> childB migration must not rewrite state already assigned to that
// earlier registration.
func TestStateMigrationRejectsClaimedState(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	upgrade := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()

		if upgrade {
			// This registration runs claims the nested childA state through its alias.
			_, err := monitor.RegisterResource("pkgA:m:typA", "childB", true, deploytest.ResourceOptions{
				Inputs:    resource.PropertyMap{"foo": resource.NewProperty("bar")},
				AliasURNs: []resource.URN{childAURN},
			})
			require.NoError(t, err)

			// The later component registration attempts to migrate the same childA state that childB claimed.
			callback := renameMigration(t, callbacks, "childA", "childB")
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

	upgrade = true
	_, err = lt.TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, "was already claimed by")
}

// TestStateMigrationRejectsRegisteredRoot verifies that a migration cannot rewrite prior root state already claimed
// by an earlier registration in the same update.
func TestStateMigrationRejectsRegisteredRoot(t *testing.T) {
	t.Parallel()

	const newCompURN = resource.URN("urn:pulumi:test::test::my:module:CompV2::comp")

	attemptConflictingMigration := false
	programF := deploytest.NewLanguageRuntimeF(func(
		_ plugin.RunInfo, monitor *deploytest.ResourceMonitor,
	) error {
		// Both updates register Comp. Only the second continues with a conflicting CompV2 registration.
		_, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{})
		if err != nil || !attemptConflictingMigration {
			return err
		}

		callbacks, err := deploytest.NewCallbacksServer()
		require.NoError(t, err)
		defer func() { require.NoError(t, callbacks.Close()) }()
		callback, err := callbacks.Allocate(
			stateMigrationFunction(func(
				urn resource.URN, resources []apitype.ResourceV3,
			) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
				require.Len(t, resources, 1)
				resources[0].URN = newCompURN
				resources[0].Type = newCompURN.Type()
				return resources, map[resource.URN]resource.URN{compURN: newCompURN}, nil
			}))
		require.NoError(t, err)

		// This second registration resolves the same prior root through its alias, then attempts to migrate it.
		_, err = monitor.RegisterResource("my:module:CompV2", "comp", false, deploytest.ResourceOptions{
			AliasURNs:       []resource.URN{compURN},
			StateMigrations: []*pulumirpc.Callback{callback},
		})
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil)
	p := &lt.TestPlan{Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true}}
	project := p.GetProject()

	snap, err := lt.TestOp(Update).Run(
		project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
	before := snapURNs(snap)

	attemptConflictingMigration = true
	snap, err = lt.TestOp(Update).Run(
		project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	require.ErrorContains(t, err, "prior state of "+string(compURN)+" was already registered earlier")
	assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")
	assert.NotContains(t, snapURNs(snap), newCompURN)
}

// TestStateMigrationTargeted verifies that a state-changing migration requires a full update. No-op migrations
// remain safe during targeted updates so callbacks can stay attached after every stack has migrated.
func TestStateMigrationTargeted(t *testing.T) {
	t.Parallel()

	t.Run("changing migration is rejected", func(t *testing.T) {
		t.Parallel()

		env := newStateMigrationEnv(t)
		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)
		before := snapURNs(snap)

		env.childName = "childB"
		env.setRenameMigration("childA", "childB")

		options := env.plan.Options
		options.UpdateOptions = UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{string(childBURN)}),
		}
		_, err = lt.TestOp(Update).Run(
			env.plan.GetProject(), env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
		require.ErrorContains(t, err, "cannot change state during a targeted or excluded update")
		assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")
	})

	t.Run("no-op migration proceeds", func(t *testing.T) {
		t.Parallel()

		env := newStateMigrationEnv(t)
		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)
		env.setRenameMigration("childZ", "childA")

		options := env.plan.Options
		options.UpdateOptions = UpdateOptions{
			Targets: deploy.NewUrnTargets([]string{string(compURN)}),
		}
		_, err = lt.TestOp(Update).Run(
			env.plan.GetProject(), env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
		require.NoError(t, err)
	})
}

func TestStateMigrationUpdatePlan(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	project := env.plan.GetProject()
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)
	before := snapURNs(snap)

	options := env.plan.Options
	options.GeneratePlan = true
	options.Experimental = true

	// A migration that would rewrite the checkpoint cannot be represented in an update plan, so reject plan
	// generation without modifying the prior snapshot.
	env.childName = "childB"
	env.setRenameMigration("childA", "childB")
	plan, err := lt.TestOp(Update).Plan(
		project, env.plan.GetTarget(t, snap), options, env.plan.BackendClient, nil)
	require.ErrorContains(t, err, "cannot change state while generating an update plan")
	assert.Nil(t, plan)
	assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")

	// An attached no-op migration does not rewrite the checkpoint, so plan generation remains available after the
	// stack has migrated.
	env.childName = "childA"
	env.setRenameMigration("childZ", "childA")
	plan, err = lt.TestOp(Update).Plan(
		project, env.plan.GetTarget(t, snap), options, env.plan.BackendClient, nil)
	require.NoError(t, err)
	require.NotNil(t, plan)

	// A changing migration cannot be introduced between plan generation and application either.
	env.childName = "childB"
	env.setRenameMigration("childA", "childB")
	options.Plan = plan.Clone()
	_, err = lt.TestOp(Update).Run(
		project, env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
	require.ErrorContains(t, err, "cannot change state while applying an update plan")
	assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")

	// The same plan remains usable when the attached migration is already a no-op.
	env.childName = "childA"
	env.setRenameMigration("childZ", "childA")
	options.Plan = plan.Clone()
	_, err = lt.TestOp(Update).Run(
		project, env.plan.GetTarget(t, snap), options, false, env.plan.BackendClient, nil)
	require.NoError(t, err)
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
		env.setRenameMigration("childA", "childB")

		_, err := runUpdate(t, env.plan, snap, nil)
		require.ErrorContains(t, err, "snapshot has 1 pending operation")
		assert.Equal(t, before, snapURNs(snap), "the rejected migration must leave prior state untouched")
		require.Len(t, snap.PendingOperations, 1)
	})

	t.Run("no-op migration proceeds", func(t *testing.T) {
		t.Parallel()

		env, snap := setup(t)
		env.setRenameMigration("childZ", "childA")

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
		env.setRenameMigration("childA", "childB")

		snap, err := runUpdate(t, env.plan, snap, nil)
		require.ErrorContains(t, err, "pending deletion from a previous update")
		// The prior state is untouched by the rejected migration.
		assert.Equal(t, before, snapURNs(snap))
	})

	t.Run("no-op migration proceeds and reaps the pending deletion", func(t *testing.T) {
		t.Parallel()

		env, snap := setup(t)

		// The rename has nothing to do ("childZ" does not exist), so the migration returns no new state.
		env.setRenameMigration("childZ", "childA")

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

	nestedName       string
	leafName         string
	migrations       func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback
	nestedMigrations func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback
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
		var nestedMigrations []*pulumirpc.Callback
		if env.nestedMigrations != nil {
			nestedMigrations = env.nestedMigrations(t, callbacks)
		}

		comp, err := monitor.RegisterResource("my:module:Comp", "comp", false, deploytest.ResourceOptions{
			StateMigrations: migrations,
		})
		if err != nil {
			return err
		}
		nested, err := monitor.RegisterResource("my:module:Nested", env.nestedName, false, deploytest.ResourceOptions{
			Parent:          comp.URN,
			StateMigrations: nestedMigrations,
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

// TestStateMigrationNestedComponents exercises this deeper subtree migration:
//
//	component                   component
//	└─ nestedA          ->      └─ nestedB
//	   └─ leafA                    └─ leafB (managed, retaining leafA's ID)
//
// It covers both one callback over the complete subtree and separate callbacks on the outer and nested components.
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
				stateMigrationFunction(func(
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

	t.Run("orders outer and nested migrations", func(t *testing.T) {
		t.Parallel()

		env := newNestedEnv(t)

		snap, err := runUpdate(t, env.plan, nil, nil)
		require.NoError(t, err)

		// The outer migration renames only the intermediate component. The engine rewrites the leaf's parent from
		// that successor mapping before invoking the migration registered by the nested component.
		env.nestedName = "nestedB"
		env.leafName = "leafB"
		env.migrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			return []*pulumirpc.Callback{renameMigration(t, callbacks, "nestedA", "nestedB")}
		}
		env.nestedMigrations = func(t *testing.T, callbacks *deploytest.CallbackServer) []*pulumirpc.Callback {
			callback, err := callbacks.Allocate(
				stateMigrationFunction(func(
					urn resource.URN, resources []apitype.ResourceV3,
				) ([]apitype.ResourceV3, map[resource.URN]resource.URN, error) {
					assert.Equal(t, nestedBURN, urn)
					require.Len(t, resources, 2)
					assert.Equal(t, nestedBURN, resources[0].URN)
					assert.Equal(t, nestedBURN, resources[1].Parent)
					return renameByName(resources, "leafA", "leafB"),
						map[resource.URN]resource.URN{leafAURN: leafBURN}, nil
				}))
			require.NoError(t, err)
			return []*pulumirpc.Callback{callback}
		}

		snap, err = runUpdate(t, env.plan, snap,
			validateOps(t, map[display.StepOp]int{deploy.OpSame: 4}))
		require.NoError(t, err)
		urns := snapURNs(snap)
		assert.Contains(t, urns, nestedBURN)
		assert.Contains(t, urns, leafBURN)
		assert.NotContains(t, urns, nestedAURN)
		assert.NotContains(t, urns, leafAURN)
		for _, state := range snap.Resources {
			if state.URN == leafBURN {
				assert.Equal(t, nestedBURN, state.Parent)
			}
		}
	})
}

// TestStateMigrationAcrossTypes exercises this managed-resource type change:
//
//	typA::childA (managed) -> typB::childB (managed, retaining childA's ID)
func TestStateMigrationAcrossTypes(t *testing.T) {
	t.Parallel()

	env := newStateMigrationEnv(t)
	snap, err := runUpdate(t, env.plan, nil, nil)
	require.NoError(t, err)

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
			stateMigrationFunction(func(
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

	snap, err = runUpdate(t, p, snap, nil)
	require.NoError(t, err)
	urns := snapURNs(snap)
	assert.Contains(t, urns, childBTypBURN)
	assert.NotContains(t, urns, childAURN)
}
