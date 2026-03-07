// Copyright 2016-2025, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- validateApprovalMode tests ---

func TestValidateApprovalMode_Valid(t *testing.T) {
	for _, mode := range ValidApprovalModes {
		err := validateApprovalMode(mode)
		assert.NoError(t, err, "mode %q should be valid", mode)
	}
}

func TestValidateApprovalMode_Invalid(t *testing.T) {
	err := validateApprovalMode("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid approval mode")
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidateApprovalMode_Empty(t *testing.T) {
	err := validateApprovalMode("")
	require.Error(t, err)
}

// --- handleApproval tests ---

func TestHandleApproval_AutoAlwaysApproves(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "high",
	}

	mode := "auto"
	result := handleApproval(d, parsed, &mode)
	assert.True(t, result)
}

func TestHandleApproval_BalancedApprovesLow(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "low",
	}

	mode := "balanced"
	result := handleApproval(d, parsed, &mode)
	assert.True(t, result)
}

func TestHandleApproval_BalancedApprovesEmpty(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "",
	}

	mode := "balanced"
	result := handleApproval(d, parsed, &mode)
	assert.True(t, result)
}

func TestHandleApproval_BalancedPromptsHigh(t *testing.T) {
	// Non-interactive display, so PromptApproval will return an error → false.
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "high",
	}

	mode := "balanced"
	result := handleApproval(d, parsed, &mode)
	assert.False(t, result) // Non-interactive can't approve
}

func TestHandleApproval_BalancedPromptsDestructive(t *testing.T) {
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "destructive",
	}

	mode := "balanced"
	result := handleApproval(d, parsed, &mode)
	assert.False(t, result)
}

func TestHandleApproval_ManualAlwaysPrompts(t *testing.T) {
	// Non-interactive display → prompt fails → returns false.
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "low",
	}

	mode := "manual"
	result := handleApproval(d, parsed, &mode)
	assert.False(t, result)
}

func TestHandleApproval_ManualPromptsEvenForLow(t *testing.T) {
	// Verify manual mode doesn't auto-approve low sensitivity.
	// With interactive mode but input "n", should deny.
	d := NewDisplay(nil, nil, nil, false, false)
	parsed := &ParsedEvent{
		Type:        "user_approval_request",
		Sensitivity: "low",
	}

	// Manual mode always prompts - non-interactive means it returns false.
	mode := "manual"
	result := handleApproval(d, parsed, &mode)
	assert.False(t, result, "manual mode should prompt even for low sensitivity")
}

// --- ValidApprovalModes ---

func TestValidApprovalModes(t *testing.T) {
	assert.Contains(t, ValidApprovalModes, "auto")
	assert.Contains(t, ValidApprovalModes, "balanced")
	assert.Contains(t, ValidApprovalModes, "manual")
	assert.Len(t, ValidApprovalModes, 3)
}

// --- postToolResponseWithRetry tests ---

func TestPostToolResponseWithRetry_SucceedsOnFirstAttempt(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(201)
	}))
	defer server.Close()

	client := NewNeoClient(server.URL, "tok", "myorg")
	display := NewDisplay(nil, nil, nil, false, false)
	result := ToolResponseEvent{
		Type:       "tool_response",
		ToolCallID: "tc_1",
		Name:       "read_file",
		Content:    "ok",
	}

	err := postToolResponseWithRetry(context.Background(), client, "task-1", result, display)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestPostToolResponseWithRetry_RetriesOnFailure(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n < 3 {
			w.WriteHeader(500)
			fmt.Fprint(w, "internal server error")
			return
		}
		w.WriteHeader(201)
	}))
	defer server.Close()

	client := NewNeoClient(server.URL, "tok", "myorg")
	display := NewDisplay(nil, nil, nil, false, false)
	result := ToolResponseEvent{
		Type:       "tool_response",
		ToolCallID: "tc_1",
		Name:       "read_file",
		Content:    "ok",
	}

	err := postToolResponseWithRetry(context.Background(), client, "task-1", result, display)
	assert.NoError(t, err)

	mu.Lock()
	assert.Equal(t, 3, callCount, "should retry until success")
	mu.Unlock()
}

