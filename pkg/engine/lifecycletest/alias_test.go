// Copyright 2016-2022, Pulumi Corporation.
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
	"errors"
	"fmt"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Resource is an abstract representation of a resource graph
type Resource struct {
	t                   tokens.Type
	name                string
	children            []Resource
	props               resource.PropertyMap
	aliasURNs           []resource.URN
	aliases             []resource.Alias
	dependencies        []resource.URN
	parent              resource.URN
	deleteBeforeReplace bool

	aliasSpecs         bool
	grpcRequestHeaders map[string]string
}

func registerResources(t *testing.T, monitor *deploytest.ResourceMonitor, resources []Resource) error {
	for _, r := range resources {
		r := r
		_, _, _, err := monitor.RegisterResource(r.t, r.name, true, deploytest.ResourceOptions{
			Parent:              r.parent,
			Dependencies:        r.dependencies,
			Inputs:              r.props,
			DeleteBeforeReplace: &r.deleteBeforeReplace,
			AliasURNs:           r.aliasURNs,
			Aliases:             r.aliases,
			AliasSpecs:          r.aliasSpecs,
			GrpcRequestHeaders:  r.grpcRequestHeaders,
		})
		if err != nil {
			return err
		}
		err = registerResources(t, monitor, r.children)
		if err != nil {
			return err
		}
	}
	return nil
}

type updateProgramWithResourceFunc func(*deploy.Snapshot, []Resource, []display.StepOp, bool) *deploy.Snapshot

func createUpdateProgramWithResourceFuncForAliasTests(
	t *testing.T,
	loaders []*deploytest.ProviderLoader,
) updateProgramWithResourceFunc {
	t.Helper()
	return func(
		snap *deploy.Snapshot, resources []Resource, allowedOps []display.StepOp, expectFailure bool,
	) *deploy.Snapshot {
		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			err := registerResources(t, monitor, resources)
			return err
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &TestPlan{
			Options: TestUpdateOptions{HostF: hostF},
			Steps: []TestStep{
				{
					Op:            Update,
					ExpectFailure: expectFailure,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, err error,
					) error {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								assert.Subset(t, allowedOps, []display.StepOp{payload.Metadata.Op})
							}
						}

						for _, entry := range entries {
							if entry.Step.Type() == "pulumi:providers:pkgA" {
								continue
							}
							switch entry.Kind {
							case JournalEntrySuccess:
								assert.Subset(t, allowedOps, []display.StepOp{entry.Step.Op()})
							case JournalEntryFailure:
								assert.Fail(t, "unexpected failure in journal")
							case JournalEntryBegin:
							case JournalEntryOutputs:
							}
						}

						return err
					},
				},
			},
		}
		return p.Run(t, snap)
	}
}

