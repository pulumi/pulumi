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
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDisplay(interactive bool) (*Display, *bytes.Buffer, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader(""), interactive, false)
	return d, stdout, stderr
}

func newJSONDisplay() (*Display, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader(""), false, true)
	return d, stdout
}

func makeSSEEvent(eventType string, data interface{}) SSEEvent {
	dataBytes, _ := json.Marshal(data)
	return SSEEvent{
		ID:    "1",
		Event: eventType,
		Data:  json.RawMessage(dataBytes),
	}
}

func makeAgentEvent(beType string, be backendEvent) SSEEvent {
	eventBody, _ := json.Marshal(be)
	consoleEvent := agentEvent{
		Type:      "agentResponse",
		ID:        "evt-1",
		EventBody: eventBody,
	}
	return makeSSEEvent("agent_event", consoleEvent)
}

// --- ParseEvent tests ---

func TestParseEvent_HeartbeatReturnsNil(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	parsed := d.RenderEvent(SSEEvent{Event: "heartbeat", Data: json.RawMessage(`{}`)})
	assert.Nil(t, parsed)
}

func TestParseEvent_TaskStatusReturnsNil(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	parsed := d.RenderEvent(SSEEvent{Event: "task_status", Data: json.RawMessage(`{"status":"running"}`)})
	assert.Nil(t, parsed)
}

func TestParseEvent_UnknownEventReturnsNil(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	parsed := d.RenderEvent(SSEEvent{Event: "unknown_type", Data: json.RawMessage(`{}`)})
	assert.Nil(t, parsed)
}

