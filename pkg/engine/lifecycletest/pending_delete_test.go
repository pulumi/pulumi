// Copyright 2020, Pulumi Corporation.
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
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack/snapshot"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDestroyWithPendingDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []lt.TestStep{{
		Op: Update,
		Validate: func(_ workspace.Project, _ deploy.Target, entries JournalEntries,
			_ []Event, err error,
		) error {
			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resource with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != TestJournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return err
		},
	}}
	p.Run(t, old)
}

func TestUpdateWithPendingDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	hostF := deploytest.NewPluginHostF(nil, nil, nil, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because different ordering makes the colouring different.
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []lt.TestStep{{
		Op: Destroy,
		Validate: func(_ workspace.Project, _ deploy.Target, entries JournalEntries,
			_ []Event, err error,
		) error {
			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resource with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != TestJournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return err
		},
	}}
	p.Run(t, old)
}

func TestDestroyWithUntargetedPendingDelete(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	hostF := deploytest.NewPluginHostF(nil, nil, nil, nil, nil, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:     t,
			HostF: hostF,
			// Skip display tests because different ordering makes the colouring different.
			SkipDisplayTests: true,
			UpdateOptions: engine.UpdateOptions{
				Targets: deploy.NewUrnTargetsFromUrns(
					[]resource.URN{"urn:pulumi:test::test::pkgA:m:typA::resA"}),
			},
		},
	}

	resAURN := p.NewURN("pkgA:m:typA", "resA", "")
	resBURN := p.NewURN("pkgA:m:typA", "resB", "")

	old := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				Type:    resAURN.Type(),
				URN:     resAURN,
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resBURN.Type(),
				URN:     resBURN,
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []lt.TestStep{{
		Op: Destroy,
	}}
	snap := p.Run(t, old)
	// ResB was not targeted, so it should remain in the snapshot.
	require.Len(t, snap.Resources, 1)
	require.Equal(t, resBURN, snap.Resources[0].URN)
	require.Equal(t, true, snap.Resources[0].Delete)
}

// The state holds two pending-delete copies of parent, one before and one after its protected child. The destroy
// cannot delete the child, but deletes the first copy of parent, which leaves the child before the only remaining
// copy of its parent.
func TestDestroyProtectedChildOfDuplicatePendingDeleteParent(t *testing.T) {
	t.Parallel()

	// TODO[https://github.com/pulumi/pulumi/issues/25004]: Fix the underlying issue and re-enable this test.
	t.Skip("Skipping: destroy deletes a pending-delete parent copy that precedes its protected child")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}

	snap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		prov := &pkgresource.State{
			Type:   "pulumi:providers:pkgA",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkgA::prov",
			Custom: true,
			ID:     "id-prov",
		}
		s.Resources = append(s.Resources, prov)

		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		parent := &pkgresource.State{
			Type:     "pkgA:m:typA",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:m:typA::parent",
			Custom:   true,
			Delete:   true,
			ID:       "id-parent",
			Provider: provRef.String(),
		}
		s.Resources = append(s.Resources, parent)

		child := &pkgresource.State{
			Type:     "pkgA:m:typB",
			URN:      "urn:pulumi:test-stack::test-project::pkgA:m:typA$pkgA:m:typB::child",
			Custom:   true,
			ID:       "id-child",
			Provider: provRef.String(),
			Parent:   parent.URN,
			Protect:  true,
		}
		s.Resources = append(s.Resources, child)

		s.Resources = append(s.Resources, parent.Copy())

		return s
	}()
	require.NoError(t, snap.VerifyIntegrity())

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	opts := lt.TestUpdateOptions{
		T:                t,
		HostF:            hostF,
		SkipDisplayTests: true,
	}

	_, err := lt.TestOp(Destroy).RunStep(p.GetProject(), p.GetTarget(t, snap), opts, false, p.BackendClient, nil, "1")
	require.ErrorContains(t, err, "cannot be deleted")
	_, isSIE := snapshot.AsSnapshotIntegrityError(err)
	require.False(t, isSIE, "unexpected snapshot integrity error: %v", err)
}