func TestPostToolResponseWithRetry_FailsAfterMaxRetries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(500)
		fmt.Fprint(w, "always fails")
	}))
	defer server.Close()

	client := NewNeoClient(server.URL, "tok", "myorg")
	display := NewDisplay(nil, nil, nil, false, false)
	result := ToolResponseEvent{
		Type:       "tool_response",
		ToolCallID: "tc_1",
		Name:       "read_file",
		Content:    "ok",
	}

	err := postToolResponseWithRetry(context.Background(), client, "task-1", result, display)
	assert.Error(t, err)
	assert.Equal(t, 3, callCount, "should attempt exactly 3 times")
}

func TestPostToolResponseWithRetry_RespectsContextCancellation(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(500)
		fmt.Fprint(w, "fail")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the retry backoff is interrupted.
	cancel()

	client := NewNeoClient(server.URL, "tok", "myorg")
	display := NewDisplay(nil, nil, nil, false, false)
	result := ToolResponseEvent{
		Type:       "tool_response",
		ToolCallID: "tc_1",
		Name:       "read_file",
		Content:    "ok",
	}

	err := postToolResponseWithRetry(ctx, client, "task-1", result, display)
	assert.Error(t, err)
	// Should exit early due to context cancellation.
	assert.LessOrEqual(t, callCount, 2)
}

// --- Tool response serialization tests ---

