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

// EventSummary is a JSON-serialisable digest of an engine event stream produced by
// `pulumi preview`, `pulumi up`, `pulumi refresh`, or `pulumi destroy`. It captures the same
// information a human would see printed by these commands: the configuration, per-resource
// steps, final stack outputs, policy results, diagnostics, and aggregate counts.
//
// The document is emitted by `pulumi events summary` as a single JSON object, the terminal
// reduction of the JSONL event stream.
type EventSummary struct {
	// Version is the schema version. Starts at 1 and will only increase on
	// backwards-incompatible changes; additive changes keep the same version.
	Version int `json:"version"`

	// IsPreview is true when the stream came from `pulumi preview`. Taken directly from
	// SummaryEvent.IsPreview — if no SummaryEvent was seen (an interrupted run) it stays false.
	IsPreview bool `json:"isPreview"`

	// StartTime is the timestamp of the first event seen, as seconds since the Unix epoch.
	StartTime int `json:"startTime,omitempty"`
	// EndTime is the timestamp of the last event seen.
	EndTime int `json:"endTime,omitempty"`

	// Config contains the stack configuration keys and values for the update, taken from
	// PreludeEvent. Secrets are blinded by the emitter.
	Config map[string]string `json:"config,omitempty"`

	// Steps is the list of resource steps excluding `OpSame`. Each URN appears at most once:
	// the final observed step (ResOutputsEvent preferred over ResourcePreEvent) wins.
	Steps []*SummaryStep `json:"steps,omitempty"`

	// Outputs is the root stack's final outputs, taken from the last ResOutputsEvent with
	// type `pulumi:pulumi:Stack`. Empty for previews — outputs aren't applied in a preview.
	Outputs map[string]any `json:"outputs,omitempty"`

	// ChangeSummary is the per-op resource change count from the final SummaryEvent.
	ChangeSummary ResourceChanges `json:"changeSummary,omitempty"`

	// Duration is how long the update took, as reported by SummaryEvent.DurationSeconds.
	// Zero for previews (no duration is reported for a preview).
	Duration time.Duration `json:"duration,omitempty"`

	// Diagnostics lists non-ephemeral DiagnosticEvents observed in the stream, in the order
	// they arrived. Ephemeral diagnostics (intended for live display only) are filtered out,
	// as are debug-level messages.
	Diagnostics []PreviewDiagnostic `json:"diagnostics,omitempty"`

	// PolicyPacks lists the policy packs that ran during the update, as name → version.
	// Sourced verbatim from SummaryEvent.PolicyPacks; encoding/json sorts map keys so the
	// output is deterministic.
	PolicyPacks map[string]string `json:"policyPacks,omitempty"`

	// PolicyViolations collects PolicyEvents (plain violations) in arrival order.
	PolicyViolations []apitype.PolicyEvent `json:"policyViolations,omitempty"`

	// PolicyRemediations collects PolicyRemediationEvents (transformed resources) in arrival
	// order, preserving the `before`/`after` payloads.
	PolicyRemediations []apitype.PolicyRemediationEvent `json:"policyRemediations,omitempty"`

	// MaybeCorrupt is true if the engine reported that one or more resources may be corrupt.
	// Sourced from SummaryEvent.MaybeCorrupt.
	MaybeCorrupt bool `json:"maybeCorrupt,omitempty"`

	// Failed is true if any `ResOpFailedEvent` was seen, or the engine reported an internal
	// error. A successful preview/up leaves this false.
	Failed bool `json:"failed,omitempty"`

	// Error is set to the message of the last `ErrorEvent` in the stream, if any. These are
	// emitted for engine-internal errors, not user-facing ones — user errors arrive as
	// DiagnosticEvents with severity=error.
	Error string `json:"error,omitempty"`
}

// SummaryStep is the reduced form of a resource step carried in the summary. It embeds the
// engine's `StepEventMetadata` verbatim and adds a `Failed` flag surfaced from
// `ResOpFailedEvent`. `Old` and `New` are zeroed before storage — the summary stays compact by
// design, so consumers that need full per-step state should read the raw event stream.
type SummaryStep struct {
	apitype.StepEventMetadata
	// Failed is true iff a `ResOpFailedEvent` was emitted for this URN.
	Failed bool `json:"failed,omitempty"`
}
