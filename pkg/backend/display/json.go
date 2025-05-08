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
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
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
		s.ImportID, s.RetainOnDelete, s.DeletedWith, s.Created, s.Modified, s.SourcePosition, s.IgnoreChanges,
		s.ReplaceOnChanges, s.ViewOf)
}

// ShowJSONEvents renders incremental engine events to stdout.
func ShowJSONEvents(events <-chan engine.StampedEvent, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	for e := range events {
		if err := logJSONEvent(encoder, e, opts); err != nil {
			logging.V(7).Infof("failed to log event: %v", err)
		}

		// In the event of cancellation, break out of the loop.
		if e.Type == engine.CancelEvent {
			break
		}
	}
}

// ShowPreviewDigest renders engine events from a preview into a well-formed JSON document. Note that this does not
// emit events incrementally so that it can guarantee anything emitted to stdout is well-formed. This means that,
// if used interactively, the experience will lead to potentially very long pauses. If run in CI, it is up to the
// end user to ensure that output is periodically printed to prevent tools from thinking preview has hung.
func ShowPreviewDigest(events <-chan engine.Event, done chan<- bool, opts Options) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()

	seen := make(map[resource.URN]engine.StepEventMetadata)

	// Now loop and accumulate our digest until the event stream is closed, or we hit a cancellation.
	var digest display.PreviewDigest
	for e := range events {
		// In the event of cancellation, break out of the loop immediately.
		if e.Type == engine.CancelEvent {
			break
		}

		// For all other events, use the payload to build up the JSON digest we'll emit later.
		switch e.Type {
		case engine.CancelEvent:
			// Pacify the linter here, this is already handled beforehand
		// Events occurring early:
		case engine.PreludeEvent:
			// Capture the config map from the prelude. Note that all secrets will remain blinded for safety.
			digest.Config = e.Payload().(engine.PreludeEventPayload).Config

		// Events throughout the execution:
		case engine.DiagEvent:
			// Skip any ephemeral or debug messages, and elide all colorization.
			p := e.Payload().(engine.DiagEventPayload)
			if !p.Ephemeral && p.Severity != diag.Debug {
				digest.Diagnostics = append(digest.Diagnostics, display.PreviewDiagnostic{
					URN:      p.URN,
					Message:  colors.Never.Colorize(p.Prefix + p.Message),
					Severity: p.Severity,
				})
			}
		case engine.StartDebuggingEvent:
			// We don't want to display debugging events in the JSON output.
			continue

		case engine.StdoutColorEvent:
			// Append stdout events as informational messages, and elide all colorization.
			p := e.Payload().(engine.StdoutEventPayload)
			digest.Diagnostics = append(digest.Diagnostics, display.PreviewDiagnostic{
				Message:  colors.Never.Colorize(p.Message),
				Severity: diag.Info,
			})
		case engine.ResourcePreEvent:
			// Create the detailed metadata for this step and the initial state of its
			// resource. Later, if new outputs arrive that we want to incorporate,
			// we'll search for and swap in those new values.
			if m := e.Payload().(engine.ResourcePreEventPayload).Metadata; shouldShow(m, opts) || isRootStack(m) {
				seen[m.URN] = m
				step := getPreviewMetadataStep(m, opts)
				digest.Steps = append(digest.Steps, step)
			}
		case engine.ResourceOutputsEvent:
			// When performing JSON serialisation, we want to include outputs that
			// occur as the results of a refresh operation. When the output event
			// arrives, these will have been rewritten to be updates or deletes, so we
			// use the `seen` map to confirm the original operation.
			if m := e.Payload().(engine.ResourceOutputsEventPayload).Metadata; shouldShow(m, opts) || isRootStack(m) {
				refresh := false
				if preM, has := seen[m.URN]; has && preM.Op == deploy.OpRefresh {
					refresh = true
				}

				if refresh && ((m.Op == deploy.OpUpdate && m.DetailedDiff != nil) || m.Op == deploy.OpDelete) {
					step := getPreviewMetadataStep(m, opts)
					for i, s := range digest.Steps {
						if s.URN == m.URN {
							digest.Steps[i] = step
						}
					}
				}
			}
		case engine.ResourceOperationFailed:
			// Because we are only JSON serializing previews, we don't need to worry
			// about operations failing.
			continue

		// Events occurring late:
		case engine.PolicyViolationEvent, engine.PolicyLoadEvent, engine.PolicyRemediationEvent:
			// At this point in time, we don't handle policy events in JSON serialization
			continue
		case engine.SummaryEvent:
			// At the end of the preview, a summary event indicates the final conclusions.
			p := e.Payload().(engine.SummaryEventPayload)
			digest.Duration = p.Duration
			digest.ChangeSummary = p.ResourceChanges
			digest.MaybeCorrupt = p.MaybeCorrupt
		case engine.ProgressEvent:
			// Progress events are ephemeral and should be skipped.
			continue
		default:
			contract.Failf("unknown event type '%s'", e.Type)
		}
	}
	// Finally, go ahead and render the JSON to stdout.
	out, err := json.MarshalIndent(&digest, "", "    ")
	contract.Assertf(err == nil, "unexpected JSON error: %v", err)
	fmt.Println(string(out))
}

// getPreviewMetadataStep constructs a preview step that can be rendered to JSON
// from the given metadata and options.
func getPreviewMetadataStep(
	m engine.StepEventMetadata,
	opts Options,
) *display.PreviewStep {
	var detailedDiff map[string]display.PropertyDiff
	if m.DetailedDiff != nil {
		detailedDiff = make(map[string]display.PropertyDiff)
		for k, v := range m.DetailedDiff {
			detailedDiff[k] = display.PropertyDiff{
				Kind:      v.Kind.String(),
				InputDiff: v.InputDiff,
			}
		}
	}

	step := &display.PreviewStep{
		Op:             m.Op,
		URN:            m.URN,
		Provider:       m.Provider,
		DiffReasons:    m.Diffs,
		ReplaceReasons: m.Keys,
		DetailedDiff:   detailedDiff,
	}

	ctx := context.TODO()
	if m.Old != nil {
		oldState := stateForJSONOutput(m.Old.State, opts)
		res, err := stack.SerializeResource(ctx, oldState, config.NewPanicCrypter(), false /* showSecrets */)
		if err == nil {
			step.OldState = &res
		} else {
			logging.V(7).Infof("not adding old state as there was an error serializing: %s", err)
		}
	}
	if m.New != nil {
		newState := stateForJSONOutput(m.New.State, opts)
		res, err := stack.SerializeResource(ctx, newState, config.NewPanicCrypter(), false /* showSecrets */)
		if err == nil {
			step.NewState = &res
		} else {
			logging.V(7).Infof("not adding new state as there was an error serializing: %s", err)
		}
	}

	return step
}
