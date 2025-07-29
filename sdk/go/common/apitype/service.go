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

package apitype

import (
	"encoding/json"
	"fmt"
)

// An APICapability is the name of a capability or feature that a service backend
// may or may not support.
type APICapability string

const (
	// Deprecated. Use DeltaCheckpointUploadsV2.
	DeltaCheckpointUploads APICapability = "delta-checkpoint-uploads"

	// DeltaCheckpointUploads is the feature that enables the CLI to upload checkpoints
	// via the PatchUpdateCheckpointDeltaRequest API to save on network bytes.
	DeltaCheckpointUploadsV2 APICapability = "delta-checkpoint-uploads-v2"

	// Indicates that the service backend supports batch encryption.
	BatchEncrypt APICapability = "batch-encrypt"

	// Indicates whether the service supports summarizing errors via Copilot.
	CopilotSummarizeError APICapability = "copilot-summarize-error"

	// Indicates whether the service supports the Copilot explainer.
	CopilotExplainPreview APICapability = "copilot-explain-preview"

	// Indicates the maximum deployment schema version that the service supports.
	DeploymentSchemaVersion APICapability = "deployment-schema-version"
)

type DeltaCheckpointUploadsConfigV2 struct {
	// CheckpointCutoffSizeBytes defines the size of a checkpoint file, in bytes,
	// at which the CLI should cutover to using delta checkpoint uploads.
	CheckpointCutoffSizeBytes int `json:"checkpointCutoffSizeBytes"`
}

type DeploymentSchemaVersionConfig struct {
	// Version is the maximum version of the deployment schema that the service supports.
	Version int `json:"version"`
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

// Represents the set of features a backend is capable of supporting.
// This is a user-friendly representation of the CapabilitiesResponse.
type Capabilities struct {
	// If non-nil, indicates that delta checkpoint updates are supported.
	DeltaCheckpointUpdates *DeltaCheckpointUploadsConfigV2

	// Indicates whether the service supports batch encryption.
	BatchEncryption bool

	// Indicates whether the service supports summarizing errors via Copilot.
	CopilotSummarizeErrorV1 bool

	// Indicates whether the service supports the Copilot explainer.
	CopilotExplainPreviewV1 bool

	// Indicates the maximum deployment schema version that the service supports.
	DeploymentSchemaVersion int
}

// Parse decodes the CapabilitiesResponse into a Capabilities struct for ease of use.
func (r CapabilitiesResponse) Parse() (Capabilities, error) {
	var parsed Capabilities
	for _, entry := range r.Capabilities {
		switch entry.Capability {
		case DeltaCheckpointUploads:
			var upcfg DeltaCheckpointUploadsConfigV2
			if err := json.Unmarshal(entry.Configuration, &upcfg); err != nil {
				return Capabilities{}, fmt.Errorf("decoding DeltaCheckpointUploadsConfig returned %w", err)
			}
			parsed.DeltaCheckpointUpdates = &upcfg
		case DeltaCheckpointUploadsV2:
			if entry.Version == 2 {
				var upcfg DeltaCheckpointUploadsConfigV2
				if err := json.Unmarshal(entry.Configuration, &upcfg); err != nil {
					return Capabilities{}, fmt.Errorf("decoding DeltaCheckpointUploadsConfigV2 returned %w", err)
				}
				parsed.DeltaCheckpointUpdates = &upcfg
			}
		case BatchEncrypt:
			parsed.BatchEncryption = true
		case CopilotSummarizeError:
			if entry.Version == 1 {
				parsed.CopilotSummarizeErrorV1 = true
			}
		case CopilotExplainPreview:
			if entry.Version == 1 {
				parsed.CopilotExplainPreviewV1 = true
			}
		case DeploymentSchemaVersion:
			if entry.Version == 1 {
				var versionConfig DeploymentSchemaVersionConfig
				if err := json.Unmarshal(entry.Configuration, &versionConfig); err != nil {
					return Capabilities{}, fmt.Errorf("decoding DeploymentSchemaVersionConfig returned %w", err)
				}
				parsed.DeploymentSchemaVersion = versionConfig.Version
			}
		default:
			continue
		}
	}
	return parsed, nil
}
