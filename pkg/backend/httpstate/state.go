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

package httpstate

import (
	"context"
	"fmt"
	"time"

	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type tokenRequest chan<- tokenResponse

type tokenResponse struct {
	token string
	err   error
}

// tokenSource is a helper type that manages the renewal of the lease token for a managed update.
type tokenSource struct {
	requests chan tokenRequest
	done     chan bool
}

func newTokenSource(ctx context.Context, token string, backend *cloudBackend, update client.UpdateIdentifier,
	duration time.Duration) (*tokenSource, error) {

	// Perform an initial lease renewal.
	newToken, err := backend.client.RenewUpdateLease(ctx, update, token, duration)
	if err != nil {
		return nil, err
	}

	requests, done := make(chan tokenRequest), make(chan bool)
	go func() {
		// We will renew the lease after 50% of the duration has elapsed to allow more time for retries.
		ticker := time.NewTicker(duration / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				newToken, err = backend.client.RenewUpdateLease(ctx, update, token, duration)
				if err != nil {
					ticker.Stop()
				} else {
					token = newToken
				}

			case c, ok := <-requests:
				if !ok {
					close(done)
					return
				}

				resp := tokenResponse{err: err}
				if err == nil {
					resp.token = token
				}
				c <- resp
			}
		}
	}()

	return &tokenSource{requests: requests, done: done}, nil
}

func (ts *tokenSource) Close() {
	close(ts.requests)
	<-ts.done
}

func (ts *tokenSource) GetToken() (string, error) {
	ch := make(chan tokenResponse)
	ts.requests <- ch
	resp := <-ch
	return resp.token, resp.err
}

// cloudUpdate is an implementation of engine.Update backed by remote state and a local program.
type cloudUpdate struct {
	context context.Context
	backend *cloudBackend

	update      client.UpdateIdentifier
	tokenSource *tokenSource

	root   string
	proj   *workspace.Project
	target *deploy.Target
}

func (u *cloudUpdate) GetRoot() string {
	return u.root
}

func (u *cloudUpdate) GetProject() *workspace.Project {
	return u.proj
}

func (u *cloudUpdate) GetTarget() *deploy.Target {
	return u.target
}

func (u *cloudUpdate) Complete(status apitype.UpdateStatus) error {
	defer u.tokenSource.Close()

	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}
	return u.backend.client.CompleteUpdate(u.context, u.update, status, token)
}

// recordEvent will record the event with the Pulumi service, enabling things like viewing
// the rendered update logs or drilling into the timeline of an update.
func (u *cloudUpdate) recordEvent(
	action apitype.UpdateKind, event engine.Event, seen map[resource.URN]engine.StepEventMetadata,
	opts display.Options) error {
	contract.Assert(u.tokenSource != nil)
	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}

	// Convert the engine event into a more structured format the Pulumi service expects.
	convertedEvent, conversionErr := convertEngineEvent(event)
	if conversionErr != nil {
		return errors.Wrap(conversionErr, "converting engine event")
	}
	if err := u.backend.client.RecordEngineEvent(u.context, u.update, convertedEvent, token); err != nil {
		return err
	}

	// The following is an older codepath where we pre-rendered the event using the diff view, and then reported
	// the result to the service. Useful, but failed to capture more detailed information.
	fields := make(map[string]interface{})
	kind := string(apitype.StdoutEvent)
	if event.Type == engine.DiagEvent {
		payload := event.Payload.(engine.DiagEventPayload)
		fields["severity"] = string(payload.Severity)
		if payload.Severity == diag.Error || payload.Severity == diag.Warning {
			kind = string(apitype.StderrEvent)
		}
	}

	// Ensure we render events with raw colorization tags.  Also, render these as 'diff' events so
	// the user has a rich diff-log they can see when the look at their logs in the service.
	opts.Color = colors.Raw
	msg := display.RenderDiffEvent(action, event, seen, opts)

	// If we have a message, upload it as <= 1MB chunks.
	for msg != "" {
		chunk := msg
		const maxLen = 1 << 20 // 1 MB
		if len(chunk) > maxLen {
			chunk = colors.TrimPartialCommand(msg)
		}
		msg = msg[len(chunk):]

		fields["text"] = chunk
		fields["colorize"] = colors.Always
		if err = u.backend.client.AppendUpdateLogEntry(u.context, u.update, kind, fields, token); err != nil {
			return err
		}
	}
	return nil
}

// RecordAndDisplayEvents inspects engine events from the given channel, and prints them to the CLI as well as
// posting them to the Pulumi service.
func (u *cloudUpdate) RecordAndDisplayEvents(
	label string, action apitype.UpdateKind, stackRef backend.StackReference, op backend.UpdateOperation,
	events <-chan engine.Event, done chan<- bool, opts display.Options, isPreview bool) {

	// Start the local display processor.  Display things however the options have been
	// set to display (i.e. diff vs progress).
	displayEvents := make(chan engine.Event)
	go display.ShowEvents(label, action, stackRef.Name(), op.Proj.Name, displayEvents, done, opts, isPreview)

	seen := make(map[resource.URN]engine.StepEventMetadata)
	for e := range events {
		// First echo the event to the local display.
		displayEvents <- e

		// Then render and record the event for posterity.
		if err := u.recordEvent(action, e, seen, opts); err != nil {
			diagEvent := engine.Event{
				Type: engine.DiagEvent,
				Payload: engine.DiagEventPayload{
					Message:  fmt.Sprintf("failed to record event: %v", err),
					Severity: diag.Infoerr,
				},
			}
			displayEvents <- diagEvent
		}

		if e.Type == engine.CancelEvent {
			return
		}
	}
}

