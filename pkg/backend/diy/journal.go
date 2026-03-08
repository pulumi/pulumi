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

package diy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"gocloud.dev/gcerrors"
	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	commonenv "github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// journalManifest is written to the journal directory before any journal entries.
// It records the base checkpoint hash so that crash recovery can detect whether
// entries are valid (not yet consolidated) or stale (already consolidated).
type journalManifest struct {
	EpochID            string `json:"epochID"`
	BaseCheckpointHash string `json:"baseCheckpointHash"`
	CreatedAt          string `json:"createdAt"`
}

// saveDIYEntry is an in-flight journal entry awaiting persistence to blob storage.
type saveDIYEntry struct {
	entry  apitype.JournalEntry
	result chan<- error
}

// diyBatchedJournaler implements engine.Journal with async batching.
//
// Entries are serialized on the calling goroutine and sent to a channel.
// A background worker collects them into batches and writes each batch to
// blob storage by issuing concurrent PutObject calls (one per entry in the batch).
//
// Non-ElideWrite entries block the calling goroutine until their result arrives,
// which preserves the engine's ordering guarantees. ElideWrite entries (no-op
// "same" steps) return immediately, allowing the engine to proceed without waiting.
// Resources processed in parallel by the engine naturally batch their entries together,
// amortising the per-batch latency across multiple resources.
type diyBatchedJournaler struct {
	// ctx is stored here because engine.Journal's AddJournalEntry and Close methods
	// do not accept a context parameter. This matches the cloudJournaler pattern.
	// nolint:containedctx
	ctx        context.Context
	sm         secrets.Manager
	bucket     Bucket
	journalDir string

	wg      sync.WaitGroup
	entries chan<- saveDIYEntry
	done    <-chan struct{}

	m             sync.Mutex
	closed        bool
	pendingElided map[int64]*promise.Promise[struct{}]
}

var _ engine.Journal = (*diyBatchedJournaler)(nil)

// AddJournalEntry implements engine.Journal.
// Mirrors the cloudJournaler pattern: elided entries are fire-and-forget;
// all others block until written to blob storage.
func (j *diyBatchedJournaler) AddJournalEntry(entry engine.JournalEntry) error {
	// If this entry depends on a pending elided entry, wait for that write first.
	// This ensures dependent entries are never written before their dependency.
	// Using a stack-allocated array (not a slice literal) avoids a heap allocation
	// on this hot path — every resource mutation calls AddJournalEntry.
	for _, depID := range [3]int64{
		ptrVal(entry.RemoveNew),
		ptrVal(entry.DeleteNew),
		ptrVal(entry.PendingReplacementNew),
	} {
		if depID <= 0 {
			continue
		}
		j.m.Lock()
		dep, hasDep := j.pendingElided[depID]
		j.m.Unlock()
		if hasDep {
			if _, err := dep.Result(j.ctx); err != nil {
				return fmt.Errorf("dependent elided entry (operation %d) failed: %w", depID, err)
			}
		}
	}

	// Grab the lock to check closed state and increment wg atomically.
	// This prevents a race where Close() calls wg.Wait() just before we add 1.
	if err := func() error {
		j.m.Lock()
		defer j.m.Unlock()
		if j.closed {
			return errors.New("journal is already closed")
		}
		j.wg.Add(1)
		return nil
	}(); err != nil {
		return err
	}
	defer j.wg.Done()

	result := make(chan error, 1)

	serialized, err := stack.BatchEncrypt(
		j.ctx, j.sm, func(ctx context.Context, enc config.Encrypter,
		) (apitype.JournalEntry, error) {
			return backend.SerializeJournalEntry(ctx, entry, enc)
		})
	if err != nil {
		return fmt.Errorf("serializing journal entry: %w", err)
	}

	j.entries <- saveDIYEntry{entry: serialized, result: result}

	if entry.ElideWrite {
		p := promise.Run(func() (struct{}, error) {
			return struct{}{}, <-result
		})
		j.m.Lock()
		j.pendingElided[entry.OperationID] = p
		j.m.Unlock()
		return nil
	}
	return <-result
}

