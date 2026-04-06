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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStreamer struct {
	events chan []byte
	errs   chan error

	mu     sync.Mutex
	posted []CliToolResult
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
	f.posted = append(f.posted, body.(CliToolResult))
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

func TestSession_DispatchesCliToolRequestAndPostsResult(t *testing.T) {
	t.Parallel()

	streamer := newFakeStreamer()
	exec := NewExecutor()
	exec.Register("filesystem", &fakeHandler{
		wantMethod: "read",
		result:     map[string]any{"content": "hi"},
	})

	streamer.events <- mustAgentResponseEnvelope(t, CliToolRequest{
		Type:      backendEventCliToolRequest,
		Timestamp: time.Now().UTC(),
		ToolCalls: []CliToolCall{
			{ToolCallID: "c1", Name: "filesystem__read", Args: json.RawMessage(`{}`)},
		},
	})
	close(streamer.events)

	s := &Session{Client: streamer, Executor: exec, OrgName: "org", TaskID: "task"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	require.Len(t, streamer.posted, 1)
	got := streamer.posted[0]
	assert.Equal(t, userEventCliToolResult, got.Type)
	require.Len(t, got.ToolResults, 1)
	assert.Equal(t, "c1", got.ToolResults[0].ToolCallID)
	assert.False(t, got.ToolResults[0].IsError)

	// Round-trip the posted result through JSON to verify the wire shape: required
	// fields are present and entity_diff is an object, not omitted.
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var asMap map[string]any
	require.NoError(t, json.Unmarshal(raw, &asMap))
	assert.Equal(t, "cli_tool_result", asMap["type"])
	assert.Contains(t, asMap, "timestamp")
	assert.Contains(t, asMap, "entity_diff")
	assert.Contains(t, asMap, "tool_results")
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
		"type":      "agent_message",
		"timestamp": time.Now().UTC(),
		"content":   "hello",
	})
	close(streamer.events)

	s := &Session{Client: streamer, Executor: NewExecutor(), OrgName: "o", TaskID: "t"}
	require.NoError(t, s.Run(t.Context()))

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	assert.Empty(t, streamer.posted)
}
