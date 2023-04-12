package main

import (
	"testing"

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
