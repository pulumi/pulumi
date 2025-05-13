// Copyright 2016-2022, Pulumi Corporation.
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
)

func TestCapabilities(t *testing.T) {
	t.Parallel()
	t.Run("parse empty", func(t *testing.T) {
		t.Parallel()
		actual, err := CapabilitiesResponse{}.Parse()
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
		assert.Equal(t, Capabilities{}, actual)
	})
}
