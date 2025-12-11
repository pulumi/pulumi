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
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateIdentifier = client.UpdateIdentifier{
	StackIdentifier: client.StackIdentifier{
		Owner:   "org",
		Project: "project",
		Stack:   tokens.MustParseStackName("stack"),
	},
	UpdateKind: apitype.UpdateUpdate,
	UpdateID:   "update-id",
}

func startTestServer(t *testing.T, handler func(w http.ResponseWriter, body apitype.JournalEntries)) *client.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		raw, err := gzip.NewReader(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var body apitype.JournalEntries
		if err := json.NewDecoder(raw).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		handler(w, body)
	}))
	t.Cleanup(srv.Close)

	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{
		Color: colors.Never,
	})
	return client.NewClient(srv.URL, "test-token", true, sink)
}

// mockTokenSource is a mock implementation of tokenSourceCapability.
type mockTokenSource struct{}

func (m *mockTokenSource) GetToken(ctx context.Context) (string, error) {
	return "mock-token", nil
}

// TestJournalerBasicBatching tests that entries are collected and batched correctly.
func TestJournalerBasicBatching(t *testing.T) {
	t.Parallel()

	var batches []apitype.JournalEntries
	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		batches = append(batches, body)
		w.WriteHeader(http.StatusOK)
	})

	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		1, // Send each entry individually
		50,
	)

	// Add some journal entries
	numEntries := 5
	for i := 0; i < numEntries; i++ {
		entry := engine.JournalEntry{SequenceID: int64(i)}
		err := journaler.AddJournalEntry(entry)
		require.NoError(t, err)
	}

	// Close the journaler to flush all entries
	err := journaler.Close()
	require.NoError(t, err)

	// Verify that all entries were sent individually to the client
	require.Len(t, batches, numEntries)

	// Count total entries sent
	totalSent := 0
	for _, batch := range batches {
		totalSent += len(batch.Entries)
	}
	assert.Equal(t, numEntries, totalSent, "All entries should be sent")
}

// TestJournalerCloseWaitsForOperations tests that Close() waits for all operations to complete.
func TestJournalerCloseWaitsForOperations(t *testing.T) {
	t.Parallel()

	waiting, release := make(chan bool), make(chan bool)
	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		close(waiting)
		<-release
		w.WriteHeader(http.StatusOK)
	})

	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		1, // Send each entry individually
		50,
	)

	// Add a journal entry and then close the journal. This should block.
	go func() {
		err := journaler.AddJournalEntry(engine.JournalEntry{})
		require.NoError(t, err)
	}()

	// Wait for the request to come in, then release it in a goroutine and wait for the journal to close.
	<-waiting
	go close(release)
	err := journaler.Close()
	require.NoError(t, err)
}

// TestJournalerSendAfterClose tests that AddJournalEntry() after Close() returns an error.
func TestJournalerSendAfterClose(t *testing.T) {
	t.Parallel()

	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		w.WriteHeader(http.StatusOK)
	})

	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		1, // Send each entry individually
		50,
	)

	// Close the journaler.
	err := journaler.Close()
	require.NoError(t, err)

	// Attempt to send an entry. This should fail.
	err = journaler.AddJournalEntry(engine.JournalEntry{})
	require.Error(t, err)
}

// TestJournalerCloseAfterClose tests that Close() after Close() does not fail.
func TestJournalerCloseAfterClose(t *testing.T) {
	t.Parallel()

	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		w.WriteHeader(http.StatusOK)
	})

	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		1, // Send each entry individually
		50,
	)

	// Close the journaler thrice.
	err := journaler.Close()
	require.NoError(t, err)

	err = journaler.Close()
	require.NoError(t, err)

	err = journaler.Close()
	require.NoError(t, err)
}