// Close implements engine.Journal.
// Waits for all pending AddJournalEntry calls, then drains and closes the channel.
func (j *diyBatchedJournaler) Close() error {
	j.m.Lock()
	if j.closed {
		j.m.Unlock()
		return nil
	}
	j.closed = true
	j.m.Unlock()

	j.wg.Wait()      // wait for all AddJournalEntry calls to complete
	close(j.entries) // signal the batch worker to stop
	<-j.done         // wait for the batch worker to finish
	return nil
}

// ptrVal dereferences an *int64, returning 0 if nil.
func ptrVal(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// flushRequest is a batch of entries ready to be written to blob storage.
type flushRequest struct {
	batch   []apitype.JournalEntry
	results []chan<- error
}

// sendDIYBatches is the background worker for diyBatchedJournaler.
// It reads entries from the channel, collects them into batches by count or
// elapsed time, then dispatches each batch to the sender function.
//
// The sender writes all entries in a batch concurrently to blob storage,
// then notifies each entry's result channel. This two-stage pipeline (batch
// collector + concurrent writer) decouples the engine from blob I/O latency.
func sendDIYBatches(
	maxBatchSize int,
	period time.Duration,
	entries <-chan saveDIYEntry,
	sender func([]apitype.JournalEntry) error,
) {
	results := make([]chan<- error, 0, maxBatchSize)
	batch := make([]apitype.JournalEntry, 0, maxBatchSize)

	// Buffered flushCh allows the batch collector to queue multiple batches
	// while the sender is still processing the previous one.
	flushCh := make(chan flushRequest, 100)
	flushDone := make(chan struct{})

	go func() {
		defer close(flushDone)
		for req := range flushCh {
			if len(req.batch) != 0 {
				logging.V(11).Infof("diy journal: flushing batch of %d entries", len(req.batch))
				err := sender(req.batch)
				for _, r := range req.results {
					r <- err
				}
			}
		}
	}()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	flush := func() {
		flushCh <- flushRequest{batch: batch, results: results}
		batch = make([]apitype.JournalEntry, 0, maxBatchSize)
		results = make([]chan<- error, 0, maxBatchSize)
	}

	for {
		select {
		case req, ok := <-entries:
			if !ok {
				ticker.Stop()
				flush()
				close(flushCh)
				<-flushDone
				return
			}
			batch = append(batch, req.entry)
			if req.result != nil {
				results = append(results, req.result)
			}
			if len(batch) >= maxBatchSize {
				ticker.Stop()
				flush()
				ticker.Reset(period)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				ticker.Stop()
				flush()
				ticker.Reset(period)
			}
		}
	}
}

// makeDIYSender returns a batch sender that writes each entry in the batch to blob
// storage concurrently, using one PutObject per entry.
// Concurrent writes within a batch unlock S3/GCS parallel I/O throughput,
// reducing effective latency from (N * RTT) to approximately (1 * RTT) per batch.
func makeDIYSender(ctx context.Context, bucket Bucket, journalDir string) func([]apitype.JournalEntry) error {
	return func(batch []apitype.JournalEntry) error {
		var g errgroup.Group
		for _, entry := range batch {
			entry := entry
			g.Go(func() error {
				data, err := json.Marshal(entry)
				if err != nil {
					return fmt.Errorf("marshalling journal entry %d: %w", entry.SequenceID, err)
				}
				key := filepath.Join(journalDir, fmt.Sprintf("%010d.journal.json", entry.SequenceID))
				return bucket.WriteAll(ctx, key, data, nil)
			})
		}
		return g.Wait()
	}
}

// newDIYJournaler constructs an async diyBatchedJournaler backed by the given bucket.
// Batch size and period are configurable via the standard PULUMI_JOURNALING_* env vars.
func (b *diyBackend) newDIYJournaler(
	ctx context.Context,
	ref *diyBackendReference,
	sm secrets.Manager,
) *diyBatchedJournaler {
	journalDir := b.store.JournalDir(ref)

	maxBatchSize := commonenv.JournalingBatchSize.Value()
	if maxBatchSize <= 0 {
		maxBatchSize = 50
	}
	periodMs := commonenv.JournalingBatchPeriod.Value()
	if periodMs <= 0 {
		periodMs = 50
	}
	period := time.Duration(periodMs) * time.Millisecond

	entries := make(chan saveDIYEntry, maxBatchSize)
	done := make(chan struct{})
	sender := makeDIYSender(ctx, b.bucket, journalDir)

	go func() {
		defer close(done)
		sendDIYBatches(maxBatchSize, period, entries, sender)
	}()

	return &diyBatchedJournaler{
		ctx:           ctx,
		sm:            sm,
		bucket:        b.bucket,
		journalDir:    journalDir,
		entries:       entries,
		done:          done,
		pendingElided: make(map[int64]*promise.Promise[struct{}]),
	}
}

// writeJournalManifest writes a manifest.json to the journal directory before
// any journal entries are saved. The manifest records the SHA-256 hash of the
// current checkpoint, which is used during crash recovery to distinguish valid
// entries (not yet consolidated) from stale entries (already consolidated).
func (b *diyBackend) writeJournalManifest(ctx context.Context, ref *diyBackendReference) error {
	hash, err := b.checkpointHash(ctx, ref)
	if err != nil {
		return fmt.Errorf("computing checkpoint hash: %w", err)
	}

	epochID, err := uuid.NewV4()
	if err != nil {
		return fmt.Errorf("generating epoch ID: %w", err)
	}

	manifest := journalManifest{
		EpochID:            epochID.String(),
		BaseCheckpointHash: hash,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshalling journal manifest: %w", err)
	}

	key := filepath.Join(b.store.JournalDir(ref), "manifest.json")
	return b.bucket.WriteAll(ctx, key, data, nil)
}

// checkpointHash computes the SHA-256 hash of the raw checkpoint bytes for the
// given stack reference. Returns an empty string if no checkpoint file exists.
func (b *diyBackend) checkpointHash(ctx context.Context, ref *diyBackendReference) (string, error) {
	chkPath := b.stackPath(ctx, ref)
	byts, err := b.bucket.ReadAll(ctx, chkPath)
	if gcerrors.Code(err) == gcerrors.NotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading checkpoint for hash: %w", err)
	}
	h := sha256.Sum256(byts)
	return hex.EncodeToString(h[:]), nil
}

