package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func TestIgnoreChanges(t *testing.T) {
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
		t.Run(c.name, func(t *testing.T) {
			olds, news := resource.NewPropertyMapFromMap(c.oldInputs), resource.NewPropertyMapFromMap(c.newInputs)

			expected := olds
			if c.expected != nil {
				expected = resource.NewPropertyMapFromMap(c.expected)
			}

			processed, res := processIgnoreChanges(news, olds, c.ignoreChanges)
			if c.expectFailure {
				assert.NotNil(t, res)
			} else {
				assert.Nil(t, res)
				assert.Equal(t, expected, processed)
			}
		})
	}
}

func TestApplyReplaceOnChangesEmptyDetailedDiff(t *testing.T) {
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
			name:             "DiffSome and empty replaceOnChanges",
			diff:             plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"a"}},
			replaceOnChanges: []string{},
			hasInitErrors:    false,
			expected:         plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"a"}},
		},
		{
			name:             "DiffSome and non-empty replaceOnChanges",
			diff:             plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"a"}},
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
			name:             "DiffSome and empty replaceOnChanges w/ init errors",
			diff:             plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"a"}},
			replaceOnChanges: []string{},
			hasInitErrors:    true,
			expected:         plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"a"}},
		},
		{
			name:             "DiffSome and non-empty replaceOnChanges w/ init errors",
			diff:             plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"a"}},
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
			newdiff, err := applyReplaceOnChanges(c.diff, c.replaceOnChanges, c.hasInitErrors)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, newdiff)
		})
	}

}
