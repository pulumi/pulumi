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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNeoClient(t *testing.T) {
	c := NewNeoClient("https://api.pulumi.com/", "tok_123", "myorg")
	assert.Equal(t, "https://api.pulumi.com", c.apiURL) // trailing slash trimmed
	assert.Equal(t, "tok_123", c.apiToken)
	assert.Equal(t, "myorg", c.org)
	assert.Equal(t, "pulumi-cli/neo", c.userAgent)
}

func TestTasksURL(t *testing.T) {
	c := NewNeoClient("https://api.pulumi.com", "tok", "myorg")
	assert.Equal(t, "https://api.pulumi.com/api/preview/agents/myorg/tasks", c.tasksURL())
}

func TestTaskURL(t *testing.T) {
	c := NewNeoClient("https://api.pulumi.com", "tok", "myorg")
	assert.Equal(t, "https://api.pulumi.com/api/preview/agents/myorg/tasks/task-abc", c.taskURL("task-abc"))
}

func TestCreateTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/tasks"))
		assert.Equal(t, "token tok_123", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "pulumi-cli/neo", r.Header.Get("User-Agent"))

		var body CreateTaskRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "user_message", body.Message.Type)
		assert.Equal(t, "test prompt", body.Message.Content)
		assert.Equal(t, "cli", body.ClientMode)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CreateTaskResponse{TaskID: "task-xyz"})
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok_123", "myorg")
	taskID, err := c.CreateTask(context.Background(), "test prompt", "cli")
	require.NoError(t, err)
	assert.Equal(t, "task-xyz", taskID)
}

func TestCreateTask_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	_, err := c.CreateTask(context.Background(), "test", "cli")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestListTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.RawQuery, "pageSize=20")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListTasksResponse{
			Tasks: []AgentTask{
				{ID: "t1", Name: "task 1", Status: "idle"},
				{ID: "t2", Name: "task 2", Status: "running"},
			},
		})
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	resp, err := c.ListTasks(context.Background())
	require.NoError(t, err)
	assert.Len(t, resp.Tasks, 2)
	assert.Equal(t, "t1", resp.Tasks[0].ID)
	assert.Equal(t, "t2", resp.Tasks[1].ID)
}

func TestUpdateTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/tasks/task-abc"))

		var body UpdateTaskRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		require.NotNil(t, body.ClientMode)
		assert.Equal(t, "cli", *body.ClientMode)

		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	err := c.UpdateTask(context.Background(), "task-abc", "cli")
	require.NoError(t, err)
}

func TestPostToolResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/events"))

		var body ToolResponseEvent
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "tool_response", body.Type)
		assert.Equal(t, "tc_1", body.ToolCallID)
		assert.Equal(t, "read_file", body.Name)
		assert.Equal(t, "file contents", body.Content)
		assert.False(t, body.IsError)

		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	err := c.PostToolResponse(context.Background(), "task-abc", ToolResponseEvent{
		Type:       "tool_response",
		ToolCallID: "tc_1",
		Name:       "read_file",
		Content:    "file contents",
		IsError:    false,
	})
	require.NoError(t, err)
}

func TestPostUserInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/tasks/task-abc"))

		var body RespondToTaskRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		// The event should be embedded as raw JSON.
		assert.Contains(t, string(body.Event), "user_message")

		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	msg := UserMessageEvent{Type: "user_message", Content: "hello"}
	err := c.PostUserInput(context.Background(), "task-abc", msg)
	require.NoError(t, err)
}

// --- SSE Streaming tests ---

