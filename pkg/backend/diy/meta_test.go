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

package diy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/memblob"
)

func TestEnsurePulumiMeta(t *testing.T) {
	t.Parallel()

	mkmap := func(value string) env.MapStore {
		m := make(env.MapStore)
		m[env.DIYBackendLegacyLayout.Var().Name()] = value
		return m
	}

	tests := []struct {
		desc string
		give map[string]string // files in the bucket
		env  env.MapStore      // environment variables
		want pulumiMeta
	}{
		{
			// Empty bucket should be initialized to
			// the current version by default.
			desc: "empty",
			want: pulumiMeta{Version: 1},
		},
		{
			// Use legacy mode even for the new bucket
			// because the environment variable is "1".
			desc: "empty/legacy",
			env:  mkmap("1"),
			want: pulumiMeta{Version: 0},
		},
		{
			// Use legacy mode even for the new bucket
			// because the environment variable is "true".
			desc: "empty/legacy/true",
			env:  mkmap("true"),
			want: pulumiMeta{Version: 0},
		},
		{
			// Legacy mode is disabled by setting the env var
			// to "false".
			// This is also the default behavior.
			desc: "empty/legacy/false",
			env:  mkmap("false"),
			want: pulumiMeta{Version: 1},
		},
		{
			// Non-empty bucket without a version file
			// should get version 0 for legacy mode.
			desc: "legacy",
			give: map[string]string{
				".pulumi/stacks/a.json": `{}`,
			},
			want: pulumiMeta{Version: 0},
		},
		{
			desc: "version 0",
			give: map[string]string{
				".pulumi/meta.yaml": `version: 0`,
			},
			want: pulumiMeta{Version: 0},
		},
		{
			// Non-empty bucket with a version file
			// should get whatever is in the file.
			desc: "version 1",
			give: map[string]string{
				".pulumi/meta.yaml": `version: 1`,
			},
			want: pulumiMeta{Version: 1},
		},
		{
			desc: "future version",
			give: map[string]string{
				".pulumi/meta.yaml": `version: 42`,
			},
			want: pulumiMeta{Version: 42},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			b := memblob.OpenBucket(nil)
			ctx := context.Background()
			for name, body := range tt.give {
				require.NoError(t, b.WriteAll(ctx, name, []byte(body), nil))
			}

			state, err := ensurePulumiMeta(ctx, b, env.NewEnv(tt.env))
			require.NoError(t, err)
			assert.Equal(t, &tt.want, state)
		})
	}
}

func TestEnsurePulumiMeta_corruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		give    string // contents of meta.yaml
		wantErr string
	}{
		{
			desc:    "empty",
			give:    ``,
			wantErr: `corrupt store: missing version in ".pulumi/meta.yaml"`,
		},
		{
			desc:    "other fields",
			give:    `foo: bar`,
			wantErr: `corrupt store: missing version in ".pulumi/meta.yaml"`,
		},
		{
			desc:    "corrupt version",
			give:    `version: foo`,
			wantErr: `corrupt store: unmarshal ".pulumi/meta.yaml"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			b := memblob.OpenBucket(nil)
			ctx := context.Background()
			require.NoError(t, b.WriteAll(ctx, ".pulumi/meta.yaml", []byte(tt.give), nil))

			_, err := ensurePulumiMeta(context.Background(), b, env.NewEnv(nil))
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// Writes a metadata file to the bucket and reads it back.
func TestMeta_roundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give pulumiMeta
	}{
		{desc: "zero", give: pulumiMeta{Version: 0}},
		{desc: "one", give: pulumiMeta{Version: 1}},
		{desc: "future", give: pulumiMeta{Version: 42}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			b := memblob.OpenBucket(nil)
			// The bucket is always non-empty,
			// so we won't automatically try to use version 1.
			require.NoError(t,
				b.WriteAll(context.Background(), ".pulumi/stacks/dev.json", []byte("bar"), nil))

			ctx := context.Background()
			require.NoError(t, tt.give.WriteTo(ctx, b))

			got, err := ensurePulumiMeta(ctx, b, env.NewEnv(nil))
			require.NoError(t, err)
			assert.Equal(t, &tt.give, got)
		})
	}
}

// Verifies that we don't write a metadata file with version 0.
func TestMeta_WriteTo_zero(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, (&pulumiMeta{
		Version: 0,
	}).WriteTo(ctx, bucket))

	assert.NoFileExists(t, filepath.Join(tmpDir, ".pulumi", "meta.yaml"))
}

// Verify that we don't create a metadata file with version 0 in buckets
// that have other files.
func TestNew_noMetaOnInit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(tmpDir, nil)
	require.NoError(t, err)
	require.NoError(t,
		bucket.WriteAll(context.Background(), ".pulumi/stacks/dev.json", []byte("bar"), nil))

	ctx := context.Background()
	_, err = New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(tmpDir, ".pulumi", "meta.yaml"))
}
