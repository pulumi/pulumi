// Copyright 2016-2024, Pulumi Corporation.
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
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/display"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
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

// Ideally we'd get rid of this method, we probably shouldn't be talking in terms on `resource.Alias` in tests (they
// should be tested against the rpc protocol), and we should probably be using a stricter type in the engine.
func aliasesFromAliases(aliases []resource.Alias) []*pulumirpc.Alias {
	result := make([]*pulumirpc.Alias, len(aliases))
	for i, alias := range aliases {
		if alias.URN != "" {
			result[i] = &pulumirpc.Alias{
				Alias: &pulumirpc.Alias_Urn{
					Urn: string(alias.URN),
				},
			}
		} else {
			spec := &pulumirpc.Alias_Spec{
				Name:    alias.Name,
				Type:    alias.Type,
				Project: alias.Project,
				Stack:   alias.Stack,
			}

			if alias.Parent != "" {
				spec.Parent = &pulumirpc.Alias_Spec_ParentUrn{
					ParentUrn: string(alias.Parent),
				}
			} else if alias.NoParent {
				spec.Parent = &pulumirpc.Alias_Spec_NoParent{
					NoParent: alias.NoParent,
				}
			}

			result[i] = &pulumirpc.Alias{
				Alias: &pulumirpc.Alias_Spec_{
					Spec: spec,
				},
			}
		}
	}
	return result
}

func makeUrnAlias(urn string) *pulumirpc.Alias {
	return &pulumirpc.Alias{
		Alias: &pulumirpc.Alias_Urn{
			Urn: urn,
		},
	}
}

func makeSpecAlias(name, typ, project, stack string) *pulumirpc.Alias {
	return &pulumirpc.Alias{
		Alias: &pulumirpc.Alias_Spec_{
			Spec: &pulumirpc.Alias_Spec{
				Name:    name,
				Type:    typ,
				Project: project,
				Stack:   stack,
			},
		},
	}
}

func makeSpecAliasWithParent(name, typ, project, stack, parent string) *pulumirpc.Alias {
	return &pulumirpc.Alias{
		Alias: &pulumirpc.Alias_Spec_{
			Spec: &pulumirpc.Alias_Spec{
				Name:    name,
				Type:    typ,
				Project: project,
				Stack:   stack,
				Parent: &pulumirpc.Alias_Spec_ParentUrn{
					ParentUrn: parent,
				},
			},
		},
	}
}

func makeSpecAliasWithNoParent(name, typ, project, stack string, parent bool) *pulumirpc.Alias {
	return &pulumirpc.Alias{
		Alias: &pulumirpc.Alias_Spec_{
			Spec: &pulumirpc.Alias_Spec{
				Name:    name,
				Type:    typ,
				Project: project,
				Stack:   stack,
				Parent: &pulumirpc.Alias_Spec_NoParent{
					NoParent: parent,
				},
			},
		},
	}
}

