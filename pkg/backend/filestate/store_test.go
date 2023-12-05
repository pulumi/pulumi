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

package filestate

import (
	"context"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/memblob"
)

func TestLegacyReferenceStore_referencePaths(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	store := newLegacyReferenceStore(bucket)

	ref, err := store.ParseReference("foo")
	require.NoError(t, err)

	assert.Equal(t, tokens.MustParseStackName("foo"), ref.Name())
	assert.Equal(t, tokens.QName("foo"), ref.FullyQualifiedName())
	assert.Equal(t, ".pulumi/stacks/foo", ref.StackBasePath())
	assert.Equal(t, ".pulumi/history/foo", ref.HistoryDir())
	assert.Equal(t, ".pulumi/backups/foo", ref.BackupDir())
}

func TestProjectReferenceStore_referencePaths(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	store := newProjectReferenceStore(bucket, func() *workspace.Project {
		return &workspace.Project{Name: "test"}
	})

	ref, err := store.ParseReference("organization/myproject/mystack")
	require.NoError(t, err)

	assert.Equal(t, ".pulumi/stacks/myproject/mystack", ref.StackBasePath())
	assert.Equal(t, ".pulumi/history/myproject/mystack", ref.HistoryDir())
	assert.Equal(t, ".pulumi/backups/myproject/mystack", ref.BackupDir())
}

func TestProjectReferenceStore_ParseReference(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	store := newProjectReferenceStore(bucket, func() *workspace.Project {
		return &workspace.Project{Name: "currentProject"}
	})

	tests := []struct {
		desc string
		give string

		fqname  tokens.QName
		name    string
		project tokens.Name
		str     string
	}{
		{
			desc:    "simple",
			give:    "foo",
			fqname:  "organization/currentProject/foo",
			name:    "foo",
			project: "currentProject",
			str:     "foo",
			// truncated because project name is the same as current project
		},
		{
			desc:    "organization",
			give:    "organization/foo",
			fqname:  "organization/currentProject/foo",
			name:    "foo",
			project: "currentProject",
			str:     "foo",
		},
		{
			desc:    "fully qualified",
			give:    "organization/project/foo",
			fqname:  "organization/project/foo",
			name:    "foo",
			project: "project",
			str:     "organization/project/foo", // doesn't match current project
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			ref, err := store.ParseReference(tt.give)
			require.NoError(t, err)

			assert.Equal(t, tt.fqname, ref.FullyQualifiedName())
			assert.Equal(t, tokens.MustParseStackName(tt.name), ref.Name())
			proj, has := ref.Project()
			assert.True(t, has)
			assert.Equal(t, tt.project, proj)
			assert.Equal(t, tt.str, ref.String())
		})
	}
}

func TestLegacyReferenceStore_ParseReference_errors(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	store := newLegacyReferenceStore(bucket)

	tests := []struct {
		desc string
		give string
	}{
		{desc: "empty", give: ""},
		{desc: "invalid name", give: "foo/bar"},
		{desc: "too many parts", give: "foo/bar/baz"},
		{
			desc: "over 100 characters",
			give: strings.Repeat("a", 101),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			_, err := store.ParseReference(tt.give)
			assert.Error(t, err)
			// If we ever make error messages here more specific,
			// we can add assert.ErrorContains here.
		})
	}
}

