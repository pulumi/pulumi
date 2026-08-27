// Copyright 2026, Pulumi Corporation.
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

package neo

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postedCancels(f *fakeStreamer) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, p := range f.posted {
		if _, ok := p.(apitype.AgentUserEventCancel); ok {
			n++
		}
	}
	return n
}

// TestRunSessionWithInterrupt_FirstPostsCancelSecondExits verifies the
// two-stage interrupt: the first Ctrl+C posts a user_cancel and keeps the
// session running; the second ends it cleanly.
func TestRunSessionWithInterrupt_FirstPostsCancelSecondExits(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()
	var stderr bytes.Buffer
	s := &Session{Client: streamer, OrgName: "org", TaskID: "task"}
	interrupt := make(chan os.Signal, 1)

	done := make(chan error, 1)
	go func() { done <- runSessionWithInterrupt(t.Context(), s, interrupt, &stderr) }()

	interrupt <- os.Interrupt
	require.Eventually(t, func() bool { return postedCancels(streamer) == 1 },
		2*time.Second, 10*time.Millisecond, "first interrupt must post a user_cancel")
	select {
	case err := <-done:
		t.Fatalf("session exited on the first interrupt: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	interrupt <- os.Interrupt
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("second interrupt did not end the session")
	}
	assert.Equal(t, 1, postedCancels(streamer))
	assert.Contains(t, stderr.String(), "Cancelling...")
}

// TestRunSessionWithInterrupt_CancelRetriesThenGivesUp verifies a failing
// cancel post is retried under the TUI's policy and, once exhausted, ends the
// session rather than leaving the user without a way out.
//
//nolint:paralleltest // mutates userMessageRetryInitialBackoff
func TestRunSessionWithInterrupt_CancelRetriesThenGivesUp(t *testing.T) {
	streamer := newFakeStreamer()
	streamer.postErr = errors.New("boom")
	var stderr bytes.Buffer
	s := &Session{Client: streamer, OrgName: "org", TaskID: "task"}
	interrupt := make(chan os.Signal, 1)

	// Collapse the backoff so the retry budget is spent quickly.
	orig := userMessageRetryInitialBackoff
	userMessageRetryInitialBackoff = time.Millisecond
	t.Cleanup(func() { userMessageRetryInitialBackoff = orig })

	done := make(chan error, 1)
	go func() { done <- runSessionWithInterrupt(t.Context(), s, interrupt, &stderr) }()
	interrupt <- os.Interrupt

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("session did not exit after the cancel retry budget was spent")
	}
	assert.Equal(t, cancelMaxPostAttempts, postedCancels(streamer))
	assert.Contains(t, stderr.String(), "cancel not sent: boom")
}
