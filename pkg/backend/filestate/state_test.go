package filestate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/memblob"
)

func TestIsPulumiDirEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		// List of files that exist in the bucket.
		files []string

		// Whether the pulumi directory is considered empty.
		empty bool
	}{
		{
			desc:  "empty",
			empty: true,
		},
		{
			desc: "non-state files",
			files: []string{
				"foo",
				"bar",
			},
			empty: true,
		},
		{
			desc: "state files",
			files: []string{
				".pulumi/stacks/foo.json",
				".pulumi/stacks/bar.json",
			},
			empty: false,
		},
		{
			desc: "has pulumi meta file",
			files: []string{
				".pulumi/meta.yaml",
			},
			empty: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			b := memblob.OpenBucket(nil)
			ctx := context.Background()
			for _, f := range tt.files {
				require.NoError(t, b.WriteAll(ctx, f, []byte{}, nil))
			}

			got, err := isPulumiDirEmpty(ctx, b)
			require.NoError(t, err)
			assert.Equal(t, tt.empty, got)
		})
	}
}
