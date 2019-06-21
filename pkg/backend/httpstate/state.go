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
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
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

type cloudQuery struct {
	root   string
	proj   *workspace.Project
	target *deploy.Target
}

func (q *cloudQuery) GetRoot() string {
	return q.root
}

func (q *cloudQuery) GetProject() *workspace.Project {
	return q.proj
}

func (q *cloudQuery) GetTarget() *deploy.Target {
	return q.target
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

// recordEngineEvents will record the events with the Pulumi Service, enabling things like viewing
// the update logs or drilling into the timeline of an update.
func (u *cloudUpdate) recordEngineEvents(startingSeqNumber int, events []engine.Event) error {
	contract.Assert(u.tokenSource != nil)
	token, err := u.tokenSource.GetToken()
	if err != nil {
		return err
	}

	var apiEvents apitype.EngineEventBatch
	for idx, event := range events {
		apiEvent, convErr := convertEngineEvent(event)
		if convErr != nil {
			return errors.Wrap(convErr, "converting engine event")
		}

		// Each event within an update must have a unique sequence number. Any request to
		// emit an update with the same sequence number will fail. (Read: the caller needs
		// to be accurate about this.)
		apiEvent.Sequence = idx + startingSeqNumber
		apiEvent.Timestamp = int(time.Now().Unix())

		apiEvents.Events = append(apiEvents.Events, apiEvent)
	}

	return u.backend.client.RecordEngineEvents(u.context, u.update, apiEvents, token)
}

// RecordAndDisplayEvents inspects engine events from the given channel, and prints them to the CLI as well as
// posting them to the Pulumi service.
func (u *cloudUpdate) RecordAndDisplayEvents(
	label string, action apitype.UpdateKind, stackRef backend.StackReference, op backend.UpdateOperation,
	events <-chan engine.Event, done chan<- bool, opts display.Options, isPreview bool) {
	// We take the channel of engine events and pass them to separate components that will display
	// them to the console or persist them on the Pulumi Service. Both should terminate as soon as
	// they see a CancelEvent, and when finished, close the "done" channel.
	displayEvents := make(chan engine.Event, 100)
	displayEventsDone := make(chan bool)

	persistEvents := make(chan engine.Event, 100)
	persistEventsDone := make(chan bool)

	// We close our own done channel when both of the dependent components have finished.
	defer func() {
		<-displayEventsDone
		<-persistEventsDone
		close(done)
	}()

	// Start the Go-routines for displaying and persisting events.
	go display.ShowEvents(
		label, action, stackRef.Name(), op.Proj.Name,
		displayEvents, displayEventsDone, opts, isPreview)
	go persistEngineEvents(
		u, opts.Debug, /* persist debug events */
		persistEvents, persistEventsDone)

	for e := range events {
		displayEvents <- e
		persistEvents <- e

		// We stop reading from the event stream as soon as we see the CancelEvent,
		// which will also signal the display/persist components to shutdown too.
		if e.Type == engine.CancelEvent {
			break
		}
	}

	// Note that we don't return immediately, the defer'd function will block until
	// the display and persistence go-routines are finished processing events.
}

func (b *cloudBackend) newQuery(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (*cloudQuery, error) {
	// Construct the query target.
	target, err := b.getTarget(ctx, stackRef, op.StackConfiguration.Config, op.StackConfiguration.Decrypter)
	if err != nil {
		return nil, err
	}

	return &cloudQuery{root: op.Root, proj: op.Proj, target: target}, nil
}

func (b *cloudBackend) newUpdate(ctx context.Context, stackRef backend.StackReference, op backend.UpdateOperation,
	update client.UpdateIdentifier, token string) (*cloudUpdate, error) {

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
	target, err := b.getTarget(ctx, stackRef, op.StackConfiguration.Config, op.StackConfiguration.Decrypter)
	if err != nil {
		return nil, err
	}

	// Construct and return a new update.
	return &cloudUpdate{
		context:     ctx,
		backend:     b,
		update:      update,
		tokenSource: tokenSource,
		root:        op.Root,
		proj:        op.Proj,
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

func (b *cloudBackend) getTarget(ctx context.Context, stackRef backend.StackReference,
	cfg config.Map, dec config.Decrypter) (*deploy.Target, error) {
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
		Config:    cfg,
		Decrypter: dec,
		Snapshot:  snapshot,
	}, nil
}

func convertStepEventMetadata(md engine.StepEventMetadata) apitype.StepEventMetadata {
	keys := make([]string, len(md.Keys))
	for i, v := range md.Keys {
		keys[i] = string(v)
	}
	var diffs []string
	for _, v := range md.Diffs {
		diffs = append(diffs, string(v))
	}

	return apitype.StepEventMetadata{
		Op:   string(md.Op),
		URN:  string(md.URN),
		Type: string(md.Type),

		Old: convertStepEventStateMetadata(md.Old),
		New: convertStepEventStateMetadata(md.New),

		Keys:     keys,
		Diffs:    diffs,
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
// EngineEvent.{ Sequence, Timestamp } are expected to be set by the caller.
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

	case engine.PolicyViolationEvent:
		p, ok := e.Payload.(engine.PolicyViolationEventPayload)
		if !ok {
			return apiEvent, eventTypePayloadMismatch
		}
		apiEvent.PolicyEvent = &apitype.PolicyEvent{
			ResourceURN:       string(p.ResourceURN),
			Message:           p.Message,
			Color:             string(p.Color),
			PolicyName:        p.PolicyName,
			PolicyPackName:    p.PolicyPackName,
			PolicyPackVersion: p.PolicyPackVersion,
			EnforcementLevel:  string(p.EnforcementLevel),
		}

	case engine.PreludeEvent:
		p, ok := e.Payload.(engine.PreludeEventPayload)
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
			Steps:    p.Steps,
		}

	default:
		return apiEvent, errors.Errorf("unknown event type %q", e.Type)
	}

	return apiEvent, nil
}

func isDebugDiagEvent(e engine.Event) bool {
	return e.Type == engine.DiagEvent && (e.Payload.(engine.DiagEventPayload)).Severity == diag.Debug
}

// persistEngineEvents reads from a channel of engine events and persists them on the
// Pulumi Service. This is the data that powers the logs display.
func persistEngineEvents(
	update *cloudUpdate, persistDebugEvents bool,
	events <-chan engine.Event, done chan<- bool) {
	// A single update can emit hundreds, if not thousands, or tens of thousands of
	// engine events. We transmit engine events in large batches to reduce the overhead
	// associated with each HTTP request to the service. We also don't block on any
	// individual HTTP request, sending as many concurrent requests as needed, to avoid
	// slowing down the update.

	// Maximum number of events to batch up before transmitting.
	const maxEventsToTransmit = 50
	// Maximum wait time before sending all batched events.
	const maxTransmissionDelay = 4 * time.Second

	// We don't want to indicate that we are done processing every engine event in the
	// provided channel until every HTTP request has completed. We use a wait group to
	// track all of those requests.
	var wg sync.WaitGroup

	defer func() {
		wg.Wait()
		close(done)
	}()

	var eventBatch []engine.Event
	maxDelayTicker := time.NewTicker(maxTransmissionDelay)

	// We maintain a sequence counter for each event to ensure that the Pulumi Service can
	// ensure events can be reconstructured in the same order they were emitted. (And not
	// out of order from parallel writes and/or network delays.)
	eventIdx := 0

	// transmitBatch sends off a batch of engine events to be persisted. Will mutate the eventIdx
	// and eventBatch values.
	transmitBatch := func() {
		if len(eventBatch) == 0 {
			return
		}

		wg.Add(1)
		go func(startEventIdx int, eventsToRecord []engine.Event) {
			defer wg.Done()
			err := update.recordEngineEvents(startEventIdx, eventsToRecord)
			if err != nil {
				glog.V(3).Infof("error recording engine events: %s", err)
			}
		}(eventIdx, eventBatch)

		// With the values of eventIdx and eventBatch copied as params
		// into the Goroutine that will start the actual HTTP request,
		// we now modify their values for the next time transmitBatch
		// is called.
		eventIdx += len(eventBatch)
		eventBatch = nil
	}

	var sawCancelEvent bool
	for {
		select {
		case e := <-events:
			// Ignore debug events unless asked to.
			if isDebugDiagEvent(e) && !persistDebugEvents {
				break
			}

			// Stop processing once we see the CancelEvent.
			if e.Type == engine.CancelEvent {
				sawCancelEvent = true
				break
			}

			eventBatch = append(eventBatch, e)
			if len(eventBatch) >= maxEventsToTransmit {
				transmitBatch()
			}

		case <-maxDelayTicker.C:
			// If the ticker has fired, send any batched events. This sets an upper bound for
			// the delay between the event being observed and persisted.
			transmitBatch()
		}

		if sawCancelEvent {
			break
		}
	}

	// Transmit any lingering events. The defer'd function will wait for any
	// outstanding HTTP requests and close the done channel.
	transmitBatch()
}
