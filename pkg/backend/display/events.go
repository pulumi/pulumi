package display

import (
	"errors"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	stack_events "github.com/pulumi/pulumi/pkg/v3/resource/stack/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ConvertEngineEvent converts a raw events.Event into an apitype.EngineEvent used in the Pulumi
// REST API. Returns an error if the engine event is unknown or not in an expected format.
// EngineEvent.{ Sequence, Timestamp } are expected to be set by the caller.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertEngineEvent(e events.Event) (apitype.EngineEvent, error) {
	var apiEvent apitype.EngineEvent

	// Error to return if the payload doesn't match expected.
	eventTypePayloadMismatch := fmt.Errorf("unexpected payload for event type %v", e.Type)

	switch e.Type {
	case events.CancelEvent:
		apiEvent.CancelEvent = &apitype.CancelEvent{}

	case events.StdoutColorEvent:
		p, ok := e.Payload().(events.StdoutEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.StdoutEvent = &apitype.StdoutEngineEvent{
			Message: p.Message,
			Color:   string(p.Color),
		}

	case events.DiagEvent:
		p, ok := e.Payload().(events.DiagEventPayload)
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

	case events.PolicyViolationEvent:
		p, ok := e.Payload().(events.PolicyViolationEventPayload)
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

	case events.PreludeEvent:
		p, ok := e.Payload().(events.PreludeEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.PreludeEvent = &apitype.PreludeEvent{
			Config: p.Config,
		}

	case events.SummaryEvent:
		p, ok := e.Payload().(events.SummaryEventPayload)
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

	case events.ResourcePreEvent:
		p, ok := e.Payload().(events.ResourcePreEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResourcePreEvent = &apitype.ResourcePreEvent{
			Metadata: convertStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		}

	case events.ResourceOutputsEvent:
		p, ok := e.Payload().(events.ResourceOutputsEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOutputsEvent = &apitype.ResOutputsEvent{
			Metadata: convertStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		}

	case events.ResourceOperationFailed:
		p, ok := e.Payload().(events.ResourceOperationFailedPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOpFailedEvent = &apitype.ResOpFailedEvent{
			Metadata: convertStepEventMetadata(p.Metadata),
			Status:   int(p.Status),
			Steps:    p.Steps,
		}

	default:
		return apiEvent, fmt.Errorf("unknown event type %q", e.Type)
	}

	return apiEvent, nil
}

func convertStepEventMetadata(md events.StepEventMetadata) apitype.StepEventMetadata {
	keys := make([]string, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = string(v)
	}
	var diffs []string
	for _, v := range md.Diffs {
		diffs = append(diffs, string(v))
	}

	return apitype.StepEventMetadata{
		Op:   apitype.OpType(md.Op),
		URN:  string(md.URN),
		Type: string(md.Type),

		Old: convertStepEventStateMetadata(md.Old),
		New: convertStepEventStateMetadata(md.New),

		Keys:         keys,
		Diffs:        diffs,
		DetailedDiff: md.DetailedDiff,
		Logical:      md.Logical,
		Provider:     md.Provider,
	}
}

// convertStepEventStateMetadata converts the internal StepEventStateMetadata to the API type
// we send over the wire.
//
// IMPORTANT: Any secret values are encrypted using the blinding encrypter. So any secret data
// in the resource state will be lost and unrecoverable.
func convertStepEventStateMetadata(md *events.StepEventStateMetadata) *apitype.StepEventStateMetadata {
	if md == nil {
		return nil
	}

	encrypter := config.BlindingCrypter
	inputs, err := stack_events.SerializeProperties(md.Inputs, encrypter, false /* showSecrets */)
	contract.IgnoreError(err)

	outputs, err := stack_events.SerializeProperties(md.Outputs, encrypter, false /* showSecrets */)
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

// ConvertJSONEvent converts an apitype.EngineEvent from the Pulumi REST API into a raw engine.Event
// Returns an error if the engine event is unknown or not in an expected format.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertJSONEvent(apiEvent apitype.EngineEvent) (events.Event, error) {
	var event events.Event

	switch {
	case apiEvent.CancelEvent != nil:
		event = events.NewEvent(events.CancelEvent, nil)

	case apiEvent.StdoutEvent != nil:
		p := apiEvent.StdoutEvent
		event = events.NewEvent(events.StdoutColorEvent, events.StdoutEventPayload{
			Message: p.Message,
			Color:   colors.Colorization(p.Color),
		})

	case apiEvent.DiagnosticEvent != nil:
		p := apiEvent.DiagnosticEvent
		event = events.NewEvent(events.DiagEvent, events.DiagEventPayload{
			URN:       resource.URN(p.URN),
			Prefix:    p.Prefix,
			Message:   p.Message,
			Color:     colors.Colorization(p.Color),
			Severity:  p.Severity,
			Ephemeral: p.Ephemeral,
		})
		apiEvent.DiagnosticEvent = &apitype.DiagnosticEvent{}

	case apiEvent.PolicyEvent != nil:
		p := apiEvent.PolicyEvent
		event = events.NewEvent(events.PolicyViolationEvent, events.PolicyViolationEventPayload{
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
		event = events.NewEvent(events.PreludeEvent, events.PreludeEventPayload{
			Config: p.Config,
		})

	case apiEvent.SummaryEvent != nil:
		p := apiEvent.SummaryEvent
		// Convert the resource changes.
		changes := events.ResourceChanges{}
		for op, count := range p.ResourceChanges {
			changes[events.StepOp(op)] = count
		}
		event = events.NewEvent(events.SummaryEvent, events.SummaryEventPayload{
			MaybeCorrupt:    p.MaybeCorrupt,
			Duration:        time.Duration(p.DurationSeconds) * time.Second,
			ResourceChanges: changes,
			PolicyPacks:     p.PolicyPacks,
		})

	case apiEvent.ResourcePreEvent != nil:
		p := apiEvent.ResourcePreEvent
		event = events.NewEvent(events.ResourcePreEvent, events.ResourcePreEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOutputsEvent != nil:
		p := apiEvent.ResOutputsEvent
		event = events.NewEvent(events.ResourceOutputsEvent, events.ResourceOutputsEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOpFailedEvent != nil:
		p := apiEvent.ResOpFailedEvent
		event = events.NewEvent(events.ResourceOperationFailed, events.ResourceOperationFailedPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Status:   resource.Status(p.Status),
			Steps:    p.Steps,
		})

	default:
		return event, errors.New("unknown event type")
	}

	return event, nil
}

func convertJSONStepEventMetadata(md apitype.StepEventMetadata) events.StepEventMetadata {
	keys := make([]resource.PropertyKey, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = resource.PropertyKey(v)
	}
	var diffs []resource.PropertyKey
	for _, v := range md.Diffs {
		diffs = append(diffs, resource.PropertyKey(v))
	}

	return events.StepEventMetadata{
		Op:   events.StepOp(md.Op),
		URN:  resource.URN(md.URN),
		Type: tokens.Type(md.Type),

		Old: convertJSONStepEventStateMetadata(md.Old),
		New: convertJSONStepEventStateMetadata(md.New),

		Keys:         keys,
		Diffs:        diffs,
		DetailedDiff: md.DetailedDiff,
		Logical:      md.Logical,
		Provider:     md.Provider,
	}
}

// convertJSONStepEventStateMetadata converts the internal StepEventStateMetadata to the API type
// we send over the wire.
//
// IMPORTANT: Any secret values are encrypted using the blinding encrypter. So any secret data
// in the resource state will be lost and unrecoverable.
func convertJSONStepEventStateMetadata(md *apitype.StepEventStateMetadata) *events.StepEventStateMetadata {
	if md == nil {
		return nil
	}

	crypter := config.BlindingCrypter
	inputs, err := stack_events.DeserializeProperties(md.Inputs, crypter, crypter)
	contract.IgnoreError(err)

	outputs, err := stack_events.DeserializeProperties(md.Outputs, crypter, crypter)
	contract.IgnoreError(err)

	return &events.StepEventStateMetadata{
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