func registerResources(t *testing.T, monitor *deploytest.ResourceMonitor, resources []Resource) error {
	for _, r := range resources {
		r := r
		_, err := monitor.RegisterResource(r.t, r.name, true, deploytest.ResourceOptions{
			Parent:              r.parent,
			Dependencies:        r.dependencies,
			Inputs:              r.props,
			DeleteBeforeReplace: &r.deleteBeforeReplace,
			AliasURNs:           r.aliasURNs,
			Aliases:             aliasesFromAliases(r.aliases),
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

type updateProgramWithResourceFunc func(*deploy.Snapshot, []Resource, []display.StepOp, bool, string) *deploy.Snapshot

func createUpdateProgramWithResourceFuncForAliasTests(
	t *testing.T,
	loaders []*deploytest.ProviderLoader,
) updateProgramWithResourceFunc {
	t.Helper()

	return func(
		snap *deploy.Snapshot, resources []Resource, allowedOps []display.StepOp, expectFailure bool, name string,
	) *deploy.Snapshot {
		programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
			err := registerResources(t, monitor, resources)
			return err
		})
		hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)
		p := &lt.TestPlan{
			Options: lt.TestUpdateOptions{T: t, HostF: hostF},
			Steps: []lt.TestStep{
				{
					Op:            Update,
					ExpectFailure: expectFailure,
					Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
						events []Event, err error,
					) error {
						for _, event := range events {
							if event.Type == ResourcePreEvent {
								payload := event.Payload().(ResourcePreEventPayload)
								if payload.Metadata.Type == "pulumi:providers:pkgA" {
									continue
								}
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
		return p.RunWithName(t, snap, name)
	}
}

func TestAliases(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					replaceKeys := []resource.PropertyKey{}
					old, hasOld := req.OldOutputs["forcesReplacement"]
					new, hasNew := req.NewInputs["forcesReplacement"]
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
	}}, []display.StepOp{deploy.OpCreate}, false, "t1")

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:       "pkgA:index:t1",
		name:    "n2",
		aliases: []resource.Alias{{Name: "n1"}},
	}}, []display.StepOp{deploy.OpSame}, false, "t1-n2")

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.Alias{
			{Name: "n2"},
			{Name: "n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false, "t1-n3")

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n2"},
			{Name: "n1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false, "t1-n3-full")

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliases: []resource.Alias{
			{URN: "urn:pulumi:test::test::pkgA:index:t1::n3"},
			{Name: "n2"},
		},
	}}, []display.StepOp{deploy.OpSame}, false, "t1-n1-alias-back")

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false, "t1-n1-remove-aliases")

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1"},
		},
	}}, []display.StepOp{deploy.OpSame}, false, "t2-n1")

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1"},
			{Type: "pkgA:index:t2"},
		},
	}}, []display.StepOp{deploy.OpSame}, false, "t3-n1")

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliases: []resource.Alias{
			{Type: "pkgA:index:t1"},
			{Type: "pkgA:othermod:t3"},
			{Type: "pkgA:index:t2"},
		},
	}}, []display.StepOp{deploy.OpSame}, false, "t3-n1-order")

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false, "t3-remove-aliases")

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
	}}, []display.StepOp{deploy.OpUpdate}, false, "t4-n2")

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
	}}, []display.StepOp{deploy.OpUpdate}, false, "t5-n3")

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
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false, "t6-n4")

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
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false, "t7-n5")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "t1-start-again")

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
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced},
		false, "t1-new-t2-new")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "t1-t2-start-again")

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
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced},
		false, "t1-new-t2-new-parent")

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
	}}, []display.StepOp{deploy.OpCreate}, true, "t1-fail-duplicate")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "different-aliases")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "parents")

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
	}}, []display.StepOp{deploy.OpSame}, false, "n1-name-change")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "multiplication")

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
	}}, []display.StepOp{deploy.OpSame}, false, "many-alaises")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "start-again-parent-child")

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
	}}, []display.StepOp{deploy.OpSame}, false, "child-same")

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
	}}, []display.StepOp{deploy.OpSame}, false, "change-child-same")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "nested-children")

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
	}}, []display.StepOp{deploy.OpSame}, false, "rename-bottom-children")

	// Start again with two resources with no relationship.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:    "pkgA:index:t2",
		name: "two",
	}}, []display.StepOp{deploy.OpCreate}, false, "no-relationship")

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
	}}, []display.StepOp{deploy.OpSame}, false, "make-children")

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
	}}, []display.StepOp{deploy.OpSame}, false, "remove-parent-relationship")
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
			}}, []display.StepOp{deploy.OpCreate}, false, "one-two")

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
			}}, []display.StepOp{deploy.OpSame}, false, "make-two-child-of-one")

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
			}}, []display.StepOp{deploy.OpSame}, false, "remove-relationship-of-two-to-one")
		})
	}
}

