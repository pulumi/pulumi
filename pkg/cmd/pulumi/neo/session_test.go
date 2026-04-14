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
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
)

type fakeHandler struct {
	wantMethod string
	result     any
	err        error
}

func (f *fakeHandler) Invoke(_ context.Context, method string, _ json.RawMessage) (any, error) {
	if method != f.wantMethod {
		return nil, errors.New("unexpected method " + method)
	}
	return f.result, f.err
}

type fakeStreamer struct {
	stream chan client.NeoStreamEvent

	mu     sync.Mutex
	posted []any
}

func newFakeStreamer() *fakeStreamer {
	return &fakeStreamer{
		stream: make(chan client.NeoStreamEvent, 8),
	}
}

func (f *fakeStreamer) StreamNeoTaskEvents(_ context.Context, _, _ string) (<-chan client.NeoStreamEvent, error) {
	return f.stream, nil
}

func (f *fakeStreamer) PostNeoTaskUserEvent(_ context.Context, _, _ string, body any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.posted = append(f.posted, body)
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
	handlers := map[string]ToolHandler{
		"filesystem": &fakeHandler{wantMethod: "read", result: map[string]any{"content": "hi"}},
	}

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
	streamer.stream <- client.NeoStreamEvent{Data: mustAgentResponseEnvelope(t, AssistantMessage{
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
	})}
	close(streamer.stream)

	s := &Session{Client: streamer, Handlers: handlers, OrgName: "org", TaskID: "task"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()

	// Expect: 1 exec_tool_call (for the cli call) + 1 tool_result.
	require.Len(t, streamer.posted, 2)

	// First event: exec_tool_call notification.
	execEvt, ok := streamer.posted[0].(ExecToolCallEvent)
	require.True(t, ok, "first posted event should be ExecToolCallEvent")
	assert.Equal(t, userEventExecToolCall, execEvt.Type)
	assert.Equal(t, "c1", execEvt.ToolCallID)
	assert.Equal(t, "filesystem__read", execEvt.Name)

	// Verify exec_tool_call wire shape: no args, no timestamp.
	raw, err := json.Marshal(execEvt)
	require.NoError(t, err)
	var execMap map[string]any
	require.NoError(t, json.Unmarshal(raw, &execMap))
	assert.Equal(t, "exec_tool_call", execMap["type"])
	assert.Contains(t, execMap, "tool_call_id")
	assert.Contains(t, execMap, "name")
	assert.NotContains(t, execMap, "args")
	assert.NotContains(t, execMap, "timestamp")

	// Second event: tool_result with the cli call's output.
	got, ok := streamer.posted[1].(ToolResultEvent)
	require.True(t, ok, "second posted event should be ToolResultEvent")
	assert.Equal(t, userEventToolResult, got.Type)
	require.Len(t, got.ToolResults, 1, "only the cli-marked call should be in the result")
	assert.Equal(t, "c1", got.ToolResults[0].ToolCallID)
	assert.False(t, got.ToolResults[0].IsError)

	// Verify tool_result wire shape.
	raw, err = json.Marshal(got)
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
	streamer.stream <- client.NeoStreamEvent{Data: mustAgentResponseEnvelope(t, AssistantMessage{
		Type: backendEventAssistantMessage,
		ToolCalls: []ToolCall{
			{ToolCallID: "c1", Name: "web_search__query", ExecutionMode: "cloud"},
		},
	})}
	close(streamer.stream)

	s := &Session{Client: streamer, Handlers: map[string]ToolHandler{}, OrgName: "o", TaskID: "t"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.Empty(t, streamer.posted)
}

func TestSession_MultipleCliCallsEmitExecToolCallPerCallThenOneResult(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()
	handlers := map[string]ToolHandler{
		"filesystem": &fakeHandler{wantMethod: "read", result: map[string]any{"content": "hello"}},
		"shell":      &fakeHandler{wantMethod: "run", result: map[string]any{"exit_code": 0}},
	}

	streamer.stream <- client.NeoStreamEvent{Data: mustAgentResponseEnvelope(t, AssistantMessage{
		Type:    backendEventAssistantMessage,
		IsFinal: true,
		ToolCalls: []ToolCall{
			{ToolCallID: "c1", Name: "filesystem__read", Args: json.RawMessage(`{}`), ExecutionMode: "cli"},
			{ToolCallID: "c2", Name: "shell__run", Args: json.RawMessage(`{}`), ExecutionMode: "cli"},
		},
	})}
	close(streamer.stream)

	s := &Session{Client: streamer, Handlers: handlers, OrgName: "org", TaskID: "task"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()

	// Expect: exec_tool_call(c1), exec_tool_call(c2), tool_result([c1, c2]).
	require.Len(t, streamer.posted, 3)

	exec1, ok := streamer.posted[0].(ExecToolCallEvent)
	require.True(t, ok, "posted[0] should be ExecToolCallEvent")
	assert.Equal(t, "c1", exec1.ToolCallID)
	assert.Equal(t, "filesystem__read", exec1.Name)

	exec2, ok := streamer.posted[1].(ExecToolCallEvent)
	require.True(t, ok, "posted[1] should be ExecToolCallEvent")
	assert.Equal(t, "c2", exec2.ToolCallID)
	assert.Equal(t, "shell__run", exec2.Name)

	result, ok := streamer.posted[2].(ToolResultEvent)
	require.True(t, ok, "posted[2] should be ToolResultEvent")
	assert.Equal(t, userEventToolResult, result.Type)
	require.Len(t, result.ToolResults, 2)
	assert.Equal(t, "c1", result.ToolResults[0].ToolCallID)
	assert.Equal(t, "c2", result.ToolResults[1].ToolCallID)
}

func TestSession_IgnoresUserInputAndUnknownBackendEvents(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()

	// userInput envelope — should be ignored.
	userInput, err := json.Marshal(ConsoleEventEnvelope{Type: "userInput", ID: "u1"})
	require.NoError(t, err)
	streamer.stream <- client.NeoStreamEvent{Data: userInput}

	// agentResponse with an unrelated backend event type.
	streamer.stream <- client.NeoStreamEvent{Data: mustAgentResponseEnvelope(t, map[string]any{
		"type":    "agent_message",
		"content": "hello",
	})}
	close(streamer.stream)

	s := &Session{Client: streamer, Handlers: map[string]ToolHandler{}, OrgName: "o", TaskID: "t"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.Empty(t, streamer.posted)
}

func TestSession_InvokeToolCallDispatchesBySplittingName(t *testing.T) {
	t.Parallel()

	s := &Session{
		Handlers: map[string]ToolHandler{
			"filesystem": &fakeHandler{wantMethod: "read", result: map[string]any{"content": "hello"}},
		},
	}
	item := s.invokeToolCall(t.Context(), ToolCall{
		ToolCallID: "c1", Name: "filesystem__read", Args: json.RawMessage(`{}`),
	})

	assert.Equal(t, "c1", item.ToolCallID)
	assert.Equal(t, "filesystem__read", item.Name)
	assert.False(t, item.IsError)
	assert.Equal(t, map[string]any{"content": "hello"}, item.Content)
}

func TestSession_InvokeToolCallUnknownServer(t *testing.T) {
	t.Parallel()

	s := &Session{Handlers: map[string]ToolHandler{}}
	item := s.invokeToolCall(t.Context(), ToolCall{ToolCallID: "c1", Name: "vcs__commit"})

	assert.True(t, item.IsError)
	assert.Contains(t, item.Content.(map[string]string)["error"], `tool "vcs" is not available`)
}

func TestSession_InvokeToolCallNameWithoutSeparator(t *testing.T) {
	t.Parallel()

	s := &Session{Handlers: map[string]ToolHandler{}}
	item := s.invokeToolCall(t.Context(), ToolCall{ToolCallID: "c1", Name: "bare_name"})

	assert.True(t, item.IsError)
	assert.Contains(t, item.Content.(map[string]string)["error"], "missing the server prefix")
}

func TestSession_InvokeToolCallHandlerError(t *testing.T) {
	t.Parallel()

	s := &Session{
		Handlers: map[string]ToolHandler{
			"shell": &fakeHandler{wantMethod: "shell_execute", err: errors.New("boom")},
		},
	}
	item := s.invokeToolCall(t.Context(), ToolCall{ToolCallID: "c", Name: "shell__shell_execute"})

	assert.True(t, item.IsError)
	assert.Equal(t, "boom", item.Content.(map[string]string)["error"])
}
