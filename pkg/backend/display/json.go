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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"os"
	"time"
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

type JSONEvent struct {
	// Timestamp is a Unix timestamp (seconds) of when the event was emitted.
	Timestamp int `json:"timestamp"`

	StdoutEvent      *apitype.StdoutEngineEvent `json:"stdoutEvent,omitempty"`
	DiagnosticEvent  *apitype.DiagnosticEvent   `json:"diagnosticEvent,omitempty"`
	PreludeEvent     *apitype.PreludeEvent      `json:"preludeEvent,omitempty"`
	SummaryEvent     *apitype.SummaryEvent      `json:"summaryEvent,omitempty"`
	ResourcePreEvent *apitype.ResourcePreEvent  `json:"resourcePreEvent,omitempty"`
	ResOutputsEvent  *apitype.ResOutputsEvent   `json:"resOutputsEvent,omitempty"`
	ResOpFailedEvent *apitype.ResOpFailedEvent  `json:"resOpFailedEvent,omitempty"`
	PolicyEvent      *apitype.PolicyEvent       `json:"policyEvent,omitempty"`
}

// ShowJSONEvents renders incremental engine events to stdout.
func ShowJSONEvents(events <-chan engine.Event, done chan<- bool) {
	// Ensure we close the done channel before exiting.
	defer func() { close(done) }()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	logEvent := func(e engine.Event) error {
		apiEvent, err := ConvertEngineEvent(e)
		if err != nil {
			return err
		}
		jsonEvent := JSONEvent{
			Timestamp:        int(time.Now().Unix()),
			StdoutEvent:      apiEvent.StdoutEvent,
			DiagnosticEvent:  apiEvent.DiagnosticEvent,
			PreludeEvent:     apiEvent.PreludeEvent,
			SummaryEvent:     apiEvent.SummaryEvent,
			ResourcePreEvent: apiEvent.ResourcePreEvent,
			ResOutputsEvent:  apiEvent.ResOutputsEvent,
			ResOpFailedEvent: apiEvent.ResOpFailedEvent,
			PolicyEvent:      apiEvent.PolicyEvent,
		}

		// If this is a diagnostic event, clean up the terminal color characters from the emitted log.
		if jsonEvent.DiagnosticEvent != nil {
			jsonEvent.DiagnosticEvent.Message = cleanColorRenderingChars(jsonEvent.DiagnosticEvent.Message)
			jsonEvent.DiagnosticEvent.Prefix = cleanColorRenderingChars(jsonEvent.DiagnosticEvent.Prefix)
		}

		return encoder.Encode(jsonEvent)
	}

	for e := range events {
		// In the event of cancellation, break out of the loop immediately.
		if e.Type == engine.CancelEvent {
			break
		}

		if err := logEvent(e); err != nil {
			logging.V(7).Infof("failed to log event: %v", err)
		}
	}
}
