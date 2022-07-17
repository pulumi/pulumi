package display

import (
	"errors"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	sdkDisplay "github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// ConvertEngineEvent converts a raw sdkDisplay.Event into an apitype.EngineEvent used in the Pulumi
// REST API. Returns an error if the engine event is unknown or not in an expected format.
// EngineEvent.{ Sequence, Timestamp } are expected to be set by the caller.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertEngineEvent(e sdkDisplay.Event, showSecrets bool) (apitype.EngineEvent, error) {
	var apiEvent apitype.EngineEvent

	// Error to return if the payload doesn't match expected.
	eventTypePayloadMismatch := fmt.Errorf("unexpected payload for event type %v", e.Type)

	switch e.Type {
	case sdkDisplay.CancelEvent:
		apiEvent.CancelEvent = &apitype.CancelEvent{}

	case sdkDisplay.StdoutColorEvent:
		p, ok := e.Payload().(sdkDisplay.StdoutEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.StdoutEvent = &apitype.StdoutEngineEvent{
			Message: p.Message,
			Color:   string(p.Color),
		}

	case sdkDisplay.DiagEvent:
		p, ok := e.Payload().(sdkDisplay.DiagEventPayload)
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

	case sdkDisplay.PolicyViolationEvent:
		p, ok := e.Payload().(sdkDisplay.PolicyViolationEventPayload)
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

	case sdkDisplay.PreludeEvent:
		p, ok := e.Payload().(sdkDisplay.PreludeEventPayload)
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

	case sdkDisplay.SummaryEvent:
		p, ok := e.Payload().(sdkDisplay.SummaryEventPayload)
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

	case sdkDisplay.ResourcePreEvent:
		p, ok := e.Payload().(sdkDisplay.ResourcePreEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResourcePreEvent = &apitype.ResourcePreEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Planning: p.Planning,
		}

	case sdkDisplay.ResourceOutputsEvent:
		p, ok := e.Payload().(sdkDisplay.ResourceOutputsEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOutputsEvent = &apitype.ResOutputsEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Planning: p.Planning,
		}

	case sdkDisplay.ResourceOperationFailed:
		p, ok := e.Payload().(sdkDisplay.ResourceOperationFailedPayload)
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

func convertStepEventMetadata(md sdkDisplay.StepEventMetadata, showSecrets bool) apitype.StepEventMetadata {
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
func convertStepEventStateMetadata(md *sdkDisplay.StepEventStateMetadata,
	showSecrets bool) *apitype.StepEventStateMetadata {

	if md == nil {
		return nil
	}

	encrypter := config.BlindingCrypter
	inputs, err := stack.SerializeProperties(md.Inputs, encrypter, showSecrets)
	contract.IgnoreError(err)

	outputs, err := stack.SerializeProperties(md.Outputs, encrypter, showSecrets)
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

// ConvertJSONEvent converts an apitype.EngineEvent from the Pulumi REST API into a raw sdkDisplay.Event
// Returns an error if the engine event is unknown or not in an expected format.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertJSONEvent(apiEvent apitype.EngineEvent) (sdkDisplay.Event, error) {
	var event sdkDisplay.Event

	switch {
	case apiEvent.CancelEvent != nil:
		event = sdkDisplay.NewEvent(sdkDisplay.CancelEvent, nil)

	case apiEvent.StdoutEvent != nil:
		p := apiEvent.StdoutEvent
		event = sdkDisplay.NewEvent(sdkDisplay.StdoutColorEvent, sdkDisplay.StdoutEventPayload{
			Message: p.Message,
			Color:   colors.Colorization(p.Color),
		})

	case apiEvent.DiagnosticEvent != nil:
		p := apiEvent.DiagnosticEvent
		event = sdkDisplay.NewEvent(sdkDisplay.DiagEvent, sdkDisplay.DiagEventPayload{
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
		event = sdkDisplay.NewEvent(sdkDisplay.PolicyViolationEvent, sdkDisplay.PolicyViolationEventPayload{
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
		event = sdkDisplay.NewEvent(sdkDisplay.PreludeEvent, sdkDisplay.PreludeEventPayload{
			Config: p.Config,
		})

	case apiEvent.SummaryEvent != nil:
		p := apiEvent.SummaryEvent
		// Convert the resource changes.
		changes := display.ResourceChanges{}
		for op, count := range p.ResourceChanges {
			changes[display.StepOp(op)] = count
		}
		event = sdkDisplay.NewEvent(sdkDisplay.SummaryEvent, sdkDisplay.SummaryEventPayload{
			MaybeCorrupt:    p.MaybeCorrupt,
			Duration:        time.Duration(p.DurationSeconds) * time.Second,
			ResourceChanges: changes,
			PolicyPacks:     p.PolicyPacks,
		})

	case apiEvent.ResourcePreEvent != nil:
		p := apiEvent.ResourcePreEvent
		event = sdkDisplay.NewEvent(sdkDisplay.ResourcePreEvent, sdkDisplay.ResourcePreEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOutputsEvent != nil:
		p := apiEvent.ResOutputsEvent
		event = sdkDisplay.NewEvent(sdkDisplay.ResourceOutputsEvent, sdkDisplay.ResourceOutputsEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOpFailedEvent != nil:
		p := apiEvent.ResOpFailedEvent
		event = sdkDisplay.NewEvent(sdkDisplay.ResourceOperationFailed, sdkDisplay.ResourceOperationFailedPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Status:   resource.Status(p.Status),
			Steps:    p.Steps,
		})

	default:
		return event, errors.New("unknown event type")
	}

	return event, nil
}

func convertJSONStepEventMetadata(md apitype.StepEventMetadata) sdkDisplay.StepEventMetadata {
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

	return sdkDisplay.StepEventMetadata{
		Op:   display.StepOp(md.Op),
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
func convertJSONStepEventStateMetadata(md *apitype.StepEventStateMetadata) *sdkDisplay.StepEventStateMetadata {
	if md == nil {
		return nil
	}

	crypter := config.BlindingCrypter
	inputs, err := stack.DeserializeProperties(md.Inputs, crypter, crypter)
	contract.IgnoreError(err)

	outputs, err := stack.DeserializeProperties(md.Outputs, crypter, crypter)
	contract.IgnoreError(err)

	return &sdkDisplay.StepEventStateMetadata{
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
