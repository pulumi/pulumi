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
	"errors"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
)

// ConvertEngineEvent converts a raw Event into an apitype.EngineEvent used in the Pulumi
// REST API. Returns an error if the engine event is unknown or not in an expected format.
// EngineEvent.{ Sequence, Timestamp } are expected to be set by the caller.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertEngineEvent(e Event, showSecrets bool) (apitype.EngineEvent, error) {
	var apiEvent apitype.EngineEvent

	// Error to return if the payload doesn't match expected.
	eventTypePayloadMismatch := fmt.Errorf("unexpected payload for event type %v", e.Type)

	switch e.Type {
	case CancelEvent:
		apiEvent.CancelEvent = &apitype.CancelEvent{}

	case StdoutColorEvent:
		p, ok := e.Payload().(StdoutEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.StdoutEvent = &apitype.StdoutEngineEvent{
			Message: p.Message,
			Color:   string(p.Color),
		}

	case DiagEvent:
		p, ok := e.Payload().(DiagEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.DiagnosticEvent = &apitype.DiagnosticEvent{
			URN:       string(p.URN),
			Prefix:    p.Prefix,
			Message:   p.Message,
			Color:     string(p.Color),
			Severity:  string(p.Severity),
			Ephemeral: p.Ephemeral,
		}

	case PolicyViolationEvent:
		p, ok := e.Payload().(PolicyViolationEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.PolicyEvent = &apitype.PolicyEvent{
			ResourceURN:          string(p.ResourceURN),
			Message:              p.Message,
			Color:                string(p.Color),
			PolicyName:           p.PolicyName,
			PolicyPackName:       p.PolicyPackName,
			PolicyPackVersion:    p.PolicyPackVersion,
			PolicyPackVersionTag: p.PolicyPackVersion,
			EnforcementLevel:     string(p.EnforcementLevel),
		}

	case PreludeEvent:
		p, ok := e.Payload().(PreludeEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		// Convert the config bag.
		cfg := make(map[string]string)
		for k, v := range p.Config {
			cfg[k] = v
		}
		apiEvent.PreludeEvent = &apitype.PreludeEvent{
			Config: cfg,
		}

	case SummaryEvent:
		p, ok := e.Payload().(SummaryEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		// Convert the resource changes.
		changes := make(map[apitype.OpType]int)
		for op, count := range p.ResourceChanges {
			changes[apitype.OpType(op)] = count
		}
		apiEvent.SummaryEvent = &apitype.SummaryEvent{
			MaybeCorrupt:    p.MaybeCorrupt,
			DurationSeconds: int(p.Duration.Seconds()),
			ResourceChanges: changes,
			PolicyPacks:     p.PolicyPacks,
		}

	case ResourcePreEvent:
		p, ok := e.Payload().(ResourcePreEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResourcePreEvent = &apitype.ResourcePreEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Planning: p.Planning,
		}

	case ResourceOutputsEvent:
		p, ok := e.Payload().(ResourceOutputsEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOutputsEvent = &apitype.ResOutputsEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Planning: p.Planning,
		}

	case ResourceOperationFailed:
		p, ok := e.Payload().(ResourceOperationFailedPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOpFailedEvent = &apitype.ResOpFailedEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Status:   int(p.Status),
			Steps:    p.Steps,
		}

	default:
		return apiEvent, fmt.Errorf("unknown event type %q", e.Type)
	}

	return apiEvent, nil
}

func convertStepEventMetadata(md StepEventMetadata, showSecrets bool) apitype.StepEventMetadata {
	keys := make([]string, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = string(v)
	}
	var diffs []string
	for _, v := range md.Diffs {
		diffs = append(diffs, string(v))
	}
	var detailedDiff map[string]apitype.PropertyDiff
	if md.DetailedDiff != nil {
		detailedDiff = make(map[string]apitype.PropertyDiff)
		for k, v := range md.DetailedDiff {
			var d apitype.DiffKind
			switch v.Kind {
			case plugin.DiffAdd:
				d = apitype.DiffAdd
			case plugin.DiffAddReplace:
				d = apitype.DiffAddReplace
			case plugin.DiffDelete:
				d = apitype.DiffDelete
			case plugin.DiffDeleteReplace:
				d = apitype.DiffDeleteReplace
			case plugin.DiffUpdate:
				d = apitype.DiffUpdate
			case plugin.DiffUpdateReplace:
				d = apitype.DiffUpdateReplace
			default:
				contract.Failf("unrecognized diff kind %v", v)
			}
			detailedDiff[k] = apitype.PropertyDiff{
				Kind:      d,
				InputDiff: v.InputDiff,
			}
		}
	}

	return apitype.StepEventMetadata{
		Op:   apitype.OpType(md.Op),
		URN:  string(md.URN),
		Type: string(md.Type),

		Old: convertStepEventStateMetadata(md.Old, showSecrets),
		New: convertStepEventStateMetadata(md.New, showSecrets),

		Keys:         keys,
		Diffs:        diffs,
		DetailedDiff: detailedDiff,
		Logical:      md.Logical,
		Provider:     md.Provider,
	}
}

// convertStepEventStateMetadata converts the internal StepEventStateMetadata to the API type
// we send over the wire.
//
// IMPORTANT: Any secret values are encrypted using the blinding encrypter. So any secret data
// in the resource state will be lost and unrecoverable.
func convertStepEventStateMetadata(md *StepEventStateMetadata,
	showSecrets bool) *apitype.StepEventStateMetadata {

	if md == nil {
		return nil
	}

	//encrypter := config.BlindingCrypter
	//inputs, err := stack.SerializeProperties(md.Inputs, encrypter, showSecrets)
	var inputs map[string]interface{}
	err := fmt.Errorf("SerializeProperties unimplemented")
	contract.IgnoreError(err)

	//outputs, err := stack.SerializeProperties(md.Outputs, encrypter, showSecrets)
	var outputs map[string]interface{}
	err = fmt.Errorf("SerializeProperties unimplemented")
	contract.IgnoreError(err)

	return &apitype.StepEventStateMetadata{
		Type: string(md.Type),
		URN:  string(md.URN),

		Custom:     md.Custom,
		Delete:     md.Delete,
		ID:         string(md.ID),
		Parent:     string(md.Parent),
		Protect:    md.Protect,
		Inputs:     inputs,
		Outputs:    outputs,
		InitErrors: md.InitErrors,
	}
}

// ConvertJSONEvent converts an apitype.EngineEvent from the Pulumi REST API into a raw Event
// Returns an error if the engine event is unknown or not in an expected format.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertJSONEvent(apiEvent apitype.EngineEvent) (Event, error) {
	var event Event

	switch {
	case apiEvent.CancelEvent != nil:
		event = NewEvent(CancelEvent, nil)

	case apiEvent.StdoutEvent != nil:
		p := apiEvent.StdoutEvent
		event = NewEvent(StdoutColorEvent, StdoutEventPayload{
			Message: p.Message,
			Color:   colors.Colorization(p.Color),
		})

	case apiEvent.DiagnosticEvent != nil:
		p := apiEvent.DiagnosticEvent
		event = NewEvent(DiagEvent, DiagEventPayload{
			URN:       resource.URN(p.URN),
			Prefix:    p.Prefix,
			Message:   p.Message,
			Color:     colors.Colorization(p.Color),
			Severity:  diag.Severity(p.Severity),
			Ephemeral: p.Ephemeral,
		})
		apiEvent.DiagnosticEvent = &apitype.DiagnosticEvent{}

	case apiEvent.PolicyEvent != nil:
		p := apiEvent.PolicyEvent
		event = NewEvent(PolicyViolationEvent, PolicyViolationEventPayload{
			ResourceURN:       resource.URN(p.ResourceURN),
			Message:           p.Message,
			Color:             colors.Colorization(p.Color),
			PolicyName:        p.PolicyName,
			PolicyPackName:    p.PolicyPackName,
			PolicyPackVersion: p.PolicyPackVersion,
			EnforcementLevel:  apitype.EnforcementLevel(p.EnforcementLevel),
		})

	case apiEvent.PreludeEvent != nil:
		p := apiEvent.PreludeEvent

		// Convert the config bag.
		event = NewEvent(PreludeEvent, PreludeEventPayload{
			Config: p.Config,
		})

	case apiEvent.SummaryEvent != nil:
		p := apiEvent.SummaryEvent
		// Convert the resource changes.
		changes := ResourceChanges{}
		for op, count := range p.ResourceChanges {
			changes[StepOp(op)] = count
		}
		event = NewEvent(SummaryEvent, SummaryEventPayload{
			MaybeCorrupt:    p.MaybeCorrupt,
			Duration:        time.Duration(p.DurationSeconds) * time.Second,
			ResourceChanges: changes,
			PolicyPacks:     p.PolicyPacks,
		})

	case apiEvent.ResourcePreEvent != nil:
		p := apiEvent.ResourcePreEvent
		event = NewEvent(ResourcePreEvent, ResourcePreEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOutputsEvent != nil:
		p := apiEvent.ResOutputsEvent
		event = NewEvent(ResourceOutputsEvent, ResourceOutputsEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOpFailedEvent != nil:
		p := apiEvent.ResOpFailedEvent
		event = NewEvent(ResourceOperationFailed, ResourceOperationFailedPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Status:   resource.Status(p.Status),
			Steps:    p.Steps,
		})

	default:
		return event, errors.New("unknown event type")
	}

	return event, nil
}

func convertJSONStepEventMetadata(md apitype.StepEventMetadata) StepEventMetadata {
	keys := make([]resource.PropertyKey, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = resource.PropertyKey(v)
	}
	var diffs []resource.PropertyKey
	for _, v := range md.Diffs {
		diffs = append(diffs, resource.PropertyKey(v))
	}
	var detailedDiff map[string]plugin.PropertyDiff
	if md.DetailedDiff != nil {
		detailedDiff = make(map[string]plugin.PropertyDiff)
		for k, v := range md.DetailedDiff {
			var d plugin.DiffKind
			switch v.Kind {
			case apitype.DiffAdd:
				d = plugin.DiffAdd
			case apitype.DiffAddReplace:
				d = plugin.DiffAddReplace
			case apitype.DiffDelete:
				d = plugin.DiffDelete
			case apitype.DiffDeleteReplace:
				d = plugin.DiffDeleteReplace
			case apitype.DiffUpdate:
				d = plugin.DiffUpdate
			case apitype.DiffUpdateReplace:
				d = plugin.DiffUpdateReplace
			default:
				contract.Failf("unrecognized diff kind %v", v)
			}
			detailedDiff[k] = plugin.PropertyDiff{
				Kind:      d,
				InputDiff: v.InputDiff,
			}
		}
	}

	old, new := convertJSONStepEventStateMetadata(md.Old), convertJSONStepEventStateMetadata(md.New)

	res := old
	if new != nil {
		res = new
	}

	return StepEventMetadata{
		Op:   StepOp(md.Op),
		URN:  resource.URN(md.URN),
		Type: tokens.Type(md.Type),

		Old: old,
		New: new,
		Res: res,

		Keys:         keys,
		Diffs:        diffs,
		DetailedDiff: detailedDiff,
		Logical:      md.Logical,
		Provider:     md.Provider,
	}
}

// convertJSONStepEventStateMetadata converts the internal StepEventStateMetadata to the API type
// we send over the wire.
//
// IMPORTANT: Any secret values are encrypted using the blinding encrypter. So any secret data
// in the resource state will be lost and unrecoverable.
func convertJSONStepEventStateMetadata(md *apitype.StepEventStateMetadata) *StepEventStateMetadata {
	if md == nil {
		return nil
	}

	var inputs resource.PropertyMap
	var outputs resource.PropertyMap
	//crypter := config.BlindingCrypter
	//inputs, err := resource.DeserializeProperties(md.Inputs, crypter, crypter)
	//contract.IgnoreError(err)

	//outputs, err := resource.DeserializeProperties(md.Outputs, crypter, crypter)
	//contract.IgnoreError(err)

	return &StepEventStateMetadata{
		Type: tokens.Type(md.Type),
		URN:  resource.URN(md.URN),

		Custom:     md.Custom,
		Delete:     md.Delete,
		ID:         resource.ID(md.ID),
		Parent:     resource.URN(md.Parent),
		Protect:    md.Protect,
		Inputs:     inputs,
		Outputs:    outputs,
		InitErrors: md.InitErrors,
	}
}

// Event represents an event generated by the engine during an operation. The underlying
// type for the `Payload` field will differ depending on the value of the `Type` field
type Event struct {
	Type    EventType
	payload interface{}
}

func NewEvent(typ EventType, payload interface{}) Event {
	ok := false
	switch typ {
	case CancelEvent:
		ok = payload == nil
	case StdoutColorEvent:
		_, ok = payload.(StdoutEventPayload)
	case DiagEvent:
		_, ok = payload.(DiagEventPayload)
	case PreludeEvent:
		_, ok = payload.(PreludeEventPayload)
	case SummaryEvent:
		_, ok = payload.(SummaryEventPayload)
	case ResourcePreEvent:
		_, ok = payload.(ResourcePreEventPayload)
	case ResourceOutputsEvent:
		_, ok = payload.(ResourceOutputsEventPayload)
	case ResourceOperationFailed:
		_, ok = payload.(ResourceOperationFailedPayload)
	case PolicyViolationEvent:
		_, ok = payload.(PolicyViolationEventPayload)
	default:
		contract.Failf("unknown event type %v", typ)
	}
	contract.Assertf(ok, "invalid payload of type %T for event type %v", payload, typ)
	return Event{
		Type:    typ,
		payload: payload,
	}
}

// EventType is the kind of event being emitted.
type EventType string

const (
	CancelEvent             EventType = "cancel"
	StdoutColorEvent        EventType = "stdoutcolor"
	DiagEvent               EventType = "diag"
	PreludeEvent            EventType = "prelude"
	SummaryEvent            EventType = "summary"
	ResourcePreEvent        EventType = "resource-pre"
	ResourceOutputsEvent    EventType = "resource-outputs"
	ResourceOperationFailed EventType = "resource-operationfailed"
	PolicyViolationEvent    EventType = "policy-violation"
)

func (e Event) Payload() interface{} {
	return deepcopy.Copy(e.payload)
}

func NewCancelEvent() Event {
	return Event{Type: CancelEvent}
}

// DiagEventPayload is the payload for an event with type `diag`
type DiagEventPayload struct {
	URN       resource.URN
	Prefix    string
	Message   string
	Color     colors.Colorization
	Severity  diag.Severity
	StreamID  int32
	Ephemeral bool
}

// PolicyViolationEventPayload is the payload for an event with type `policy-violation`.
type PolicyViolationEventPayload struct {
	ResourceURN       resource.URN
	Message           string
	Color             colors.Colorization
	PolicyName        string
	PolicyPackName    string
	PolicyPackVersion string
	EnforcementLevel  apitype.EnforcementLevel
	Prefix            string
}

type StdoutEventPayload struct {
	Message string
	Color   colors.Colorization
}

type PreludeEventPayload struct {
	IsPreview bool              // true if this prelude is for a plan operation
	Config    map[string]string // the keys and values for config. For encrypted config, the values may be blinded
}

type SummaryEventPayload struct {
	IsPreview       bool              // true if this summary is for a plan operation
	MaybeCorrupt    bool              // true if one or more resources may be corrupt
	Duration        time.Duration     // the duration of the entire update operation (zero values for previews)
	ResourceChanges ResourceChanges   // count of changed resources, useful for reporting
	PolicyPacks     map[string]string // {policy-pack: version} for each policy pack applied
}

type ResourceOperationFailedPayload struct {
	Metadata StepEventMetadata
	Status   resource.Status
	Steps    int
}

type ResourceOutputsEventPayload struct {
	Metadata StepEventMetadata
	Planning bool
	Debug    bool
}

type ResourcePreEventPayload struct {
	Metadata StepEventMetadata
	Planning bool
	Debug    bool
}

// StepEventMetadata contains the metadata associated with a step the engine is performing.
type StepEventMetadata struct {
	Op           StepOp                         // the operation performed by this step.
	URN          resource.URN                   // the resource URN (for before and after).
	Type         tokens.Type                    // the type affected by this step.
	Old          *StepEventStateMetadata        // the state of the resource before performing this step.
	New          *StepEventStateMetadata        // the state of the resource after performing this step.
	Res          *StepEventStateMetadata        // the latest state for the resource that is known (worst case, old).
	Keys         []resource.PropertyKey         // the keys causing replacement (only for CreateStep and ReplaceStep).
	Diffs        []resource.PropertyKey         // the keys causing diffs
	DetailedDiff map[string]plugin.PropertyDiff // the rich, structured diff
	Logical      bool                           // true if this step represents a logical operation in the program.
	Provider     string                         // the provider that performed this step.
}

// StepEventStateMetadata contains detailed metadata about a resource's state pertaining to a given step.
type StepEventStateMetadata struct {
	// State contains the raw, complete state, for this resource.
	State *resource.State
	// the resource's type.
	Type tokens.Type
	// the resource's object urn, a human-friendly, unique name for the resource.
	URN resource.URN
	// true if the resource is custom, managed by a plugin.
	Custom bool
	// true if this resource is pending deletion due to a replacement.
	Delete bool
	// the resource's unique ID, assigned by the resource provider (or blank if none/uncreated).
	ID resource.ID
	// an optional parent URN that this resource belongs to.
	Parent resource.URN
	// true to "protect" this resource (protected resources cannot be deleted).
	Protect bool
	// the resource's input properties (as specified by the program). Note: because this will cross
	// over rpc boundaries it will be slightly different than the Inputs found in resource_state.
	// Specifically, secrets will have been filtered out, and large values (like assets) will be
	// have a simple hash-based representation.  This allows clients to display this information
	// properly, without worrying about leaking sensitive data, and without having to transmit huge
	// amounts of data.
	Inputs resource.PropertyMap
	// the resource's complete output state (as returned by the resource provider).  See "Inputs"
	// for additional details about how data will be transformed before going into this map.
	Outputs resource.PropertyMap
	// the resource's provider reference
	Provider string
	// InitErrors is the set of errors encountered in the process of initializing resource (i.e.,
	// during create or update).
	InitErrors []string
}
