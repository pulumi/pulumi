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
	"reflect"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine/events"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

func cancelEvent() events.Event {
	return events.Event{Type: events.CancelEvent}
}

func makeEventEmitter(eventsC chan<- events.Event, update UpdateInfo) (eventEmitter, error) {
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

	buffer, done := make(chan events.Event), make(chan bool)
	go queueEvents(eventsC, buffer, done)

	return eventEmitter{
		done: done,
		ch:   buffer,
	}, nil
}

func makeQueryEventEmitter(eventsC chan<- events.Event) (eventEmitter, error) {
	buffer, done := make(chan events.Event), make(chan bool)

	go queueEvents(eventsC, buffer, done)

	return eventEmitter{
		done: done,
		ch:   buffer,
	}, nil
}

type eventEmitter struct {
	done <-chan bool
	ch   chan<- events.Event
}

func queueEvents(eventsC chan<- events.Event, buffer chan events.Event, done chan bool) {
	// Instead of sending to the source channel directly, buffer events to account for slow receivers.
	//
	// Buffering is done by a goroutine that concurrently receives from the senders and attempts to send events to the
	// receiver. Events that are received while waiting for the receiver to catch up are buffered in a slice.
	//
	// We do not use a buffered channel because it is empirically less likely that the goroutine reading from a
	// buffered channel will be scheduled when new data is placed in the channel.

	defer close(done)

	var queue []events.Event
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
						eventsC <- e
					}
					return
				}
				queue = append(queue, e)
			case eventsC <- queue[0]:
				queue = queue[1:]
			}
		}
	}
}