// TestJournalerErrorHandling tests that errors from SaveJournalEntries are propagated.
func TestJournalerErrorHandling(t *testing.T) {
	t.Parallel()

	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		w.WriteHeader(http.StatusBadRequest)
	})

	numEntries := 5
	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		numEntries, // Send a single batch of entries
		10000000,
	)

	// Add some journal entries
	var wg sync.WaitGroup
	wg.Add(numEntries)
	errors := make([]error, numEntries)
	for i := 0; i < numEntries; i++ {
		go func() {
			defer wg.Done()
			entry := engine.JournalEntry{SequenceID: int64(i)}
			errors[i] = journaler.AddJournalEntry(entry)
		}()
	}
	wg.Wait()

	// Close the journaler to flush all entries
	err := journaler.Close()
	require.NoError(t, err)

	// Check the errors
	for _, err := range errors {
		assert.Error(t, err, "Expected a non-nil error")
	}
}

// TestJournaler413ErrorHandling tests that 413 errors trigger batch splitting.
func TestJournaler413ErrorHandling(t *testing.T) {
	t.Parallel()

	var batches []apitype.JournalEntries
	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		if len(body.Entries) > 1 {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		batches = append(batches, body)
		w.WriteHeader(http.StatusOK)
	})

	numEntries := 40
	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		numEntries/4,
		50,
	)

	// Add some journal entries
	var wg sync.WaitGroup
	wg.Add(numEntries)
	for i := 0; i < numEntries; i++ {
		go func() {
			defer wg.Done()
			entry := engine.JournalEntry{SequenceID: int64(i)}
			err := journaler.AddJournalEntry(entry)
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	// Close the journaler to flush all entries
	err := journaler.Close()
	require.NoError(t, err)

	// Verify that all entries were sent individually to the client
	require.Len(t, batches, numEntries)

	// Count total entries sent
	totalSent := 0
	for _, batch := range batches {
		totalSent += len(batch.Entries)
	}
	assert.Equal(t, numEntries, totalSent, "All entries should be sent")
}

// TestJournaler413MinimumBatchSize tests that 413 errors do not split when the batch size is too small.
func TestJournaler413MinimumBatchSize(t *testing.T) {
	t.Parallel()

	c := startTestServer(t, func(w http.ResponseWriter, body apitype.JournalEntries) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	})

	numEntries := 5
	journaler := newJournaler(
		t.Context(),
		c,
		updateIdentifier,
		&mockTokenSource{},
		b64.NewBase64SecretsManager(),
		numEntries,
		50,
	)

	// Add some journal entries
	var wg sync.WaitGroup
	wg.Add(numEntries)
	for i := 0; i < numEntries; i++ {
		go func() {
			defer wg.Done()
			entry := engine.JournalEntry{SequenceID: int64(i)}
			err := journaler.AddJournalEntry(entry)
			require.Error(t, err)
		}()
	}
	wg.Wait()

	// Close the journaler to flush all entries
	err := journaler.Close()
	require.NoError(t, err)
}

// mockTicker is a manually controlled ticker for testing.
type mockTicker struct {
	c      chan time.Time
	mu     sync.Mutex
	period time.Duration
}

func newMockTicker() *mockTicker {
	return &mockTicker{
		c: make(chan time.Time, 1),
	}
}

func (m *mockTicker) C() <-chan time.Time {
	return m.c
}

func (m *mockTicker) Stop() {
	// no-op for testing
}

func (m *mockTicker) Reset(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.period = d
}

func (m *mockTicker) Tick() {
	m.c <- time.Now()
}

