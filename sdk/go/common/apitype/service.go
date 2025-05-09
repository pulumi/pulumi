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
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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
	CopilotSummarizeErrorV1 APICapability = "copilot-summarize-error"

	// Indicates whether the service supports the Copilot explainer.
	CopilotExplainPreviewV1 APICapability = "copilot-explain-preview"
)

type DeltaCheckpointUploadsConfigV2 struct {
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

	// Dynamic map for supported capabilities
	supported map[APICapability]bool
}

// Parse decodes the CapabilitiesResponse into a Capabilities struct for ease of use.
func (r CapabilitiesResponse) Parse() (Capabilities, error) {
	var parsed Capabilities
	parsed.supported = make(map[APICapability]bool)
	for _, entry := range r.Capabilities {
		switch entry.Capability {
		case DeltaCheckpointUploads:
			var upcfg DeltaCheckpointUploadsConfigV2
			if err := json.Unmarshal(entry.Configuration, &upcfg); err != nil {
				return Capabilities{}, fmt.Errorf("decoding DeltaCheckpointUploadsConfig returned %w", err)
			}
			parsed.DeltaCheckpointUpdates = &upcfg
			parsed.supported[DeltaCheckpointUploads] = true
		case DeltaCheckpointUploadsV2:
			if entry.Version == 2 {
				var upcfg DeltaCheckpointUploadsConfigV2
				if err := json.Unmarshal(entry.Configuration, &upcfg); err != nil {
					return Capabilities{}, fmt.Errorf("decoding DeltaCheckpointUploadsConfigV2 returned %w", err)
				}
				parsed.DeltaCheckpointUpdates = &upcfg
				parsed.supported[DeltaCheckpointUploadsV2] = true
			}
		case BatchEncrypt:
			parsed.BatchEncryption = true
			parsed.supported[BatchEncrypt] = true
		case CopilotSummarizeErrorV1:
			if entry.Version == 1 {
				parsed.CopilotSummarizeErrorV1 = true
				parsed.supported[CopilotSummarizeErrorV1] = true
			}
		case CopilotExplainPreviewV1:
			if entry.Version == 1 {
				parsed.CopilotExplainPreviewV1 = true
				parsed.supported[CopilotExplainPreviewV1] = true
			}
		default:
			continue
		}
	}
	return parsed, nil
}

// Supports returns true if the given capability is supported by this backend.
func (c Capabilities) Supports(capability APICapability) bool {
	if c.supported == nil {
		logging.V(7).Infof("Capabilities not parsed, assuming %q is not supported", capability)
		return false
	}
	return c.supported[capability]
}
