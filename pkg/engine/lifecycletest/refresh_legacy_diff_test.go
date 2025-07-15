// Copyright 2024, Pulumi Corporation.
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
	"reflect"
	"strconv"
	"testing"

	"github.com/blang/semver"
	combinations "github.com/mxschmitt/golang-combinations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestRefreshBasicsWithLegacyDiff(t *testing.T) {
	t.Parallel()

	names := []string{"resA", "resB", "resC"}

	// Try refreshing a stack with every combination of the three above resources as a target to
	// refresh.
	subsets := combinations.All(names)

	// combinations.All doesn't return the empty set.  So explicitly test that case (i.e. test no
	// targets specified)
	validateRefreshBasicsWithLegacyDiffCombination(t, names, []string{}, "all")

	for i, subset := range subsets {
		validateRefreshBasicsWithLegacyDiffCombination(t, names, subset, strconv.Itoa(i))
	}
}

func validateRefreshBasicsWithLegacyDiffCombination(
	t *testing.T,
	names []string,
	targets []string,
	name string,
) {
	p := &lt.TestPlan{}

	// NOTE: This is the only difference between this test and TestRefreshBasics.
	// Setting this flag should trigger old behaviour, where refresh diffs only
	// consider outputs and not the desired state. When we remove this flag, we
	// should be able to remove this test.
	p.Options.UseLegacyRefreshDiff = true

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

	newResource := func(urn resource.URN, id resource.ID, del bool, dependencies ...resource.URN) *resource.State {
		return &resource.State{
			Type:         urn.Type(),
			URN:          urn,
			Custom:       true,
			Delete:       del,
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

	p.Steps = []lt.TestStep{{
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
					// If there were changes to the outputs, we want the result op to be an OpUpdate. Otherwise we want
					// an OpSame.
					if reflect.DeepEqual(old.Outputs, expected.Outputs) {
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
		require.NoError(t, err)

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
