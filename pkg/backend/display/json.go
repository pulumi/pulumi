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
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	stack_events "github.com/pulumi/pulumi/pkg/v3/resource/stack/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// massagePropertyValue takes a property value and strips out the secrets annotations from it.  If showSecrets is
// not true any secret values are replaced with "[secret]".
func massagePropertyValue(v resource.PropertyValue, showSecrets bool) resource.PropertyValue {
	switch {
	case v.IsArray():
		new := make([]resource.PropertyValue, len(v.ArrayValue()))
		for i, e := range v.ArrayValue() {
			new[i] = massagePropertyValue(e, showSecrets)
		}
		return resource.NewArrayProperty(new)
	case v.IsObject():
		new := make(resource.PropertyMap, len(v.ObjectValue()))
		for k, e := range v.ObjectValue() {
			new[k] = massagePropertyValue(e, showSecrets)
		}
		return resource.NewObjectProperty(new)
	case v.IsSecret() && showSecrets:
		return massagePropertyValue(v.SecretValue().Element, showSecrets)
	case v.IsSecret():
		return resource.NewStringProperty("[secret]")
	default:
		return v
	}
}

// MassageSecrets takes a property map and returns a new map by transforming each value with massagePropertyValue
// This allows us to serialize the resulting map using our existing serialization logic we use for deployments, to
// produce sane output for stackOutputs.  If we did not do this, SecretValues would be serialized as objects
// with the signature key and value.
func MassageSecrets(m resource.PropertyMap, showSecrets bool) resource.PropertyMap {
	new := make(resource.PropertyMap, len(m))
	for k, e := range m {
		new[k] = massagePropertyValue(e, showSecrets)
	}
	return new
}

// stateForJSONOutput prepares some resource's state for JSON output. This includes filtering the output based
// on the supplied options, in addition to massaging secret fields.
func stateForJSONOutput(s *resource.State, opts Options) *resource.State {
	var inputs resource.PropertyMap
	var outputs resource.PropertyMap
	if !isRootURN(s.URN) || !opts.SuppressOutputs {
		// For now, replace any secret properties as the string [secret] and then serialize what we have.
		inputs = MassageSecrets(s.Inputs, false)
		outputs = MassageSecrets(s.Outputs, false)
	} else {
		// If we're suppressing outputs, don't show the root stack properties.
		inputs = resource.PropertyMap{}
		outputs = resource.PropertyMap{}
	}

	return resource.NewState(s.Type, s.URN, s.Custom, s.Delete, s.ID, inputs,
		outputs, s.Parent, s.Protect, s.External, s.Dependencies, s.InitErrors, s.Provider,
		s.PropertyDependencies, s.PendingReplacement, s.AdditionalSecretOutputs, s.Aliases, &s.CustomTimeouts,
		s.ImportID)
}

// ShowJSONEvents renders incremental engine events to stdout.
func ShowJSONEvents(eventsC <-chan events.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()

	sequence := 0
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	for e := range eventsC {
		err := logJSONEvent(encoder, e, opts, sequence)
		contract.IgnoreError(err)
		sequence++

		// In the event of cancellation, break out of the loop.
		if e.Type == events.CancelEvent {
			break
		}
	}
}