func TestAliasURNs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				// The `forcesReplacement` key forces replacement and all other keys can update in place
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					replaceKeys := []resource.PropertyKey{}
					old, hasOld := req.OldOutputs["forcesReplacement"]
					new, hasNew := req.NewInputs["forcesReplacement"]
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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-t1-n1")

	// Ensure that rename produces Same
	snap = updateProgramWithResource(snap, []Resource{{
		t:         "pkgA:index:t1",
		name:      "n2",
		aliasURNs: []resource.URN{"urn:pulumi:test::test::pkgA:index:t1::n1"},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-t1-n2")

	// Ensure that rename produces Same with multiple aliases
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
		},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-t1-n3")

	// Ensure that rename produces Same with multiple aliases (reversed)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n3",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-t1-n3-rename")

	// Ensure that aliasing back to original name is okay
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n3",
			"urn:pulumi:test::test::pkgA:index:t1::n2",
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-alias-original")

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t1",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false, "urn-remove-alias")

	// Ensure that changing the type works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:index:t2",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-t2-n1")

	// Ensure that changing the type again works
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-change-type")

	// Ensure that order of aliases doesn't matter
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
		aliasURNs: []resource.URN{
			"urn:pulumi:test::test::pkgA:index:t1::n1",
			"urn:pulumi:test::test::pkgA:othermod:t3::n1",
			"urn:pulumi:test::test::pkgA:index:t2::n1",
		},
	}}, []display.StepOp{deploy.OpSame}, false, "urn-alias-order")

	// Ensure that removing aliases is okay (once old names are gone from all snapshots)
	snap = updateProgramWithResource(snap, []Resource{{
		t:    "pkgA:othermod:t3",
		name: "n1",
	}}, []display.StepOp{deploy.OpSame}, false, "urn-remove-alias-2")

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
	}}, []display.StepOp{deploy.OpUpdate}, false, "urn-t4-n2")

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
	}}, []display.StepOp{deploy.OpUpdate}, false, "urn-change-everything-again")

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
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false, "urn-t6-n4")

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
	}}, []display.StepOp{deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced}, false, "urn-force-new")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-depends-relationship")

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
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced},
		false, "urn-depends-relationship-2")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-parent-relationship")

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
	}}, []display.StepOp{deploy.OpSame, deploy.OpReplace, deploy.OpCreateReplacement, deploy.OpDeleteReplaced},
		false, "urn-parent-relationship-2")

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
	}}, []display.StepOp{deploy.OpCreate}, true, "urn-fail-duplicate")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-different-aliases")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-parent-alias-resolution")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-n1-change-name-and-type")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-multiplication")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-multiple-aliases")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-parent-child")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-rename-child")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-change-child-type")

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
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-nested-children")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-rename-bottom-child")

	// Start again with two resources with no relationship.
	snap = updateProgramWithResource(nil, []Resource{{
		t:    "pkgA:index:t1",
		name: "one",
	}, {
		t:    "pkgA:index:t2",
		name: "two",
	}}, []display.StepOp{deploy.OpCreate}, false, "urn-two-resources-no-relationship")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-make-two-child-of-one")

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
	}}, []display.StepOp{deploy.OpSame}, false, "urn-remove-parent-relationship")
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	mode := 0
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, just make resA
			_, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// First test case, try and create a new B that aliases to A. First make the A like normal...
			_, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			// ... then make B with an alias, it should error
			_, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						makeSpecAlias("resA", "", "", ""),
					},
				})
			assert.Error(t, err)

		case 2:
			// Second test case, try and create a new B that aliases to A. First make the B with an alias...
			_, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						makeSpecAlias("resA", "", "", ""),
					},
				})
			assert.NoError(t, err)

			// ... then try to make the A like normal. It should error that it's already been aliased away
			_, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.Error(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the starting A resource
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Set mode to try and create A then a B that aliases to it, this should fail
	mode = 1
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Set mode to try and create B first then a A, this should fail
	mode = 2
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	mode := 0
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, make "resA", "resB with resA as its parent", and "resB with no parent".
			respA, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{Parent: respA.URN})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// Next case, make "resA" and "resB with no parent and alias to have resA as its parent".
			respA, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource(
				"pkgA:m:typA",
				"resB",
				true,
				deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						makeSpecAliasWithParent("", "", "", "", string(respA.URN)),
					},
				},
			)
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update for initial state with "resA", "resB with resA as its parent", and "resB with no parent".
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 4)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typA::resB"), snap.Resources[2].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resB"), snap.Resources[3].URN)

	// Run the next case, with "resA" and "resB with no parent and alias to have resA as its parent".
	mode = 1
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("1")
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, nil
				},
				DiffF: func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error) {
					return plugin.DiffResult{}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	insA := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})
	createA := func(monitor *deploytest.ResourceMonitor) {
		_, err := monitor.RegisterResource("prog::myType", "resA", false, deploytest.ResourceOptions{
			Inputs: insA,
		})
		assert.NoError(t, err)
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		createA(monitor)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 1)
	assert.Equal(t, tokens.Type("prog::myType"), snap.Resources[0].Type)
	assert.False(t, snap.Resources[0].Custom)

	// Now update A from a component to custom with an alias
	createA = func(monitor *deploytest.ResourceMonitor) {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs: insA,
			Aliases: []*pulumirpc.Alias{
				makeSpecAlias("", "prog::myType", "", ""),
			},
		})
		assert.NoError(t, err)
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	// Assert that A is now a custom
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	// Now two because we'll have a provider now
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, tokens.Type("pkgA:m:typA"), snap.Resources[1].Type)
	assert.True(t, snap.Resources[1].Custom)

	// Now update A back to a component (with an alias)
	createA = func(monitor *deploytest.ResourceMonitor) {
		_, err := monitor.RegisterResource("prog::myType", "resA", false, deploytest.ResourceOptions{
			Inputs: insA,
			Aliases: []*pulumirpc.Alias{
				makeSpecAlias("", "pkgA:m:typA", "", ""),
			},
		})
		assert.NoError(t, err)
	}
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("1")
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	firstRun := true
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource(
			"prog:index:myStandardType", "resA", false, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		if firstRun {
			respB, err := monitor.RegisterResource("prog:index:myType", "resB", false, deploytest.ResourceOptions{})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Parent: respB.URN,
			})
			assert.NoError(t, err)
		} else {
			respB, err := monitor.RegisterResource("prog:index:myType", "resB", false, deploytest.ResourceOptions{
				Parent: respA.URN,
				Aliases: []*pulumirpc.Alias{
					makeSpecAliasWithNoParent("", "", "", "", true),
				},
			})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
				Parent: respA.URN,
				Aliases: []*pulumirpc.Alias{
					makeSpecAliasWithParent("", "", "", "", string(respB.URN)),
				},
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)

	// Now run again with the rearranged parents, we don't expect to see any replaces
	firstRun = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(project workspace.Project, target deploy.Target,
			entries JournalEntries, events []Event, err error,
		) error {
			for _, entry := range entries {
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}
			return err
		}, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 4)
}