func TestAliases(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(res resource.URN, id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					replaceKeys := []resource.PropertyKey{}
					old, hasOld := oldOutputs["forcesReplacement"]
					new, hasNew := newInputs["forcesReplacement"]
					if hasOld && !hasNew || hasNew && !hasOld || hasOld && hasNew && old.Diff(new) != nil {
						replaceKeys = append(replaceKeys, "forcesReplacement")
					}
					return plugin.DiffResult{ReplaceKeys: replaceKeys}, nil
				},
			}, nil
		}),
	}

	updateProgramWithResource := createUpdateProgramWithResourceFuncForAliasTests(t, loaders)

	snap := updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1",
		name:    "n2",
		aliases: []resource.Alias{{Name: "n1"}},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.Alias{
			{Name: "n2"},
			{Name: "n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n2"},
			{Name: "n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n3"},
			{Name: "n2"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1"},
			{Type: "pkgA:index:t2"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1"},
			{Type: "pkgA:othermod:t3"},
			{Type: "pkgA:index:t2"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing everything (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t4",
		name: "n2",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(42),
		},
		aliases: []resource.Alias{
			{Type: "pkgA:othermod:t3", Name: "n1"},
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing everything again (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t5",
		name: "n3",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.Alias{
			{Type: "pkgA:index:t4", Name: "n2"},
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing a forceNew property while also changing type and name leads to replacement not delete+create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t6",
		name: "n4",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1000),
		},
		aliases: []resource.Alias{
			{Type: "pkgA:index:t5", Name: "n3"},
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Ensure that changing a forceNew property and deleteBeforeReplace while also changing type and name leads to
	// replacement not delete+create
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t7",
		name: "n5",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(999),
		},
		deleteBeforeReplace: true,
		aliases: []resource.Alias{
			{Type: "pkgA:index:t6", Name: "n4"},
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with depends on relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:            "pkgA:index:t2",
		name:         "n2",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1", Name: "n1"},
		},
	}, {
		t:            "pkgA:index:t2-new",
		name:         "n2-new",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1-new::n1-new"},
		aliases: []resource.Alias{
			{Type: "pkgA:index:t2", Name: "n2"},
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with parent relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:      "pkgA:index:t2",
		name:   "n2",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1", Name: "n1"},
		},
	}, {
		t:      "pkgA:index:t2-new",
		name:   "n2-new",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliases: []resource.Alias{
			{Type: "pkgA:index:t2", Name: "n2"},
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// ensure failure when different resources use duplicate aliases
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n2",
		aliases: []resource.Alias{
			{Name: "n1"},
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n3",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1", Name: "n1"},
		},
	}}, []display.StepOp{deploy.OpCreate}, true)

	// ensure different resources can use different aliases
	_ = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.Alias{
			{Name: "n1"},
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliases: []resource.Alias{
			{Type: "index:t1"},
		},
	}}, []display.StepOp{deploy.OpCreate}, false)

	// ensure that aliases of parents of parents resolves correctly
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1", Name: "n1"},
		},
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-new-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliases: []resource.Alias{
			{Type: "pkgA:index:t2", Name: "n1-sub"},
		},
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-new-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new$pkgA:index:t2::n1-new-sub"),
		aliases: []resource.Alias{
			{Type: "pkgA:index:t3", Name: "n1-sub-sub"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Test catastrophic multiplication out of aliases doesn't crash out of memory
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1-v0",
		name: "n1",
	}, {
		t:      "pkgA:index:t2-v0",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0$pkgA:index:t2-v0::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type and n2's type, but also add a load of aliases and pre-multiply them out
	// before sending to the engine
	n1Aliases := make([]resource.Alias, 0)
	n2Aliases := make([]resource.Alias, 0)
	n3Aliases := make([]resource.Alias, 0)
	for i := 0; i < 100; i++ {
		n1Aliases = append(n1Aliases, resource.Alias{URN: resource.URN(
			fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d::n1", i),
		)})

		for j := 0; j < 10; j++ {
			n2Aliases = append(n2Aliases, resource.Alias{
				URN: resource.URN(fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d$pkgA:index:t2-v%d::n1-sub", i, j)),
			})
			n3Aliases = append(n3Aliases, resource.Alias{
				Name:    "n1-sub-sub",
				Type:    fmt.Sprintf("pkgA:index:t1-v%d$pkgA:index:t2-v%d$pkgA:index:t3", i, j),
				Stack:   "test",
				Project: "test",
			})
		}
	}

	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1-v100",
		name:    "n1-new",
		aliases: n1Aliases,
	}, {
		t:       "pkgA:index:t2-v10",
		name:    "n1-new-sub",
		parent:  resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100::n1-new"),
		aliases: n2Aliases,
	}, {
		t:       "pkgA:index:t3",
		name:    "n1-new-sub-sub",
		parent:  resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100$pkgA:index:t2-v10::n1-new-sub"),
		aliases: n3Aliases,
	}}, []display.StepOp{deploy.OpSame}, false)

	var err error
	_, err = snap.NormalizeURNReferences()
	assert.NoError(t, err)

	// Start again with a parent and child.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:parent",
		name: "parent",
	}, {
		t:      "pkgA:index:child",
		parent: "urn:pulumi:test::test::pkgA:index:parent::parent",
		name:   "child",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure renaming just the child produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:parent",
		name: "parent",
	}, {
		t:      "pkgA:index:child",
		parent: "urn:pulumi:test::test::pkgA:index:parent::parent",
		name:   "childnew",
		aliases: []resource.Alias{
			{Name: "child"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure changing just the child's type produces Same
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:parent",
		name: "parent",
	}, {
		t:      "pkgA:index:child2",
		parent: "urn:pulumi:test::test::pkgA:index:parent::parent",
		name:   "childnew",
		aliases: []resource.Alias{
			{Type: "pkgA:index:child"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Start again with multiple nested children.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "parent",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1::parent",
		name:   "sub",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t1::sub",
		name:   "sub-sub",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure renaming the bottom child produces Same.
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "parent",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1::parent",
		name:   "sub",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t1::sub",
		name:   "sub-sub-new",
		aliases: []resource.Alias{
			{Name: "sub-sub"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Start again with two resources with no relationship.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:    "pkgA:index:t2",
		name: "two",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now make "two" a child of "one" ensuring no changes.
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:      "pkgA:index:t2",
		parent: "urn:pulumi:test::test::pkgA:index:t1::one",
		name:   "two",
		aliases: []resource.Alias{
			{NoParent: true},
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Now remove the parent relationship.
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:    "pkgA:index:t2",
		name: "two",
		aliases: []resource.Alias{
			{Parent: "urn:pulumi:test::test::pkgA:index:t1::one"},
		},
	}}, []display.StepOp{deploy.OpSame}, false)
}

func TestAliasesNodeJSBackCompat(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	updateProgramWithResource := createUpdateProgramWithResourceFuncForAliasTests(t, loaders)

	tests := []struct {
		name               string
		aliasSpecs         bool
		grpcRequestHeaders map[string]string
		noParentAlias      resource.Alias
	}{
		{
			name:               "Old Node SDK",
			grpcRequestHeaders: map[string]string{"pulumi-runtime": "nodejs"},
			// Old Node.js SDKs set Parent to "" rather than setting NoParent to true,
			noParentAlias: resource.Alias{Parent: ""},
		},
		{
			name:               "New Node SDK",
			grpcRequestHeaders: map[string]string{"pulumi-runtime": "nodejs"},
			// Indicate we're sending alias specs correctly.
			aliasSpecs: true,
			// Properly set NoParent to true.
			noParentAlias: resource.Alias{NoParent: true},
		},
		{
			name: "Unknown SDK",
			// Properly set NoParent to true.
			noParentAlias: resource.Alias{NoParent: true},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			snap := updateProgramWithResource(nil, []Resource{{
				t:                  "pkgA:index:t1",
				name:               "one",
				aliasSpecs:         tt.aliasSpecs,
				grpcRequestHeaders: tt.grpcRequestHeaders,
			}, {
				t:                  "pkgA:index:t2",
				name:               "two",
				aliasSpecs:         tt.aliasSpecs,
				grpcRequestHeaders: tt.grpcRequestHeaders,
			}}, []display.StepOp{deploy.OpCreate}, false)

			// Now make "two" a child of "one" ensuring no changes.
			snap = updateProgramWithResource(snap, []Resource{{
				t:                  "pkgA:index:t1",
				name:               "one",
				aliasSpecs:         tt.aliasSpecs,
				grpcRequestHeaders: tt.grpcRequestHeaders,
			}, {
				t:      "pkgA:index:t2",
				parent: "urn:pulumi:test::test::pkgA:index:t1::one",
				name:   "two",
				aliases: []resource.Alias{
					tt.noParentAlias,
				},
				aliasSpecs:         tt.aliasSpecs,
				grpcRequestHeaders: tt.grpcRequestHeaders,
			}}, []display.StepOp{deploy.OpSame}, false)

			// Now remove the parent relationship.
			_ = updateProgramWithResource(snap, []Resource{{
				t:    "pkgA:index:t1",
				name: "one",
			}, {
				t:    "pkgA:index:t2",
				name: "two",
				aliases: []resource.Alias{
					{Parent: "urn:pulumi:test::test::pkgA:index:t1::one"},
				},
				aliasSpecs:         tt.aliasSpecs,
				grpcRequestHeaders: tt.grpcRequestHeaders,
			}}, []display.StepOp{deploy.OpSame}, false)
		})
	}
}

func TestAliasURNs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(res resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					replaceKeys := []resource.PropertyKey{}
					old, hasOld := oldOutputs["forcesReplacement"]
					new, hasNew := newInputs["forcesReplacement"]
					if hasOld && !hasNew || hasNew && !hasOld || hasOld && hasNew && old.Diff(new) != nil {
						replaceKeys = append(replaceKeys, "forcesReplacement")
					}
					return plugin.DiffResult{ReplaceKeys: replaceKeys}, nil
				},
			}, nil
		}),
	}

	updateProgramWithResource := createUpdateProgramWithResourceFuncForAliasTests(t, loaders)

	snap := updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:         "pkgA:index:t1",
		name:      "n2",
		aliasURNs: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n3",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure that changing everything (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t4",
		name: "n2",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(42),
		},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing everything again (including props) leads to update not delete and re-create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t5",
		name: "n3",
		props: resource.PropertyMap{
			resource.PropertyKey("x"): resource.NewNumberProperty(1000),
		},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t4::n2",
		},
	}}, []display.StepOp{deploy.OpUpdate}, false)

	// Ensure that changing a forceNew property while also changing type and name leads to replacement not delete+create
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t6",
		name: "n4",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1000),
		},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t5::n3",
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Ensure that changing a forceNew property and deleteBeforeReplace while also changing type and name leads to
	// replacement not delete+create
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t7",
		name: "n5",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(999),
		},
		deleteBeforeReplace: true,
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t6::n4",
		},
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with depends on relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:            "pkgA:index:t2",
		name:         "n2",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:            "pkgA:index:t2-new",
		name:         "n2-new",
		dependencies: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1-new::n1-new"},
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t2::n2",
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// Start again - this time with two resources with parent relationship
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(1),
		},
		deleteBeforeReplace: true,
	}, {
		t:      "pkgA:index:t2",
		name:   "n2",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		props: resource.PropertyMap{
			resource.PropertyKey("forcesReplacement"): resource.NewNumberProperty(2),
		},
		deleteBeforeReplace: true,
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:      "pkgA:index:t2-new",
		name:   "n2-new",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n2",
		},
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false)

	// ensure failure when different resources use duplicate aliases
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n2",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpCreate}, true)

	// ensure different resources can use different aliases
	_ = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:    "pkgA:index:t2",
		name: "n2",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []display.StepOp{deploy.OpCreate}, false)

	// ensure that aliases of parents of parents resolves correctly
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1-new",
		name: "n1-new",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}, {
		t:      "pkgA:index:t2",
		name:   "n1-new-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new::n1-new"),
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::n1-sub",
		},
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-new-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-new$pkgA:index:t2::n1-new-sub"),
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2$pkgA:index:t3::n1-sub-sub",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Test catastrophic multiplication out of aliases doesn't crash out of memory
	// first create a chain of resources such that we have n1 -> n1-sub -> n1-sub-sub
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1-v0",
		name: "n1",
	}, {
		t:      "pkgA:index:t2-v0",
		name:   "n1-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0::n1"),
	}, {
		t:      "pkgA:index:t3",
		name:   "n1-sub-sub",
		parent: resource.URN("urn:pulumi:test::test::pkgA:index:t1-v0$pkgA:index:t2-v0::n1-sub"),
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now change n1's name and type and n2's type, but also add a load of aliases and pre-multiply them out
	// before sending to the engine
	n1Aliases := make([]resource.URN, 0)
	n2Aliases := make([]resource.URN, 0)
	n3Aliases := make([]resource.URN, 0)
	for i := 0; i < 100; i++ {
		n1Aliases = append(n1Aliases, resource.URN(
			fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d::n1", i)))

		for j := 0; j < 10; j++ {
			n2Aliases = append(n2Aliases, resource.URN(
				fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d$pkgA:index:t2-v%d::n1-sub", i, j)))

			n3Aliases = append(n3Aliases, resource.URN(
				fmt.Sprintf("urn:pulumi:test::test::pkgA:index:t1-v%d$pkgA:index:t2-v%d$pkgA:index:t3::n1-sub-sub", i, j)))
		}
	}

	snap = updateProgramWithResource(snap, []Resource{{
		t:         "pkgA:index:t1-v100",
		name:      "n1-new",
		aliasURNs: n1Aliases,
	}, {
		t:         "pkgA:index:t2-v10",
		name:      "n1-new-sub",
		parent:    resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100::n1-new"),
		aliasURNs: n2Aliases,
	}, {
		t:         "pkgA:index:t3",
		name:      "n1-new-sub-sub",
		parent:    resource.URN("urn:pulumi:test::test::pkgA:index:t1-v100$pkgA:index:t2-v10::n1-new-sub"),
		aliasURNs: n3Aliases,
	}}, []display.StepOp{deploy.OpSame}, false)

	var err error
	_, err = snap.NormalizeURNReferences()
	assert.NoError(t, err)

	// Start again with a parent and child.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:parent",
		name: "parent",
	}, {
		t:      "pkgA:index:child",
		parent: "urn:pulumi:test::test::pkgA:index:parent::parent",
		name:   "child",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure renaming just the child produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:parent",
		name: "parent",
	}, {
		t:      "pkgA:index:child",
		parent: "urn:pulumi:test::test::pkgA:index:parent::parent",
		name:   "childnew",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:parent$pkgA:index:child::child",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Ensure changing just the child's type produces Same
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:parent",
		name: "parent",
	}, {
		t:      "pkgA:index:child2",
		parent: "urn:pulumi:test::test::pkgA:index:parent::parent",
		name:   "childnew",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:parent$pkgA:index:child::childnew",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Start again with multiple nested children.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "parent",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1::parent",
		name:   "sub",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t1::sub",
		name:   "sub-sub",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Ensure renaming the bottom child produces Same.
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "parent",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1::parent",
		name:   "sub",
	}, {
		t:      "pkgA:index:t1",
		parent: "urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t1::sub",
		name:   "sub-sub-new",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t1$pkgA:index:t1::sub-sub",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Start again with two resources with no relationship.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:    "pkgA:index:t2",
		name: "two",
	}}, []display.StepOp{deploy.OpCreate}, false)

	// Now make "two" a child of "one" ensuring no changes.
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:      "pkgA:index:t2",
		parent: "urn:pulumi:test::test::pkgA:index:t1::one",
		name:   "two",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t2::two",
		},
	}}, []display.StepOp{deploy.OpSame}, false)

	// Now remove the parent relationship.
	_ = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:    "pkgA:index:t2",
		name: "two",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1$pkgA:index:t2::two",
		},
	}}, []display.StepOp{deploy.OpSame}, false)
}

