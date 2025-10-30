// Copyright 2016-2018, Pulumi Corporation.
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

package display

import (
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// StepOp represents the kind of operation performed by a step.  It evaluates to its string label.
type StepOp string

// ResourceChanges contains the aggregate resource changes by operation type.
type ResourceChanges map[StepOp]int

// PreviewDigest is a JSON-serializable overview of a preview operation.
type PreviewDigest struct {
	// Config contains a map of configuration keys/values used during the preview. Any secrets will be blinded.
	Config map[string]string `json:"config,omitempty"`

	// Steps contains a detailed list of all resource step operations.
	Steps []*PreviewStep `json:"steps,omitempty"`
	// Diagnostics contains a record of all warnings/errors that took place during the preview. Note that
	// ephemeral and debug messages are omitted from this list, as they are meant for display purposes only.
	Diagnostics []PreviewDiagnostic `json:"diagnostics,omitempty"`

	// Duration records the amount of time it took to perform the preview.
	Duration time.Duration `json:"duration,omitempty"`
	// ChangeSummary contains a map of count per operation (create, update, etc).
	ChangeSummary ResourceChanges `json:"changeSummary,omitempty"`
	// MaybeCorrupt indicates whether one or more resources may be corrupt.
	MaybeCorrupt bool `json:"maybeCorrupt,omitempty"`
}

// PropertyDiff contains information about the difference in a single property value.
type PropertyDiff struct {
	// Kind is the kind of difference.
	Kind string `json:"kind"`
	// InputDiff is true if this is a difference between old and new inputs instead of old state and new inputs.
	InputDiff bool `json:"inputDiff"`
}

// PreviewStep is a detailed overview of a step the engine intends to take.
type PreviewStep struct {
	// Op is the kind of operation being performed.
	Op StepOp `json:"op"`
	// URN is the resource being affected by this operation.
	URN resource.URN `json:"urn"`
	// Provider is the provider that will perform this step.
	Provider string `json:"provider,omitempty"`
	// OldState is the old state for this resource, if appropriate given the operation type.
	OldState *apitype.ResourceV3 `json:"oldState,omitempty"`
	// NewState is the new state for this resource, if appropriate given the operation type.
	NewState *apitype.ResourceV3 `json:"newState,omitempty"`
	// DiffReasons is a list of keys that are causing a diff (for updating steps only).
	DiffReasons []resource.PropertyKey `json:"diffReasons,omitempty"`
	// ReplaceReasons is a list of keys that are causing replacement (for replacement steps only).
	ReplaceReasons []resource.PropertyKey `json:"replaceReasons,omitempty"`
	// DetailedDiff is a structured diff that indicates precise per-property differences.
	DetailedDiff map[string]PropertyDiff `json:"detailedDiff"`
}

// PreviewDiagnostic is a warning or error emitted during the execution of the preview.
type PreviewDiagnostic struct {
	URN      resource.URN  `json:"urn,omitempty"`
	Prefix   string        `json:"prefix,omitempty"`
	Message  string        `json:"message,omitempty"`
	Severity diag.Severity `json:"severity,omitempty"`
}