func makeStepEventMetadata(op deploy.StepOp, step deploy.Step, debug bool) events.StepEventMetadata {
	contract.Assert(op == step.Op() || step.Op() == deploy.OpRefresh)

	var keys, diffs []resource.PropertyKey
	if keyer, hasKeys := step.(interface{ Keys() []resource.PropertyKey }); hasKeys {
		keys = keyer.Keys()
	}
	if differ, hasDiffs := step.(interface{ Diffs() []resource.PropertyKey }); hasDiffs {
		diffs = differ.Diffs()
	}

	var detailedDiff map[string]apitype.PropertyDiff
	if detailedDiffer, hasDetailedDiff := step.(interface {
		DetailedDiff() map[string]plugin.PropertyDiff
	}); hasDetailedDiff {
		if d := detailedDiffer.DetailedDiff(); d != nil {
			detailedDiff = map[string]apitype.PropertyDiff{}
			for pk, pd := range d {
				var kind apitype.DiffKind
				switch pd.Kind {
				case plugin.DiffAdd:
					kind = apitype.DiffAdd
				case plugin.DiffAddReplace:
					kind = apitype.DiffAddReplace
				case plugin.DiffUpdate:
					kind = apitype.DiffUpdate
				case plugin.DiffUpdateReplace:
					kind = apitype.DiffUpdateReplace
				case plugin.DiffDelete:
					kind = apitype.DiffDelete
				case plugin.DiffDeleteReplace:
					kind = apitype.DiffDeleteReplace
				}
				detailedDiff[string(pk)] = apitype.PropertyDiff{
					Kind:      kind,
					InputDiff: pd.InputDiff,
				}
			}
		}
	}

	return events.StepEventMetadata{
		Op:           events.StepOp(op),
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

func makeStepEventStateMetadata(state *resource.State, debug bool) *events.StepEventStateMetadata {
	if state == nil {
		return nil
	}

	return &events.StepEventStateMetadata{
		State:      state,
		Type:       state.Type,
		URN:        state.URN,
		Custom:     state.Custom,
		Delete:     state.Delete,
		ID:         state.ID,
		Parent:     state.Parent,
		Protect:    state.Protect,
		Inputs:     filterPropertyMap(state.Inputs, debug),
		Outputs:    filterPropertyMap(state.Outputs, debug),
		Provider:   state.Provider,
		InitErrors: state.InitErrors,
	}
}

func filterPropertyMap(propertyMap resource.PropertyMap, debug bool) resource.PropertyMap {
	mappable := propertyMap.Mappable()

	var filterValue func(v interface{}) interface{}

	filterPropertyValue := func(pv resource.PropertyValue) resource.PropertyValue {
		return resource.NewPropertyValue(filterValue(pv.Mappable()))
	}

	// filter values walks unwrapped (i.e. non-PropertyValue) values and applies the filter function
	// to them recursively.  The only thing the filter actually applies to is strings.
	//
	// The return value of this function should have the same type as the input value.
	filterValue = func(v interface{}) interface{} {
		if v == nil {
			return nil
		}

		// Else, check for some known primitive types.
		switch t := v.(type) {
		case bool, int, uint, int32, uint32,
			int64, uint64, float32, float64:
			// simple types.  map over as is.
			return v
		case string:
			// have to ensure we filter out secrets.
			return logging.FilterString(t)
		case *resource.Asset:
			text := t.Text
			if text != "" {
				// we don't want to include the full text of an asset as we serialize it over as
				// events.  They represent user files and are thus are unbounded in size.  Instead,
				// we only include the text if it represents a user's serialized program code, as
				// that is something we want the receiver to see to display as part of
				// progress/diffs/etc.
				if t.IsUserProgramCode() {
					// also make sure we filter this in case there are any secrets in the code.
					text = logging.FilterString(resource.MassageIfUserProgramCodeAsset(t, debug).Text)
				} else {
					// We need to have some string here so that we preserve that this is a
					// text-asset
					text = "<stripped>"
				}
			}

			return &resource.Asset{
				Sig:  t.Sig,
				Hash: t.Hash,
				Text: text,
				Path: t.Path,
				URI:  t.URI,
			}
		case *resource.Archive:
			return &resource.Archive{
				Sig:    t.Sig,
				Hash:   t.Hash,
				Path:   t.Path,
				URI:    t.URI,
				Assets: filterValue(t.Assets).(map[string]interface{}),
			}
		case resource.Secret:
			return "[secret]"
		case resource.Computed:
			return resource.Computed{
				Element: filterPropertyValue(t.Element),
			}
		case resource.Output:
			return resource.Output{
				Element: filterPropertyValue(t.Element),
			}
		case resource.ResourceReference:
			return resource.ResourceReference{
				URN:            resource.URN(filterValue(string(t.URN)).(string)),
				ID:             resource.PropertyValue{V: filterValue(t.ID.V)},
				PackageVersion: filterValue(t.PackageVersion).(string),
			}
		}

		// Next, see if it's an array, slice, pointer or struct, and handle each accordingly.
		rv := reflect.ValueOf(v)
		switch rk := rv.Type().Kind(); rk {
		case reflect.Array, reflect.Slice:
			// If an array or slice, just create an array out of it.
			var arr []interface{}
			for i := 0; i < rv.Len(); i++ {
				arr = append(arr, filterValue(rv.Index(i).Interface()))
			}
			return arr
		case reflect.Ptr:
			if rv.IsNil() {
				return nil
			}

			v1 := filterValue(rv.Elem().Interface())
			return &v1
		case reflect.Map:
			obj := make(map[string]interface{})
			for _, key := range rv.MapKeys() {
				k := key.Interface().(string)
				v := rv.MapIndex(key).Interface()
				obj[k] = filterValue(v)
			}
			return obj
		default:
			contract.Failf("Unrecognized value type: type=%v kind=%v", rv.Type(), rk)
		}

		return nil
	}

	return resource.NewPropertyMapFromMapRepl(
		mappable, nil, /*replk*/
		func(v interface{}) (resource.PropertyValue, bool) {
			return resource.NewPropertyValue(filterValue(v)), true
		})
}

func (e *eventEmitter) Close() {
	close(e.ch)
	<-e.done
}

func (e *eventEmitter) resourceOperationFailedEvent(
	step deploy.Step, status resource.Status, steps int, debug bool) {

	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- events.NewEvent(events.ResourceOperationFailed, events.ResourceOperationFailedPayload{
		Metadata: makeStepEventMetadata(step.Op(), step, debug),
		Status:   status,
		Steps:    steps,
	})
}

func (e *eventEmitter) resourceOutputsEvent(op deploy.StepOp, step deploy.Step, planning bool, debug bool) {
	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- events.NewEvent(events.ResourceOutputsEvent, events.ResourceOutputsEventPayload{
		Metadata: makeStepEventMetadata(op, step, debug),
		Planning: planning,
		Debug:    debug,
	})
}

func (e *eventEmitter) resourcePreEvent(
	step deploy.Step, planning bool, debug bool) {

	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- events.NewEvent(events.ResourcePreEvent, events.ResourcePreEventPayload{
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

	e.ch <- events.NewEvent(events.PreludeEvent, events.PreludeEventPayload{
		IsPreview: isPreview,
		Config:    configStringMap,
	})
}

func (e *eventEmitter) summaryEvent(preview, maybeCorrupt bool, duration time.Duration, resourceChanges events.ResourceChanges,
	policyPacks map[string]string) {

	contract.Requiref(e != nil, "e", "!= nil")

	e.ch <- events.NewEvent(events.SummaryEvent, events.SummaryEventPayload{
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

	e.ch <- events.NewEvent(events.PolicyViolationEvent, events.PolicyViolationEventPayload{
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

	e.ch <- events.NewEvent(events.DiagEvent, events.DiagEventPayload{
		URN:       d.URN,
		Prefix:    logging.FilterString(prefix),
		Message:   logging.FilterString(msg),
		Color:     colors.Raw,
		Severity:  string(sev),
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