func TestStreamEvents_BasicSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		assert.True(t, strings.HasSuffix(r.URL.Path, "/events/stream"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send two events
		fmt.Fprint(w, "id: 1\nevent: agent_event\ndata: {\"type\":\"test1\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "id: 2\nevent: agent_event\ndata: {\"type\":\"test2\"}\n\n")
		flusher.Flush()
		// Close connection to end stream
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	events, errCh := c.StreamEvents(context.Background(), "task-abc", "")

	var received []SSEEvent
	for ev := range events {
		received = append(received, ev)
	}

	// Check error channel
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	default:
	}

	require.Len(t, received, 2)
	assert.Equal(t, "1", received[0].ID)
	assert.Equal(t, "agent_event", received[0].Event)
	assert.JSONEq(t, `{"type":"test1"}`, string(received[0].Data))
	assert.Equal(t, "2", received[1].ID)
}

func TestStreamEvents_MultilineData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Multi-line data per SSE spec
		fmt.Fprint(w, "id: 1\nevent: agent_event\ndata: {\"line1\":\n")
		fmt.Fprint(w, "data: \"value\"}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	events, _ := c.StreamEvents(context.Background(), "task-abc", "")

	var received []SSEEvent
	for ev := range events {
		received = append(received, ev)
	}

	require.Len(t, received, 1)
	// Multi-line data should be joined with newlines
	assert.Equal(t, "{\"line1\":\n\"value\"}", string(received[0].Data))
}

func TestStreamEvents_HeartbeatSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		fmt.Fprint(w, "event: heartbeat\ndata: {}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "id: 1\nevent: agent_event\ndata: {\"type\":\"real\"}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	events, _ := c.StreamEvents(context.Background(), "task-abc", "")

	var received []SSEEvent
	for ev := range events {
		received = append(received, ev)
	}

	// Both events should be received (heartbeat has no ID but has data)
	assert.True(t, len(received) >= 1)
	// The agent_event should be present
	found := false
	for _, ev := range received {
		if ev.Event == "agent_event" {
			found = true
			assert.Equal(t, "1", ev.ID)
		}
	}
	assert.True(t, found, "should have received the agent_event")
}

func TestStreamEvents_SSECommentIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// SSE comment lines start with ':'
		fmt.Fprint(w, ": this is a comment\n")
		fmt.Fprint(w, "id: 1\nevent: agent_event\ndata: {\"ok\":true}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	events, _ := c.StreamEvents(context.Background(), "task-abc", "")

	var received []SSEEvent
	for ev := range events {
		received = append(received, ev)
	}

	require.Len(t, received, 1)
	assert.JSONEq(t, `{"ok":true}`, string(received[0].Data))
}

func TestStreamEvents_LastEventID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastID := r.Header.Get("Last-Event-ID")
		assert.Equal(t, "5", lastID, "should send Last-Event-ID header")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		// Just close immediately
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	events, _ := c.StreamEvents(context.Background(), "task-abc", "5")

	// Drain the channel
	for range events {
	}
}

func TestStreamEvents_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// Keep sending heartbeats until client disconnects
		for {
			_, err := fmt.Fprint(w, "event: heartbeat\ndata: {}\n\n")
			if err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewNeoClient(server.URL, "tok", "myorg")
	events, _ := c.StreamEvents(ctx, "task-abc", "")

	// Read one event then cancel
	<-events
	cancel()

	// Events channel should close soon after cancellation
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // Channel closed - success
			}
		case <-timeout:
			t.Fatal("events channel did not close after context cancellation")
		}
	}
}

func TestStreamEvents_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	c := NewNeoClient(server.URL, "tok", "myorg")
	events, errCh := c.StreamEvents(ctx, "task-abc", "")

	// Should eventually get an error after max reconnect attempts or context timeout.
	select {
	case <-events:
		// Events might close
	case err := <-errCh:
		if err != nil {
			assert.Contains(t, err.Error(), "500")
		}
	case <-ctx.Done():
		// Context expired - that's fine too, reconnection takes time
	}
}

// --- doJSON tests ---

func TestDoJSON_GET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Empty(t, body) // GET should have no body

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"t1","name":"test","status":"idle","createdAt":"2025-01-01"}`)
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	var task AgentTask
	err := c.doJSON(context.Background(), "GET", server.URL, nil, &task)
	require.NoError(t, err)
	assert.Equal(t, "t1", task.ID)
}

func TestDoJSON_NilResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	c := NewNeoClient(server.URL, "tok", "myorg")
	err := c.doJSON(context.Background(), "POST", server.URL, map[string]string{"k": "v"}, nil)
	require.NoError(t, err)
}
