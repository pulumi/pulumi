// Copyright 2016-2023, Pulumi Corporation.
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

package deploy

import (
	"os"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/graph"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestIgnoreChanges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		oldInputs     map[string]interface{}
		newInputs     map[string]interface{}
		expected      map[string]interface{}
		ignoreChanges []string
		expectFailure bool
	}{
		{
			name: "Present in old and new sets",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "bar",
				},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name: "Missing in new sets",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name: "Present in old and new sets, using [\"\"]",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "foo",
					},
				},
			},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "bar",
					},
				},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "foo",
					},
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b[\"c\"]"},
		},
		{
			name: "Missing in new sets, using [\"\"]",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "foo",
					},
				},
			},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{},
				},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "foo",
					},
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b[\"c\"]"},
		},
		{
			name:      "Missing in old deletes",
			oldInputs: map[string]interface{}{},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name:      "Missing keys in old and new are OK",
			oldInputs: map[string]interface{}{},
			newInputs: map[string]interface{}{},
			ignoreChanges: []string{
				"a",
				"a.b",
				"a.c[0]",
			},
		},
		{
			name: "Missing parent keys in only new fail",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			newInputs:     map[string]interface{}{},
			ignoreChanges: []string{"a.b"},
			expectFailure: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			olds, news := resource.NewPropertyMapFromMap(c.oldInputs), resource.NewPropertyMapFromMap(c.newInputs)

			expected := olds
			if c.expected != nil {
				expected = resource.NewPropertyMapFromMap(c.expected)
			}

			processed, err := processIgnoreChanges(news, olds, c.ignoreChanges)
			if c.expectFailure {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expected, processed)
			}
		})
	}
}

func TestApplyReplaceOnChangesEmptyDetailedDiff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		diff             plugin.DiffResult
		replaceOnChanges []string
		hasInitErrors    bool
		expected         plugin.DiffResult
	}{
		{
			name:             "Empty diff and replaceOnChanges",
			diff:             plugin.DiffResult{},
			replaceOnChanges: []string{},
			hasInitErrors:    false,
			expected:         plugin.DiffResult{},
		},
		{
			name: "DiffSome and empty replaceOnChanges",
			diff: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
			},
			replaceOnChanges: []string{},
			hasInitErrors:    false,
			expected: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
			},
		},
		{
			name: "DiffSome and non-empty replaceOnChanges",
			diff: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
			},
			replaceOnChanges: []string{"a"},
			hasInitErrors:    false,
			expected: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
				ReplaceKeys: []resource.PropertyKey{"a"},
			},
		},
		{
			name:             "Empty diff and replaceOnChanges w/ init errors",
			diff:             plugin.DiffResult{},
			replaceOnChanges: []string{},
			hasInitErrors:    true,
			expected:         plugin.DiffResult{},
		},
		{
			name: "DiffSome and empty replaceOnChanges w/ init errors",
			diff: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
			},
			replaceOnChanges: []string{},
			hasInitErrors:    true,
			expected: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
			},
		},
		{
			name: "DiffSome and non-empty replaceOnChanges w/ init errors",
			diff: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
			},
			replaceOnChanges: []string{"a"},
			hasInitErrors:    true,
			expected: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ChangedKeys: []resource.PropertyKey{"a"},
				ReplaceKeys: []resource.PropertyKey{"a"},
			},
		},
		{
			name:             "Empty diff and non-empty replaceOnChanges w/ init errors",
			diff:             plugin.DiffResult{},
			replaceOnChanges: []string{"*"},
			hasInitErrors:    true,
			expected: plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ReplaceKeys: []resource.PropertyKey{"#initerror"},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			newdiff, err := applyReplaceOnChanges(c.diff, c.replaceOnChanges, c.hasInitErrors)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, newdiff)
		})
	}
}

