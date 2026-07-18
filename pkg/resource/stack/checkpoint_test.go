// Copyright 2016, Pulumi Corporation.
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
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

type compactJSONMarshaler struct{}

func (compactJSONMarshaler) Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func (compactJSONMarshaler) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

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
		resources        []*pkgresource.State
		expectedVersion  int
		expectedFeatures []string
	}{
		{
			name: "v3 deployment with no features",
			resources: []*pkgresource.State{
				{
					URN: "urn1",
				},
			},
			expectedVersion:  3,
			expectedFeatures: nil,
		},
		{
			name: "v4 deployment with refreshBeforeUpdate",
			resources: []*pkgresource.State{
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
			resources: []*pkgresource.State{
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
			resources: []*pkgresource.State{
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
		{
			name: "v4 deployment with taint",
			resources: []*pkgresource.State{
				{
					URN:   "urn1",
					Taint: true,
				},
			},
			expectedVersion:  4,
			expectedFeatures: []string{"taint"},
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

func TestMarshalUntypedDeploymentToVersionedCheckpointWithMarshaler(t *testing.T) {
	t.Parallel()

	deployment := &apitype.UntypedDeployment{
		Version: 4,
		Features: []string{
			"refreshBeforeUpdate",
		},
		Deployment: json.RawMessage(`{
			"big": 100000000000000000000000000000000000000000,
			"duplicate": 1,
			"duplicate": 2,
			"resources": []
		}`),
	}

	checkpoint, err := MarshalUntypedDeploymentToVersionedCheckpointWithMarshaler(
		compactJSONMarshaler{},
		"stack",
		deployment,
	)
	require.NoError(t, err)
	require.Equal(t, deployment.Version, checkpoint.Version)
	require.Equal(t, deployment.Features, checkpoint.Features)
	require.Equal(t,
		`{"stack":"stack","latest":{"big":100000000000000000000000000000000000000000,`+
			`"duplicate":1,"duplicate":2,"resources":[]}}`,
		string(checkpoint.Checkpoint))
}

// TestRoundtripCheckpoint tests that various values survive a roundtrip of serialization
// and deserialization.
func TestRoundtripCheckpoint(t *testing.T) {
	t.Parallel()

	originalSnap := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				URN:     "pulumi:stack::project::pulumi:root::project-stack",
				Type:    resource.RootStackType,
				Inputs:  property.Map{},
				Outputs: property.Map{},
			},
			{
				URN:    "pulumi:stack::project::custom:resource:MyResource::res1",
				Type:   "custom:resource:MyResource",
				ID:     "res1-id",
				Custom: true,
				Inputs: property.NewMap(map[string]property.Value{
					"stringProp": property.New("inputValue"),
					"numberProp": property.New(42.0),
					"boolProp":   property.New(true),
					"nullProp":   {},
					"infProp":    property.New(math.Inf(1)),
					"negInfProp": property.New(math.Inf(-1)),
				}),
				Outputs: property.NewMap(map[string]property.Value{
					"outputProp": property.New("outputValue"),
				}),
				Parent: "pulumi:stack::project::pulumi:root::project-stack",
			},
		},
	}
	checkpoint, err := SerializeCheckpoint("stack", originalSnap, false /*showSecrets*/)
	require.NoError(t, err)
	require.NotNil(t, checkpoint)

	var v3checkpoint apitype.CheckpointV3
	err = json.Unmarshal(checkpoint.Checkpoint, &v3checkpoint)
	require.NoError(t, err)

	loadedSnap, err := DeserializeCheckpoint(t.Context(), nil, &v3checkpoint)
	require.NoError(t, err)
	require.NotNil(t, loadedSnap)
	require.Equal(t, originalSnap, loadedSnap)
}

// TestRoundtripNaNCheckpoint tests that NaN values survive a roundtrip of serialization and deserialization.
func TestRoundtripNanCheckpoint(t *testing.T) {
	t.Parallel()

	originalSnap := &deploy.Snapshot{
		Resources: []*pkgresource.State{
			{
				URN:  "pulumi:stack::project::pulumi:root::project-stack",
				Type: resource.RootStackType,
				Inputs: property.NewMap(map[string]property.Value{
					"nan": property.New(math.NaN()),
				}),
				Outputs: property.Map{},
			},
		},
	}
	checkpoint, err := SerializeCheckpoint("stack", originalSnap, false /*showSecrets*/)
	require.NoError(t, err)
	require.NotNil(t, checkpoint)

	var v3checkpoint apitype.CheckpointV3
	err = json.Unmarshal(checkpoint.Checkpoint, &v3checkpoint)
	require.NoError(t, err)

	loadedSnap, err := DeserializeCheckpoint(t.Context(), nil, &v3checkpoint)
	require.NoError(t, err)
	require.NotNil(t, loadedSnap)

	// We can't just use require.Equal because NaN != NaN, so we need to check the property specifically.
	loadedProp, ok := loadedSnap.Resources[0].Inputs.GetOk("nan")
	require.True(t, ok)
	require.True(t, loadedProp.IsNumber())
	require.True(t, math.IsNaN(loadedProp.AsNumber()))
}
