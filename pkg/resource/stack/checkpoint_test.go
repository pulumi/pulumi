// Copyright 2016-2025, Pulumi Corporation.
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

package stack

import (
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func TestLoadV0Checkpoint(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("testdata/checkpoint-v0.json")
	require.NoError(t, err)

	chk, version, features, err := UnmarshalVersionedCheckpointToLatestCheckpoint(encoding.JSON, bytes)
	require.NoError(t, err)
	require.Equal(t, 3, version)
	require.Empty(t, features)
	require.NotNil(t, chk.Latest)
	require.Len(t, chk.Latest.Resources, 30)
}

func TestLoadV1Checkpoint(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("testdata/checkpoint-v1.json")
	require.NoError(t, err)

	chk, version, features, err := UnmarshalVersionedCheckpointToLatestCheckpoint(encoding.JSON, bytes)
	require.NoError(t, err)
	require.Equal(t, 3, version)
	require.Empty(t, features)
	require.NotNil(t, chk.Latest)
	require.Len(t, chk.Latest.Resources, 30)
}

func TestLoadV3Checkpoint(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("testdata/checkpoint-v3.json")
	require.NoError(t, err)

	chk, version, features, err := UnmarshalVersionedCheckpointToLatestCheckpoint(encoding.JSON, bytes)
	require.NoError(t, err)
	require.Equal(t, 3, version)
	require.Empty(t, features)
	require.NotNil(t, chk.Latest)
	require.Len(t, chk.Latest.Resources, 30)
}

func TestLoadV4Checkpoint(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("testdata/checkpoint-v4.json")
	require.NoError(t, err)

	chk, version, features, err := UnmarshalVersionedCheckpointToLatestCheckpoint(encoding.JSON, bytes)
	require.NoError(t, err)
	require.Equal(t, 4, version)
	require.Equal(t, []string{"refreshBeforeUpdate"}, features)
	require.NotNil(t, chk.Latest)
	require.Len(t, chk.Latest.Resources, 30)
}

func TestLoadV4CheckpointUnsupportedFeature(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("testdata/checkpoint-v4-unsupported-feature.json")
	require.NoError(t, err)

	chk, version, features, err := UnmarshalVersionedCheckpointToLatestCheckpoint(encoding.JSON, bytes)
	require.Nil(t, chk)
	require.Equal(t, 0, version)
	require.Nil(t, features)
	var expectedErr *ErrDeploymentUnsupportedFeatures
	require.ErrorAs(t, err, &expectedErr)
	require.Equal(t, []string{"unsupported-feature"}, expectedErr.Features)
}

// TestSerializeCheckpoint tests that the appropriate version and features are used when
// serializing a checkpoint.
func TestSerializeCheckpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		resources        []*resource.State
		expectedVersion  int
		expectedFeatures []string
	}{
		{
			name: "v3 deployment with no features",
			resources: []*resource.State{
				{
					URN: "urn1",
				},
			},
			expectedVersion:  3,
			expectedFeatures: nil,
		},
		{
			name: "v4 deployment with refreshBeforeUpdate",
			resources: []*resource.State{
				{
					URN:                 "urn1",
					RefreshBeforeUpdate: true,
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"refreshBeforeUpdate"},
		},
		{
			name: "v4 deployment with views",
			resources: []*resource.State{
				{
					URN: "urn1",
				},
				{
					URN:    "urn2",
					Parent: "urn1",
					ViewOf: "urn1",
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"views"},
		},
		{
			name: "v4 deployment with hooks",
			resources: []*resource.State{
				{
					URN: "urn1",
					ResourceHooks: map[resource.HookType][]string{
						resource.AfterCreate: {"hook1"},
					},
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"hooks"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			snap := &deploy.Snapshot{
				Resources: tt.resources,
			}
			checkpoint, err := SerializeCheckpoint("stack", snap, false /*showSecrets*/)
			require.NoError(t, err)
			require.NotNil(t, checkpoint)
			require.Equal(t, tt.expectedVersion, checkpoint.Version)
			require.Equal(t, tt.expectedFeatures, checkpoint.Features)
		})
	}
}