func TestDuplicatesDueToAliases(t *testing.T) {
	t.Parallel()

	// This is a test for https://github.com/pulumi/pulumi/issues/11173
	// to check that we don't allow resource aliases to refer to other resources.
	// That is if you have A, then try and add B saying it's alias is A we should error that's a duplicate.
	// We need to be careful that we handle this regardless of the order we send the RegisterResource requests for A and B.

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	mode := 0
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, just make resA
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// First test case, try and create a new B that aliases to A. First make the A like normal...
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			// ... then make B with an alias, it should error
			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []resource.Alias{{Name: "resA"}},
				})
			assert.Error(t, err)

		case 2:
			// Second test case, try and create a new B that aliases to A. First make the B with an alias...
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []resource.Alias{{Name: "resA"}},
				})
			assert.NoError(t, err)

			// ... then try to make the A like normal. It should error that it's already been aliased away
			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.Error(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the starting A resource
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Set mode to try and create A then a B that aliases to it, this should fail
	mode = 1
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Set mode to try and create B first then a A, this should fail
	mode = 2
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	// Because we made the B first that's what should end up in the state file
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[1].URN)
}

func TestCorrectResourceChosen(t *testing.T) {
	t.Parallel()

	// This is a test for https://github.com/pulumi/pulumi/issues/13848
	// to check that a resource's URN is used first when looking for old resources in the state
	// rather than aliases, and that we don't end up with a corrupt state after an update.

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	mode := 0
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, make "resA", "resB with resA as its parent", and "resB with no parent".
			aURN, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{Parent: aURN})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// Next case, make "resA" and "resB with no parent and alias to have resA as its parent".
			aURN, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{Aliases: []resource.Alias{{Parent: aURN}}})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()

	// Run an update for initial state with "resA", "resB with resA as its parent", and "resB with no parent".
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[3].URN)

	// Run the next case, with "resA" and "resB with no parent and alias to have resA as its parent".
	mode = 1
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[2].URN)
	assert.Len(t, snap.Resources[2].Aliases, 0)
}