func TestParseEvent_AssistantMessage(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("assistant_message", backendEvent{
		Type:    "assistant_message",
		Content: "Hello, I can help with that.",
		IsFinal: false,
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "assistant_message", parsed.Type)
	assert.Equal(t, "Hello, I can help with that.", parsed.Message)
	assert.False(t, parsed.IsFinal)
}

func TestParseEvent_AssistantMessageFinal(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("assistant_message", backendEvent{
		Type:    "assistant_message",
		Content: "All done!",
		IsFinal: true,
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.True(t, parsed.IsFinal)
}

func TestParseEvent_ExecToolCall(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("exec_tool_call", backendEvent{
		Type:       "exec_tool_call",
		ToolCallID: "tc_1",
		Name:       "read_file",
		Args:       json.RawMessage(`{"path":"Pulumi.yaml"}`),
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "exec_tool_call", parsed.Type)
	assert.Equal(t, "tc_1", parsed.ToolCallID)
	assert.Equal(t, "read_file", parsed.ToolName)
	assert.JSONEq(t, `{"path":"Pulumi.yaml"}`, string(parsed.ToolArgs))
}

func TestParseEvent_UserApprovalRequest(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("user_approval_request", backendEvent{
		Type:        "user_approval_request",
		ApprovalID:  "apr_1",
		Message:     "Neo wants to run: pulumi up",
		Sensitivity: "high",
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "user_approval_request", parsed.Type)
	assert.Equal(t, "apr_1", parsed.ApprovalID)
	assert.Equal(t, "Neo wants to run: pulumi up", parsed.Message)
	assert.Equal(t, "high", parsed.Sensitivity)
}

func TestParseEvent_Error(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("error", backendEvent{
		Type:    "error",
		Message: "something went wrong",
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "error", parsed.Type)
	assert.True(t, parsed.IsError)
	assert.Equal(t, "something went wrong", parsed.Message)
}

func TestParseEvent_Warning(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("warning", backendEvent{
		Type:    "warning",
		Message: "be careful",
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "warning", parsed.Type)
	assert.Equal(t, "be careful", parsed.Message)
}

func TestParseEvent_SetTaskName(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("set_task_name", backendEvent{
		Type:     "set_task_name",
		TaskName: "Upgrade Kubernetes",
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "set_task_name", parsed.Type)
	assert.Equal(t, "Upgrade Kubernetes", parsed.Message)
}

func TestParseEvent_Cancelled(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("cancelled", backendEvent{
		Type: "cancelled",
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "cancelled", parsed.Type)
	assert.True(t, parsed.IsFinal)
}

func TestParseEvent_ToolResponse(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("tool_response", backendEvent{
		Type:    "tool_response",
		Name:    "read_file",
		Content: "file contents here",
		IsError: false,
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.Equal(t, "tool_response", parsed.Type)
	assert.Equal(t, "read_file", parsed.ToolName)
	assert.False(t, parsed.IsError)
}

func TestParseEvent_ToolResponseError(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("tool_response", backendEvent{
		Type:    "tool_response",
		Name:    "read_file",
		Content: "file not found",
		IsError: true,
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	assert.True(t, parsed.IsError)
}

// --- Terminal rendering tests ---

func TestRenderTerminal_AssistantMessage(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	event := makeAgentEvent("assistant_message", backendEvent{
		Type:    "assistant_message",
		Content: "I'll read the file.",
	})

	d.RenderEvent(event)
	assert.Contains(t, stdout.String(), "Neo:")
	assert.Contains(t, stdout.String(), "I'll read the file.")
}

func TestRenderTerminal_ToolCallSpinner(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	event := makeAgentEvent("exec_tool_call", backendEvent{
		Type:       "exec_tool_call",
		ToolCallID: "tc_1",
		Name:       "read_file",
	})

	d.RenderEvent(event)
	assert.Contains(t, stdout.String(), "Reading file")
	assert.True(t, d.spinning)
	assert.Equal(t, "read_file", d.currentTool)
}

func TestRenderTerminal_SpinnerStop(t *testing.T) {
	d, _, _ := newTestDisplay(false)

	// Start spinner
	d.RenderEvent(makeAgentEvent("exec_tool_call", backendEvent{
		Type: "exec_tool_call", Name: "read_file",
	}))
	assert.True(t, d.spinning)

	// Stop spinner with tool response
	d.RenderEvent(makeAgentEvent("tool_response", backendEvent{
		Type: "tool_response", Name: "read_file",
	}))
	assert.False(t, d.spinning)
}

func TestRenderTerminal_ErrorToolResponse(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	event := makeAgentEvent("tool_response", backendEvent{
		Type:    "tool_response",
		Name:    "write_file",
		Content: "permission denied",
		IsError: true,
	})

	d.RenderEvent(event)
	assert.Contains(t, stderr.String(), "write_file")
	assert.Contains(t, stderr.String(), "permission denied")
}

func TestRenderTerminal_ApprovalRequest(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	event := makeAgentEvent("user_approval_request", backendEvent{
		Type:    "user_approval_request",
		Message: "Neo wants to run: pulumi up",
	})

	d.RenderEvent(event)
	assert.Contains(t, stdout.String(), "Approval required")
	assert.Contains(t, stdout.String(), "pulumi up")
}

func TestRenderTerminal_ErrorEvent(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	event := makeAgentEvent("error", backendEvent{
		Type:    "error",
		Message: "something went wrong",
	})

	d.RenderEvent(event)
	assert.Contains(t, stderr.String(), "Error:")
	assert.Contains(t, stderr.String(), "something went wrong")
}

func TestRenderTerminal_Cancelled(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	event := makeAgentEvent("cancelled", backendEvent{Type: "cancelled"})

	d.RenderEvent(event)
	assert.Contains(t, stdout.String(), "cancelled")
}

// --- JSON mode ---

func TestRenderJSON_OutputsRawEvent(t *testing.T) {
	d, stdout := newJSONDisplay()
	event := makeAgentEvent("assistant_message", backendEvent{
		Type:    "assistant_message",
		Content: "hello",
	})

	parsed := d.RenderEvent(event)
	require.NotNil(t, parsed)
	// JSON mode should output the raw SSE data
	assert.Contains(t, stdout.String(), "agentResponse")
}

// --- PromptApproval tests ---

func TestPromptApproval_Yes(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader("y\n"), true, false)

	approved, err := d.PromptApproval("run dangerous command?")
	require.NoError(t, err)
	assert.True(t, approved)
}

func TestPromptApproval_No(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader("n\n"), true, false)

	approved, err := d.PromptApproval("run dangerous command?")
	require.NoError(t, err)
	assert.False(t, approved)
}

func TestPromptApproval_EmptyIsNo(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader("\n"), true, false)

	approved, err := d.PromptApproval("run dangerous command?")
	require.NoError(t, err)
	assert.False(t, approved)
}

func TestPromptApproval_NonInteractiveFails(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader(""), false, false)

	approved, err := d.PromptApproval("run dangerous command?")
	require.Error(t, err)
	assert.False(t, approved)
	assert.Contains(t, err.Error(), "non-interactive")
}

func TestPromptApproval_YesFull(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader("yes\n"), true, false)

	approved, err := d.PromptApproval("approve?")
	require.NoError(t, err)
	assert.True(t, approved)
}

// --- PromptUserInput tests ---

func TestPromptUserInput(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader("hello neo\n"), true, false)

	input, err := d.PromptUserInput()
	require.NoError(t, err)
	assert.Equal(t, "hello neo", input)
}

func TestPromptUserInput_Trimmed(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader("  spaced  \n"), true, false)

	input, err := d.PromptUserInput()
	require.NoError(t, err)
	assert.Equal(t, "spaced", input)
}

func TestPromptUserInput_EOF(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	d := NewDisplay(stdout, stderr, strings.NewReader(""), true, false)

	_, err := d.PromptUserInput()
	require.Error(t, err)
}

// --- Formatting helpers ---

func TestTruncate(t *testing.T) {
	d, _, _ := newTestDisplay(false)

	assert.Equal(t, "hello", d.truncate("hello", 10))
	assert.Equal(t, "hel...", d.truncate("hello world", 3))
	assert.Equal(t, "", d.truncate("", 5))
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := SSEEvent{
		Event: "agent_event",
		Data:  json.RawMessage(`not valid json`),
	}
	parsed := d.RenderEvent(event)
	assert.Nil(t, parsed)
}

func TestParseEvent_UnknownBackendEventType(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("completely_unknown_type", backendEvent{
		Type: "completely_unknown_type",
	})
	parsed := d.RenderEvent(event)
	assert.Nil(t, parsed)
}

// --- Tool label tests ---

func TestToolLabel(t *testing.T) {
	d, _, _ := newTestDisplay(false)

	assert.Equal(t, "Reading file", d.toolLabel("read_file", nil))
	assert.Equal(t, "Reading Pulumi.yaml",
		d.toolLabel("read_file", json.RawMessage(`{"path":"Pulumi.yaml"}`)))
	assert.Equal(t, "Writing file", d.toolLabel("write_file", nil))
	assert.Equal(t, "Writing main.go",
		d.toolLabel("write_file", json.RawMessage(`{"path":"main.go"}`)))
	assert.Equal(t, "Running command", d.toolLabel("execute_command", nil))
	assert.Equal(t, "Running `echo hello`",
		d.toolLabel("execute_command", json.RawMessage(`{"command":"echo hello"}`)))
	assert.Equal(t, "Running pulumi preview", d.toolLabel("pulumi_preview", nil))
	assert.Equal(t, "Running pulumi up", d.toolLabel("pulumi_up", nil))
	assert.Equal(t, "Checking git status", d.toolLabel("git_status", nil))
	assert.Equal(t, "some_other_tool", d.toolLabel("some_other_tool", nil))
}

func TestExtractArg(t *testing.T) {
	assert.Equal(t, "", extractArg(nil, "path"))
	assert.Equal(t, "", extractArg(json.RawMessage(`{}`), "path"))
	assert.Equal(t, "hello", extractArg(json.RawMessage(`{"path":"hello"}`), "path"))
	assert.Equal(t, "", extractArg(json.RawMessage(`{"path":123}`), "path"))
	assert.Equal(t, "", extractArg(json.RawMessage(`not json`), "path"))
}

func TestRenderTerminal_ToolResponseWithLabel(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)

	// Start a spinner with args so the label includes the path.
	d.RenderEvent(makeAgentEvent("exec_tool_call", backendEvent{
		Type: "exec_tool_call", Name: "read_file",
		Args: json.RawMessage(`{"path":"Pulumi.yaml"}`),
	}))

	// Complete successfully — the label should carry through.
	d.RenderEvent(makeAgentEvent("tool_response", backendEvent{
		Type: "tool_response", Name: "read_file",
	}))

	assert.Contains(t, stdout.String(), "✓")
	assert.Contains(t, stdout.String(), "Reading Pulumi.yaml")
}

func TestRenderSessionStart(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderSessionStart("task-abc-123")
	assert.Contains(t, stderr.String(), "Neo session: task-abc-123")
}

func TestRenderSessionAttach(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderSessionAttach("task-abc-123")
	assert.Contains(t, stderr.String(), "Attached to session: task-abc-123")
}