func (b *cloudBackend) newUpdate(ctx context.Context, stackRef backend.StackReference, proj *workspace.Project,
	root string, update client.UpdateIdentifier, token string) (*cloudUpdate, error) {

	// Create a token source for this update if necessary.
	var tokenSource *tokenSource
	if token != "" {
		ts, err := newTokenSource(ctx, token, b, update, 5*time.Minute)
		if err != nil {
			return nil, err
		}
		tokenSource = ts
	}

	// Construct the deployment target.
	target, err := b.getTarget(ctx, stackRef)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &cloudUpdate{
		context:     ctx,
		backend:     b,
		update:      update,
		tokenSource: tokenSource,
		root:        root,
		proj:        proj,
		target:      target,
	}, nil
}

func (b *cloudBackend) getSnapshot(ctx context.Context, stackRef backend.StackReference) (*deploy.Snapshot, error) {
	untypedDeployment, err := b.ExportDeployment(ctx, stackRef)
	if err != nil {
		return nil, err
	}

	snapshot, err := stack.DeserializeUntypedDeployment(untypedDeployment)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (b *cloudBackend) getTarget(ctx context.Context, stackRef backend.StackReference) (*deploy.Target, error) {
	// Pull the local stack info so we can get at its configuration bag.
	stk, err := workspace.DetectProjectStack(stackRef.Name())
	if err != nil {
		return nil, err
	}

	decrypter, err := b.GetStackCrypter(stackRef)
	if err != nil {
		return nil, err
	}
	snapshot, err := b.getSnapshot(ctx, stackRef)
	if err != nil {
		switch err {
		case stack.ErrDeploymentSchemaVersionTooOld:
			return nil, fmt.Errorf("the stack '%s' is too old to be used by this version of the Pulumi CLI",
				stackRef.Name())
		case stack.ErrDeploymentSchemaVersionTooNew:
			return nil, fmt.Errorf("the stack '%s' is newer than what this version of the Pulumi CLI understands. "+
				"Please update your version of the Pulumi CLI", stackRef.Name())
		default:
			return nil, errors.Wrap(err, "could not deserialize deployment")
		}
	}

	return &deploy.Target{
		Name:      stackRef.Name(),
		Config:    stk.Config,
		Decrypter: decrypter,
		Snapshot:  snapshot,
	}, nil
}

func convertStepEventMetadata(md engine.StepEventMetadata) apitype.StepEventMetadata {
	keys := make([]string, 0, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = string(v)
	}

	return apitype.StepEventMetadata{
		Op:   string(md.Op),
		URN:  string(md.URN),
		Type: string(md.Type),

		Old: convertStepEventStateMetadata(md.Old),
		New: convertStepEventStateMetadata(md.New),
		Res: convertStepEventStateMetadata(md.Res),

		Keys:     keys,
		Logical:  md.Logical,
		Provider: md.Provider,
	}
}

func convertStepEventStateMetadata(md *engine.StepEventStateMetadata) *apitype.StepEventStateMetadata {
	if md == nil {
		return nil
	}

	inputs := make(map[string]interface{})
	for k, v := range md.Inputs {
		inputs[string(k)] = v
	}

	outputs := make(map[string]interface{})
	for k, v := range md.Outputs {
		outputs[string(k)] = v
	}

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

// convertEngineEvent converts a raw engine.Event into an apitype.EngineEvent used in the Pulumi
// REST API. Returns an error if the engine event is unknown or not in an expected format.
func convertEngineEvent(e engine.Event) (apitype.EngineEvent, error) {
	var apiEvent apitype.EngineEvent

	// Error to return if the payload doesn't match expected.
	eventTypePayloadMismatch := errors.Errorf("unexpected payload for event type %v", e.Type)

	switch e.Type {
	case engine.CancelEvent:
		apiEvent.CancelEvent = &apitype.CancelEvent{}

	case engine.StdoutColorEvent:
		p, ok := e.Payload.(engine.StdoutEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.StdoutEvent = &apitype.StdoutEngineEvent{
			Message: p.Message,
			Color:   string(p.Color),
		}

	case engine.DiagEvent:
		p, ok := e.Payload.(engine.DiagEventPayload)
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

	case engine.PreludeEvent:
		p, ok := e.Payload.(engine.PreludeEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		// Convert the config bag.
		cfg := make(map[string]string)
		for k, v := range p.Config {
			cfg[string(k)] = v
		}
		apiEvent.PreludeEvent = &apitype.PreludeEvent{
			Config: cfg,
		}

	case engine.SummaryEvent:
		p, ok := e.Payload.(engine.SummaryEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		// Convert the resource changes.
		changes := make(map[string]int)
		for op, count := range p.ResourceChanges {
			changes[string(op)] = count
		}
		apiEvent.SummaryEvent = &apitype.SummaryEvent{
			MaybeCorrupt:    p.MaybeCorrupt,
			DurationSeconds: int(p.Duration.Seconds()),
			ResourceChanges: changes,
		}

	case engine.ResourcePreEvent:
		p, ok := e.Payload.(engine.ResourcePreEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResourcePreEvent = &apitype.ResourcePreEvent{
			Metadata: convertStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		}

	case engine.ResourceOutputsEvent:
		p, ok := e.Payload.(engine.ResourceOutputsEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOutputsEvent = &apitype.ResOutputsEvent{
			Metadata: convertStepEventMetadata(p.Metadata),
			Planning: p.Planning,
		}

	case engine.ResourceOperationFailed:
		p, ok := e.Payload.(engine.ResourceOperationFailedPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.ResOpFailedEvent = &apitype.ResOpFailedEvent{
			Metadata: convertStepEventMetadata(p.Metadata),
			Status:   int(p.Status),
			Steps:    int(p.Steps),
		}

	default:
		return apiEvent, errors.Errorf("unknown event type %q", e.Type)
	}

	return apiEvent, nil
}