func TestToolResponsesSerialized(t *testing.T) {
	// Verify that when multiple tool results are produced concurrently,
	// PostToolResponse calls arrive at the server sequentially (not overlapping).
	var mu sync.Mutex
	activeRequests := 0
	maxConcurrent := 0
	var receivedOrder []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		activeRequests++
		if activeRequests > maxConcurrent {
			maxConcurrent = activeRequests
		}
		mu.Unlock()

		// Simulate some processing time to detect concurrent requests.
		time.Sleep(50 * time.Millisecond)

		var body ToolResponseEvent
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			mu.Lock()
			receivedOrder = append(receivedOrder, body.ToolCallID)
			mu.Unlock()
		}

		mu.Lock()
		activeRequests--
		mu.Unlock()

		w.WriteHeader(201)
	}))
	defer server.Close()

	client := NewNeoClient(server.URL, "tok", "myorg")
	display := NewDisplay(nil, nil, nil, false, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Simulate the poster goroutine that serializes PostToolResponse calls.
	toolExecCh := make(chan ToolResponseEvent, 16)
	toolPosterDoneCh := make(chan toolResult, 16)

	go func() {
		defer close(toolPosterDoneCh)
		for {
			select {
			case result, ok := <-toolExecCh:
				if !ok {
					return
				}
				postErr := postToolResponseWithRetry(ctx, client, "task-1", result, display)
				select {
				case toolPosterDoneCh <- toolResult{result: result, err: postErr}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Send 5 tool results "concurrently" (as goroutines would).
	for i := 0; i < 5; i++ {
		toolExecCh <- ToolResponseEvent{
			Type:       "tool_response",
			ToolCallID: fmt.Sprintf("tc_%d", i),
			Name:       "test_tool",
			Content:    fmt.Sprintf("result_%d", i),
		}
	}

	// Collect all results.
	for i := 0; i < 5; i++ {
		select {
		case tr := <-toolPosterDoneCh:
			assert.NoError(t, tr.err)
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for tool result")
		}
	}

	cancel()

	mu.Lock()
	defer mu.Unlock()

	// The poster goroutine serializes requests, so max concurrent should be 1.
	assert.Equal(t, 1, maxConcurrent,
		"PostToolResponse calls should be serialized (max 1 concurrent), got %d", maxConcurrent)
	assert.Len(t, receivedOrder, 5, "all 5 tool responses should have been posted")
}

// --- Live API integration test ---
//
// TestLiveSSE_ConcurrentToolResponses tests against a real API to verify that
// tool responses posted concurrently are all visible via the SSE stream.
//
// Run with:
//
//	PULUMI_LIVE_TEST=1 go test -v -run TestLiveSSE_ConcurrentToolResponses ./cmd/pulumi/neo/
//
// Set PULUMI_BACKEND_URL to point to a review stack (e.g., https://api-joeduffy.review-stacks.pulumi-dev.io).
// Requires valid Pulumi credentials (run `pulumi login` first).
func TestLiveSSE_ConcurrentToolResponses(t *testing.T) {
	if os.Getenv("PULUMI_LIVE_TEST") == "" {
		t.Skip("skipping live test; set PULUMI_LIVE_TEST=1 to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use the same credential resolution as the CLI.
	apiURL, apiToken, org, _, err := getBackendInfo(ctx, "")
	require.NoError(t, err, "must be logged in to run live test")

	client := NewNeoClient(apiURL, apiToken, org)

	// Create a task.
	taskID, err := client.CreateTask(ctx, "Test: concurrent tool responses", "cli-noninteractive")
	require.NoError(t, err, "creating task")
	t.Logf("Created task %s on %s (org=%s)", taskID, apiURL, org)

	// Start streaming events so we can verify all tool_responses appear.
	eventsCh, errCh := client.StreamEvents(ctx, taskID, "")

	// Wait a moment for the stream to establish.
	time.Sleep(500 * time.Millisecond)

	// Post tool_response events — test both concurrent and sequential.
	const numEvents = 5

	t.Run("sequential", func(t *testing.T) {
		for i := 0; i < numEvents; i++ {
			resp := ToolResponseEvent{
				Type:       "tool_response",
				ToolCallID: fmt.Sprintf("seq_tc_%d", i),
				Name:       "test_tool",
				Content:    fmt.Sprintf("sequential result %d", i),
			}
			err := client.PostToolResponse(ctx, taskID, resp)
			require.NoError(t, err, "posting sequential tool_response %d", i)
		}
		t.Logf("Posted %d sequential tool_response events", numEvents)
	})

	t.Run("concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make([]error, numEvents)
		for i := 0; i < numEvents; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				resp := ToolResponseEvent{
					Type:       "tool_response",
					ToolCallID: fmt.Sprintf("conc_tc_%d", i),
					Name:       "test_tool",
					Content:    fmt.Sprintf("concurrent result %d", i),
				}
				errs[i] = client.PostToolResponse(ctx, taskID, resp)
			}(i)
		}
		wg.Wait()
		for i, e := range errs {
			if e != nil {
				t.Logf("WARNING: concurrent tool_response %d failed: %v", i, e)
			}
		}
		t.Logf("Posted %d concurrent tool_response events", numEvents)
	})

	// Collect events from SSE stream and verify all tool_responses appear.
	// The server wraps events in AgentBackendEvent envelope, so tool_call_id
	// may be nested. We search for it anywhere in the raw JSON.
	seenToolCallIDs := make(map[string]bool)
	expectedIDs := make(map[string]bool)
	for i := 0; i < numEvents; i++ {
		expectedIDs[fmt.Sprintf("seq_tc_%d", i)] = true
		expectedIDs[fmt.Sprintf("conc_tc_%d", i)] = true
	}

	totalEventsReceived := 0

	// Poll for up to 15 seconds to see all events.
	deadline := time.After(15 * time.Second)
	for {
		select {
		case ev, ok := <-eventsCh:
			if !ok {
				t.Logf("SSE stream closed, seen %d/%d tool call IDs (%d total events)",
					len(seenToolCallIDs), len(expectedIDs), totalEventsReceived)
				goto done
			}
			totalEventsReceived++

			// Log every event for diagnostics.
			rawData := string(ev.Data)
			if len(rawData) > 200 {
				rawData = rawData[:200] + "..."
			}
			t.Logf("SSE event #%d: type=%s id=%s data=%s", totalEventsReceived, ev.Event, ev.ID, rawData)

			// Search for tool_call_id anywhere in the raw JSON data.
			// The server wraps tool_response events in AgentBackendEvent,
			// so the structure varies. We use a flexible approach.
			for id := range expectedIDs {
				if seenToolCallIDs[id] {
					continue
				}
				if strings.Contains(string(ev.Data), id) {
					seenToolCallIDs[id] = true
					t.Logf("  -> Found tool_call_id: %s", id)
				}
			}
			if len(seenToolCallIDs) >= len(expectedIDs) {
				goto done
			}

		case err := <-errCh:
			if err != nil {
				t.Logf("SSE error: %v", err)
			}
			goto done

		case <-deadline:
			t.Logf("Timed out waiting for events, seen %d/%d (%d total events received)",
				len(seenToolCallIDs), len(expectedIDs), totalEventsReceived)
			goto done
		}
	}

done:
	cancel()

	// Report results.
	var missing []string
	for id := range expectedIDs {
		if !seenToolCallIDs[id] {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Missing %d tool_response events from SSE stream (received %d total events): %v\n"+
			"This may indicate the AUTO_INCREMENT gap-skipping bug — concurrent event inserts "+
			"can cause events to be permanently skipped by cursor-based pagination.",
			len(missing), totalEventsReceived, missing)
	} else {
		t.Logf("All %d tool_response events were visible via SSE stream (%d total events)",
			len(expectedIDs), totalEventsReceived)
	}
}

