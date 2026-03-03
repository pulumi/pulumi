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
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// newJournalingTestBackend creates a diyBackend with journaling enabled, backed by a temp-dir file store.
func newJournalingTestBackend(t *testing.T) *diyBackend {
	t.Helper()
	stateDir := t.TempDir()
	store := make(env.MapStore)
	store[env.DIYBackendJournaling.Var().Name()] = "true"
	b, err := newDIYBackend(
		context.Background(),
		diagtest.LogSink(t),
		"file://"+filepath.ToSlash(stateDir),
		&workspace.Project{Name: tokens.PackageName("testproject")},
		&diyBackendOptions{Env: env.NewEnv(store)},
	)
	require.NoError(t, err)
	return b
}

// newTestRef parses and returns a *diyBackendReference for the given qualified stack name.
func newTestRef(t *testing.T, b *diyBackend, stackRef string) *diyBackendReference {
	t.Helper()
	ref, err := b.store.ParseReference(stackRef)
	require.NoError(t, err)
	return ref
}

// writeRawJournalEntry writes a pre-serialized apitype.JournalEntry directly to the bucket
// at the canonical path for the given journal directory and sequenceID.
// This bypasses the engine pipeline, allowing tests to set up journal state directly.
func writeRawJournalEntry(
	t *testing.T,
	ctx context.Context,
	b *diyBackend,
	journalDir string,
	entry apitype.JournalEntry,
) {
	t.Helper()
	data, err := json.Marshal(entry)
	require.NoError(t, err)
	key := filepath.Join(journalDir, fmt.Sprintf("%010d.journal.json", entry.SequenceID))
	err = b.bucket.WriteAll(ctx, key, data, nil)
	require.NoError(t, err)
}

// writeRawManifest writes a journalManifest directly to the bucket.
func writeRawManifest(t *testing.T, ctx context.Context, b *diyBackend, journalDir string, m journalManifest) {
	t.Helper()
	data, err := json.Marshal(m)
	require.NoError(t, err)
	key := filepath.Join(journalDir, "manifest.json")
	err = b.bucket.WriteAll(ctx, key, data, nil)
	require.NoError(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Batching logic
// ─────────────────────────────────────────────────────────────────────────────

// TestSendDIYBatches_FlushOnMaxSize verifies that sendDIYBatches groups
// entries into batches of exactly maxBatchSize when entries arrive faster
// than the flush timer fires.
func TestSendDIYBatches_FlushOnMaxSize(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var batches [][]apitype.JournalEntry
	sender := func(batch []apitype.JournalEntry) error {
		mu.Lock()
		batches = append(batches, append([]apitype.JournalEntry{}, batch...))
		mu.Unlock()
		return nil
	}

	entries := make(chan saveDIYEntry, 20)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendDIYBatches(3, time.Minute, entries, sender) // large timer: only size-based flushes
	}()

	// Send 7 entries: expect batches of [3, 3, 1]
	results := make([]chan error, 7)
	for i := range results {
		results[i] = make(chan error, 1)
		entries <- saveDIYEntry{
			entry:  apitype.JournalEntry{SequenceID: int64(i)},
			result: results[i],
		}
	}
	close(entries)
	<-done

	// All entries should have been acknowledged with nil errors.
	for i, r := range results {
		assert.NoError(t, <-r, "result channel %d should be nil", i)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, batches, 3, "expected 3 batches for 7 entries with maxBatchSize=3")
	assert.Len(t, batches[0], 3)
	assert.Len(t, batches[1], 3)
	assert.Len(t, batches[2], 1)
}

// TestSendDIYBatches_FlushOnTimer verifies that entries are flushed by the
// periodic timer even when the max batch size has not been reached.
func TestSendDIYBatches_FlushOnTimer(t *testing.T) {
	t.Parallel()

	flushed := make(chan []apitype.JournalEntry, 10)
	sender := func(batch []apitype.JournalEntry) error {
		flushed <- append([]apitype.JournalEntry{}, batch...)
		return nil
	}

	entries := make(chan saveDIYEntry, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendDIYBatches(100, time.Millisecond, entries, sender) // 1ms timer fires quickly
	}()

	result := make(chan error, 1)
	entries <- saveDIYEntry{
		entry:  apitype.JournalEntry{SequenceID: 42},
		result: result,
	}

	// Timer-triggered flush produces a single-entry batch.
	batch := <-flushed
	require.Len(t, batch, 1)
	assert.Equal(t, int64(42), batch[0].SequenceID)
	require.NoError(t, <-result)

	close(entries)
	<-done
}

// TestSendDIYBatches_SenderErrorPropagates verifies that a sender error is
// returned on all result channels in the failed batch.
func TestSendDIYBatches_SenderErrorPropagates(t *testing.T) {
	t.Parallel()

	sentinelErr := fmt.Errorf("blob write failed")
	sender := func(batch []apitype.JournalEntry) error {
		return sentinelErr
	}

	entries := make(chan saveDIYEntry, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		sendDIYBatches(10, time.Minute, entries, sender)
	}()

	r1 := make(chan error, 1)
	r2 := make(chan error, 1)
	entries <- saveDIYEntry{entry: apitype.JournalEntry{SequenceID: 1}, result: r1}
	entries <- saveDIYEntry{entry: apitype.JournalEntry{SequenceID: 2}, result: r2}
	close(entries)
	<-done

	assert.ErrorIs(t, <-r1, sentinelErr)
	assert.ErrorIs(t, <-r2, sentinelErr)
}

// ─────────────────────────────────────────────────────────────────────────────
// Manifest and crash-recovery logic
// ─────────────────────────────────────────────────────────────────────────────

// TestWriteJournalManifest verifies that writeJournalManifest creates a readable
// manifest file containing the current checkpoint hash.
func TestWriteJournalManifest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")

	// CreateStack writes an initial (empty) checkpoint, giving us a non-empty hash.
	_, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	err = b.writeJournalManifest(ctx, ref)
	require.NoError(t, err)

	// Read the manifest back directly from the bucket.
	journalDir := b.store.JournalDir(ref)
	manifestKey := filepath.Join(journalDir, "manifest.json")
	data, err := b.bucket.ReadAll(ctx, manifestKey)
	require.NoError(t, err)

	var got journalManifest
	require.NoError(t, json.Unmarshal(data, &got))
	assert.NotEmpty(t, got.EpochID)
	assert.NotEmpty(t, got.BaseCheckpointHash)
	assert.NotEmpty(t, got.CreatedAt)

	// The stored hash should match a fresh computation.
	want, err := b.checkpointHash(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, want, got.BaseCheckpointHash)
}

