package main

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImportFile_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		give     importFile
		wantErrs []string
	}{
		{
			desc: "missing everything",
			give: importFile{Resources: []importSpec{{}}},
			wantErrs: []string{
				"3 errors occurred",
				"resource 0 has no type",
				"resource 0 has no name",
				"resource 0 has no ID",
			},
		},
		{
			desc: "missing name and type",
			give: importFile{
				Resources: []importSpec{
					{ID: "thing"},
				},
			},
			wantErrs: []string{
				"2 errors occurred",
				"resource 'thing' has no type",
				"resource 'thing' has no name",
			},
		},
		{
			desc: "missing ID and type",
			give: importFile{
				Resources: []importSpec{
					{Name: "foo"},
				},
			},
			wantErrs: []string{
				"2 errors occurred",
				"resource 'foo' has no type",
				"resource 'foo' has no ID",
			},
		},
		{
			desc: "missing type",
			give: importFile{
				Resources: []importSpec{
					{
						Name: "foo",
						ID:   "bar",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"resource 'foo' has no type",
			},
		},
		{
			desc: "missing name",
			give: importFile{
				Resources: []importSpec{
					{
						ID:   "bar",
						Type: "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"resource 'bar' of type 'foo:bar:baz' has no name",
			},
		},
		{
			desc: "missing id",
			give: importFile{
				Resources: []importSpec{
					{
						Name: "foo",
						Type: "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"resource 'foo' of type 'foo:bar:baz' has no ID",
			},
		},
		{
			desc: "missing parent",
			give: importFile{
				Resources: []importSpec{
					{
						Name:   "thing",
						ID:     "thing",
						Parent: "unknown",
						Type:   "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"the parent 'unknown' for resource 'thing' of type 'foo:bar:baz' has no name",
			},
		},
		{
			desc: "missing provider",
			give: importFile{
				Resources: []importSpec{
					{
						Name:     "thing",
						ID:       "thing",
						Provider: "unknown",
						Type:     "foo:bar:baz",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"the provider 'unknown' for resource 'thing' of type 'foo:bar:baz' has no name",
			},
		},
		{
			desc: "bad version",
			give: importFile{
				Resources: []importSpec{
					{
						Name:    "thing",
						ID:      "thing",
						Type:    "foo:bar:baz",
						Version: "not-a-semver",
					},
				},
			},
			wantErrs: []string{
				"1 error occurred",
				"could not parse version 'not-a-semver' for resource 'thing' of type 'foo:bar:baz'",
			},
		},
		{
			desc: "ambiguous parent",
			give: importFile{
				Resources: []importSpec{
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:bar",
						Version: "0.0.0",
					},
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:baz",
						Version: "0.0.0",
					},
					{
						Name:    "res-2",
						ID:      "res-2",
						Type:    "foo:bar:a",
						Parent:  "res",
						Version: "0.0.0",
					},
				},
			},
			wantErrs: []string{
				"resource 'res-2' of type 'foo:bar:a' has an ambiguous parent",
			},
		},
		{
			desc: "ambiguous provider",
			give: importFile{
				NameTable: map[string]resource.URN{
					"res": "whatever",
				},
				Resources: []importSpec{
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:bar",
						Version: "0.0.0",
					},
					{
						Name:    "res",
						ID:      "res",
						Type:    "foo:bar:baz",
						Version: "0.0.0",
					},
					{
						Name:     "res-2",
						ID:       "res-2",
						Type:     "foo:bar:a",
						Provider: "res",
						Version:  "0.0.0",
					},
				},
			},
			wantErrs: []string{
				"resource 'res-2' of type 'foo:bar:a' has an ambiguous provider",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			require.NotEmpty(t, tt.wantErrs, "invalid test: wantErrs must not be empty")

			_, _, err := parseImportFile(tt.give, false)
			require.Error(t, err)
			for _, wantErr := range tt.wantErrs {
				assert.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestParseImportFileSameName(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			{
				Name:    "thing",
				ID:      "thing",
				Type:    "foo:bar:bar",
				Version: "0.0.0",
			},
			{
				Name:    "thing",
				ID:      "thing",
				Type:    "foo:bar:bar",
				Version: "0.0.0",
			},
		},
	}
	imports, _, err := parseImportFile(f, false)
	assert.NoError(t, err)
	resourceNames := map[tokens.QName]struct{}{}
	for _, imp := range imports {
		_, exists := resourceNames[imp.Name]
		assert.False(t, exists, "name %s should not have been seen already", imp.Name)
		resourceNames[imp.Name] = struct{}{}
	}

	// Check expected names are present.
	for _, name := range []tokens.QName{"thing", "thing_1"} {
		_, exists := resourceNames[name]
		assert.True(t, exists, "expected resource with name '%v' to be in the imports", name)
	}
}

func TestParseImportFileRenameNoClash(t *testing.T) {
	t.Parallel()
	f := importFile{
		Resources: []importSpec{
			{
				Name:    "thing",
				ID:      "thing",
				Type:    "foo:bar:a",
				Version: "0.0.0",
			},
			{
				Name:    "thing",
				ID:      "thing",
				Type:    "foo:bar:a",
				Version: "0.0.0",
			},
			{
				Name:    "thing_1",
				ID:      "thing",
				Type:    "foo:bar:a",
				Version: "0.0.0",
			},
		},
	}
	imports, _, err := parseImportFile(f, false)
	assert.NoError(t, err)
	resourceNames := map[tokens.QName]struct{}{}
	// Check resource names are unique.
	for _, imp := range imports {
		_, exists := resourceNames[imp.Name]
		assert.False(t, exists, "name %s should not have been seen already", imp.Name)
		resourceNames[imp.Name] = struct{}{}
	}

	// Check expected names are present.
	for _, name := range []tokens.QName{"thing", "thing_1", "thing_2"} {
		_, exists := resourceNames[name]
		assert.True(t, exists, "expected resource with name '%v' to be in the imports", name)
	}
}
