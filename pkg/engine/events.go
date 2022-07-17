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

package engine

import (
	"bytes"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/display"
	. "github.com/pulumi/pulumi/sdk/v3/go/common/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

func makeEventEmitter(events chan<- Event, update UpdateInfo) (eventEmitter, error) {
	target := update.GetTarget()
	var secrets []string
	if target != nil && target.Config.HasSecureValue() {
		for k, v := range target.Config {
			if !v.Secure() {
				continue
			}

			secureValues, err := v.SecureValues(target.Decrypter)
			if err != nil {
				return eventEmitter{}, DecryptError{
					Key: k,
					Err: err,
				}
			}
			secrets = append(secrets, secureValues...)
		}
	}

	logging.AddGlobalFilter(logging.CreateFilter(secrets, "[secret]"))

	buffer, done := make(chan Event), make(chan bool)
	go queueEvents(events, buffer, done)

	return eventEmitter{
		done: done,
		ch:   buffer,
	}, nil
}

func makeQueryEventEmitter(events chan<- Event) (eventEmitter, error) {
	buffer, done := make(chan Event), make(chan bool)

	go queueEvents(events, buffer, done)

	return eventEmitter{
		done: done,
		ch:   buffer,
	}, nil
}

type eventEmitter struct {
	done <-chan bool
	ch   chan<- Event
}

func queueEvents(events chan<- Event, buffer chan Event, done chan bool) {
	// Instead of sending to the source channel directly, buffer events to account for slow receivers.
	//
	// Buffering is done by a goroutine that concurrently receives from the senders and attempts to send events to the
	// receiver. Events that are received while waiting for the receiver to catch up are buffered in a slice.
	//
	// We do not use a buffered channel because it is empirically less likely that the goroutine reading from a
	// buffered channel will be scheduled when new data is placed in the channel.

	defer close(done)

	var queue []Event
	for {
		contract.Assert(buffer != nil)

		e, ok := <-buffer
		if !ok {
			return
		}
		queue = append(queue, e)

		// While there are events in the queue, attempt to send them to the waiting receiver. If the receiver is
		// blocked and an event is received from the event senders, stick that event in the queue.
		for len(queue) > 0 {
			select {
			case e, ok := <-buffer:
				if !ok {
					// If the event source has been closed, flush the queue.
					for _, e := range queue {
						events <- e
					}
					return
				}
				queue = append(queue, e)
			case events <- queue[0]:
				queue = queue[1:]
			}
		}
	}
}

func makeStepEventMetadata(op display.StepOp, step deploy.Step, debug bool) StepEventMetadata {
	contract.Assert(op == step.Op() || step.Op() == deploy.OpRefresh)

	var keys, diffs []resource.PropertyKey
	if keyer, hasKeys := step.(interface{ Keys() []resource.PropertyKey }); hasKeys {
		keys = keyer.Keys()
	}
	if differ, hasDiffs := step.(interface{ Diffs() []resource.PropertyKey }); hasDiffs {
		diffs = differ.Diffs()
	}

	var detailedDiff map[string]plugin.PropertyDiff
	if detailedDiffer, hasDetailedDiff := step.(interface {
		DetailedDiff() map[string]plugin.PropertyDiff
	}); hasDetailedDiff {
		detailedDiff = detailedDiffer.DetailedDiff()
	}

	return StepEventMetadata{
		Op:           op,
		URN:          step.URN(),
		Type:         step.Type(),
		Keys:         keys,
		Diffs:        diffs,
		DetailedDiff: detailedDiff,
		Old:          makeStepEventStateMetadata(step.Old(), debug),
		New:          makeStepEventStateMetadata(step.New(), debug),
		Res:          makeStepEventStateMetadata(step.Res(), debug),
		Logical:      step.Logical(),
		Provider:     step.Provider(),
	}
}

func makeStepEventStateMetadata(state *resource.State, debug bool) *StepEventStateMetadata {
	if state == nil {
		return nil
	}

	return &StepEventStateMetadata{
		State:      state,
		Type:       state.Type,
		URN:        state.URN,
		Custom:     state.Custom,
		Delete:     state.Delete,
		ID:         state.ID,
		Parent:     state.Parent,
		Protect:    state.Protect,
		Inputs:     filterResourceProperties(state.Inputs, debug),
		Outputs:    filterResourceProperties(state.Outputs, debug),
		Provider:   state.Provider,
		InitErrors: state.InitErrors,
	}
}

func (e *eventEmitter) Close() {
	close(e.ch)
	<-e.done
}

func (e *eventEmitter) resourceOperationFailedEvent(
	step deploy.Step, status resource.Status, steps int, debug bool) {

	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- NewEvent(ResourceOperationFailed, ResourceOperationFailedPayload{
		Metadata: makeStepEventMetadata(step.Op(), step, debug),
		Status:   status,
		Steps:    steps,
	})
}

func (e *eventEmitter) resourceOutputsEvent(op display.StepOp, step deploy.Step, planning bool, debug bool) {
	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- NewEvent(ResourceOutputsEvent, ResourceOutputsEventPayload{
		Metadata: makeStepEventMetadata(op, step, debug),
		Planning: planning,
		Debug:    debug,
	})
}

func (e *eventEmitter) resourcePreEvent(
	step deploy.Step, planning bool, debug bool) {

	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- NewEvent(ResourcePreEvent, ResourcePreEventPayload{
		Metadata: makeStepEventMetadata(step.Op(), step, debug),
		Planning: planning,
		Debug:    debug,
	})
}

func (e *eventEmitter) preludeEvent(isPreview bool, cfg config.Map) {
	contract.Requiref(e != nil, "e", "!= nil")

	configStringMap := make(map[string]string, len(cfg))
	for k, v := range cfg {
		keyString := k.String()
		valueString, err := v.Value(config.NewBlindingDecrypter())
		contract.AssertNoError(err)
		configStringMap[keyString] = valueString
	}

	e.ch <- NewEvent(PreludeEvent, PreludeEventPayload{
		IsPreview: isPreview,
		Config:    configStringMap,
	})
}

func (e *eventEmitter) summaryEvent(preview, maybeCorrupt bool, duration time.Duration,
	resourceChanges display.ResourceChanges, policyPacks map[string]string) {

	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- NewEvent(SummaryEvent, SummaryEventPayload{
		IsPreview:       preview,
		MaybeCorrupt:    maybeCorrupt,
		Duration:        duration,
		ResourceChanges: resourceChanges,
		PolicyPacks:     policyPacks,
	})
}

func (e *eventEmitter) policyViolationEvent(urn resource.URN, d plugin.AnalyzeDiagnostic) {

	contract.Requiref(e != nil, "e", "!= nil")

	// Write prefix.
	var prefix bytes.Buffer
	switch d.EnforcementLevel {
	case apitype.Mandatory:
		prefix.WriteString(colors.SpecError)
	case apitype.Advisory:
		prefix.WriteString(colors.SpecWarning)
	default:
		contract.Failf("Unrecognized diagnostic severity: %v", d)
	}

	prefix.WriteString(string(d.EnforcementLevel))
	prefix.WriteString(": ")
	prefix.WriteString(colors.Reset)

	// Write the message itself.
	var buffer bytes.Buffer
	buffer.WriteString(colors.SpecNote)

	buffer.WriteString(d.Message)

	buffer.WriteString(colors.Reset)
	buffer.WriteRune('\n')

	e.ch <- NewEvent(PolicyViolationEvent, PolicyViolationEventPayload{
		ResourceURN:       urn,
		Message:           logging.FilterString(buffer.String()),
		Color:             colors.Raw,
		PolicyName:        d.PolicyName,
		PolicyPackName:    d.PolicyPackName,
		PolicyPackVersion: d.PolicyPackVersion,
		EnforcementLevel:  d.EnforcementLevel,
		Prefix:            logging.FilterString(prefix.String()),
	})
}

func diagEvent(e *eventEmitter, d *diag.Diag, prefix, msg string, sev diag.Severity,
	ephemeral bool) {
	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- NewEvent(DiagEvent, DiagEventPayload{
		URN:       d.URN,
		Prefix:    logging.FilterString(prefix),
		Message:   logging.FilterString(msg),
		Color:     colors.Raw,
		Severity:  sev,
		StreamID:  d.StreamID,
		Ephemeral: ephemeral,
	})
}

func (e *eventEmitter) diagDebugEvent(d *diag.Diag, prefix, msg string, ephemeral bool) {
	diagEvent(e, d, prefix, msg, diag.Debug, ephemeral)
}

func (e *eventEmitter) diagInfoEvent(d *diag.Diag, prefix, msg string, ephemeral bool) {
	diagEvent(e, d, prefix, msg, diag.Info, ephemeral)
}

func (e *eventEmitter) diagInfoerrEvent(d *diag.Diag, prefix, msg string, ephemeral bool) {
	diagEvent(e, d, prefix, msg, diag.Infoerr, ephemeral)
}

func (e *eventEmitter) diagErrorEvent(d *diag.Diag, prefix, msg string, ephemeral bool) {
	diagEvent(e, d, prefix, msg, diag.Error, ephemeral)
}

func (e *eventEmitter) diagWarningEvent(d *diag.Diag, prefix, msg string, ephemeral bool) {
	diagEvent(e, d, prefix, msg, diag.Warning, ephemeral)
}

func filterResourceProperties(m resource.PropertyMap, debug bool) resource.PropertyMap {
	return filterPropertyValue(resource.NewObjectProperty(m), debug).ObjectValue()
}

func filterPropertyValue(v resource.PropertyValue, debug bool) resource.PropertyValue {
	switch {
	case v.IsNull(), v.IsBool(), v.IsNumber():
		return v
	case v.IsString():
		// have to ensure we filter out secrets.
		return resource.NewStringProperty(logging.FilterString(v.StringValue()))
	case v.IsAsset():
		return resource.NewAssetProperty(filterAsset(v.AssetValue(), debug))
	case v.IsArchive():
		return resource.NewArchiveProperty(filterArchive(v.ArchiveValue(), debug))
	case v.IsArray():
		arr := make([]resource.PropertyValue, len(v.ArrayValue()))
		for i, v := range v.ArrayValue() {
			arr[i] = filterPropertyValue(v, debug)
		}
		return resource.NewArrayProperty(arr)
	case v.IsObject():
		obj := make(resource.PropertyMap, len(v.ObjectValue()))
		for k, v := range v.ObjectValue() {
			obj[k] = filterPropertyValue(v, debug)
		}
		return resource.NewObjectProperty(obj)
	case v.IsComputed():
		return resource.MakeComputed(filterPropertyValue(v.Input().Element, debug))
	case v.IsOutput():
		return resource.MakeComputed(filterPropertyValue(v.OutputValue().Element, debug))
	case v.IsSecret():
		return resource.MakeSecret(resource.NewStringProperty("[secret]"))
	case v.IsResourceReference():
		ref := v.ResourceReferenceValue()
		return resource.NewResourceReferenceProperty(resource.ResourceReference{
			URN:            resource.URN(logging.FilterString(string(ref.URN))),
			ID:             filterPropertyValue(ref.ID, debug),
			PackageVersion: logging.FilterString(ref.PackageVersion),
		})
	default:
		contract.Failf("unexpected property value type %T", v.V)
		return resource.PropertyValue{}
	}
}

func filterAsset(v *resource.Asset, debug bool) *resource.Asset {
	if !v.IsText() {
		return v
	}

	// we don't want to include the full text of an asset as we serialize it over as
	// events.  They represent user files and are thus are unbounded in size.  Instead,
	// we only include the text if it represents a user's serialized program code, as
	// that is something we want the receiver to see to display as part of
	// progress/diffs/etc.
	var text string
	if v.IsUserProgramCode() {
		// also make sure we filter this in case there are any secrets in the code.
		text = logging.FilterString(resource.MassageIfUserProgramCodeAsset(v, debug).Text)
	} else {
		// We need to have some string here so that we preserve that this is a
		// text-asset
		text = "<contents elided>"
	}

	return &resource.Asset{
		Sig:  v.Sig,
		Hash: v.Hash,
		Text: text,
	}
}

func filterArchive(v *resource.Archive, debug bool) *resource.Archive {
	if !v.IsAssets() {
		return v
	}

	assets := make(map[string]interface{})
	for k, v := range v.Assets {
		switch v := v.(type) {
		case *resource.Asset:
			assets[k] = filterAsset(v, debug)
		case *resource.Archive:
			assets[k] = filterArchive(v, debug)
		default:
			contract.Failf("Unrecognized asset map type %T", v)
		}
	}
	return &resource.Archive{
		Sig:    v.Sig,
		Hash:   v.Hash,
		Assets: assets,
	}
}
