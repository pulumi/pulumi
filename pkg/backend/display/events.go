// Copyright 2019-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var matchAnsiControlCodes = regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

// ConvertEngineEvent converts a raw engine.Event into an apitype.EngineEvent used in the Pulumi
// REST API. Returns an error if the engine event is unknown or not in an expected format.
// EngineEvent.{ Sequence, Timestamp } are expected to be set by the caller.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertEngineEvent(e engine.Event, showSecrets bool) (apitype.EngineEvent, error) {
	var apiEvent apitype.EngineEvent

	// Error to return if the payload doesn't match expected.
	eventTypePayloadMismatch := fmt.Errorf("unexpected payload for event type %v", e.Type)

	switch e.Type {
	case engine.CancelEvent:
		apiEvent.CancelEvent = &apitype.CancelEvent{}

	case engine.StdoutColorEvent:
		p, ok := e.Payload().(engine.StdoutEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.StdoutEvent = &apitype.StdoutEngineEvent{
			Message: p.Message,
			Color:   string(p.Color),
		}

	case engine.DiagEvent:
		p, ok := e.Payload().(engine.DiagEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}

		// Clean up ANSI control codes.
		cleanedMsg := matchAnsiControlCodes.ReplaceAllString(p.Message, "")

		apiEvent.DiagnosticEvent = &apitype.DiagnosticEvent{
			URN:       string(p.URN),
			Prefix:    p.Prefix,
			Message:   cleanedMsg,
			Color:     string(p.Color),
			Severity:  string(p.Severity),
			Ephemeral: p.Ephemeral,
		}

	case engine.StartDebuggingEvent:
		p, ok := e.Payload().(engine.StartDebuggingEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}

		apiEvent.StartDebuggingEvent = &apitype.StartDebuggingEvent{
			Config: p.Config,
		}

	case engine.PolicyViolationEvent:
		p, ok := e.Payload().(engine.PolicyViolationEventPayload)
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

	case engine.PolicyRemediationEvent:
		p, ok := e.Payload().(engine.PolicyRemediationEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}

		// Serialize properties, ignoring errors, as with other event types.
		ctx := context.TODO()
		encrypter := config.BlindingCrypter
		before, err := stack.SerializeProperties(ctx, p.Before, encrypter, showSecrets)
		contract.IgnoreError(err)
		after, err := stack.SerializeProperties(ctx, p.After, encrypter, showSecrets)
		contract.IgnoreError(err)

		apiEvent.PolicyRemediationEvent = &apitype.PolicyRemediationEvent{
			ResourceURN:          string(p.ResourceURN),
			Color:                string(p.Color),
			PolicyName:           p.PolicyName,
			PolicyPackName:       p.PolicyPackName,
			PolicyPackVersion:    p.PolicyPackVersion,
			PolicyPackVersionTag: p.PolicyPackVersion,
			Before:               before,
			After:                after,
		}

	case engine.PreludeEvent:
		p, ok := e.Payload().(engine.PreludeEventPayload)
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

	case engine.SummaryEvent:
		p, ok := e.Payload().(engine.SummaryEventPayload)
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
			DurationSeconds: int(math.Ceil(p.Duration.Seconds())),
			ResourceChanges: changes,
			PolicyPacks:     p.PolicyPacks,
			IsPreview:       p.IsPreview,
		}

	case engine.ResourcePreEvent:
		p, ok := e.Payload().(engine.ResourcePreEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResourcePreEvent = &apitype.ResourcePreEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Planning: p.Planning,
		}

	case engine.ResourceOutputsEvent:
		p, ok := e.Payload().(engine.ResourceOutputsEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOutputsEvent = &apitype.ResOutputsEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Planning: p.Planning,
		}

	case engine.ResourceOperationFailed:
		p, ok := e.Payload().(engine.ResourceOperationFailedPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOpFailedEvent = &apitype.ResOpFailedEvent{
			Metadata: convertStepEventMetadata(p.Metadata, showSecrets),
			Status:   int(p.Status),
			Steps:    int(p.Steps),
		}

	case engine.PolicyLoadEvent:
		apiEvent.PolicyLoadEvent = &apitype.PolicyLoadEvent{}

	case engine.ProgressEvent:
		p, ok := e.Payload().(engine.ProgressEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ProgressEvent = &apitype.ProgressEvent{
			Type:      apitype.ProgressType(p.Type),
			ID:        p.ID,
			Message:   p.Message,
			Completed: p.Completed,
			Total:     p.Total,
			Done:      p.Done,
		}

	default:
		return apiEvent, fmt.Errorf("unknown event type %q", e.Type)
	}

	return apiEvent, nil
}

func convertStepEventMetadata(md engine.StepEventMetadata, showSecrets bool) apitype.StepEventMetadata {
	keys := make([]string, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = string(v)
	}

	diffs := slice.Prealloc[string](len(md.Diffs))
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
func convertStepEventStateMetadata(md *engine.StepEventStateMetadata,
	showSecrets bool,
) *apitype.StepEventStateMetadata {
	if md == nil {
		return nil
	}

	ctx := context.TODO()
	encrypter := config.BlindingCrypter
	inputs, err := stack.SerializeProperties(ctx, md.Inputs, encrypter, showSecrets)
	contract.IgnoreError(err)

	outputs, err := stack.SerializeProperties(ctx, md.Outputs, encrypter, showSecrets)
	contract.IgnoreError(err)

	return &apitype.StepEventStateMetadata{
		Type: string(md.Type),
		URN:  string(md.URN),

		Custom:         md.Custom,
		Delete:         md.Delete,
		ID:             string(md.ID),
		Parent:         string(md.Parent),
		Provider:       md.Provider,
		Protect:        md.Protect,
		RetainOnDelete: md.RetainOnDelete,
		Inputs:         inputs,
		Outputs:        outputs,
		InitErrors:     md.InitErrors,
	}
}

// ConvertJSONEvent converts an apitype.EngineEvent from the Pulumi REST API into a raw engine.Event
// Returns an error if the engine event is unknown or not in an expected format.
//
// IMPORTANT: Any resource secret data stored in the engine event will be encrypted using the
// blinding encrypter, and unrecoverable. So this operation is inherently lossy.
func ConvertJSONEvent(apiEvent apitype.EngineEvent) (engine.Event, error) {
	var event engine.Event

	switch {
	case apiEvent.CancelEvent != nil:
		event = engine.NewCancelEvent()

	case apiEvent.StdoutEvent != nil:
		p := apiEvent.StdoutEvent
		event = engine.NewEvent(engine.StdoutEventPayload{
			Message: p.Message,
			Color:   colors.Colorization(p.Color),
		})

	case apiEvent.DiagnosticEvent != nil:
		p := apiEvent.DiagnosticEvent
		event = engine.NewEvent(engine.DiagEventPayload{
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
		event = engine.NewEvent(engine.PolicyViolationEventPayload{
			ResourceURN:       resource.URN(p.ResourceURN),
			Message:           p.Message,
			Color:             colors.Colorization(p.Color),
			PolicyName:        p.PolicyName,
			PolicyPackName:    p.PolicyPackName,
			PolicyPackVersion: p.PolicyPackVersion,
			EnforcementLevel:  apitype.EnforcementLevel(p.EnforcementLevel),
		})

	case apiEvent.PolicyRemediationEvent != nil:
		p := apiEvent.PolicyRemediationEvent

		// Deserialize the before and after properties, ignoring serialization
		// errors as the other event types do (e.g., step events).
		crypter := config.BlindingCrypter
		before, err := stack.DeserializeProperties(p.Before, crypter)
		contract.IgnoreError(err)
		after, err := stack.DeserializeProperties(p.After, crypter)
		contract.IgnoreError(err)

		event = engine.NewEvent(engine.PolicyRemediationEventPayload{
			ResourceURN:       resource.URN(p.ResourceURN),
			Color:             colors.Colorization(p.Color),
			PolicyName:        p.PolicyName,
			PolicyPackName:    p.PolicyPackName,
			PolicyPackVersion: p.PolicyPackVersion,
			Before:            before,
			After:             after,
		})

	case apiEvent.PreludeEvent != nil:
		p := apiEvent.PreludeEvent

		// Convert the config bag.
		event = engine.NewEvent(engine.PreludeEventPayload{
			Config: p.Config,
		})

	case apiEvent.SummaryEvent != nil:
		p := apiEvent.SummaryEvent
		// Convert the resource changes.
		changes := display.ResourceChanges{}
		for op, count := range p.ResourceChanges {
			changes[display.StepOp(op)] = count
		}
		event = engine.NewEvent(engine.SummaryEventPayload{
			MaybeCorrupt:    p.MaybeCorrupt,
			Duration:        time.Duration(p.DurationSeconds) * time.Second,
			ResourceChanges: changes,
			PolicyPacks:     p.PolicyPacks,
			IsPreview:       p.IsPreview,
		})

	case apiEvent.ResourcePreEvent != nil:
		p := apiEvent.ResourcePreEvent
		event = engine.NewEvent(engine.ResourcePreEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOutputsEvent != nil:
		p := apiEvent.ResOutputsEvent
		event = engine.NewEvent(engine.ResourceOutputsEventPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		})

	case apiEvent.ResOpFailedEvent != nil:
		p := apiEvent.ResOpFailedEvent
		event = engine.NewEvent(engine.ResourceOperationFailedPayload{
			Metadata: convertJSONStepEventMetadata(p.Metadata),
			Status:   resource.Status(p.Status),
			Steps:    int32(p.Steps), //nolint:gosec // We only ever write int32 sized values to the API.
		})

	case apiEvent.PolicyLoadEvent != nil:
		event = engine.NewEvent(engine.PolicyLoadEventPayload{})

	case apiEvent.ProgressEvent != nil:
		p := apiEvent.ProgressEvent
		event = engine.NewEvent(engine.ProgressEventPayload{
			Type:      engine.ProgressType(p.Type),
			ID:        p.ID,
			Message:   p.Message,
			Completed: p.Completed,
			Total:     p.Total,
			Done:      p.Done,
		})

	default:
		return event, errors.New("unknown event type")
	}

	return event, nil
}

func convertJSONStepEventMetadata(md apitype.StepEventMetadata) engine.StepEventMetadata {
	keys := make([]resource.PropertyKey, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = resource.PropertyKey(v)
	}
	diffs := slice.Prealloc[resource.PropertyKey](len(md.Diffs))
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

	return engine.StepEventMetadata{
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
func convertJSONStepEventStateMetadata(md *apitype.StepEventStateMetadata) *engine.StepEventStateMetadata {
	if md == nil {
		return nil
	}

	crypter := config.BlindingCrypter
	inputs, err := stack.DeserializeProperties(md.Inputs, crypter)
	contract.IgnoreError(err)

	outputs, err := stack.DeserializeProperties(md.Outputs, crypter)
	contract.IgnoreError(err)

	return &engine.StepEventStateMetadata{
		Type: tokens.Type(md.Type),
		URN:  resource.URN(md.URN),

		Custom:         md.Custom,
		Delete:         md.Delete,
		ID:             resource.ID(md.ID),
		Parent:         resource.URN(md.Parent),
		Provider:       md.Provider,
		Protect:        md.Protect,
		RetainOnDelete: md.RetainOnDelete,
		Inputs:         inputs,
		Outputs:        outputs,
		InitErrors:     md.InitErrors,
	}
}