// ShowPreviewDigest renders engine events from a preview into a well-formed JSON document. Note that this does not
// emit events incrementally so that it can guarantee anything emitted to stdout is well-formed. This means that,
// if used interactively, the experience will lead to potentially very long pauses. If run in CI, it is up to the
// end user to ensure that output is periodically printed to prevent tools from thinking preview has hung.
func ShowPreviewDigest(eventsC <-chan events.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()

	// Now loop and accumulate our digest until the event stream is closed, or we hit a cancellation.
	var digest previewDigest
	for e := range eventsC {
		// In the event of cancellation, break out of the loop immediately.
		if e.Type == events.CancelEvent {
			break
		}

		// For all other events, use the payload to build up the JSON digest we'll emit later.
		switch e.Type {
		// Events occurring early:
		case events.PreludeEvent:
			// Capture the config map from the prelude. Note that all secrets will remain blinded for safety.
			digest.Config = e.Payload().(events.PreludeEventPayload).Config

		// Events throughout the execution:
		case events.DiagEvent:
			// Skip any ephemeral or debug messages, and elide all colorization.
			p := e.Payload().(events.DiagEventPayload)
			if !p.Ephemeral && p.Severity != apitype.SeverityDebug {
				digest.Diagnostics = append(digest.Diagnostics, previewDiagnostic{
					URN:      p.URN,
					Message:  colors.Never.Colorize(p.Prefix + p.Message),
					Severity: p.Severity,
				})
			}
		case events.StdoutColorEvent:
			// Append stdout events as informational messages, and elide all colorization.
			p := e.Payload().(events.StdoutEventPayload)
			digest.Diagnostics = append(digest.Diagnostics, previewDiagnostic{
				Message:  colors.Never.Colorize(p.Message),
				Severity: apitype.SeverityInfo,
			})
		case events.ResourcePreEvent:
			// Create the detailed metadata for this step and the initial state of its resource. Later,
			// if new outputs arrive, we'll search for and swap in those new values.
			if m := e.Payload().(events.ResourcePreEventPayload).Metadata; shouldShow(m, opts) || isRootStack(m) {
				var detailedDiff map[string]propertyDiff
				if m.DetailedDiff != nil {
					detailedDiff = make(map[string]propertyDiff)
					for k, v := range m.DetailedDiff {
						detailedDiff[k] = propertyDiff{
							Kind:      string(v.Kind),
							InputDiff: v.InputDiff,
						}
					}
				}

				step := &previewStep{
					Op:             m.Op,
					URN:            m.URN,
					Provider:       m.Provider,
					DiffReasons:    m.Diffs,
					ReplaceReasons: m.Keys,
					DetailedDiff:   detailedDiff,
				}

				if m.Old != nil {
					oldState := stateForJSONOutput(m.Old.State, opts)
					res, err := stack_events.SerializeResource(oldState, config.NewPanicCrypter(), false /* showSecrets */)
					if err == nil {
						step.OldState = &res
					}
				}
				if m.New != nil {
					newState := stateForJSONOutput(m.New.State, opts)
					res, err := stack_events.SerializeResource(newState, config.NewPanicCrypter(), false /* showSecrets */)
					if err == nil {
						step.NewState = &res
					}
				}

				digest.Steps = append(digest.Steps, step)
			}
		case events.ResourceOutputsEvent, events.ResourceOperationFailed:
		// Because we are only JSON serializing previews, we don't need to worry about outputs
		// resolving or operations failing.

		// Events occurring late:
		case events.PolicyViolationEvent:
			// At this point in time, we don't handle policy events in JSON serialization
			continue
		case events.SummaryEvent:
			// At the end of the preview, a summary event indicates the final conclusions.
			p := e.Payload().(events.SummaryEventPayload)
			digest.Duration = p.Duration
			digest.ChangeSummary = p.ResourceChanges
			digest.MaybeCorrupt = p.MaybeCorrupt
		default:
			contract.Failf("unknown event type '%s'", e.Type)
		}
	}
	// Finally, go ahead and render the JSON to stdout.
	out, err := json.MarshalIndent(&digest, "", "    ")
	contract.Assertf(err == nil, "unexpected JSON error: %v", err)
	fmt.Println(string(out))
}

// previewDigest is a JSON-serializable overview of a preview operation.
type previewDigest struct {
	// Config contains a map of configuration keys/values used during the preview. Any secrets will be blinded.
	Config map[string]string `json:"config,omitempty"`

	// Steps contains a detailed list of all resource step operations.
	Steps []*previewStep `json:"steps,omitempty"`
	// Diagnostics contains a record of all warnings/errors that took place during the preview. Note that
	// ephemeral and debug messages are omitted from this list, as they are meant for display purposes only.
	Diagnostics []previewDiagnostic `json:"diagnostics,omitempty"`

	// Duration records the amount of time it took to perform the preview.
	Duration time.Duration `json:"duration,omitempty"`
	// ChangeSummary contains a map of count per operation (create, update, etc).
	ChangeSummary events.ResourceChanges `json:"changeSummary,omitempty"`
	// MaybeCorrupt indicates whether one or more resources may be corrupt.
	MaybeCorrupt bool `json:"maybeCorrupt,omitempty"`
}

// propertyDiff contains information about the difference in a single property value.
type propertyDiff struct {
	// Kind is the kind of difference.
	Kind string `json:"kind"`
	// InputDiff is true if this is a difference between old and new inputs instead of old state and new inputs.
	InputDiff bool `json:"inputDiff"`
}

// previewStep is a detailed overview of a step the engine intends to take.
type previewStep struct {
	// Op is the kind of operation being performed.
	Op events.StepOp `json:"op"`
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
	DetailedDiff map[string]propertyDiff `json:"detailedDiff"`
}

// previewDiagnostic is a warning or error emitted during the execution of the preview.
type previewDiagnostic struct {
	URN      resource.URN `json:"urn,omitempty"`
	Prefix   string       `json:"prefix,omitempty"`
	Message  string       `json:"message,omitempty"`
	Severity string       `json:"severity,omitempty"`
}
