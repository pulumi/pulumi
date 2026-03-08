// Copyright 2025-2026, Pulumi Corporation.
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

package journal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

var _ engine.Journal = (*cloudJournaler)(nil)

type saveJournalEntry struct {
	entry  apitype.JournalEntry
	result chan<- error
}

type cloudJournaler struct {
	context context.Context         // The context to use for client requests.
	sm      secrets.Manager         // Secrets manager for encrypting values when serializing the journal entries.
	wg      sync.WaitGroup          // Wait group to ensure all operations are completed before closing.
	entries chan<- saveJournalEntry // Channel for sending journal entries to the batch worker.
	done    <-chan struct{}         // Channel for tracking whether or not the batch worker has finished.

	m      sync.Mutex // Controls access to the closed field and pendingElided map.
	closed bool       // True if the journaler is closed.

	// Map of operation IDs to elided entries that may have dependents. We use this to track elided entries
	// that need to be flushed to the service, and make sure any entries depending on the elided entries wait
	// for the elided entry to be flushed, or to return an error.
	//
	// Currently it can happen that an elided entry is in a batch that failed to be persisted, but the corresponding
	// step never gets an error. This can cause us to have subsequent entries that depend on such a failed entry. In
	// such a case the subsequent entry is not valid without the earlier entry being sent, and thus we need to return
	// an error.
	//
	// If we start sending entries concurrently in the future, this also makes sure that we wait for elided entries to be
	// persisted before sending any dependent entries, to avoid out-of-order sends.
	pendingElided map[int64]*promise.Promise[struct{}]
}

func (j *cloudJournaler) AddJournalEntry(entry engine.JournalEntry) error {
	// Check if this entry depends on any pending elided entries and wait for them first.
	dependsOn := []int64{}
	if entry.RemoveNew != nil && *entry.RemoveNew > 0 {
		dependsOn = append(dependsOn, *entry.RemoveNew)
	}
	if entry.DeleteNew != nil && *entry.DeleteNew > 0 {
		dependsOn = append(dependsOn, *entry.DeleteNew)
	}
	if entry.PendingReplacementNew != nil && *entry.PendingReplacementNew > 0 {
		dependsOn = append(dependsOn, *entry.PendingReplacementNew)
	}

	for _, depID := range dependsOn {
		j.m.Lock()
		elidedDep, hasDep := j.pendingElided[depID]
		j.m.Unlock()

		if hasDep {
			_, err := elidedDep.Result(j.context)
			if err != nil {
				return fmt.Errorf("dependent elided entry (operation %d) failed: %w", depID, err)
			}
		}
	}

	// Return an error if the journal is closed.
	//
	// Note that we also add this call to the j.wg under the lock to avoid races between this method and Close.
	err := func() error {
		j.m.Lock()
		defer j.m.Unlock()

		if j.closed {
			return errors.New("journal is already closed")
		}

		j.wg.Add(1)
		return nil
	}()
	if err != nil {
		return err
	}
	defer j.wg.Done()

	result := make(chan error, 1)

	serialized, err := stack.BatchEncrypt(
		j.context, j.sm, func(ctx context.Context, enc config.Encrypter,
		) (apitype.JournalEntry, error) {
			return backend.SerializeJournalEntry(ctx, entry, enc)
		})
	if err != nil {
		return fmt.Errorf("serializing journal entry: %w", err)
	}

	j.entries <- saveJournalEntry{
		entry:  serialized,
		result: result,
	}
	if entry.ElideWrite {
		promise := promise.Run(func() (struct{}, error) {
			return struct{}{}, <-result
		})
		j.m.Lock()
		j.pendingElided[entry.OperationID] = promise
		j.m.Unlock()

		return nil
	}
	return <-result
}

func (j *cloudJournaler) Close() error {
	j.m.Lock()
	if j.closed {
		j.m.Unlock()
		return nil
	}
	j.closed = true
	j.m.Unlock()

	j.wg.Wait()      // Wait for all operations to complete before closing.
	close(j.entries) // Notify the batch worker that there's nothing more to do.
	<-j.done         // Wait for the batch worker to finish.

	return nil
}

type tokenSourceCapability interface {
	GetToken(ctx context.Context) (string, error)
}