// TestRecoverJournal_NoManifest verifies that when no manifest exists, any
// orphan .journal.json files are cleaned up and recovery succeeds.
func TestRecoverJournal_NoManifest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")
	_, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	// Write orphan journal entry files (no manifest).
	journalDir := b.store.JournalDir(ref)
	writeRawJournalEntry(t, ctx, b, journalDir, apitype.JournalEntry{
		Version:    1,
		Kind:       apitype.JournalEntryKindBegin,
		SequenceID: 1,
		OperationID: 1,
	})
	writeRawJournalEntry(t, ctx, b, journalDir, apitype.JournalEntry{
		Version:    1,
		Kind:       apitype.JournalEntryKindFailure,
		SequenceID: 2,
		OperationID: 1,
	})

	err = b.recoverJournal(ctx, ref)
	require.NoError(t, err)

	// Journal directory should be empty — no manifest, no orphan entries.
	files, err := listBucket(ctx, b.bucket, journalDir)
	require.NoError(t, err)
	assert.Empty(t, files, "orphan journal entries should be deleted when no manifest exists")
}

// TestRecoverJournal_ValidEntries verifies that when the manifest hash matches
// the current checkpoint, recovery consolidates the journal and cleans up.
func TestRecoverJournal_ValidEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")
	_, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	// writeJournalManifest captures the current hash — this simulates a crash
	// that happened after the manifest was written but before consolidation.
	err = b.writeJournalManifest(ctx, ref)
	require.NoError(t, err)

	// Write zero journal entries — consolidation is a no-op but must succeed.
	// (A crash-before-any-entries scenario.)

	err = b.recoverJournal(ctx, ref)
	require.NoError(t, err)

	// After recovery, the journal directory should be empty.
	journalDir := b.store.JournalDir(ref)
	files, err := listBucket(ctx, b.bucket, journalDir)
	require.NoError(t, err)
	assert.Empty(t, files, "journal dir should be empty after successful recovery")
}

// TestRecoverJournal_StaleEntries verifies that when the manifest hash does NOT
// match the current checkpoint (consolidation already completed), recovery
// discards the stale journal files without replaying.
func TestRecoverJournal_StaleEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")
	_, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	// Write a manifest with a deliberately wrong hash (simulating a
	// consolidated checkpoint that was already written).
	journalDir := b.store.JournalDir(ref)
	writeRawManifest(t, ctx, b, journalDir, journalManifest{
		EpochID:            "stale-epoch",
		BaseCheckpointHash: "000000000000000000000000000000000000000000000000000000000000dead",
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	})
	// Also write some stale entry files.
	writeRawJournalEntry(t, ctx, b, journalDir, apitype.JournalEntry{
		Version:    1,
		Kind:       apitype.JournalEntryKindBegin,
		SequenceID: 1,
		OperationID: 1,
	})

	err = b.recoverJournal(ctx, ref)
	require.NoError(t, err)

	// Both manifest and stale entries should be gone.
	files, err := listBucket(ctx, b.bucket, journalDir)
	require.NoError(t, err)
	assert.Empty(t, files, "stale journal files should be deleted when hash mismatches")
}