func TestEngineDiff(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                 string
		oldInputs, newInputs resource.PropertyMap
		ignoreChanges        []string
		expected             []resource.PropertyKey
		expectedChanges      plugin.DiffChanges
	}{
		{
			name: "Empty diff",
			oldInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewPropertyValue(8),
				"val2": resource.NewPropertyValue("hello"),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewPropertyValue(8),
				"val2": resource.NewPropertyValue("hello"),
			}),
			expected:        nil,
			expectedChanges: plugin.DiffNone,
		},
		{
			name: "All changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val0": resource.NewPropertyValue(3.14),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewNumberProperty(42),
				"val2": resource.NewPropertyValue("world"),
			}),
			expected:        []resource.PropertyKey{"val0", "val1", "val2"},
			expectedChanges: plugin.DiffSome,
		},
		{
			name: "Some changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewPropertyValue(42),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewNumberProperty(42),
				"val2": resource.NewPropertyValue("world"),
			}),
			expected:        []resource.PropertyKey{"val2"},
			expectedChanges: plugin.DiffSome,
		},
		{
			name: "Ignore some changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewPropertyValue("hello"),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val2": resource.NewPropertyValue(8),
			}),

			ignoreChanges:   []string{"val1"},
			expected:        []resource.PropertyKey{"val2"},
			expectedChanges: plugin.DiffSome,
		},
		{
			name: "Ignore all changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val1": resource.NewPropertyValue("hello"),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]interface{}{
				"val2": resource.NewPropertyValue(8),
			}),

			ignoreChanges:   []string{"val1", "val2"},
			expected:        nil,
			expectedChanges: plugin.DiffNone,
		},
	}
	urn := resource.URN("urn:pulumi:dev::website-and-lambda::aws:s3/bucket:Bucket::my-bucket")
	id := resource.ID("someid")
	var oldOutputs resource.PropertyMap
	allowUnknowns := false
	provider := deploytest.Provider{}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			diff, err := diffResource(urn, id, c.oldInputs, oldOutputs, c.newInputs, &provider, allowUnknowns, c.ignoreChanges)
			t.Logf("diff.ChangedKeys = %v", diff.ChangedKeys)
			t.Logf("diff.StableKeys = %v", diff.StableKeys)
			t.Logf("diff.ReplaceKeys = %v", diff.ReplaceKeys)
			assert.NoError(t, err)
			assert.Equal(t, c.expectedChanges, diff.Changes)
			assert.EqualValues(t, c.expected, diff.ChangedKeys)
		})
	}
}

