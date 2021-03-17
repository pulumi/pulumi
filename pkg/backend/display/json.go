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
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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

// ShowJSONEvents renders engine events from a preview into a well-formed JSON document. Note that this does not
// emit events incrementally so that it can guarantee anything emitted to stdout is well-formed. This means that,
// if used interactively, the experience will lead to potentially very long pauses. If run in CI, it is up to the
// end user to ensure that output is periodically printed to prevent tools from thinking preview has hung.
func ShowJSONEvents(op string, action apitype.UpdateKind, events <-chan engine.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()

	// Now loop and accumulate our digest until the event stream is closed, or we hit a cancellation.
	var digest previewDigest
	for e := range events {
		// In the event of cancelation, break out of the loop immediately.
		if e.Type == engine.CancelEvent {
			break
		}

		// For all other events, use the payload to build up the JSON digest we'll emit later.
		switch e.Type {
		// Events ocurring early:
		case engine.PreludeEvent:
			// Capture the config map from the prelude. Note that all secrets will remain blinded for safety.
			digest.Config = e.Payload().(engine.PreludeEventPayload).Config

		// Events throughout the execution:
		case engine.DiagEvent:
			// Skip any ephemeral or debug messages, and elide all colorization.
			p := e.Payload().(engine.DiagEventPayload)
			if !p.Ephemeral && p.Severity != diag.Debug {
				digest.Diagnostics = append(digest.Diagnostics, previewDiagnostic{
					URN:      p.URN,
					Message:  colors.Never.Colorize(p.Prefix + p.Message),
					Severity: p.Severity,
				})
			}
		case engine.StdoutColorEvent:
			// Append stdout events as informational messages, and elide all colorization.
			p := e.Payload().(engine.StdoutEventPayload)
			digest.Diagnostics = append(digest.Diagnostics, previewDiagnostic{
				Message:  colors.Never.Colorize(p.Message),
				Severity: diag.Info,
			})
		case engine.ResourcePreEvent:
			// Create the detailed metadata for this step and the initial state of its resource. Later,
			// if new outputs arrive, we'll search for and swap in those new values.
			if m := e.Payload().(engine.ResourcePreEventPayload).Metadata; shouldShow(m, opts) || isRootStack(m) {
				var detailedDiff map[string]propertyDiff
				if m.DetailedDiff != nil {
					detailedDiff = make(map[string]propertyDiff)
					for k, v := range m.DetailedDiff {
						detailedDiff[k] = propertyDiff{
							Kind:      v.Kind.String(),
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
					res, err := stack.SerializeResource(oldState, config.NewPanicCrypter(), false /* showSecrets */)
					if err == nil {
						step.OldState = &res
					} else {
						logging.V(7).Infof("not adding old state as there was an error serialzing: %s", err)
					}
				}
				if m.New != nil {
					newState := stateForJSONOutput(m.New.State, opts)
					res, err := stack.SerializeResource(newState, config.NewPanicCrypter(), false /* showSecrets */)
					if err == nil {
						step.NewState = &res
					} else {
						logging.V(7).Infof("not adding new state as there was an error serialzing: %s", err)
					}
				}

				digest.Steps = append(digest.Steps, step)
			}
		case engine.ResourceOutputsEvent, engine.ResourceOperationFailed:
			// Because we are only JSON serializing previews, we don't need to worry about outputs
			// resolving or operations failing. In the future, if we serialize actual deployments, we will
			// need to come up with a scheme for matching the failure to the associated step.

		// Events ocurring late:
		case engine.PolicyViolationEvent:
			// At this point in time, we don't handle policy events in JSON serialization
			continue
		case engine.SummaryEvent:
			// At the end of the preview, a summary event indicates the final conclusions.
			p := e.Payload().(engine.SummaryEventPayload)
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
	ChangeSummary engine.ResourceChanges `json:"changeSummary,omitempty"`
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
	Op deploy.StepOp `json:"op"`
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
	URN      resource.URN  `json:"urn,omitempty"`
	Prefix   string        `json:"prefix,omitempty"`
	Message  string        `json:"message,omitempty"`
	Severity diag.Severity `json:"severity,omitempty"`
}