func TestComponentToCustomUpdate(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/12550, check that if we change a component resource
	// into a custom resource the engine handles that best it can. This depends on the provider being able to
	// cope with the component state being passed as custom state.

	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := resource.ID("")
					if !preview {
						id = resource.ID("1")
					}
					return id, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN,
					id resource.ID, olds resource.PropertyMap, timeout float64,
				) (resource.Status, error) {
					return resource.StatusOK, nil
				},
				DiffF: func(urn resource.URN,
					id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap, ignoreChanges []string,
				) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	insA := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createA := func(monitor *deploytest.ResourceMonitor) {
		_, _, _, err := monitor.RegisterResource("prog::myType", "resA", false, deploytest.ResourceOptions{
			Inputs: insA,
		})
		assert.NoError(t, err)
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		createA(monitor)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 1)
	assert.Equal(t, tokens.Type("prog::myType"), snap.Resources[0].Type)
	assert.False(t, snap.Resources[0].Custom)

	// Now update A from a component to custom with an alias
	createA = func(monitor *deploytest.ResourceMonitor) {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: insA,
			Aliases: []resource.Alias{
				{
					Type: "prog::myType",
				},
			},
		})
		assert.NoError(t, err)
	}
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert that A is now a custom
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	// Now two because we'll have a provider now
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].Type)
	assert.True(t, snap.Resources[1].Custom)

	// Now update A back to a component (with an alias)
	createA = func(monitor *deploytest.ResourceMonitor) {
		_, _, _, err := monitor.RegisterResource("prog::myType", "resA", false, deploytest.ResourceOptions{
			Inputs: insA,
			Aliases: []resource.Alias{
				{
					Type: "pkgA:m:typA",
				},
			},
		})
		assert.NoError(t, err)
	}
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	// Assert that A is now a custom
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	// Back to one because the provider should have been cleaned up as well
	assert.Len(t, snap.Resources, 1)
	assert.Equal(t, tokens.Type("prog::myType"), snap.Resources[0].Type)
	assert.False(t, snap.Resources[0].Custom)
}

