package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImportFile_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		give    importFile
		wantErr string
	}{
		{
			desc:    "missing everything",
			give:    importFile{Resources: []importSpec{{}}},
			wantErr: "resource 0 has no type",
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
			wantErr: "resource 'foo' has no type",
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
			wantErr: "resource 'bar' of type 'foo:bar:baz' has no name",
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
			wantErr: "resource 'foo' of type 'foo:bar:baz' has no ID",
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
			wantErr: "the parent 'unknown' for resource 'thing' of type 'foo:bar:baz' has no name",
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
			wantErr: "the provider 'unknown' for resource 'thing' of type 'foo:bar:baz' has no name",
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
			wantErr: "could not parse version 'not-a-semver' for resource 'thing' of type 'foo:bar:baz'",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			require.NotEmpty(t, tt.wantErr, "invalid test: wantErr must not be empty")

			_, _, err := parseImportFile(tt.give, false)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
