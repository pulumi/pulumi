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

	assert.Equal(t, tokens.Name("foo"), ref.Name())
	assert.Equal(t, tokens.QName("foo"), ref.FullyQualifiedName())
	assert.Equal(t, ".pulumi/stacks/foo", ref.StackBasePath())
	assert.Equal(t, ".pulumi/history/foo", ref.HistoryDir())
	assert.Equal(t, ".pulumi/backups/foo", ref.BackupDir())
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

			refs, err := store.ListReferences()
			require.NoError(t, err)

			got := make([]tokens.QName, len(refs))
			for i, ref := range refs {
				got[i] = ref.FullyQualifiedName()
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