func TestParentAlias(t *testing.T) {
	// Test for https://github.com/pulumi/pulumi/issues/13324, check that if we change a parent resource which
	// is also aliased at the same time that we track this correctly.

	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					id := resource.ID("")
					if !preview {
						id = resource.ID("1")
					}
					return id, news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	firstRun := true
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		urnA, _, _, err := monitor.RegisterResource("prog:index:myStandardType", "resA", false, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		if firstRun {
			urnB, _, _, err := monitor.RegisterResource("prog:index:myType", "resB", false, deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Parent: urnB,
			})
			assert.NoError(t, err)
		} else {
			urnB, _, _, err := monitor.RegisterResource("prog:index:myType", "resB", false, deploytest.ResourceOptions{
				Parent: urnA,
				Aliases: []resource.Alias{
					{
						NoParent: true,
					},
				},
			})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Parent: urnA,
				Aliases: []resource.Alias{
					{
						Parent: urnB,
					},
				},
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)

	// Now run again with the rearranged parents, we don't expect to see any replaces
	firstRun = false
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(project workspace.Project, target deploy.Target,
			entries JournalEntries, events []Event, err error,
		) error {
			for _, entry := range entries {
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}
			return err
		})
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
}

func TestSplitUpdateComponentAliases(t *testing.T) {
	t.Parallel()

	// This is a test for https://github.com/pulumi/pulumi/issues/13903 to check that if a component is
	// aliased the internal resources follow it across a split deployment.

	mode := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					// We should only create things in the first pass
					assert.Equal(t, 0, mode, "%s tried to create but should be aliased", urn)

					return "created-id", news, resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, make "resA", and "resB" a component with "resA" as its parent and "resC" in the component
			aURN, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			bURN, _, _, err := monitor.RegisterResource(
				"pkgA:m:typB",
				"resB",
				false,
				deploytest.ResourceOptions{
					Parent: aURN,
				})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typC",
				"resC",
				true,
				deploytest.ResourceOptions{
					Parent: bURN,
				})
			assert.NoError(t, err)

		case 1:
			// Delete "resA" and re-parent "resB" to the root but then fail before getting to resC.
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typB",
				"resB",
				false,
				deploytest.ResourceOptions{
					AliasURNs: []resource.URN{
						"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB::resB",
					},
				})
			assert.NoError(t, err)

			return errors.New("something went bang")

		case 2:
			// Update again but this time succeed and register C.
			bURN, _, _, err := monitor.RegisterResource(
				"pkgA:m:typB",
				"resB",
				false,
				deploytest.ResourceOptions{
					AliasURNs: []resource.URN{
						"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB::resB",
					},
				})
			assert.NoError(t, err)

			_, _, _, err = monitor.RegisterResource(
				"pkgA:m:typC",
				"resC",
				true,
				deploytest.ResourceOptions{
					Parent: bURN,
				})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()

	// Run an update for initial state with "resA", "resB", and "resC".
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB::resB"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[2].Parent)
	assert.Equal(t,
		resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB$pkgA:m:typC::resC"),
		snap.Resources[3].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB::resB"), snap.Resources[3].Parent)

	// Run the next case, resB should be re-parented to the root. A should still be left (because we couldn't
	// tell it needed to delete due to the error), C should have it's old URN but new parent because it wasn't
	// registered.
	mode = 1
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resB"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN(""), snap.Resources[0].Parent)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[2].URN)
	// Even though we didn't register C its URN must update to take the re-parenting into account.
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB$pkgA:m:typC::resC"), snap.Resources[3].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resB"), snap.Resources[3].Parent)

	mode = 2
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resB"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN(""), snap.Resources[0].Parent)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB$pkgA:m:typC::resC"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resB"), snap.Resources[2].Parent)
}

