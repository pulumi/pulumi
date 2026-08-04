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

package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func stateMigrationResource(name string) apitype.ResourceV3 {
	return apitype.ResourceV3{
		URN:    resource.URN("urn:pulumi:test::test::pkgA:m:typA::" + name),
		Type:   "pkgA:m:typA",
		Custom: true,
		ID:     resource.ID("id-" + name),
	}
}

func stateMigrationBase() *apitype.DeploymentV3 {
	return &apitype.DeploymentV3{
		Resources: []apitype.ResourceV3{
			stateMigrationResource("a"),
			stateMigrationResource("b"),
			stateMigrationResource("c"),
		},
	}
}

func validStateMigrationEntry(base *apitype.DeploymentV3) apitype.JournalEntry {
	successor := stateMigrationResource("successor")
	successor.ID = base.Resources[0].ID
	return apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{successor},
	}
}

func TestValidateStateMigrationPatch(t *testing.T) {
	t.Parallel()

	t.Run("allows reference rewrites", func(t *testing.T) {
		t.Parallel()

		original := stateMigrationResource("original")
		patched := original
		patched.Inputs = map[string]any{"reference": "new-input"}
		patched.Outputs = map[string]any{"reference": "new-output"}
		patched.Parent = "urn:pulumi:test::test::pkgA:m:typA::parent"
		patched.Dependencies = []resource.URN{"urn:pulumi:test::test::pkgA:m:typA::dependency"}
		patched.Provider = "urn:pulumi:test::test::pulumi:providers:pkgA::provider::id"
		patched.PropertyDependencies = map[resource.PropertyKey][]resource.URN{
			"reference": {"urn:pulumi:test::test::pkgA:m:typA::property-dependency"},
		}
		patched.DeletedWith = "urn:pulumi:test::test::pkgA:m:typA::deleted-with"
		patched.ReplaceWith = []resource.URN{"urn:pulumi:test::test::pkgA:m:typA::replace-with"}
		patched.ReplacementTrigger = map[string]any{"reference": "new-trigger"}
		patched.ViewOf = "urn:pulumi:test::test::pkgA:m:typA::view-of"

		require.NoError(t, validateStateMigrationPatch(original, patched))
	})

	t.Run("rejects non-reference state", func(t *testing.T) {
		t.Parallel()

		original := stateMigrationResource("original")
		patched := original
		patched.SnippetID = "new-snippet"

		require.ErrorContains(t, validateStateMigrationPatch(original, patched),
			"changes non-reference resource state")
	})

	t.Run("allows checkpoint canonicalization", func(t *testing.T) {
		t.Parallel()

		zone := time.FixedZone("test", 60*60)
		created := time.Date(2026, time.July, 30, 12, 0, 0, 0, zone)
		modified := created.Add(time.Minute)
		original := stateMigrationResource("original")
		original.InitErrors = []string{}
		original.AdditionalSecretOutputs = []resource.PropertyKey{}
		original.Aliases = []resource.URN{}
		original.CustomTimeouts = &resource.CustomTimeouts{}
		original.Created = &created
		original.Modified = &modified
		original.StackTrace = []apitype.StackFrameV1{}
		original.IgnoreChanges = []string{}
		original.HideDiff = []resource.PropertyPath{}
		original.ReplaceOnChanges = []string{}
		original.ResourceHooks = map[resource.HookType][]string{resource.BeforeCreate: {}}

		patched := original
		patched.InitErrors = nil
		patched.AdditionalSecretOutputs = nil
		patched.Aliases = nil
		patched.CustomTimeouts = nil
		createdUTC, modifiedUTC := created.UTC(), modified.UTC()
		patched.Created = &createdUTC
		patched.Modified = &modifiedUTC
		patched.StackTrace = nil
		patched.IgnoreChanges = nil
		patched.HideDiff = nil
		patched.ReplaceOnChanges = nil
		patched.ResourceHooks = map[resource.HookType][]string{resource.BeforeCreate: nil}

		require.NoError(t, validateStateMigrationPatch(original, patched))
	})
}