func sendBatch(
	ctx context.Context,
	client *client.Client,
	update client.UpdateIdentifier,
	tokenSource tokenSourceCapability,
	batch []apitype.JournalEntry,
) error {
	// Try to send the batch as-is. If there's no error or if the error is _not_ a 413 Content Too Large,
	// we're done. Otherwise, try to send two smaller batches. If the batch is too small to split, we're done.
	var apiError *apitype.ErrorResponse
	err := client.SaveJournalEntries(ctx, update, batch, tokenSource)
	if err == nil || !errors.As(err, &apiError) || apiError.Code != http.StatusRequestEntityTooLarge || len(batch) <= 1 {
		return err
	}

	logging.V(11).Infof("encountered a 413 sending a batch of %v journal entries; splitting batch", len(batch))
	if err = sendBatch(ctx, client, update, tokenSource, batch[:len(batch)/2]); err != nil {
		return err
	}
	return sendBatch(ctx, client, update, tokenSource, batch[len(batch)/2:])
}

// batchSend is a function type for sending batches of journal entries.
type batchSend = func(batch []apitype.JournalEntry) error

// ticker is an interface for time-based triggers.
type ticker interface {
	C() <-chan time.Time
	Stop()
	Reset(d time.Duration)
}

// realTicker wraps time.Ticker to implement the ticker interface.
type realTicker struct {
	*time.Ticker
}

func (t *realTicker) C() <-chan time.Time {
	return t.Ticker.C
}

// flushRequest represents a request to flush a batch of journal entries.
type flushRequest struct {
	batch   []apitype.JournalEntry
	results []chan<- error
}

// sendBatches reads journal entries off of the entries channel and sends batches when either the maximum batch size
// or the maximum period between batches is reached. Batches are sent sequentially.
func sendBatches(
	maxBatchSize int,
	period time.Duration,
	entries <-chan saveJournalEntry,
	sender batchSend,
	ticker ticker,
) {
	results := make([]chan<- error, 0, maxBatchSize)
	batch := make([]apitype.JournalEntry, 0, maxBatchSize)

	// Use a buffered channel, so we can queue up multiple batches
	// that can then be sent as quickly as the network allows. This
	// unblocks the engine and allows it to continue working.
	flushCh := make(chan flushRequest, 100)

	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		for req := range flushCh {
			if len(req.batch) != 0 {
				logging.V(11).Infof("flushing journal entries: len=%v, cap=%v", len(req.batch), cap(req.batch))

				err := sender(req.batch)
				for _, r := range req.results {
					r <- err
				}
			}
			ticker.Reset(period)
		}
	}()

	// Wait for the entries channel to close, a journal entry to arrive, or a periodic send. Then flush the current
	// batch if necessary.
	for {
		select {
		case req, ok := <-entries:
			if !ok {
				// Channel closed, we're done
				flushCh <- flushRequest{batch: batch, results: results}
				close(flushCh)
				<-flushDone
				return
			}

			batch = append(batch, req.entry)
			if req.result != nil {
				results = append(results, req.result)
			}
			if len(batch) == cap(batch) {
				ticker.Stop()
				flushCh <- flushRequest{batch: batch, results: results}
				batch = make([]apitype.JournalEntry, 0, maxBatchSize)
				results = make([]chan<- error, 0, maxBatchSize)
			}
		case <-ticker.C():
			if len(batch) > 0 {
				ticker.Stop()
				flushCh <- flushRequest{batch: batch, results: results}
				batch = make([]apitype.JournalEntry, 0, maxBatchSize)
				results = make([]chan<- error, 0, maxBatchSize)
			}
		}
	}
}

func newJournaler(
	ctx context.Context,
	client *client.Client,
	update client.UpdateIdentifier,
	tokenSource tokenSourceCapability,
	sm secrets.Manager,
	maxBatchSize int,
	period time.Duration,
) *cloudJournaler {
	// Start the batch worker.
	entries := make(chan saveJournalEntry, maxBatchSize)
	done := make(chan struct{})

	// Create the sender function that calls sendBatch with the context and client.
	sender := func(batch []apitype.JournalEntry) error {
		return sendBatch(ctx, client, update, tokenSource, batch)
	}

	// Create the ticker.
	tick := &realTicker{time.NewTicker(period)}

	go func() {
		defer close(done)

		sendBatches(maxBatchSize, period, entries, sender, tick)
	}()

	return &cloudJournaler{
		context:       ctx,
		sm:            sm,
		entries:       entries,
		done:          done,
		pendingElided: make(map[int64]*promise.Promise[struct{}]),
	}
}

func NewJournaler(
	ctx context.Context,
	client *client.Client,
	update client.UpdateIdentifier,
	tokenSource tokenSourceCapability,
	sm secrets.Manager,
) engine.Journal {
	maxBatchSize := env.JournalingBatchSize.Value()
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}

	periodMilliseconds := env.JournalingBatchPeriod.Value()
	if periodMilliseconds <= 0 {
		periodMilliseconds = 50
	}
	period := time.Duration(periodMilliseconds) * time.Millisecond

	return newJournaler(ctx, client, update, tokenSource, sm, maxBatchSize, period)
}
