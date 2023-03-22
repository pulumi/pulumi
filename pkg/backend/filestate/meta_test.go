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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob/memblob"
)

func TestEnsurePulumiMeta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give map[string]string // files in the bucket
		env  map[string]string // environment variables
		want pulumiMeta
	}{
		{
			// Empty bucket should be initialized to
			// the current version by default.
			desc: "empty",
			want: pulumiMeta{Version: 0},
		},
		{
			desc: "version 0",
			give: map[string]string{
				".pulumi/Pulumi.yaml": `version: 0`,
			},
			want: pulumiMeta{Version: 0},
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

			state, err := ensurePulumiMeta(ctx, b)
			require.NoError(t, err)
			assert.Equal(t, &tt.want, state)
		})
	}
}

func TestEnsurePulumiMeta_corruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc    string
		give    string // contents of Pulumi.yaml
		wantErr string
	}{
		{
			desc:    "empty",
			give:    ``,
			wantErr: `corrupt store: missing version in ".pulumi/Pulumi.yaml"`,
		},
		{
			desc:    "other fields",
			give:    `foo: bar`,
			wantErr: `corrupt store: missing version in ".pulumi/Pulumi.yaml"`,
		},
		{
			desc:    "corrupt version",
			give:    `version: foo`,
			wantErr: `corrupt store: unmarshal ".pulumi/Pulumi.yaml"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			b := memblob.OpenBucket(nil)
			ctx := context.Background()
			require.NoError(t, b.WriteAll(ctx, ".pulumi/Pulumi.yaml", []byte(tt.give), nil))

			_, err := ensurePulumiMeta(context.Background(), b)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