func TestEmptyParentAlias(t *testing.T) {
	// Test for backwards compatibility with Python, an alias that sets parent explicitly to "" should be treated the
	// same as not setting parent at all.

	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					id := resource.ID("")
					if !req.Preview {
						id = resource.ID("1")
					}
					return plugin.CreateResponse{
						ID:         id,
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	firstRun := true
	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		respA, err := monitor.RegisterResource(
			"prog:index:myStandardType", "resA", false, deploytest.ResourceOptions{})
		assert.NoError(t, err)

		if firstRun {
			_, err := monitor.RegisterResource("prog:index:myType", "resB", false, deploytest.ResourceOptions{
				Parent: respA.URN,
			})
			assert.NoError(t, err)
		} else {
			_, err := monitor.RegisterResource("prog:index:myType", "resC", false, deploytest.ResourceOptions{
				Parent: respA.URN,
				Aliases: []*pulumirpc.Alias{
					makeSpecAliasWithParent("resB", "", "", "", ""),
				},
			})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update to create the resources
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)

	// Now run again with the rearranged parents, we don't expect to see any replaces
	firstRun = false
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient,
		func(project workspace.Project, target deploy.Target,
			entries JournalEntries, events []Event, err error,
		) error {
			for _, entry := range entries {
				assert.Equal(t, deploy.OpSame, entry.Step.Op())
			}
			return err
		}, "1")
	require.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
}