func TestGenerateAliases(t *testing.T) {
	t.Parallel()

	const (
		project = "project"
	)
	stack := tokens.MustParseStackName("stack")

	parentTypeAlias := resource.CreateURN("myres", "test:resource:type2", "", project, stack.String())
	parentNameAlias := resource.CreateURN("myres2", "test:resource:type", "", project, stack.String())

	cases := []struct {
		name         string
		parentAlias  *resource.URN
		childAliases []resource.Alias
		expected     []resource.URN
	}{
		{
			name:     "no aliases",
			expected: nil,
		},
		{
			name: "child alias (type), no parent aliases",
			childAliases: []resource.Alias{
				{Type: "test:resource:child2"},
			},
			expected: []resource.URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres-child",
			},
		},
		{
			name: "child alias (name), no parent aliases",
			childAliases: []resource.Alias{
				{Name: "child2"},
			},
			expected: []resource.URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::child2",
			},
		},
		{
			name: "child alias (type, noParent), no parent aliases",
			childAliases: []resource.Alias{
				{
					Type:     "test:resource:child2",
					NoParent: true,
				},
			},
			expected: []resource.URN{
				"urn:pulumi:stack::project::test:resource:child2::myres-child",
			},
		},
		{
			name: "child alias (type, parent), no parent aliases",
			childAliases: []resource.Alias{
				{
					Type:   "test:resource:child2",
					Parent: resource.CreateURN("originalparent", "test:resource:original", "", project, stack.String()),
				},
			},
			expected: []resource.URN{
				"urn:pulumi:stack::project::test:resource:original$test:resource:child2::myres-child",
			},
		},
		{
			name:        "child alias (name), parent alias (type)",
			parentAlias: &parentTypeAlias,
			childAliases: []resource.Alias{
				{Name: "myres-child2"},
			},
			expected: []resource.URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
				"urn:pulumi:stack::project::test:resource:type2$test:resource:child::myres-child",
				"urn:pulumi:stack::project::test:resource:type2$test:resource:child::myres-child2",
			},
		},
		{
			name:        "child alias (name), parent alias (name)",
			parentAlias: &parentNameAlias,
			childAliases: []resource.Alias{
				{Name: "myres-child2"},
			},
			expected: []resource.URN{
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres-child2",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child",
				"urn:pulumi:stack::project::test:resource:type$test:resource:child::myres2-child2",
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parentURN := resource.CreateURN("myres", "test:resource:type", "", project, stack.String())
			goal := &resource.Goal{
				Parent:  parentURN,
				Name:    "myres-child",
				Type:    "test:resource:child",
				Aliases: tt.childAliases,
			}

			sg := newStepGenerator(&Deployment{
				target: &Target{
					Name: stack,
				},
				source: NewNullSource(project),
			}, Options{})

			if tt.parentAlias != nil {
				sg.aliases = map[resource.URN]resource.URN{
					parentURN: *tt.parentAlias,
				}
			}

			actual := sg.generateAliases(goal)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestDeleteProtectedErrorUsesCorrectQuotesOnOS(t *testing.T) {
	t.Parallel()
	err := deleteProtectedError{urn: "resource:urn"}

	expectations := map[string]string{
		`windows`: `"`,
		`linux`:   `'`,
		`darwin`:  `'`,
	}

	t.Run(runtime.GOOS, func(t *testing.T) {
		t.Parallel()
		gotErrMsg := err.Error()
		contains, ok := expectations[runtime.GOOS]
		if !ok {
			t.Skipf("no quoting expectation for %s", runtime.GOOS)
			return
		}
		assert.Contains(t, gotErrMsg, contains)
	})
}

func TestStepGenerator(t *testing.T) {
	t.Parallel()
	t.Run("isTargetedForUpdate", func(t *testing.T) {
		t.Parallel()
		t.Run("has targeted dependencies", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				nil,
				Options{
					TargetDependents: true,
					Targets: UrnTargets{
						literals: []resource.URN{
							"b",
							"c",
						},
					},
				})
			assert.False(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{"a"},
			}))
			assert.True(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{
					"a",
					"b", // targeted
				},
			}))
		})

		t.Run("No target dependents", func(t *testing.T) {
			sg := newStepGenerator(
				nil,
				Options{
					TargetDependents: false,
					Targets: UrnTargets{
						literals: []resource.URN{
							"b",
							"c",
						},
					},
				})
			assert.False(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{"a"},
			}))
		})

		t.Run("provider maybe a target", func(t *testing.T) {
			providerUrn := "urn:pulumi:stack::project::pulumi:providers:provider::foo"
			notAProviderUrn := "urn:pulumi:stack::project::pulumi:providers:provider::bar"
			sg := newStepGenerator(
				nil,
				Options{
					TargetDependents: true,
					Targets: UrnTargets{
						literals: []resource.URN{
							"b",
							"c",
							resource.URN(providerUrn),
						},
					},
				})
			assert.True(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{"a"},
				Provider:     providerUrn + "::uuid",
			}))
			assert.False(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{"a"},
				Provider:     notAProviderUrn + "::uuid",
			}))
		})

		t.Run("provider maybe the parent", func(t *testing.T) {
			sg := newStepGenerator(
				nil,
				Options{
					TargetDependents: true,
					Targets: UrnTargets{
						literals: []resource.URN{
							"b",
							"c",
						},
					},
				})
			assert.True(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{"a"},
				Parent:       "c",
			}))
			assert.False(t, sg.isTargetedForUpdate(&resource.State{
				Dependencies: []resource.URN{"a"},
				Parent:       "d",
			}))
		})
	})

	t.Run("isTargetedReplace", func(t *testing.T) {
		sg := newStepGenerator(
			nil,
			Options{
				TargetDependents: true,
				ReplaceTargets: UrnTargets{
					literals: []resource.URN{
						"b",
						"c",
					},
				},
			})
		t.Run("urn is not in replace targets", func(t *testing.T) {
			t.Parallel()
			assert.False(t, sg.isTargetedReplace("a"))
		})
		t.Run("urn is in replace targets", func(t *testing.T) {
			t.Parallel()
			assert.True(t, sg.isTargetedReplace("b"))
		})
	})

	t.Run("checkParent", func(t *testing.T) {
		t.Parallel()
		t.Run("could not find parent resource", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(nil, Options{})
			_, err := sg.checkParent("does-not-exist", "")
			assert.ErrorContains(t, err, "could not find parent resource")
		})
	})
	t.Run("GenerateReadSteps", func(t *testing.T) {
		t.Parallel()
		t.Run("could not find parent resource", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(nil, Options{})
			_, err := sg.GenerateReadSteps(&readResourceEvent{
				parent: "does-not-exist",
			})
			assert.ErrorContains(t, err, "could not find parent resource")
		})
		t.Run("fail generateURN", func(t *testing.T) {
			t.Parallel()
			os.Setenv("PULUMI_DISABLE_VALIDATION", "true")

			sg := newStepGenerator(
				&Deployment{
					target: &Target{
						Name: tokens.MustParseStackName("stack"),
					},
					source: &nullSource{},
					ctx:    &plugin.Context{Diag: &deploytest.NoopSink{}},
				},
				Options{})
			sg.urns["urn:pulumi:stack::::::"] = true
			_, err := sg.GenerateReadSteps(&readResourceEvent{})
			assert.ErrorContains(t, err, "Duplicate resource URN")
		})
	})
	t.Run("generateSteps", func(t *testing.T) {
		t.Parallel()
		t.Run("could not find parent resource", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					target: &Target{
						Name: tokens.MustParseStackName("stack"),
					},
					source: &nullSource{},
				}, Options{})
			_, err := sg.generateSteps(&registerResourceEvent{
				goal: &resource.Goal{
					Parent: "does-not-exist",
				},
			})
			assert.ErrorContains(t, err, "could not find parent resource")
		})
	})
	t.Run("determineAllowedResourcesToDeleteFromTargets", func(t *testing.T) {
		t.Parallel()
		t.Run("handle non-existent target", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					prev: &Snapshot{
						Resources: []*resource.State{
							{
								URN: "a",
							},
						},
					},
					olds: map[resource.URN]*resource.State{},
				},
				Options{})
			targets, err := sg.determineAllowedResourcesToDeleteFromTargets(UrnTargets{
				literals: []resource.URN{"a"},
			})
			assert.NoError(t, err)
			assert.Empty(t, targets)
		})

		t.Run("handle existing target", func(t *testing.T) {
			sg := newStepGenerator(
				&Deployment{
					prev: &Snapshot{
						Resources: []*resource.State{
							{URN: "a"},
						},
					},
				},
				Options{})
			oldResources, olds, err := buildResourceMap(sg.deployment.prev, true)
			assert.NoError(t, err)
			sg.deployment.olds = olds
			sg.deployment.depGraph = graph.NewDependencyGraph(oldResources)

			sg.generatorMutex.Lock()
			defer sg.generatorMutex.Unlock()

			targets, err := sg.determineAllowedResourcesToDeleteFromTargets(UrnTargets{
				literals: []resource.URN{"a"},
			})
			assert.NoError(t, err)
			assert.Contains(t, targets, resource.URN("a"))
		})

		t.Run("handle existing target with parent", func(t *testing.T) {
			sg := newStepGenerator(
				&Deployment{
					prev: &Snapshot{
						Resources: []*resource.State{
							{URN: "a"},
							{URN: "b", Parent: "a"},
						},
					},
				},
				Options{})
			oldResources, olds, err := buildResourceMap(sg.deployment.prev, true)
			assert.NoError(t, err)
			sg.deployment.olds = olds
			sg.deployment.depGraph = graph.NewDependencyGraph(oldResources)

			sg.generatorMutex.Lock()
			defer sg.generatorMutex.Unlock()

			targets, err := sg.determineAllowedResourcesToDeleteFromTargets(UrnTargets{
				literals: []resource.URN{"a"},
			})
			assert.NoError(t, err)
			assert.Contains(t, targets, resource.URN("a"))
			assert.Contains(t, targets, resource.URN("b"))
		})
	})

	t.Run("ScheduleDeletes", func(t *testing.T) {
		t.Parallel()
		t.Run("don't TrustDependencies", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					prev: &Snapshot{},
					olds: map[resource.URN]*resource.State{},
				},
				Options{
					TrustDependencies: false,
				})
			antichains := sg.ScheduleDeletes([]Step{
				&DeleteStep{},
				&CreateStep{},
				&UpdateStep{},
			})
			assert.Len(t, antichains, 3)
		})
	})
	t.Run("providerChanged", func(t *testing.T) {
		t.Parallel()
		t.Run("invalid old ProviderReference", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					prev: &Snapshot{},
					olds: map[resource.URN]*resource.State{},
				}, Options{})
			_, err := sg.providerChanged("",
				&resource.State{
					Provider: "invalid-old-provider",
				},
				&resource.State{
					Provider: "urn:pulumi:stack::project::pulumi:providers:provider::name::uuid",
				},
			)
			assert.ErrorContains(t, err, "expected '::' in provider reference 'invalid-old-provider'")
		})
		t.Run("invalid new ProviderReference", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					prev: &Snapshot{},
					olds: map[resource.URN]*resource.State{},
				}, Options{})
			_, err := sg.providerChanged("",
				&resource.State{
					Provider: "urn:pulumi:stack::project::pulumi:providers:provider::name::uuid",
				},
				&resource.State{
					Provider: "invalid-new-provider",
				},
			)
			assert.ErrorContains(t, err, "expected '::' in provider reference 'invalid-new-provider'")
		})
		t.Run("error getting new default provider", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					prev:      &Snapshot{},
					olds:      map[resource.URN]*resource.State{},
					providers: &providers.Registry{},
				}, Options{})
			_, err := sg.providerChanged("",
				&resource.State{
					Provider: "urn:pulumi:stack::project::pulumi:providers:provider::default_name::uuid",
				},
				&resource.State{
					Provider: "urn:pulumi:stack::project::pulumi:providers:provider::default_new::uuid",
				},
			)
			assert.ErrorContains(t, err, "failed to resolve provider reference")
		})

		t.Run("Simple urn comparisons", func(t *testing.T) {
			t.Parallel()
			sg := newStepGenerator(
				&Deployment{
					prev:      &Snapshot{},
					olds:      map[resource.URN]*resource.State{},
					providers: &providers.Registry{},
				}, Options{})

			cases := []struct {
				old       string
				new       string
				expecting bool
			}{
				{old: "a", new: "a", expecting: false},
				{old: "a", new: "", expecting: true},
				{old: "", new: "a", expecting: true},
				{old: "", new: "", expecting: false},
			}

			for _, testcase := range cases {
				res, err := sg.providerChanged("",
					&resource.State{Provider: testcase.old},
					&resource.State{Provider: testcase.new})
				assert.NoError(t, err)
				assert.Equalf(t, testcase.expecting, res, "expecting %v for %#v", testcase.expecting, testcase)
			}
		})

		t.Run("alias urns return false", func(t *testing.T) {
			t.Parallel()
			fooProviderUrn := "urn:pulumi:stack::project::pulumi:providers:provider::foo"
			barProviderUrn := "urn:pulumi:stack::project::pulumi:providers:provider::bar"

			sg := newStepGenerator(
				&Deployment{
					prev:      &Snapshot{},
					olds:      map[resource.URN]*resource.State{},
					providers: &providers.Registry{},
				}, Options{})

			sg.aliased[resource.URN(fooProviderUrn)] = resource.URN(barProviderUrn)

			res, err := sg.providerChanged("",
				&resource.State{Provider: fooProviderUrn + "::uuid"},
				&resource.State{Provider: barProviderUrn + "::uuid"})
			assert.NoError(t, err)
			assert.False(t, res)
		})

		t.Run("Either ref not being a default provider returns true", func(t *testing.T) {
			t.Parallel()
			providerPrefix := "urn:pulumi:stack::project::pulumi:providers:provider::"

			sink := diagtest.LogSink(t)
			statusSink := diagtest.LogSink(t)
			program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
				return nil
			}
			lang := deploytest.NewLanguageRuntime(program)
			host := deploytest.NewPluginHost(sink, statusSink, lang)
			ctx, err := plugin.NewContext(sink, statusSink, host, nil, "", nil, false, nil)
			require.NoError(t, err)

			provider := &deploytest.Provider{
				Package: tokens.Package("provider"),
				CheckConfigF: func(urn resource.URN, olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {
					return news, nil, nil
				},
				ConfigureF: func(news resource.PropertyMap) error {
					return nil
				},
			}

			registry := providers.NewRegistry(host, true, provider)

			d := &Deployment{
				prev:      &Snapshot{},
				olds:      map[resource.URN]*resource.State{},
				providers: registry,
				ctx:       ctx,
			}
			providerUrn := providerPrefix + "default_foo"
			inputs := resource.PropertyMap{}
			inputs["version"] = resource.NewPropertyValue("1.0.0")
			inputs, _, err = d.providers.Check(resource.URN(providerUrn), nil, inputs, false, nil)
			assert.NoError(t, err)
			providerId, _, _, err := d.providers.Create(resource.URN(providerUrn), inputs, -1, true)
			assert.NoError(t, err)

			sg := newStepGenerator(d, Options{})
			// fooRef, err := providers.ParseReference(providerUrn)

			testcases := []struct {
				desc        string
				old         string
				new         string
				expected    bool
				errContains string
			}{
				{
					desc:     "Neither is default",
					old:      providerPrefix + "foo::uuid",
					new:      providerPrefix + "bar::uuid",
					expected: true,
				},
				{
					desc:     "Both are default",
					old:      providerPrefix + "default_foo::uuid1",
					new:      providerPrefix + "default_foo::" + providerId.String(),
					expected: false,
					//`errContains: "failed to resolve provider reference",
				},
				{
					desc:     "Only old is default",
					old:      providerPrefix + "default_foo::uuid",
					new:      providerPrefix + "bar::uuid",
					expected: true,
				},
				{
					desc:     "Only new is default",
					old:      providerPrefix + "foo::uuid",
					new:      providerPrefix + "default_bar::uuid",
					expected: true,
				},
			}

			for _, testcase := range testcases {
				oldRef, _ := providers.ParseReference(testcase.old)
				sg.deployment.olds[oldRef.URN()] = &resource.State{
					URN: oldRef.URN(),
				}
				newRef, _ := providers.ParseReference(testcase.new)
				sg.providers[newRef.URN()] = &resource.State{
					URN: newRef.URN(),
				}
				res, err := sg.providerChanged("",
					&resource.State{Provider: testcase.old},
					&resource.State{Provider: testcase.new})
				if testcase.errContains != "" {
					assert.ErrorContainsf(t, err, testcase.errContains, testcase.desc)
				} else {
					assert.NoError(t, err, testcase.desc)
					assert.Equalf(t, testcase.expected, res,
						"%s expected %v for old: %v and new: %v",
						testcase.desc,
						testcase.expected,
						testcase.old,
						testcase.new)
				}
			}
		})
	})
}