// recoverJournal runs at the start of every apply() to handle any incomplete
// journal left by a previously crashed operation.
//
// Recovery algorithm:
//  1. Read manifest.json; if absent, delete any orphan entry files and return.
//  2. Compare the current checkpoint hash to manifest.BaseCheckpointHash.
//  3. If they match, the checkpoint has not been consolidated yet — replay entries.
//  4. If they differ, consolidation already completed (crash during cleanup) — delete stale entries.
func (b *diyBackend) recoverJournal(ctx context.Context, ref *diyBackendReference) error {
	journalDir := b.store.JournalDir(ref)
	manifestKey := filepath.Join(journalDir, "manifest.json")

	manifestData, err := b.bucket.ReadAll(ctx, manifestKey)
	if gcerrors.Code(err) == gcerrors.NotFound {
		// No manifest: clean up any orphan .journal.json files and return.
		return b.clearJournalEntries(ctx, journalDir)
	}
	if err != nil {
		return fmt.Errorf("reading journal manifest: %w", err)
	}

	var manifest journalManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		// Corrupt manifest: best effort cleanup.
		logging.V(5).Infof("diy journal: corrupt manifest, cleaning up: %v", err)
		return b.clearJournal(ctx, journalDir)
	}

	currentHash, err := b.checkpointHash(ctx, ref)
	if err != nil {
		return fmt.Errorf("computing checkpoint hash during recovery: %w", err)
	}

	if currentHash != manifest.BaseCheckpointHash {
		// Hash mismatch: the checkpoint has already been consolidated (entries are stale).
		// This happens when consolidation succeeded but cleanup was interrupted.
		logging.V(5).Infof("diy journal: checkpoint hash mismatch — stale journal from epoch %s, cleaning up",
			manifest.EpochID)
		return b.clearJournal(ctx, journalDir)
	}

	// Hash matches: the entries reflect an operation that crashed before consolidation.
	logging.V(5).Infof("diy journal: recovering incomplete operation from epoch %s", manifest.EpochID)
	return b.consolidateJournal(ctx, ref)
}