// TestConsolidateJournal_NoEntries verifies that consolidation on an empty
// journal is a no-op: the checkpoint is unchanged and consolidation succeeds.
func TestConsolidateJournal_NoEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")
	_, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)

	// Record the checkpoint hash before consolidation.
	beforeHash, err := b.checkpointHash(ctx, ref)
	require.NoError(t, err)

	// Write the manifest (required before consolidation).
	err = b.writeJournalManifest(ctx, ref)
	require.NoError(t, err)

	// Consolidate with no entries.
	err = b.consolidateJournal(ctx, ref)
	require.NoError(t, err)

	// Checkpoint should be unchanged (no resources added/modified).
	afterHash, err := b.checkpointHash(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, beforeHash, afterHash, "checkpoint should be unchanged after consolidating zero entries")

	// Journal directory should be empty.
	journalDir := b.store.JournalDir(ref)
	files, err := listBucket(ctx, b.bucket, journalDir)
	require.NoError(t, err)
	assert.Empty(t, files, "journal dir should be empty after consolidation")
}

// TestConsolidateJournal_NewStack verifies that consolidation on a brand-new
// stack (no checkpoint file yet) succeeds without error. This is the regression
// test for the bug where getCheckpoint returned NotFound and was not handled.
func TestConsolidateJournal_NewStack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)

	// Use a ref for a stack that was NEVER created (no checkpoint file exists).
	ref := newTestRef(t, b, "organization/testproject/brandnewstack")

	// Write a Begin entry (simulates a first-ever deploy that crashed before
	// the checkpoint could be written).
	journalDir := b.store.JournalDir(ref)
	writeRawJournalEntry(t, ctx, b, journalDir, apitype.JournalEntry{
		Version:     1,
		Kind:        apitype.JournalEntryKindBegin,
		SequenceID:  1,
		OperationID: 1,
	})

	// Must not return an error even though no checkpoint file exists.
	err := b.consolidateJournal(ctx, ref)
	require.NoError(t, err, "consolidateJournal should handle missing checkpoint (new stack)")
}

// ─────────────────────────────────────────────────────────────────────────────
// Entry ordering
// ─────────────────────────────────────────────────────────────────────────────

// TestReadJournalEntries_OrderedBySequenceID verifies that readJournalEntries
// returns entries sorted by SequenceID regardless of blob listing order,
// since blob storage backends (S3, GCS) offer no ordering guarantee.
func TestReadJournalEntries_OrderedBySequenceID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")

	journalDir := b.store.JournalDir(ref)

	// Write entries in REVERSE sequence order.
	for seqID := int64(5); seqID >= 1; seqID-- {
		writeRawJournalEntry(t, ctx, b, journalDir, apitype.JournalEntry{
			Version:    1,
			Kind:       apitype.JournalEntryKindBegin,
			SequenceID: seqID,
		})
	}

	got, err := b.readJournalEntries(ctx, journalDir)
	require.NoError(t, err)
	require.Len(t, got, 5)

	for i, e := range got {
		assert.Equal(t, int64(i+1), e.SequenceID,
			"entry %d: expected SequenceID %d, got %d", i, i+1, e.SequenceID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Environment variable gating
// ─────────────────────────────────────────────────────────────────────────────

// TestDIYJournaler_WritesEntries verifies that the async diyBatchedJournaler
// writes entries to blob storage at the expected paths.
func TestDIYJournaler_WritesEntries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newJournalingTestBackend(t)
	ref := newTestRef(t, b, "organization/testproject/teststack")

	sm := b64.NewBase64SecretsManager()
	journaler := b.newDIYJournaler(ctx, ref, sm)

	// Add minimal journal entries (Begin + Failure — no resource state needed).
	err := journaler.AddJournalEntry(engine.JournalEntry{
		Kind:        engine.JournalEntryBegin,
		SequenceID:  1,
		OperationID: 1,
	})
	require.NoError(t, err)

	err = journaler.AddJournalEntry(engine.JournalEntry{
		Kind:        engine.JournalEntryFailure,
		SequenceID:  2,
		OperationID: 1,
	})
	require.NoError(t, err)

	// Close drains the channel and waits for all I/O to complete.
	err = journaler.Close()
	require.NoError(t, err)

	// Verify the .journal.json files were written at the correct paths.
	journalDir := b.store.JournalDir(ref)
	for _, seqID := range []int64{1, 2} {
		key := filepath.Join(journalDir, fmt.Sprintf("%010d.journal.json", seqID))
		exists, err := b.bucket.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists, "journal entry %d should exist at %s", seqID, key)
	}
}
