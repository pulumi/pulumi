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

package apitype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilities(t *testing.T) {
	t.Parallel()
	t.Run("parse empty", func(t *testing.T) {
		t.Parallel()
		actual, err := CapabilitiesResponse{}.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{}, actual)
	})
	t.Run("parse delta v1", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    DeltaCheckpointUploads,
					Configuration: json.RawMessage(`{}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			DeltaCheckpointUpdates: &DeltaCheckpointUploadsConfigV2{},
		}, actual)
	})
	t.Run("parse delta v1 with config", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    DeltaCheckpointUploads,
					Configuration: json.RawMessage(`{"checkpointCutoffSizeBytes": 1024}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			DeltaCheckpointUpdates: &DeltaCheckpointUploadsConfigV2{
				CheckpointCutoffSizeBytes: 1024,
			},
		}, actual)
	})
	t.Run("parse delta v2", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    DeltaCheckpointUploadsV2,
					Version:       2,
					Configuration: json.RawMessage("{}"),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			DeltaCheckpointUpdates: &DeltaCheckpointUploadsConfigV2{},
		}, actual)
	})
	t.Run("parse delta v2 with config", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    DeltaCheckpointUploadsV2,
					Version:       2,
					Configuration: json.RawMessage(`{"checkpointCutoffSizeBytes": 1024}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			DeltaCheckpointUpdates: &DeltaCheckpointUploadsConfigV2{
				CheckpointCutoffSizeBytes: 1024,
			},
		}, actual)
	})
	t.Run("parse batch encrypt", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{Capability: BatchEncrypt},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			BatchEncryption: true,
		}, actual)
	})

	t.Run("parse copilot summarize error v1", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{Capability: CopilotSummarizeError, Version: 1},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			CopilotSummarizeErrorV1: true,
		}, actual)
	})

	t.Run("parse copilot summarize error with newer version", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{Capability: CopilotSummarizeError, Version: 2},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{}, actual)
	})

	t.Run("parse deployment schema v4", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    DeploymentSchemaVersion,
					Version:       1,
					Configuration: json.RawMessage(`{"version": 4}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			DeploymentSchemaVersion: 4,
		}, actual)
	})

	t.Run("parse stack policy packs", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability: StackPolicyPacks,
					Version:    1,
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			StackPolicyPacks: true,
		}, actual)
	})

	t.Run("parse api version v1", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    APIVersion,
					Version:       1,
					Configuration: json.RawMessage(`{"maxVersion":9,"minVersion":3,"defaultVersion":8}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			APIVersion: &APIVersionCapabilityConfig{
				MaxVersion:     9,
				MinVersion:     3,
				DefaultVersion: 8,
			},
		}, actual)
	})

	t.Run("parse api version with newer version ignored", func(t *testing.T) {
		t.Parallel()
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    APIVersion,
					Version:       2,
					Configuration: json.RawMessage(`{"maxVersion":9,"minVersion":3,"defaultVersion":8}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{}, actual)
	})

	t.Run("parse response from older server without api version capability", func(t *testing.T) {
		t.Parallel()
		// A server that predates the api-version capability still emits other capabilities.
		// Parse must leave Capabilities.APIVersion nil rather than synthesizing a value.
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{Capability: BatchEncrypt},
				{Capability: StackPolicyPacks, Version: 1},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Nil(t, actual.APIVersion)
		assert.Equal(t, Capabilities{
			BatchEncryption:  true,
			StackPolicyPacks: true,
		}, actual)
	})

	t.Run("parse api version rejects malformed payloads", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			config string
			errSub string
		}{
			{
				name:   "invalid json",
				config: `{"maxVersion":`,
				errSub: "decoding APIVersionCapabilityConfig",
			},
			{
				name:   "empty object (missing fields default to zero)",
				config: `{}`,
				errSub: "minVersion must be >= 1",
			},
			{
				name:   "zero minVersion",
				config: `{"maxVersion":9,"minVersion":0,"defaultVersion":0}`,
				errSub: "minVersion must be >= 1",
			},
			{
				name:   "negative minVersion",
				config: `{"maxVersion":9,"minVersion":-1,"defaultVersion":9}`,
				errSub: "minVersion must be >= 1",
			},
			{
				name:   "maxVersion less than minVersion",
				config: `{"maxVersion":3,"minVersion":5,"defaultVersion":3}`,
				errSub: "maxVersion (3) must be >= minVersion (5)",
			},
			{
				name:   "defaultVersion below range",
				config: `{"maxVersion":9,"minVersion":5,"defaultVersion":3}`,
				errSub: "defaultVersion (3) must be in [minVersion, maxVersion] = [5, 9]",
			},
			{
				name:   "defaultVersion above range",
				config: `{"maxVersion":9,"minVersion":5,"defaultVersion":10}`,
				errSub: "defaultVersion (10) must be in [minVersion, maxVersion] = [5, 9]",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				response := CapabilitiesResponse{
					Capabilities: []APICapabilityConfig{
						{
							Capability:    APIVersion,
							Version:       1,
							Configuration: json.RawMessage(tc.config),
						},
					},
				}
				_, err := response.Parse()
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSub)
			})
		}
	})

	t.Run("parse api version accepts min equals max equals default", func(t *testing.T) {
		t.Parallel()
		// Boundary case: a server that only speaks one API version.
		response := CapabilitiesResponse{
			Capabilities: []APICapabilityConfig{
				{
					Capability:    APIVersion,
					Version:       1,
					Configuration: json.RawMessage(`{"maxVersion":5,"minVersion":5,"defaultVersion":5}`),
				},
			},
		}
		actual, err := response.Parse()
		require.NoError(t, err)
		assert.Equal(t, Capabilities{
			APIVersion: &APIVersionCapabilityConfig{
				MaxVersion:     5,
				MinVersion:     5,
				DefaultVersion: 5,
			},
		}, actual)
	})
}
