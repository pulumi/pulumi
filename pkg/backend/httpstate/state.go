// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pulumi/pulumi/pkg/v3/channel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// recordEngineEvents will record the events with the Pulumi Service, enabling things like viewing
// the update logs or drilling into the timeline of an update.
func (b *cloudBackend) recordEngineEvents(
	ctx context.Context,
	tokenSource *tokenSource,
	update client.UpdateIdentifier,
	startingSeqNumber int,
	events []engine.Event,
) error {
	contract.Assertf(tokenSource != nil, "cloud update requires a token source")

	var apiEvents apitype.EngineEventBatch
	for idx, event := range events {
		apiEvent, convErr := display.ConvertEngineEvent(event, false /* showSecrets */)
		if convErr != nil {
			return fmt.Errorf("converting engine event: %w", convErr)
		}

		// Each event within an update must have a unique sequence number. Any request to
		// emit an update with the same sequence number will fail. (Read: the caller needs
		// to be accurate about this.)
		apiEvent.Sequence = idx + startingSeqNumber
		apiEvent.Timestamp = int(time.Now().Unix())

		apiEvents.Events = append(apiEvents.Events, apiEvent)
	}

	return b.client.RecordEngineEvents(ctx, update, apiEvents, tokenSource)
}

// RecordAndDisplayEvents inspects engine events from the given channel, and prints them to the CLI as well as
// posting them to the Pulumi service.
func (b *cloudBackend) recordAndDisplayEvents(
	ctx context.Context, tokenSource *tokenSource, update client.UpdateIdentifier,
	label string, action apitype.UpdateKind, stackRef backend.StackReference, op backend.UpdateOperation,
	permalink string, events <-chan engine.Event, done chan<- bool, opts display.Options, isPreview bool,
) {
	// We take the channel of engine events and pass them to separate components that will display
	// them to the console or persist them on the Pulumi Service. Both should terminate as soon as
	// they see a CancelEvent, and when finished, close the "done" channel.
	displayEvents := make(chan engine.Event) // Note: unbuffered, but we assume it won't matter in practice.
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
		label, action, stackRef.Name(), op.Proj.Name, permalink,
		displayEvents, displayEventsDone, opts, isPreview)
	go b.persistEngineEvents(
		ctx, tokenSource, update,
		opts.Debug, /* persist debug events */
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

func RenewLeaseFunc(
	client *client.Client, update client.UpdateIdentifier, assumedExpires func() time.Time,
) func(ctx context.Context, duration time.Duration, currentToken string) (string, time.Time, error) {
	return func(ctx context.Context, duration time.Duration, currentToken string) (string, time.Time, error) {
		tok, err := client.RenewUpdateLease(
			ctx, update, currentToken, duration)
		if err != nil {
			// Translate 403 status codes to expired token errors to stop the token refresh loop.
			var apierr *apitype.ErrorResponse
			if errors.As(err, &apierr) && apierr.Code == 403 {
				return "", time.Time{}, expiredTokenError{err}
			}
			return "", time.Time{}, err
		}
		return tok, assumedExpires(), err
	}
}

func (b *cloudBackend) newUpdate(ctx context.Context, stackRef backend.StackReference, op backend.UpdateOperation,
	update client.UpdateIdentifier, token string,
) (engine.UpdateInfo, *tokenSource, error) {
	// Create a token source for this update if necessary.
	var tokenSource *tokenSource
	if token != "" {
		// TODO[pulumi/pulumi#10482] instead of assuming
		// expiration, consider expiration times returned by
		// the backend, if any.
		duration := 5 * time.Minute
		assumedExpires := func() time.Time {
			return time.Now().Add(duration)
		}

		renewLease := RenewLeaseFunc(b.Client(), update, assumedExpires)

		ts, err := newTokenSource(ctx, clockwork.NewRealClock(), token, assumedExpires(), duration, renewLease)
		if err != nil {
			return engine.UpdateInfo{}, nil, err
		}
		tokenSource = ts
	}

	// Construct the deployment target.
	target, err := b.getTarget(ctx, op.SecretsProvider, stackRef,
		op.StackConfiguration.Config, op.StackConfiguration.Decrypter)
	if err != nil {
		return engine.UpdateInfo{}, nil, err
	}

	info := engine.UpdateInfo{
		Root:    op.Root,
		Project: op.Proj,
		Target:  target,
	}

	return info, tokenSource, nil
}

func (b *cloudBackend) completeUpdate(
	ctx context.Context,
	tokenSource *tokenSource,
	update client.UpdateIdentifier,
	status apitype.UpdateStatus,
) error {
	defer tokenSource.Close()

	return b.client.CompleteUpdate(ctx, update, status, tokenSource)
}

func (b *cloudBackend) getSnapshot(ctx context.Context,
	secretsProvider secrets.Provider, stackRef backend.StackReference,
) (*deploy.Snapshot, error) {
	untypedDeployment, err := b.exportDeployment(ctx, stackRef, nil /* get latest */)
	if err != nil {
		return nil, err
	}

	snapshot, err := stack.DeserializeUntypedDeployment(ctx, untypedDeployment, secretsProvider)
	if err != nil {
		return nil, err
	}

	// Ensure the snapshot passes verification before returning it, to catch bugs early.
	if !backend.DisableIntegrityChecking {
		if err := snapshot.VerifyIntegrity(); err != nil {
			if sie, ok := deploy.AsSnapshotIntegrityError(err); ok {
				return nil, fmt.Errorf("snapshot integrity failure; refusing to use it: %w", sie.ForRead(snapshot))
			}

			return nil, fmt.Errorf("snapshot integrity failure; refusing to use it: %w", err)
		}
	}

	return snapshot, nil
}

func (b *cloudBackend) getTarget(ctx context.Context, secretsProvider secrets.Provider, stackRef backend.StackReference,
	cfg config.Map, dec config.Decrypter,
) (*deploy.Target, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	snapshot, err := b.getSnapshot(ctx, secretsProvider, stackRef)
	if err != nil {
		return nil, stack.FormatDeploymentDeserializationError(err, stackRef.Name().String())
	}

	return &deploy.Target{
		Name:         stackID.Stack,
		Organization: tokens.Name(stackID.Owner),
		Config:       cfg,
		Decrypter:    dec,
		Snapshot:     snapshot,
	}, nil
}

func isDebugDiagEvent(e engine.Event) bool {
	return e.Type == engine.DiagEvent && (e.Payload().(engine.DiagEventPayload)).Severity == diag.Debug
}

type engineEventBatch struct {
	sequenceStart int
	events        []engine.Event
}

// persistEngineEvents reads from a channel of engine events and persists them on the
// Pulumi Service. This is the data that powers the logs display.
func (b *cloudBackend) persistEngineEvents(
	ctx context.Context,
	tokenSource *tokenSource,
	update client.UpdateIdentifier,
	persistDebugEvents bool,
	events <-chan engine.Event,
	done chan<- bool,
) {
	// A single update can emit hundreds, if not thousands, or tens of thousands of
	// engine events. We transmit engine events in large batches to reduce the overhead
	// associated with each HTTP request to the service. We also send multiple HTTP
	// requests concurrently, as to not block processing subsequent engine events.

	// Maximum number of events to batch up before transmitting.
	const maxEventsToTransmit = 50
	// Maximum wait time before sending all batched events.
	const maxTransmissionDelay = 4 * time.Second
	// Maximum number of concurrent requests to the Pulumi Service to persist
	// engine events.
	const maxConcurrentRequests = 3

	// We don't want to indicate that we are done processing every engine event in the
	// provided channel until every HTTP request has completed. We use a wait group to
	// track all of those requests.
	var wg sync.WaitGroup

	defer func() {
		wg.Wait()
		close(done)
	}()

	// We need to filter the engine events here to exclude any internal and
	// ephemeral events, since these by definition should not be persisted.
	events = channel.FilterRead(events, func(e engine.Event) bool {
		return !e.Internal() && !e.Ephemeral()
	})

	var eventBatch []engine.Event
	maxDelayTicker := time.NewTicker(maxTransmissionDelay)

	// We maintain a sequence counter for each event to ensure that the Pulumi Service can
	// ensure events can be reconstructed in the same order they were emitted. (And not
	// out of order from parallel writes and/or network delays.)
	eventIdx := 0

	// As we identify batches of engine events to transmit, we put them into a channel.
	// This will allow us to issue HTTP requests concurrently, but also limit the maximum
	// number of requests in-flight at any one time.
	//
	// This channel isn't buffered, so adding a new batch of events to persist will block
	// until a go-routine is available to send the batch.
	batchesToTransmit := make(chan engineEventBatch)

	transmitBatchLoop := func() {
		defer wg.Done()

		for eventBatch := range batchesToTransmit {
			err := b.recordEngineEvents(ctx, tokenSource, update, eventBatch.sequenceStart, eventBatch.events)
			if err != nil {
				logging.V(3).Infof("error recording engine events: %s", err)
			}
		}
	}
	// Start N different go-routines which will all pull from the batchesToTransmit channel
	// and persist those engine events until the channel is closed.
	for i := 0; i < maxConcurrentRequests; i++ {
		wg.Add(1)
		go transmitBatchLoop()
	}

	// transmitBatch sends off the current batch of engine events (eventIdx, eventBatch) to the
	// batchesToTransmit channel. Will mutate eventIdx, eventBatch as a side effect.
	transmitBatch := func() {
		if len(eventBatch) == 0 {
			return
		}

		batch := engineEventBatch{
			sequenceStart: eventIdx,
			events:        eventBatch,
		}
		// This will block until one of the spawned go-routines is available to read the data.
		// Effectively providing a global rate limit for how quickly we can send data to the
		// Pulumi Service, if an update is particularly chatty.
		batchesToTransmit <- batch

		// With the values of eventIdx and eventBatch copied into engineEventBatch,
		// we now modify their values for the next time transmitBatch is called.
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

	// Transmit any lingering events.
	transmitBatch()
	// Closing the batchesToTransmit channel will signal the worker persistence routines to
	// terminate, which will trigger the `wg` WaitGroup to be marked as complete, which will
	// finally close the `done` channel so the caller knows we are finished processing the
	// engine event stream.
	close(batchesToTransmit)
}
