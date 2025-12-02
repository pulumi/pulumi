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
	"testing"

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

	// Close the journaler twice.
	err := journaler.Close()
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