func TestSplitUpdateComponentAliases(t *testing.T) {
	t.Parallel()

	// This is a test for https://github.com/pulumi/pulumi/issues/13903 to check that if a component is
	// aliased the internal resources follow it across a split deployment.

	mode := 0
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// We should only create things in the first pass
					assert.Equal(t, 0, mode, "%s tried to create but should be aliased", req.URN)

					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(info plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		switch mode {
		case 0:
			// Default case, make "resA", and "resB" a component with "resA" as its parent and "resC" in the component
			respA, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

			respB, err := monitor.RegisterResource(
				"pkgA:m:typB",
				"resB",
				false,
				deploytest.ResourceOptions{
					Parent: respA.URN,
				})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource(
				"pkgA:m:typC",
				"resC",
				true,
				deploytest.ResourceOptions{
					Parent: respB.URN,
				})
			assert.NoError(t, err)

		case 1:
			// Delete "resA" and re-parent "resB" to the root but then fail before getting to resC.
			_, err := monitor.RegisterResource(
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
			respB, err := monitor.RegisterResource(
				"pkgA:m:typB",
				"resB",
				false,
				deploytest.ResourceOptions{
					AliasURNs: []resource.URN{
						"urn:pulumi:test::test::pkgA:m:typA$pkgA:m:typB::resB",
					},
				})
			assert.NoError(t, err)

			_, err = monitor.RegisterResource(
				"pkgA:m:typC",
				"resC",
				true,
				deploytest.ResourceOptions{
					Parent: respB.URN,
				})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update for initial state with "resA", "resB", and "resC".
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
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
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 5)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resB"), snap.Resources[0].URN)
	assert.Equal(t, resource.URN(""), snap.Resources[0].Parent)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[2].URN)
	// Even though we didn't register C its URN must update to take the re-parenting into account.
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB$pkgA:m:typC::resC"), snap.Resources[4].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typB::resB"), snap.Resources[4].Parent)

	mode = 2
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
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
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					// We should only create things in the first and last pass
					ok := mode == 0 || mode == 2
					assert.True(t, ok, "%s tried to create but should be aliased", req.URN)

					return plugin.CreateResponse{
						ID:         "created-id",
						Properties: req.Properties,
						Status:     resource.StatusOK,
					}, nil
				},
				DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					// We should only delete things in the last pass
					ok := mode == 2
					assert.True(t, ok, "%s tried to delete but should be aliased", req.URN)

					return plugin.DeleteResponse{Status: resource.StatusUnknown}, errors.New("can't delete")
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
			_, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resA",
				true,
				deploytest.ResourceOptions{})
			assert.NoError(t, err)

		case 1:
			// Rename "resA" to "resAX" with an alias
			_, err := monitor.RegisterResource(
				"pkgA:m:typA",
				"resAX",
				true,
				deploytest.ResourceOptions{
					Aliases: []*pulumirpc.Alias{
						makeSpecAlias("resA", "", "", ""),
					},
				})
			assert.NoError(t, err)
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{T: t, HostF: hostF},
	}

	project := p.GetProject()

	// Run an update for initial state with "resA"
	snap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)

	// Run the next case, resA should be aliased
	mode = 1
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resAX"), snap.Resources[1].URN)

	// Run the last case, resAX should try to delete and resA should be created. We can't possibly know that resA ==
	// resAX at this point because we're not being sent aliases.
	mode = 2
	snap, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "3")
	assert.Error(t, err)
	assert.NotNil(t, snap)
	assert.Nil(t, snap.VerifyIntegrity())
	assert.Len(t, snap.Resources, 3)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resA"), snap.Resources[1].URN)
	assert.Equal(t, resource.URN("urn:pulumi:test::test::pkgA:m:typA::resAX"), snap.Resources[2].URN)
}

// Tests that aliases in provider dependencies are correctly normalized when snapshots are written. That is, if a
// resource's provider reference points to a URN that is now an alias for a newly renamed resource, the provider
// reference should be updated to the new URN before the snapshot is persisted.
func TestAliasesInProvidersAreNormalized(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Set up the initial program:
	//
	// * res0 and res1, which will serve as parents -- res1 initially, then res0 in the second update;
	// * prov, which will be the child that moves from res1 to res0 and that is aliased in the second update;
	// * res3, which will have prov as its provider.
	//
	// Note: for the URN of prov to change, res0 and res1 must have different types, since only parent types are included
	// in the URNs of their children. The choice of type0/type1 etc. is thus important in this test.
	setupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{
			Parent: res1.URN,
		})
		require.NoError(t, err)
		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
			Parent:   res1.URN,
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, loaders...)
	setupOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF,
	}
	setupSnap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Set up the test program:
	//
	// * res0 and res1, which will serve as parents as before;
	// * prov, which will now be moved to a child of res0, with an alias to its URN when it was a child of res0;
	// * res3, which will have prov as its provider.
	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		res0, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		prov, err := monitor.RegisterResource("pulumi:providers:pkgA", "prov", true, deploytest.ResourceOptions{
			Parent: res0.URN,
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		provRef, err := providers.NewReference(prov.URN, prov.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Provider: provRef.String(),
			Parent:   res0.URN,
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, loaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

// Tests that aliases in dependencies are correctly normalized when snapshots are written. That is, if a resource's
// dependency list contains to a URN that is now an alias for a newly renamed resource, the reference should be updated
// to the new URN before the snapshot is persisted.
func TestAliasesInDependenciesAreNormalized(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Set up the initial program:
	//
	// * res0 and res1, which will serve as parents -- res1 initially, then res0 in the second update;
	// * res2, which will be the child that moves from res1 to res0 and that is aliased in the second update;
	// * res3, which will depend on res2.
	//
	// Note: for the URN of res2 to change, res0 and res1 must have different types, since only parent types are included
	// in the URNs of their children. The choice of type0/type1 etc. is thus important in this test.
	setupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkgA:modA:type2", "res2", true, deploytest.ResourceOptions{
			Parent: res1.URN,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Parent:       res1.URN,
			Dependencies: []resource.URN{res2.URN},
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, loaders...)
	setupOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF,
	}
	setupSnap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Set up the test program:
	//
	// * res0 and res1, which will serve as parents as before;
	// * res2, which will now be moved to a child of res0, with an alias to its URN when it was a child of res1;
	// * res3, which will depend on res2.
	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		res0, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkgA:modA:type2", "res2", true, deploytest.ResourceOptions{
			Parent: res0.URN,
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Parent:       res0.URN,
			Dependencies: []resource.URN{res2.URN},
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, loaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

// Tests that aliases in property dependencies are correctly normalized when snapshots are written. That is, if a
// resource's property dependency map contains to a URN that is now an alias for a newly renamed resource, the reference
// should be updated to the new URN before the snapshot is persisted.
func TestAliasesInPropertyDependenciesAreNormalized(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Set up the initial program:
	//
	// * res0 and res1, which will serve as parents -- res1 initially, then res0 in the second update;
	// * res2, which will be the child that moves from res1 to res0 and that is aliased in the second update;
	// * res3, which will depend on res2 via a property dependency.
	//
	// Note: for the URN of res2 to change, res0 and res1 must have different types, since only parent types are included
	// in the URNs of their children. The choice of type0/type1 etc. is thus important in this test.
	setupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkgA:modA:type2", "res2", true, deploytest.ResourceOptions{
			Parent: res1.URN,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Parent:       res1.URN,
			PropertyDeps: map[resource.PropertyKey][]resource.URN{"propA": {res2.URN}},
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, loaders...)
	setupOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF,
	}
	setupSnap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Set up the test program:
	//
	// * res0 and res1, which will serve as parents as before;
	// * res2, which will now be moved to a child of res0, with an alias to its URN when it was a child of res1;
	// * res3, which will depend on res2 via a property dependency.
	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		res0, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkgA:modA:type2", "res2", true, deploytest.ResourceOptions{
			Parent: res0.URN,
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Parent:       res0.URN,
			PropertyDeps: map[resource.PropertyKey][]resource.URN{"propA": {res2.URN}},
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, loaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}

