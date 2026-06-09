// Copyright 2016, Pulumi Corporation.
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
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestIgnoreChanges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		oldInputs     map[string]any
		newInputs     map[string]any
		expected      map[string]any
		ignoreChanges []string
		expectMessage bool
	}{
		{
			name: "Present in old and new sets",
			oldInputs: map[string]any{
				"a": map[string]any{
					"b": "foo",
				},
			},
			newInputs: map[string]any{
				"a": map[string]any{
					"b": "bar",
				},
				"c": 42,
			},
			expected: map[string]any{
				"a": map[string]any{
					"b": "foo",
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name: "Missing in new sets",
			oldInputs: map[string]any{
				"a": map[string]any{
					"b": "foo",
				},
			},
			newInputs: map[string]any{
				"a": map[string]any{},
				"c": 42,
			},
			expected: map[string]any{
				"a": map[string]any{
					"b": "foo",
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name: "Present in old and new sets, using [\"\"]",
			oldInputs: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "foo",
					},
				},
			},
			newInputs: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "bar",
					},
				},
				"c": 42,
			},
			expected: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "foo",
					},
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b[\"c\"]"},
		},
		{
			name: "Missing in new sets, using [\"\"]",
			oldInputs: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "foo",
					},
				},
			},
			newInputs: map[string]any{
				"a": map[string]any{
					"b": map[string]any{},
				},
				"c": 42,
			},
			expected: map[string]any{
				"a": map[string]any{
					"b": map[string]any{
						"c": "foo",
					},
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b[\"c\"]"},
		},
		{
			name:      "Missing in old deletes",
			oldInputs: map[string]any{},
			newInputs: map[string]any{
				"a": map[string]any{
					"b": "foo",
				},
				"c": 42,
			},
			expected: map[string]any{
				"a": map[string]any{},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name:      "Missing keys in old and new are OK",
			oldInputs: map[string]any{},
			newInputs: map[string]any{},
			ignoreChanges: []string{
				"a",
				"a.b",
				"a.c[0]",
			},
		},
		{
			name: "Missing parent keys in only new",
			oldInputs: map[string]any{
				"a": map[string]any{
					"b": "foo",
				},
			},
			newInputs:     map[string]any{},
			expected:      map[string]any{},
			ignoreChanges: []string{"a.b"},
		},
		{
			name: "Arrays with different lengths",
			oldInputs: map[string]any{
				"a": []any{
					map[string]string{"b": "foo", "c": "bar"},
					map[string]string{"b": "bar", "c": "baz"},
				},
			},
			newInputs: map[string]any{
				"a": []any{
					map[string]string{"b": "bar", "c": "bar"},
					map[string]string{"b": "qux", "c": "baz"},
					map[string]string{"b": "baz", "c": "qux"},
				},
			},
			expected: map[string]any{
				"a": []any{
					map[string]string{"b": "foo", "c": "bar"},
					map[string]string{"b": "bar", "c": "baz"},
					map[string]string{"b": "baz", "c": "qux"},
				},
			},
			ignoreChanges: []string{"a[*].b"},
		},
		{
			name: "Shorter new array",
			oldInputs: map[string]any{
				"a": []any{
					map[string]string{"b": "foo", "c": "bar"},
					map[string]string{"b": "bar", "c": "baz"},
				},
			},
			newInputs: map[string]any{
				"a": []any{
					map[string]string{"b": "bar", "c": "bar"},
				},
			},
			expected: map[string]any{
				"a": []any{
					map[string]string{"b": "foo", "c": "bar"},
				},
			},
			ignoreChanges: []string{"a[*].b"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			olds, news := resource.NewPropertyMapFromMap(c.oldInputs), resource.NewPropertyMapFromMap(c.newInputs)

			expected := olds
			if c.expected != nil {
				expected = resource.NewPropertyMapFromMap(c.expected)
			}

			d := &diag.MockSink{}
			urn := resource.URN("urn:pulumi:dev::test::test:resource:Resource::my-resource")

			processed := processIgnoreChanges(d, urn, news, olds, c.ignoreChanges)
			assert.Equal(t, expected, processed)
			if c.expectMessage {
				infomsgs := d.Messages[diag.Info]
				require.Len(t, infomsgs, 1, "Expected an info message for %q", c.name)
				infomsg := infomsgs[0]
				require.Len(t, infomsg.Args, 1, "Expected one argument in info message for %q", c.name)
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
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			newdiff, err := applyReplaceOnChanges(c.diff, c.replaceOnChanges, c.hasInitErrors)
			require.NoError(t, err)
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
			oldInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewPropertyValue(8),
				"val2": resource.NewPropertyValue("hello"),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewPropertyValue(8),
				"val2": resource.NewPropertyValue("hello"),
			}),
			expected:        nil,
			expectedChanges: plugin.DiffNone,
		},
		{
			name: "All changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val0": resource.NewPropertyValue(3.14),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewProperty(42.0),
				"val2": resource.NewPropertyValue("world"),
			}),
			expected:        []resource.PropertyKey{"val0", "val1", "val2"},
			expectedChanges: plugin.DiffSome,
		},
		{
			name: "Some changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewPropertyValue(42),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewProperty(42.0),
				"val2": resource.NewPropertyValue("world"),
			}),
			expected:        []resource.PropertyKey{"val2"},
			expectedChanges: plugin.DiffSome,
		},
		{
			name: "Ignore some changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewPropertyValue("hello"),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val2": resource.NewPropertyValue(8),
			}),

			ignoreChanges:   []string{"val1"},
			expected:        []resource.PropertyKey{"val2"},
			expectedChanges: plugin.DiffSome,
		},
		{
			name: "Ignore all changes",
			oldInputs: resource.NewPropertyMapFromMap(map[string]any{
				"val1": resource.NewPropertyValue("hello"),
			}),
			newInputs: resource.NewPropertyMapFromMap(map[string]any{
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
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			d := &diag.MockSink{}
			diff, err := diffResource(
				d, urn, id, c.oldInputs, oldOutputs, c.newInputs, &provider, allowUnknowns, c.ignoreChanges,
			)
			t.Logf("diff.ChangedKeys = %v", diff.ChangedKeys)
			t.Logf("diff.StableKeys = %v", diff.StableKeys)
			t.Logf("diff.ReplaceKeys = %v", diff.ReplaceKeys)
			require.NoError(t, err)
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

			actual := sg.generateAliases(goal.Name, goal.Type, goal.Parent, goal.Aliases)
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

		apUrn := resource.NewURN("test", "test", "", sdkproviders.MakeProviderType("pkgA"), "a")
		apRef, err := sdkproviders.NewReference(apUrn, "0")
		require.NoError(t, err)

		bpUrn := resource.NewURN("test", "test", "", sdkproviders.MakeProviderType("pkgB"), "b")
		bpRef, err := sdkproviders.NewReference(bpUrn, "1")
		require.NoError(t, err)

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
		apUrn := resource.NewURN("test", "test", "", sdkproviders.MakeProviderType("pkgA"), "a")
		apRef, err := sdkproviders.NewReference(apUrn, "0")
		require.NoError(t, err)

		bpUrn := resource.NewURN("test", "test", "", sdkproviders.MakeProviderType("pkgB"), "b")
		bpRef, err := sdkproviders.NewReference(bpUrn, "1")
		require.NoError(t, err)

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
			_, _, err := sg.generateSteps(t.Context(), &registerResourceEvent{
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
			require.NoError(t, err)
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
			_, err := sg.providerChanged(
				"",
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
			_, err := sg.providerChanged(
				"",
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
			_, err := sg.providerChanged(
				"",
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

	t.Run("generateSteps emits ExtensionParameterizeStep for an extension event", func(t *testing.T) {
		t.Parallel()

		// Pre-load a fake provider into the registry so GetProvider(ref)
		// returns it later. The registry's plugin map is unexported, so we
		// can't insert directly — but Registry.Same() will load a plugin
		// via host.Provider() and store it. By making the host return our
		// fake, the registry ends up holding the fake.
		fakeProvider := &deploytest.Provider{}
		host := &plugin.MockHost{
			ProviderF: func(workspace.PluginDescriptor, env.Env) (plugin.Provider, error) {
				return fakeProvider, nil
			},
		}
		registry := providers.NewRegistry(host, false, nil)

		providerURN := resource.URN("urn:pulumi:stack::project::pulumi:providers:k8s::default")
		err := registry.Same(context.Background(), &resource.State{
			URN:    providerURN,
			Custom: true,
			Type:   tokens.Type("pulumi:providers:k8s"),
			ID:     "id-1",
			Inputs: resource.PropertyMap{"version": resource.NewProperty("1.0.0")},
		}, false)
		require.NoError(t, err)

		providerRef, err := sdkproviders.NewReference(providerURN, "id-1")
		require.NoError(t, err)

		eventsChan := make(chan SourceEvent, 1)
		sg := &stepGenerator{
			deployment: &Deployment{
				opts:       &Options{},
				target:     &Target{Name: tokens.MustParseStackName("stack")},
				source:     NewNullSource("project"),
				extensions: map[sdkproviders.Reference][]inFlightExtension{},
				providers:  registry,
				panicErrs:  make(chan error, 1),
			},
			urns:   map[resource.URN]bool{},
			events: eventsChan,
		}

		event := &testRegEvent{
			goal: &resource.Goal{
				Type:     "k8s:apiextensions.k8s.io/v1:CustomResource",
				Name:     "my-cr",
				Provider: providerRef.String(),
			},
			extension:    &apitype.Extension{Name: "gateway-api", Version: "1.0.0", Value: []byte("blob")},
			extensionRef: apitype.ExtensionRef("extension-a"),
		}

		steps, async, err := sg.generateSteps(t.Context(), event)
		require.NoError(t, err)
		assert.True(t, async)
		require.Len(t, steps, 1)
		ps, ok := steps[0].(*ExtensionParameterizeStep)
		require.True(t, ok, "got %T", steps[0])

		// Simulate ExtensionParameterizeStep.Apply succeeding so the spawned
		// goroutine produces its continue-event.
		ps.cts.MustFulfill(struct{}{})

		select {
		case ev := <-eventsChan:
			cev, ok := ev.(*continueExtensionEvent)
			require.True(t, ok, "got %T", ev)
			expectedURN := resource.NewURN(
				"stack", "project", "",
				"k8s:apiextensions.k8s.io/v1:CustomResource", "my-cr",
			)
			assert.Equal(t, expectedURN, cev.URN())
			require.NoError(t, cev.Error())
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for continueExtensionEvent")
		}
	})
}

func TestExtensionParameterizeStepApply_Success(t *testing.T) {
	t.Parallel()

	extension := apitype.Extension{
		Name:    "gateway-api",
		Version: "1.2.3",
		Value:   []byte(`{"crd":"Gateway"}`),
	}

	var capturedRequest plugin.ParameterizeRequest
	prov := &deploytest.Provider{
		ParameterizeF: func(_ context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			capturedRequest = req
			return plugin.ParameterizeResponse{Name: extension.Name}, nil
		},
	}

	completionSource := &promise.CompletionSource[struct{}]{}
	step := NewExtensionParameterizeStep(&Deployment{}, prov, apitype.ExtensionRef("ref-success"), extension, completionSource)

	status, _, err := step.Apply()
	require.NoError(t, err)
	assert.Equal(t, resource.StatusOK, status)

	// Provider was called with the blob's contents, parsed into semver.
	val, ok := capturedRequest.Parameters.(*plugin.ParameterizeValue)
	require.True(t, ok, "expected ParameterizeValue (not ParameterizeArgs)")
	assert.Equal(t, extension.Name, val.Name)
	assert.Equal(t, semver.MustParse("1.2.3"), val.Version)
	assert.Equal(t, extension.Value, val.Value)

	// Waiters on this CompletionSource should now see a fulfilled promise.
	_, err = completionSource.Promise().Result(context.Background())
	require.NoError(t, err, "successful parameterize should fulfill the CompletionSource")
}

func TestExtensionParameterizeStepApply_ProviderError(t *testing.T) {
	t.Parallel()

	want := errors.New("provider blew up")
	prov := &deploytest.Provider{
		ParameterizeF: func(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			return plugin.ParameterizeResponse{}, want
		},
	}

	completionSource := &promise.CompletionSource[struct{}]{}
	step := NewExtensionParameterizeStep(
		&Deployment{},
		prov,
		apitype.ExtensionRef("ref-err"),
		apitype.Extension{Name: "x", Version: "1.0.0", Value: []byte{}},
		completionSource,
	)

	_, _, err := step.Apply()
	assert.ErrorIs(t, err, want, "Apply must surface the provider error")

	_, err = completionSource.Promise().Result(context.Background())
	assert.ErrorIs(t, err, want, "failed parameterize must reject the CompletionSource")
}

func TestExtensionParameterizeStepApply_MalformedVersion(t *testing.T) {
	t.Parallel()

	var called bool
	prov := &deploytest.Provider{
		ParameterizeF: func(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			called = true
			return plugin.ParameterizeResponse{}, nil
		},
	}

	completionSource := &promise.CompletionSource[struct{}]{}
	step := NewExtensionParameterizeStep(
		&Deployment{},
		prov,
		apitype.ExtensionRef("ref-badver"),
		apitype.Extension{Name: "x", Version: "not-a-version", Value: nil},
		completionSource,
	)

	_, _, err := step.Apply()
	require.Error(t, err, "malformed version must fail Apply")
	assert.False(t, called, "Apply must reject the blob without calling the provider")

	_, err = completionSource.Promise().Result(context.Background())
	assert.Error(t, err, "malformed version must reject the CompletionSource")
}