// TestSendBatchesOneBatchAtATime ensures only one batch is being sent at a time.
func TestSendBatchesOneBatchAtATime(t *testing.T) {
	t.Parallel()

	entries := make(chan saveJournalEntry, 10)
	tick := newMockTicker()

	// Track concurrent sends
	var activeSends int32
	var maxConcurrentSends int32
	var sendCount int32
	var mu sync.Mutex
	sendDelay := 50 * time.Millisecond

	sender := func(batch []apitype.JournalEntry) error {
		current := atomic.AddInt32(&activeSends, 1)
		defer atomic.AddInt32(&activeSends, -1)

		mu.Lock()
		maxConcurrentSends = max(maxConcurrentSends, current)
		atomic.AddInt32(&sendCount, 1)
		mu.Unlock()

		// Simulate slow send to make concurrency issues more apparent
		time.Sleep(sendDelay)

		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sendBatches(1, 100*time.Millisecond, entries, sender, tick)
	}()

	results := make([]chan error, 3)
	for i := 0; i < 3; i++ {
		results[i] = make(chan error, 1)
		entries <- saveJournalEntry{
			entry:  apitype.JournalEntry{SequenceID: int64(i)},
			result: results[i],
		}
	}

	for _, result := range results {
		<-result
	}

	close(entries)
	<-done

	assert.EqualValues(t, 1, maxConcurrentSends, "Only one batch should be sent at a time")
	assert.EqualValues(t, 3, sendCount, "Expected 3 batches to be sent")
}

// TestSendBatchesSendsAfterTimerTick ensures batches are sent when the timer ticks.
func TestSendBatchesSendsAfterTimerTick(t *testing.T) {
	t.Parallel()

	entries := make(chan saveJournalEntry, 10)
	tick := newMockTicker()

	var batchesSent int32
	batches := [][]apitype.JournalEntry{}

	sender := func(batch []apitype.JournalEntry) error {
		batches = append(batches, batch)
		atomic.AddInt32(&batchesSent, 1)
		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sendBatches(5, 100*time.Millisecond, entries, sender, tick)
	}()

	result := make(chan error, 2)
	for i := 0; i < 2; i++ {
		entries <- saveJournalEntry{
			entry:  apitype.JournalEntry{SequenceID: int64(i)},
			result: result,
		}
	}

	// Spin until all entries have been received from the channel
	timeout := time.After(50 * time.Millisecond)
	for len(entries) > 0 {
		select {
		case <-timeout:
			break
		default:
			continue
		}
	}

	assert.Equal(t, int32(0), atomic.LoadInt32(&batchesSent), "No batches should be sent before timer tick")

	tick.Tick()

	timeout = time.After(200 * time.Millisecond)
	for range 2 {
		select {
		case <-result:
			t.Log("Received result for entry")
			continue
		case <-timeout:
			require.Fail(t, "Timed out waiting for batch send after timer tick")
		}
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&batchesSent), "One batch should be sent after timer tick")

	require.Len(t, batches, 1)
	require.Len(t, batches[0], 2, "Batch should contain 2 entries")

	close(entries)
	<-done
}

// TestSendBatchesWaitsForInFlightBatches ensures the function doesn't return before all in-flight batches are done.
func TestSendBatchesWaitsForInFlightBatches(t *testing.T) {
	t.Parallel()

	entries := make(chan saveJournalEntry, 10)
	tick := newMockTicker()

	sendStarted := make(chan struct{})
	sendComplete := make(chan struct{})
	sentJournalEntries := 0
	sentBatches := 0

	sender := func(batch []apitype.JournalEntry) error {
		if sentJournalEntries == 0 {
			close(sendStarted)
		}
		// Simulate slow send
		time.Sleep(200 * time.Millisecond)
		sentJournalEntries += len(batch)
		sentBatches += 1
		if sentJournalEntries >= 9 {
			close(sendComplete)
		}
		return nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sendBatches(3, 100*time.Millisecond, entries, sender, tick)
	}()

	for i := range 9 {
		result := make(chan error, 1)
		entries <- saveJournalEntry{
			entry:  apitype.JournalEntry{SequenceID: int64(i)},
			result: result,
		}
	}

	<-sendStarted

	close(entries)

	select {
	case <-done:
		t.Fatal("sendBatches returned before send completed")
	case <-time.After(10 * time.Millisecond):
	}

	<-sendComplete
	require.Equal(t, 9, sentJournalEntries, "All journal entries should be sent")
	require.Equal(t, 3, sentBatches, "Three batches should be sent")

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sendBatches did not return after send completed")
	}
}