func TestProjectReferenceStore_ParseReference_errors(t *testing.T) {
	t.Parallel()

	bucket := memblob.OpenBucket(nil)
	store := newProjectReferenceStore(bucket, func() *workspace.Project {
		return nil // current project is not set
	})

	tests := []struct {
		desc    string
		give    string
		wantErr string
	}{
		{
			desc:    "empty",
			wantErr: "must not be empty",
		},
		{
			desc:    "bad organization",
			give:    "foo/bar/baz",
			wantErr: "organization name must be 'organization'",
		},
		{
			desc:    "long project name",
			give:    "organization/" + strings.Repeat("a", 101) + "/foo",
			wantErr: "project names are limited to 100 characters",
		},
		{
			desc:    "long project stack name",
			give:    "organization/foo/" + strings.Repeat("a", 101),
			wantErr: "a stack name cannot exceed 100 characters",
		},
		{
			desc:    "no current project",
			give:    "organization/foo",
			wantErr: "pass the fully qualified name",
		},
		{
			desc:    "invalid project name",
			give:    "organization/foo:bar/baz",
			wantErr: "may only contain alphanumeric",
		},
		{
			desc:    "invalid stack name",
			give:    "organization/foo/baz:qux",
			wantErr: "may only contain alphanumeric",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			require.NotEmpty(t, tt.wantErr,
				"bad test case: wantErr must be non-empty")

			_, err := store.ParseReference(tt.give)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLegacyReferenceStore_ListReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		// List of file paths relative to the storage root
		// that should exist before ListReferences is called.
		files []string

		// List of fully-qualified stack names that should be returned
		// by ListReferences.
		want []tokens.QName
	}{
		{
			desc: "empty",
			want: []tokens.QName{},
		},
		{
			desc: "json",
			files: []string{
				".pulumi/stacks/foo.json",
			},
			want: []tokens.QName{"foo"},
		},
		{
			desc: "gzipped",
			files: []string{
				".pulumi/stacks/foo.json.gz",
			},
			want: []tokens.QName{"foo"},
		},
		{
			desc: "multiple",
			files: []string{
				".pulumi/stacks/foo.json",
				".pulumi/stacks/bar.json.gz",
				".pulumi/stacks/baz.json",
			},
			want: []tokens.QName{"bar", "baz", "foo"},
		},
		{
			desc: "extraneous directories",
			files: []string{
				".pulumi/stacks/foo.json",
				".pulumi/stacks/bar.json/baz.json", // not a file
			},
			want: []tokens.QName{"foo"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			bucket := memblob.OpenBucket(nil)
			store := newLegacyReferenceStore(bucket)

			ctx := context.Background()
			for _, f := range tt.files {
				require.NoError(t, bucket.WriteAll(ctx, f, []byte{}, nil))
			}

			refs, err := store.ListReferences(ctx)
			require.NoError(t, err)

			got := make([]tokens.QName, len(refs))
			for i, ref := range refs {
				got[i] = ref.FullyQualifiedName()
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProjectReferenceStore_List(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		// List of file paths relative to the storage root
		// that should exist before ListReferences is called.
		files []string

		// List of fully-qualified stack names that should be returned
		// by ListReferences.
		stacks []tokens.QName

		// List of project names that should be returned by ListProjects.
		projects []tokens.Name
	}{
		{
			desc:     "empty",
			stacks:   []tokens.QName{},
			projects: nil,
		},
		{
			desc: "json",
			files: []string{
				".pulumi/stacks/proj/foo.json",
			},
			stacks:   []tokens.QName{"organization/proj/foo"},
			projects: []tokens.Name{"proj"},
		},
		{
			desc: "gzipped",
			files: []string{
				".pulumi/stacks/foo/bar.json.gz",
			},
			stacks:   []tokens.QName{"organization/foo/bar"},
			projects: []tokens.Name{"foo"},
		},
		{
			desc: "multiple",
			files: []string{
				".pulumi/stacks/a/foo.json",
				".pulumi/stacks/b/bar.json.gz",
				".pulumi/stacks/c/baz.json",
			},
			stacks: []tokens.QName{
				"organization/a/foo",
				"organization/b/bar",
				"organization/c/baz",
			},
			projects: []tokens.Name{"a", "b", "c"},
		},
		{
			desc: "extraneous files and directories",
			files: []string{
				".pulumi/stacks/a/foo.json",
				".pulumi/stacks/foo.json",
				".pulumi/stacks/bar/baz/qux.json", // nested too deep
				".pulumi/stacks/a b/c.json",       // bad project name
			},
			stacks:   []tokens.QName{"organization/a/foo"},
			projects: []tokens.Name{"a", "bar"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			bucket := memblob.OpenBucket(nil)
			store := newProjectReferenceStore(bucket, func() *workspace.Project {
				return &workspace.Project{Name: "test"}
			})

			ctx := context.Background()
			for _, f := range tt.files {
				require.NoError(t, bucket.WriteAll(ctx, f, []byte{}, nil))
			}

			t.Run("Projects", func(t *testing.T) {
				t.Parallel()

				projects, err := store.ListProjects(ctx)
				require.NoError(t, err)

				assert.Equal(t, tt.projects, projects)
			})

			t.Run("References", func(t *testing.T) {
				t.Parallel()

				refs, err := store.ListReferences(ctx)
				require.NoError(t, err)

				got := make([]tokens.QName, len(refs))
				for i, ref := range refs {
					got[i] = ref.FullyQualifiedName()
				}

				assert.Equal(t, tt.stacks, got)
			})
		})
	}
}

func TestProjectReferenceStore_ProjectExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string

		// List of file paths relative to the storage root
		// that should exist before ListReferences is called.
		files []string

		// Project name that should exist before ProjectExists is called.
		projectName string

		// Result that should be returned by ProjectExists.
		exist bool
	}{
		{
			desc: "project exists",
			files: []string{
				".pulumi/stacks/a/foo.json",
			},
			projectName: "a",
			exist:       true,
		},
		{
			desc: "project exists as empty directory",
			files: []string{
				".pulumi/stacks/a",
			},
			projectName: "a",
			exist:       false,
		},
		{
			desc: "project does not exist",
			files: []string{
				".pulumi/stacks/a",
			},
			projectName: "b",
			exist:       false,
		},
		{
			desc: "subproject exist",
			files: []string{
				".pulumi/stacks/b/a", // Project name exist, but as a subproject
			},
			projectName: "a",
			exist:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			bucket := memblob.OpenBucket(nil)
			store := newProjectReferenceStore(bucket, func() *workspace.Project {
				return &workspace.Project{Name: "test"}
			})

			ctx := context.Background()
			for _, f := range tt.files {
				require.NoError(t, bucket.WriteAll(ctx, f, []byte{}, nil))
			}

			exist, err := store.ProjectExists(ctx, tt.projectName)
			assert.NoError(t, err)
			assert.Equal(t, tt.exist, exist)
		})
	}
}
