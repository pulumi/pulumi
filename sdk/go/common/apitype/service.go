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

import "encoding/json"

// An APICapability is the name of a capability or feature that a service backend
// may or may not support.
type APICapability string

const (
	// DeltaCheckpointUploads is the feature that enables the CLI to upload checkpoints
	// via the PatchUpdateCheckpointDeltaRequest API to save on network bytes.
	DeltaCheckpointUploads APICapability = "delta-checkpoint-uploads"
)

type DeltaCheckpointUploadsConfigV1 struct {
	// CheckpointCutoffSizeBytes defines the size of a checkpoint file, in bytes,
	// at which the CLI should cutover to using delta checkpoint uploads.
	CheckpointCutoffSizeBytes int `json:"checkpointCutoffSizeBytes"`
}

// APICapabilityConfig captures a service backend capability and any associated
// configuration that may be required for integration.
type APICapabilityConfig struct {
	Capability    APICapability   `json:"capability"`
	Version       int             `json:"version,omitempty"`
	Configuration json.RawMessage `json:"configuration,omitempty"`
}

// CapabilitiesResponse defines all feature sets that are available in the service
// backend and are therefore available for the CLI to integrate against.
type CapabilitiesResponse struct {
	Capabilities []APICapabilityConfig `json:"capabilities"`
}
