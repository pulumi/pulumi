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

package deploy

import (
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
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

			diff, err := diffResource(
				urn, id, c.oldInputs, oldOutputs, c.newInputs,
				&provider, allowUnknowns, c.ignoreChanges, nil)
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
				opts: &Options{},
				target: &Target{
					Name: stack,
				},
				source: NewNullSource(project),
			}, false, updateMode, nil)

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

	t.Run("isTargetedForUpdate (no target dependents)", func(t *testing.T) {
		t.Parallel()

		apUrn := resource.NewURN("test", "test", "", providers.MakeProviderType("pkgA"), "a")
		apRef, err := providers.NewReference(apUrn, "0")
		assert.NoError(t, err)

		bpUrn := resource.NewURN("test", "test", "", providers.MakeProviderType("pkgB"), "b")
		bpRef, err := providers.NewReference(bpUrn, "1")
		assert.NoError(t, err)

		// Arrange.
		sg := &stepGenerator{
			deployment: &Deployment{
				opts: &Options{
					TargetDependents: false,
					Targets: UrnTargets{
						literals: []resource.URN{"b"},
					},
				},
			},
			targetsActual: UrnTargets{
				literals: []resource.URN{bpRef.URN()},
			},
		}

		t.Run("is targeted directly", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			a := &resource.State{URN: "a"}
			b := &resource.State{URN: "b"}

			// Act.
			aIsTargeted := sg.isTargetedForUpdate(a)
			bIsTargeted := sg.isTargetedForUpdate(b)

			// Assert.
			assert.False(t, aIsTargeted)
			assert.True(t, bIsTargeted)
		})

		t.Run("has a targeted provider", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			hasAAsProvider := &resource.State{Provider: apRef.String()}
			hasBAsProvider := &resource.State{Provider: bpRef.String()}

			// Act.
			hasAAsProviderIsTargeted := sg.isTargetedForUpdate(hasAAsProvider)
			hasBAsProviderIsTargeted := sg.isTargetedForUpdate(hasBAsProvider)

			// Assert.
			assert.False(t, hasAAsProviderIsTargeted)
			assert.False(t, hasBAsProviderIsTargeted)
		})

		t.Run("has a targeted parent", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			hasAAsParent := &resource.State{Parent: "a"}
			hasBAsParent := &resource.State{Parent: "b"}

			// Act.
			hasAAsParentIsTargeted := sg.isTargetedForUpdate(hasAAsParent)
			hasBAsParentIsTargeted := sg.isTargetedForUpdate(hasBAsParent)

			// Assert.
			assert.False(t, hasAAsParentIsTargeted)
			assert.False(t, hasBAsParentIsTargeted)
		})

		t.Run("has a targeted dependency", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			dependsOnA := &resource.State{Dependencies: []resource.URN{"a"}}
			dependsOnB := &resource.State{Dependencies: []resource.URN{"a", "b"}}

			// Act.
			dependsOnAIsTargeted := sg.isTargetedForUpdate(dependsOnA)
			dependsOnBIsTargeted := sg.isTargetedForUpdate(dependsOnB)

			// Assert.
			assert.False(t, dependsOnAIsTargeted)
			assert.False(t, dependsOnBIsTargeted)
		})

		t.Run("has a targeted property dependency", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			dependsOnA := &resource.State{PropertyDependencies: map[resource.PropertyKey][]resource.URN{"p": {"a"}}}
			dependsOnB := &resource.State{PropertyDependencies: map[resource.PropertyKey][]resource.URN{"p": {"a", "b"}}}

			// Act.
			dependsOnAIsTargeted := sg.isTargetedForUpdate(dependsOnA)
			dependsOnBIsTargeted := sg.isTargetedForUpdate(dependsOnB)

			// Assert.
			assert.False(t, dependsOnAIsTargeted)
			assert.False(t, dependsOnBIsTargeted)
		})

		t.Run("is deleted with a target", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			isDeletedWithA := &resource.State{DeletedWith: "a"}
			isDeletedWithB := &resource.State{DeletedWith: "b"}

			// Act.
			isDeletedWithAIsTargeted := sg.isTargetedForUpdate(isDeletedWithA)
			isDeletedWithBIsTargeted := sg.isTargetedForUpdate(isDeletedWithB)

			// Assert.
			assert.False(t, isDeletedWithAIsTargeted)
			assert.False(t, isDeletedWithBIsTargeted)
		})
	})

	t.Run("isTargetedForUpdate (target dependents)", func(t *testing.T) {
		t.Parallel()

		// Arrange.
		apUrn := resource.NewURN("test", "test", "", providers.MakeProviderType("pkgA"), "a")
		apRef, err := providers.NewReference(apUrn, "0")
		assert.NoError(t, err)

		bpUrn := resource.NewURN("test", "test", "", providers.MakeProviderType("pkgB"), "b")
		bpRef, err := providers.NewReference(bpUrn, "1")
		assert.NoError(t, err)

		sg := &stepGenerator{
			deployment: &Deployment{
				opts: &Options{
					TargetDependents: true,
					Targets: UrnTargets{
						literals: []resource.URN{"c"},
					},
				},
			},
			targetsActual: UrnTargets{
				literals: []resource.URN{"b", bpRef.URN()},
			},
		}

		t.Run("is targeted directly", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			a := &resource.State{URN: "a"}
			c := &resource.State{URN: "c"}

			// Act.
			aIsTargeted := sg.isTargetedForUpdate(a)
			cIsTargeted := sg.isTargetedForUpdate(c)

			// Assert.
			assert.False(t, aIsTargeted)
			assert.True(t, cIsTargeted)
		})

		t.Run("has a targeted provider", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			hasAAsProvider := &resource.State{Provider: apRef.String()}
			hasBAsProvider := &resource.State{Provider: bpRef.String()}

			// Act.
			hasAAsProviderIsTargeted := sg.isTargetedForUpdate(hasAAsProvider)
			hasBAsProviderIsTargeted := sg.isTargetedForUpdate(hasBAsProvider)

			// Assert.
			assert.False(t, hasAAsProviderIsTargeted)
			assert.True(t, hasBAsProviderIsTargeted)
		})

		t.Run("has a targeted parent", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			hasAAsParent := &resource.State{Parent: "a"}
			hasBAsParent := &resource.State{Parent: "b"}

			// Act.
			hasAAsParentIsTargeted := sg.isTargetedForUpdate(hasAAsParent)
			hasBAsParentIsTargeted := sg.isTargetedForUpdate(hasBAsParent)

			// Assert.
			assert.False(t, hasAAsParentIsTargeted)
			assert.True(t, hasBAsParentIsTargeted)
		})

		t.Run("has a targeted dependency", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			dependsOnA := &resource.State{Dependencies: []resource.URN{"a"}}
			dependsOnB := &resource.State{Dependencies: []resource.URN{"a", "b"}}

			// Act.
			dependsOnAIsTargeted := sg.isTargetedForUpdate(dependsOnA)
			dependsOnBIsTargeted := sg.isTargetedForUpdate(dependsOnB)

			// Assert.
			assert.False(t, dependsOnAIsTargeted)
			assert.True(t, dependsOnBIsTargeted)
		})

		t.Run("has a targeted property dependency", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			dependsOnA := &resource.State{PropertyDependencies: map[resource.PropertyKey][]resource.URN{"p": {"a"}}}
			dependsOnB := &resource.State{PropertyDependencies: map[resource.PropertyKey][]resource.URN{"p": {"a", "b"}}}

			// Act.
			dependsOnAIsTargeted := sg.isTargetedForUpdate(dependsOnA)
			dependsOnBIsTargeted := sg.isTargetedForUpdate(dependsOnB)

			// Assert.
			assert.False(t, dependsOnAIsTargeted)
			assert.True(t, dependsOnBIsTargeted)
		})

		t.Run("is deleted with a target", func(t *testing.T) {
			t.Parallel()

			// Arrange.
			isDeletedWithA := &resource.State{DeletedWith: "a"}
			isDeletedWithB := &resource.State{DeletedWith: "b"}

			// Act.
			isDeletedWithAIsTargeted := sg.isTargetedForUpdate(isDeletedWithA)
			isDeletedWithBIsTargeted := sg.isTargetedForUpdate(isDeletedWithB)

			// Assert.
			assert.False(t, isDeletedWithAIsTargeted)
			assert.True(t, isDeletedWithBIsTargeted)
		})
	})

	t.Run("checkParent", func(t *testing.T) {
		t.Parallel()
		t.Run("could not find parent resource", func(t *testing.T) {
			t.Parallel()
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
			}
			_, err := sg.checkParent("does-not-exist", "")
			assert.ErrorContains(t, err, "could not find parent resource")
		})
	})

	t.Run("GenerateReadSteps", func(t *testing.T) {
		t.Parallel()
		t.Run("could not find parent resource", func(t *testing.T) {
			t.Parallel()
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
			}
			_, err := sg.GenerateReadSteps(&readResourceEvent{
				parent: "does-not-exist",
			})
			assert.ErrorContains(t, err, "could not find parent resource")
		})

		t.Run("fail generateURN", func(t *testing.T) {
			t.Parallel()
			sg := &stepGenerator{
				urns: map[resource.URN]bool{
					"urn:pulumi:stack::::::": true,
				},
				deployment: &Deployment{
					ctx: &plugin.Context{
						Diag: &deploytest.NoopSink{},
					},
					opts: &Options{},
					target: &Target{
						Name: tokens.MustParseStackName("stack"),
					},
					source: &nullSource{},
				},
			}
			_, err := sg.GenerateReadSteps(&readResourceEvent{})
			assert.ErrorContains(t, err, "Duplicate resource URN")
		})
	})

	t.Run("generateSteps", func(t *testing.T) {
		t.Parallel()
		t.Run("could not find parent resource", func(t *testing.T) {
			t.Parallel()
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
				deployment: &Deployment{
					target: &Target{
						Name: tokens.MustParseStackName("stack"),
					},
					source: &nullSource{},
				},
			}
			_, _, err := sg.generateSteps(&registerResourceEvent{
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
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
				deployment: &Deployment{
					prev: &Snapshot{
						Resources: []*resource.State{
							{
								URN: "a",
							},
						},
					},
					olds: map[resource.URN]*resource.State{},
				},
			}
			targets, err := sg.determineAllowedResourcesToDeleteFromTargets(
				UrnTargets{literals: []resource.URN{"a"}},
			)
			assert.NoError(t, err)
			assert.Empty(t, targets)
		})
	})

	t.Run("providerChanged", func(t *testing.T) {
		t.Parallel()
		t.Run("invalid old ProviderReference", func(t *testing.T) {
			t.Parallel()
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
				deployment: &Deployment{
					prev: &Snapshot{},
					olds: map[resource.URN]*resource.State{},
				},
			}
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
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
				deployment: &Deployment{
					prev: &Snapshot{},
					olds: map[resource.URN]*resource.State{},
				},
			}
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
			sg := &stepGenerator{
				urns: map[resource.URN]bool{},
				deployment: &Deployment{
					prev:      &Snapshot{},
					olds:      map[resource.URN]*resource.State{},
					providers: &providers.Registry{},
				},
			}
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
	})
}