// TestJournalReplayerStateMigration tests that a state migration journal entry removes the given base indices
// and inserts the migrated states at the position of the last removed resource.
func TestJournalReplayerStateMigration(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	base.Resources[2].ID = base.Resources[0].ID
	replayer := NewJournalReplayer(base)

	migrated := apitype.ResourceV3{
		URN:    "urn:pulumi:test::test::pkgA:m:typA::d",
		Type:   "pkgA:m:typA",
		Custom: true,
		ID:     "id-a",
	}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 2},
		States:     []apitype.ResourceV3{migrated},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	urns := make([]resource.URN, len(deployment.Deployment.Resources))
	for i, res := range deployment.Deployment.Resources {
		urns[i] = res.URN
	}
	// "a" and "c" are removed; "d" takes the position of "c", the last removed resource.
	assert.Equal(t, []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::b",
		"urn:pulumi:test::test::pkgA:m:typA::d",
	}, urns)
}

// TestJournalReplayerStateMigrationRemapsIndices tests that base snapshot indices recorded by entries before a
// state migration keep referring to the same resources after the migration rewrites the base snapshot.
func TestJournalReplayerStateMigrationRemapsIndices(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	dependent := stateMigrationResource("e")
	dependent.Dependencies = []resource.URN{base.Resources[2].URN}
	base.Resources = append(base.Resources, dependent)
	replayer := NewJournalReplayer(base)

	// Refresh deletes base resource "c" (index 2) before the migration runs.
	removeOld := int64(2)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindRefreshSuccess,
		OperationID: 1,
		RemoveOld:   &removeOld,
	}))

	// The migration folds "a" and "b" into "d". After the rewrite, "c" moves to index 1; the recorded refresh
	// deletion must follow it there, and deployment assembly must remove "c" from "e"'s dependencies.
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 1},
		States: []apitype.ResourceV3{{
			URN:    "urn:pulumi:test::test::pkgA:m:typA::d",
			Type:   "pkgA:m:typA",
			Custom: true,
			ID:     "id-a",
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)

	urns := make([]resource.URN, len(deployment.Deployment.Resources))
	for i, res := range deployment.Deployment.Resources {
		urns[i] = res.URN
	}
	assert.Equal(t, []resource.URN{
		"urn:pulumi:test::test::pkgA:m:typA::d",
		"urn:pulumi:test::test::pkgA:m:typA::e",
	}, urns)
	assert.Empty(t, deployment.Deployment.Resources[1].Dependencies)
}

// TestJournalReplayerStateMigrationBasePatchSupersedesOverlay verifies that a prepared base patch incorporates and
// supersedes any earlier outputs overlay for the same retained base resource, even when the migration moves its index.
func TestJournalReplayerStateMigrationBasePatchSupersedesOverlay(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	base.Resources[0].Custom, base.Resources[0].ID = false, ""
	base.Resources[1].Custom, base.Resources[1].ID = false, ""
	replayer := NewJournalReplayer(base)

	staleOverlay := base.Resources[2]
	staleOverlay.Inputs = map[string]any{"reference": string(base.Resources[0].URN)}
	removeOld := int64(2)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:   1,
		Kind:      apitype.JournalEntryKindOutputs,
		State:     &staleOverlay,
		RemoveOld: &removeOld,
	}))

	migrated := stateMigrationResource("d")
	migrated.Custom, migrated.ID = false, ""
	patched := staleOverlay
	patched.Inputs = map[string]any{"reference": string(migrated.URN)}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 1},
		States:     []apitype.ResourceV3{migrated},
		BaseStatePatches: []apitype.JournalBaseStatePatch{{
			Index: 2,
			State: patched,
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	require.Len(t, deployment.Deployment.Resources, 2)
	assert.Equal(t, migrated, deployment.Deployment.Resources[0])
	assert.Equal(t, patched, deployment.Deployment.Resources[1])
}

// TestJournalReplayerStateMigrationAppliesPatches tests that replay installs exact prepared replacements for both
// retained base resources and resources produced by earlier operations.
func TestJournalReplayerStateMigrationAppliesPatches(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	base.Resources[2].ID = base.Resources[0].ID
	dependent := stateMigrationResource("e")
	dependent.Inputs = map[string]any{"reference": string(base.Resources[0].URN)}
	base.Resources = append(base.Resources, dependent)
	replayer := NewJournalReplayer(base)

	current := stateMigrationResource("current")
	current.Inputs = map[string]any{"reference": string(base.Resources[0].URN)}
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindSuccess,
		OperationID: 42,
		State:       &current,
	}))

	d := stateMigrationResource("d")
	d.ID = base.Resources[0].ID
	patched := dependent
	patched.Inputs = map[string]any{"reference": string(d.URN)}
	patchedCurrent := current
	patchedCurrent.Inputs = map[string]any{"reference": string(d.URN)}
	// A migration folds "a" and "c" into "d".
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0, 2},
		States:     []apitype.ResourceV3{d},
		BaseStatePatches: []apitype.JournalBaseStatePatch{{
			Index: 3,
			State: patched,
		}},
		NewStatePatches: []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patchedCurrent,
		}},
	}))

	deployment, err := replayer.GenerateDeployment()
	require.NoError(t, err)
	byURN := make(map[resource.URN]apitype.ResourceV3, len(deployment.Deployment.Resources))
	for _, state := range deployment.Deployment.Resources {
		byURN[state.URN] = state
	}
	assert.Equal(t, patched, byURN[dependent.URN])
	assert.Equal(t, patchedCurrent, byURN[current.URN])
}