// This runs combinations of public calls to stepGenerator to ensure that it never deadlocks or
// panics
func TestStepGenerator_randomActions(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		sink := diagtest.LogSink(t)
		statusSink := diagtest.LogSink(t)
		program := func(_ plugin.RunInfo, resmon *deploytest.ResourceMonitor) error {
			return nil
		}
		lang := deploytest.NewLanguageRuntime(program)
		host := deploytest.NewPluginHost(sink, statusSink, lang)
		ctx, err := plugin.NewContext(sink, statusSink, host, nil, "", nil, false, nil)
		require.NoError(rt, err)
		deployment := &Deployment{
			prev:      &Snapshot{},
			olds:      map[resource.URN]*resource.State{},
			providers: &providers.Registry{},
			target:    &Target{},
			source:    &nullSource{},
			ctx:       ctx,
			goals:     &goalMap{},
			news:      &resourceMap{},
		}
		sg := newStepGenerator(deployment, Options{})
		sg.urns["urn:pulumi::stack::project::qualified$type$name::parent"] = true

		rt.Run(map[string]func(*rapid.T){
			"GetURNs": func(t *rapid.T) {
				sg.GetURNs()
			},
			"GenerateReadSteps": func(t *rapid.T) {
				tok, err := tokens.ParseTypeToken("test:resource:parent")
				require.NoError(t, err)
				//nolint:errcheck
				sg.GenerateReadSteps(&readResourceEvent{
					baseType: tok,
					props:    make(resource.PropertyMap),
				})
			},
			"GenerateSteps": func(t *rapid.T) {
				tok, err := tokens.ParseTypeToken("test:resource:child")
				require.NoError(t, err)
				//nolint:errcheck
				sg.GenerateSteps(&registerResourceEvent{
					goal: &resource.Goal{
						Parent:     "urn:pulumi::stack::project::qualified$type$name::parent",
						Type:       tok,
						Properties: make(resource.PropertyMap),
					},
				})
			},
			"GenerateDeletes": func(*rapid.T) {
				//nolint:errcheck
				sg.GenerateDeletes(NewUrnTargets([]string{}))
			},
			"ScheduleDeletes": func(*rapid.T) {
				//nolint:errcheck
				sg.ScheduleDeletes([]Step{})
			},
			"Errored": func(*rapid.T) {
				sg.Errored()
			},
			"AnalyzeResources": func(*rapid.T) {
				//nolint:errcheck
				sg.AnalyzeResources()
			},
		})
	})
}
