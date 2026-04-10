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
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStreamer struct {
	events chan []byte
	errs   chan error

	mu     sync.Mutex
	posted []ToolResultEvent
}

func newFakeStreamer() *fakeStreamer {
	return &fakeStreamer{
		events: make(chan []byte, 8),
		errs:   make(chan error, 1),
	}
}

func (f *fakeStreamer) StreamNeoTaskEvents(_ context.Context, _, _ string) (<-chan []byte, <-chan error, error) {
	return f.events, f.errs, nil
}

func (f *fakeStreamer) PostNeoTaskUserEvent(_ context.Context, _, _ string, body any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.posted = append(f.posted, body.(ToolResultEvent))
	return nil
}

func mustAgentResponseEnvelope(t *testing.T, inner any) []byte {
	t.Helper()
	body, err := json.Marshal(inner)
	require.NoError(t, err)
	out, err := json.Marshal(ConsoleEventEnvelope{
		Type:      consoleEventAgentResponse,
		ID:        "evt-1",
		EventBody: body,
	})
	require.NoError(t, err)
	return out
}

func TestSession_DispatchesCliMarkedToolCallsAndPostsResult(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()
	exec := NewExecutor()
	exec.Register("filesystem", &fakeHandler{
		wantMethod: "read",
		result:     map[string]any{"content": "hi"},
	})

	// Lock down the inbound wire shape: the discriminator must be the snake_case
	// "assistant_message" the service emits, and each tool call's identifier must
	// serialize under "id" (not "tool_call_id"). Both have drifted before and
	// silently broke the loop — assert here so it can't drift again.
	inbound, err := json.Marshal(AssistantMessage{
		Type:      backendEventAssistantMessage,
		ToolCalls: []ToolCall{{ToolCallID: "c1", Name: "filesystem__read"}},
	})
	require.NoError(t, err)
	var inboundMap map[string]any
	require.NoError(t, json.Unmarshal(inbound, &inboundMap))
	assert.Equal(t, "assistant_message", inboundMap["type"])
	calls, _ := inboundMap["tool_calls"].([]any)
	require.Len(t, calls, 1)
	call, _ := calls[0].(map[string]any)
	assert.Equal(t, "c1", call["id"])
	assert.NotContains(t, call, "tool_call_id")

	// Mixed-mode assistantMessage: one cli call (must be executed), one cloud call
	// (must be ignored — the agent runtime handles it).
	streamer.events <- mustAgentResponseEnvelope(t, AssistantMessage{
		Type:    backendEventAssistantMessage,
		IsFinal: true,
		ToolCalls: []ToolCall{
			{
				ToolCallID:    "c1",
				Name:          "filesystem__read",
				Args:          json.RawMessage(`{}`),
				ExecutionMode: "cli",
			},
			{
				ToolCallID:    "c2",
				Name:          "web_search__query",
				Args:          json.RawMessage(`{}`),
				ExecutionMode: "cloud",
			},
		},
	})
	close(streamer.events)

	s := &Session{Client: streamer, Executor: exec, OrgName: "org", TaskID: "task"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	require.Len(t, streamer.posted, 1)
	got := streamer.posted[0]
	assert.Equal(t, userEventToolResult, got.Type)
	require.Len(t, got.ToolResults, 1, "only the cli-marked call should be in the result")
	assert.Equal(t, "c1", got.ToolResults[0].ToolCallID)
	assert.False(t, got.ToolResults[0].IsError)

	// Verify the wire shape posted to the backend matches the new generic contract.
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var asMap map[string]any
	require.NoError(t, json.Unmarshal(raw, &asMap))
	assert.Equal(t, "tool_result", asMap["type"])
	assert.Contains(t, asMap, "tool_results")
	assert.NotContains(t, asMap, "entity_diff")
	assert.NotContains(t, asMap, "timestamp")
}

func TestSession_AssistantMessageWithoutCliCallsPostsNothing(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()

	// assistantMessage with only a cloud-marked call: the CLI must not respond at all,
	// otherwise the agent (which is not paused) would receive a stray tool_result.
	streamer.events <- mustAgentResponseEnvelope(t, AssistantMessage{
		Type: backendEventAssistantMessage,
		ToolCalls: []ToolCall{
			{ToolCallID: "c1", Name: "web_search__query", ExecutionMode: "cloud"},
		},
	})
	close(streamer.events)

	s := &Session{Client: streamer, Executor: NewExecutor(), OrgName: "o", TaskID: "t"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.Empty(t, streamer.posted)
}

func TestSession_IgnoresUserInputAndUnknownBackendEvents(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()

	// userInput envelope — should be ignored.
	userInput, err := json.Marshal(ConsoleEventEnvelope{Type: consoleEventUserInput, ID: "u1"})
	require.NoError(t, err)
	streamer.events <- userInput

	// agentResponse with an unrelated backend event type.
	streamer.events <- mustAgentResponseEnvelope(t, map[string]any{
		"type":    "agent_message",
		"content": "hello",
	})
	close(streamer.events)

	s := &Session{Client: streamer, Executor: NewExecutor(), OrgName: "o", TaskID: "t"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.Empty(t, streamer.posted)
}
