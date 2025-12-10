// Copyright 2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

	m      sync.Mutex // Controls access to the closed field.
	closed bool       // True if the journaler is closed.
}

func (j *cloudJournaler) AddJournalEntry(entry engine.JournalEntry) error {
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

	var result chan error
	if !entry.ElideWrite {
		result = make(chan error, 1)
	}

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
		contract.Assertf(result == nil, "expected elided write to have nil result channel")
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

// sendBatches reads journal entries off of the entries channel and sends batches when either the maximum batch size
// or the maximum period between batches is reached. Batches are sent sequentially.
func sendBatches(
	ctx context.Context,
	client *client.Client,
	update client.UpdateIdentifier,
	tokenSource tokenSourceCapability,
	maxBatchSize int,
	period time.Duration,
	entries <-chan saveJournalEntry,
) {
	ticker := time.NewTicker(period)

	results := make([]chan<- error, 0, maxBatchSize)
	batch := make([]apitype.JournalEntry, 0, maxBatchSize)
	flush := func() {
		if len(batch) != 0 {
			logging.V(11).Infof("flushing journal entries: len=%v, cap=%v", len(batch), cap(batch))

			err := sendBatch(ctx, client, update, tokenSource, batch)
			for _, r := range results {
				if r != nil {
					r <- err
				}
			}
			results, batch = results[:0], batch[:0]
		}
	}

	// Wait for the entries channel to close, a journal entry to arrive, or a periodic send. Then flush the current
	// batch if necessary.
	for {
		select {
		case req, ok := <-entries:
			if !ok {
				// Channel closed, we're done
				flush()
				return
			}

			batch, results = append(batch, req.entry), append(results, req.result)
			if cap(batch) == 0 {
				flush()
			}
		case <-ticker.C:
			flush()
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
	go func() {
		defer close(done)

		sendBatches(ctx, client, update, tokenSource, maxBatchSize, period, entries)
	}()

	return &cloudJournaler{
		context: ctx,
		sm:      sm,
		entries: entries,
		done:    done,
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