func TestJournalReplayerStateMigrationAllowsElidedSameBegin(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)
	// Pulumi Cloud may persist an elided Same entry as a Begin without an Operation. Since there is no provider
	// operation or embedded resource state, it does not participate in the migration transaction.
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindBegin,
		OperationID: 42,
	}))

	successor := stateMigrationResource("successor")
	successor.ID = base.Resources[0].ID
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{successor},
	}))
}

func TestJournalReplayerStateMigrationRejectsIncompleteOperation(t *testing.T) {
	t.Parallel()

	base := stateMigrationBase()
	replayer := NewJournalReplayer(base)
	require.NoError(t, replayer.Add(apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindBegin,
		OperationID: 42,
		Operation: &apitype.OperationV2{
			Resource: base.Resources[1],
			Type:     apitype.OperationTypeUpdating,
		},
	}))

	successor := stateMigrationResource("successor")
	successor.ID = base.Resources[0].ID
	err := replayer.Add(apitype.JournalEntry{
		Version:    2,
		Kind:       apitype.JournalEntryKindStateMigration,
		RemoveOlds: []int64{0},
		States:     []apitype.ResourceV3{successor},
	})
	require.ErrorContains(t, err, "cannot be applied with incomplete operation 42")
}

func TestJournalReplayerRejectsMalformedStateMigration(t *testing.T) {
	t.Parallel()

	t.Run("wrong version", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		entry.Version = 1
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "must use version 2")
	})

	t.Run("patch changes lifecycle state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		patched := base.Resources[1]
		patched.Protect = !patched.Protect
		entry.BaseStatePatches = []apitype.JournalBaseStatePatch{{Index: 1, State: patched}}
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "changes non-reference resource state")
	})

	t.Run("patch includes earlier base lifecycle markers", func(t *testing.T) {
		t.Parallel()
		for name, mark := range map[string]func(*apitype.JournalEntry, *int64){
			"delete": func(entry *apitype.JournalEntry, index *int64) {
				entry.DeleteOld = index
			},
			"pending replacement": func(entry *apitype.JournalEntry, index *int64) {
				entry.PendingReplacementOld = index
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				base := stateMigrationBase()
				replayer := NewJournalReplayer(base)
				index := int64(1)
				marker := apitype.JournalEntry{Version: 1, Kind: apitype.JournalEntryKindSuccess}
				mark(&marker, &index)
				require.NoError(t, replayer.Add(marker))

				entry := validStateMigrationEntry(base)
				patched := base.Resources[index]
				if name == "delete" {
					patched.Delete = true
				} else {
					patched.PendingReplacement = true
				}
				entry.BaseStatePatches = []apitype.JournalBaseStatePatch{{Index: index, State: patched}}
				require.NoError(t, replayer.Add(entry))

				deployment, err := replayer.GenerateDeployment()
				require.NoError(t, err)
				require.Len(t, deployment.Deployment.Resources, 3)
				if name == "delete" {
					assert.True(t, deployment.Deployment.Resources[1].Delete)
				} else {
					assert.True(t, deployment.Deployment.Resources[1].PendingReplacement)
				}
			})
		}
	})

	t.Run("inserted pending-delete state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		entry.States[0].Delete = true
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "is marked for deletion")
	})

	t.Run("inserted view state", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		entry.States[0].ViewOf = base.Resources[1].URN
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "is a view of")
	})

	t.Run("inserted custom state without id", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		entry.States[0].ID = ""
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "has no physical ID")
	})

	t.Run("missing dependency", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		entry.States[0].Dependencies = []resource.URN{
			"urn:pulumi:test::test::pkgA:m:typA::missing",
		}
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "refers to missing dependency")
	})

	t.Run("unknown extension", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		entry := validStateMigrationEntry(base)
		entry.States[0].ExtensionRef = "missing"
		err := NewJournalReplayer(base).Add(entry)
		require.ErrorContains(t, err, "references unknown extension")
	})

	t.Run("malformed provider is rejected before normalization", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		replayer := NewJournalReplayer(base)
		entry := validStateMigrationEntry(base)
		entry.States[0].Provider = "not-a-provider-reference"

		var err error
		require.NotPanics(t, func() {
			err = replayer.Add(entry)
		})
		require.ErrorContains(t, err, "failed to parse provider reference")

		deployment, generateErr := replayer.GenerateDeployment()
		require.NoError(t, generateErr)
		assert.Equal(t, base.Resources, deployment.Deployment.Resources)
	})

	t.Run("surviving new state duplicate is rejected atomically", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		replayer := NewJournalReplayer(base)
		entry := validStateMigrationEntry(base)

		current := entry.States[0]
		current.Inputs = map[string]any{"value": "before"}
		require.NoError(t, replayer.Add(apitype.JournalEntry{
			Version:     1,
			Kind:        apitype.JournalEntryKindSuccess,
			OperationID: 42,
			State:       &current,
		}))

		patched := current
		patched.Inputs = map[string]any{"value": "after"}
		entry.NewStatePatches = []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patched,
		}}
		err := replayer.Add(entry)
		require.ErrorContains(t, err, "duplicate resource")

		deployment, generateErr := replayer.GenerateDeployment()
		require.NoError(t, generateErr)
		require.Len(t, deployment.Deployment.Resources, 4)
		assert.Equal(t, map[string]any{"value": "before"}, deployment.Deployment.Resources[0].Inputs)
		assert.Equal(t, base.Resources[0].URN, deployment.Deployment.Resources[1].URN)
	})

	t.Run("new-state patch is included in prospective integrity", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		replayer := NewJournalReplayer(base)
		current := stateMigrationResource("current")
		require.NoError(t, replayer.Add(apitype.JournalEntry{
			Version:     1,
			Kind:        apitype.JournalEntryKindSuccess,
			OperationID: 42,
			State:       &current,
		}))

		entry := validStateMigrationEntry(base)
		patched := current
		patched.Dependencies = []resource.URN{
			"urn:pulumi:test::test::pkgA:m:typA::missing",
		}
		entry.NewStatePatches = []apitype.JournalNewStatePatch{{
			OperationID: 42,
			State:       patched,
		}}
		err := replayer.Add(entry)
		require.ErrorContains(t, err, "refers to missing dependency")
	})

	t.Run("unrelated pending-delete duplicate urn", func(t *testing.T) {
		t.Parallel()
		base := stateMigrationBase()
		pendingDelete := base.Resources[1]
		pendingDelete.Delete = true
		base.Resources = append(base.Resources, pendingDelete)
		entry := validStateMigrationEntry(base)
		require.NoError(t, NewJournalReplayer(base).Add(entry))
	})
}