func TestFailDeleteDuplicateAliases(t *testing.T) {
	t.Parallel()

	// This is a test for https://github.com/pulumi/pulumi/issues/14041 to check that we don't courrupt state files when
	// old aliased resources are left in state because they can't delete.
	//
	// Imagine the following update flow from an old engine version where we saved aliases to state:
	//
	// 1) Creates "resA".
	//
	// 2) Makes "resAX" with an alias to "resA" so end up with one resource in the state file "resAX", but it records
	// it's alias as "resA".
	//
	// 3) Tries to destroy "resA" but fails so leaves it in the state file as is, this step doesn't actually seem
	// important for triggering the error.
	//
	// 4) Creates "resA" and again tries to delete "resAX" but again fails, now we try and save the snapshot but before
	// we save the snapshot we do URN normalisation - That looks at all the resources and sees which ones we're aliased
	// by looking at the Aliases field on their state - "resAX" still has it's alias recorded from Update 2 so it
	// registers a URN fixup of "resA" -> "resAX" - We then rewrite all the resources with that rule which means any new
	// resources created in the update (new resources are always first in the snapshot list) that were either called
	// resA or tried to use resA as a parent now all rewrite their pointers and URNs to "resAX" which is at the end of
	// the snapshot list (this explains why we we're seeing child before parent before 3.83, and now see duplicate
	// resource) - Voila bad snapshot.
	//
	// To prevent the above the engine now doesn't save Aliases in state anymore, so when we run update 4 above it
	// doesn't see any aliases saved for "resAX" and so doesn't do the broken normalisation.

	mode := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					// We should only create things in the first and last pass
					ok := mode == 0 || mode == 2
					assert.True(t, ok, "%s tried to create but should be aliased", urn)

					return "created-id", news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN, id resource.ID, olds resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					// We should only delete things in the last pass
					ok := mode == 2
					assert.True(t, ok, "%s tried to delete but should be aliased", urn)

					return resource.StatusUnknown, errors.New("can't delete")
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			fallthrough
		case 2:
			// Default case, and also the last case make "resA"
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// Rename "resA" to "resAX" with an alias
			_, _, _, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resAX",
				true,
				deploytest.ResourceOptions{
					Aliases: []resource.Alias{
						{
							Name: "resA",
						},
					},
				})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{
		Options: TestUpdateOptions{HostF: hostF},
	}

	project := p.GetProject()

	// Run an update for initial state with "resA"
	snap, err := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Run the next case, resA should be aliased
	mode = 1
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resAX"), snap.Resources[1].URN)

	// Run the last case, resAX should try to delete and resA should be created. We can't possibly know that resA ==
	// resAX at this point because we're not being sent aliases.
	mode = 2
	snap, err = TestOp(Update).Run(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resAX"), snap.Resources[2].URN)
}