// consolidateJournal replays all journal entries into the base checkpoint and
// writes the resulting deployment as the new checkpoint, then cleans up all
// journal files. It is called both at the end of a successful apply() and
// during crash recovery.
//
// Consolidation is idempotent: replaying the same entries against the same
// base checkpoint always produces the same result, so a crash during
// consolidation is safe to retry.
func (b *diyBackend) consolidateJournal(ctx context.Context, ref *diyBackendReference) error {
	journalDir := b.store.JournalDir(ref)

	entries, err := b.readJournalEntries(ctx, journalDir)
	if err != nil {
		return fmt.Errorf("reading journal entries: %w", err)
	}

	if len(entries) > 0 {
		// Read the base checkpoint (the pre-deploy state that journal entries were recorded against).
		// For brand-new stacks the checkpoint file does not exist yet; treat that as a nil base.
		chk, _, _, err := b.getCheckpoint(ctx, ref)
		if err != nil && gcerrors.Code(err) != gcerrors.NotFound {
			return fmt.Errorf("reading checkpoint for journal consolidation: %w", err)
		}

		var base *apitype.DeploymentV3
		if chk != nil {
			base = chk.Latest
		}

		replayer := backend.NewJournalReplayer(base)
		for _, entry := range entries {
			if err := replayer.Add(entry); err != nil {
				return fmt.Errorf("replaying journal entry (sequenceID=%d): %w", entry.SequenceID, err)
			}
		}

		deployment, err := replayer.GenerateDeployment()
		if err != nil {
			return fmt.Errorf("generating deployment from journal: %w", err)
		}

		if _, err := b.saveStack(ctx, ref, deployment); err != nil {
			return fmt.Errorf("saving consolidated checkpoint: %w", err)
		}
	}

	return b.clearJournal(ctx, journalDir)
}

// readJournalEntries reads all *.journal.json files from journalDir and returns
// them sorted by SequenceID (ascending), ready for replay.
func (b *diyBackend) readJournalEntries(ctx context.Context, journalDir string) ([]apitype.JournalEntry, error) {
	files, err := listBucket(ctx, b.bucket, journalDir)
	if err != nil {
		return nil, fmt.Errorf("listing journal entries: %w", err)
	}

	var entries []apitype.JournalEntry
	for _, file := range files {
		if file.IsDir {
			continue
		}
		name := objectName(file)
		if !strings.HasSuffix(name, ".journal.json") {
			continue
		}

		data, err := b.bucket.ReadAll(ctx, file.Key)
		if err != nil {
			return nil, fmt.Errorf("reading journal entry %s: %w", file.Key, err)
		}

		var entry apitype.JournalEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("parsing journal entry %s: %w", file.Key, err)
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SequenceID < entries[j].SequenceID
	})
	return entries, nil
}

// clearJournal deletes all files in journalDir (entries + manifest).
func (b *diyBackend) clearJournal(ctx context.Context, journalDir string) error {
	return removeAllByPrefix(ctx, b.bucket, journalDir)
}

// clearJournalEntries deletes only the *.journal.json entry files, leaving the
// manifest in place if present. Used when no manifest exists but orphan entries
// were found (e.g. from an older version that crashed before writing manifest).
func (b *diyBackend) clearJournalEntries(ctx context.Context, journalDir string) error {
	files, err := listBucket(ctx, b.bucket, journalDir)
	if err != nil {
		return nil // directory doesn't exist; nothing to clean up
	}

	var errs []error
	for _, file := range files {
		if file.IsDir {
			continue
		}
		if !strings.HasSuffix(objectName(file), ".journal.json") {
			continue
		}
		if err := b.bucket.Delete(ctx, file.Key); err != nil {
			errs = append(errs, fmt.Errorf("deleting orphan journal entry %s: %w", file.Key, err))
		}
	}
	return errors.Join(errs...)
}