// Tests that aliases in deleted-with references are correctly normalized when snapshots are written. That is, if a
// resource's deleted-with reference points to to a URN that is now an alias for a newly renamed resource, the reference
// should be updated to the new URN before the snapshot is persisted.
func TestAliasesInDeletedWithAreNormalized(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	// Set up the initial program:
	//
	// * res0 and res1, which will serve as parents -- res1 initially, then res0 in the second update;
	// * res2, which will be the child that moves from res1 to res0 and that is aliased in the second update;
	// * res3, which will depend on res2 via a deleted-with relationship.
	//
	// Note: for the URN of res2 to change, res0 and res1 must have different types, since only parent types are included
	// in the URNs of their children. The choice of type0/type1 etc. is thus important in this test.
	setupProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkgA:modA:type2", "res2", true, deploytest.ResourceOptions{
			Parent: res1.URN,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Parent:      res1.URN,
			DeletedWith: res2.URN,
		})
		require.NoError(t, err)

		return nil
	})

	setupHostF := deploytest.NewPluginHostF(nil, nil, setupProgramF, loaders...)
	setupOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: setupHostF,
	}
	setupSnap, err := lt.TestOp(Update).RunStep(project, p.GetTarget(t, nil), setupOpts, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	// Set up the test program:
	//
	// * res0 and res1, which will serve as parents as before;
	// * res2, which will now be moved to a child of res0, with an alias to its URN when it was a child of res1;
	// * res3, which will depend on res2 via a deleted-with relationship.
	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		res0, err := monitor.RegisterResource("pkgA:modA:type0", "res0", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res1, err := monitor.RegisterResource("pkgA:modA:type1", "res1", false, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkgA:modA:type2", "res2", true, deploytest.ResourceOptions{
			Parent: res0.URN,
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkgA:modA:type3", "res3", true, deploytest.ResourceOptions{
			Parent:      res0.URN,
			DeletedWith: res2.URN,
			Aliases: []*pulumirpc.Alias{
				{
					Alias: &pulumirpc.Alias_Spec_{
						Spec: &pulumirpc.Alias_Spec{
							Parent: &pulumirpc.Alias_Spec_ParentUrn{
								ParentUrn: string(res1.URN),
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, loaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
	}

	_, err = lt.TestOp(Update).RunStep(project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
