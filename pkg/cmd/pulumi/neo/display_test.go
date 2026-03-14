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
	"fmt"
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
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

var testEventSeq int

func makeSSEEvent(eventType string, data interface{}) SSEEvent {
	testEventSeq++
	dataBytes, _ := json.Marshal(data)
	return SSEEvent{
		ID:    fmt.Sprintf("%d", testEventSeq),
		Event: eventType,
		Data:  json.RawMessage(dataBytes),
	}
}

func makeAgentEvent(beType string, be backendEvent) SSEEvent {
	testEventSeq++
	eventBody, _ := json.Marshal(be)
	consoleEvent := agentEvent{
		Type:      "agentResponse",
		ID:        fmt.Sprintf("evt-%d", testEventSeq),
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

func TestParseEvent_TaskStatusRunningReturnsNil(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	parsed := d.RenderEvent(SSEEvent{Event: "task_status", Data: json.RawMessage(`{"status":"running"}`)})
	assert.Nil(t, parsed)
}

func TestParseEvent_TaskStatusFailed(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	parsed := d.RenderEvent(SSEEvent{Event: "task_status", Data: json.RawMessage(`{"status":"failed"}`)})
	assert.NotNil(t, parsed)
	assert.Equal(t, "error", parsed.Type)
	assert.True(t, parsed.IsError)
}

func TestParseEvent_TaskStatusIdle(t *testing.T) {
	d, _, _ := newTestDisplay(false)
	parsed := d.RenderEvent(SSEEvent{Event: "task_status", Data: json.RawMessage(`{"status":"idle"}`)})
	assert.NotNil(t, parsed)
	assert.Equal(t, "task_idle", parsed.Type)
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
		IsFinal: true,
	})

	d.RenderEvent(event)
	assert.Contains(t, stdout.String(), "Neo:")
	assert.Contains(t, stdout.String(), "I'll read the file.")
}

func TestRenderTerminal_StreamingAssistantMessage(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)

	// Simulate 3 partial events with increasing content.
	partials := []string{
		"Hello",
		"Hello, I can",
		"Hello, I can help with that.",
	}
	for _, text := range partials {
		event := makeAgentEvent("assistant_message", backendEvent{
			Type:    "assistant_message",
			Content: text,
			IsFinal: false,
		})
		parsed := d.RenderEvent(event)
		require.NotNil(t, parsed)
		assert.Equal(t, "assistant_message", parsed.Type)
		assert.False(t, parsed.IsFinal)
	}

	// Non-TTY mode: partials are silently consumed, stdout should be empty.
	assert.Empty(t, stdout.String())

	// Final event with complete content.
	finalEvent := makeAgentEvent("assistant_message", backendEvent{
		Type:    "assistant_message",
		Content: "Hello, I can help with that.",
		IsFinal: true,
	})
	parsed := d.RenderEvent(finalEvent)
	require.NotNil(t, parsed)
	assert.True(t, parsed.IsFinal)
	assert.Contains(t, stdout.String(), "Neo:")
	assert.Contains(t, stdout.String(), "Hello, I can help with that.")

	// Streaming state should be reset.
	assert.Empty(t, d.streamingText)
	assert.False(t, d.streamingStarted)
	assert.Equal(t, 0, d.streamingLines)
}

func TestRenderTerminal_ToolCallSpinner(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	event := makeAgentEvent("exec_tool_call", backendEvent{
		Type:       "exec_tool_call",
		ToolCallID: "tc_1",
		Name:       "read_file",
	})

	d.RenderEvent(event)
	// Non-TTY: prints label with * prefix.
	assert.Contains(t, stdout.String(), "Read")
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
	d, _, _ := newTestDisplay(false)
	event := makeAgentEvent("tool_response", backendEvent{
		Type:    "tool_response",
		Name:    "write_file",
		Content: "permission denied",
		IsError: true,
	})

	d.RenderEvent(event)
	// Tool responses are now silently counted; errors increment the error counter.
	assert.Equal(t, 1, d.toolCount)
	assert.Equal(t, 1, d.toolErrorCount)
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
	// toolLabel returns FuncName("arg") style labels.
	assert.Equal(t, "Read", toolLabel("read_file", nil))
	assert.Equal(t, "Read", toolLabel("read", nil))
	assert.Equal(t, `Read("Pulumi.yaml")`,
		toolLabel("read_file", json.RawMessage(`{"path":"Pulumi.yaml"}`)))
	assert.Equal(t, `Read("/home/user/project/Pulumi.yaml")`,
		toolLabel("read", json.RawMessage(`{"file_path":"/home/user/project/Pulumi.yaml"}`)))

	assert.Equal(t, "Write", toolLabel("write_file", nil))
	assert.Equal(t, "Write", toolLabel("write", nil))
	assert.Equal(t, `Write("main.go")`,
		toolLabel("write_file", json.RawMessage(`{"path":"main.go"}`)))
	assert.Equal(t, `Write("main.go")`,
		toolLabel("write", json.RawMessage(`{"file_path":"main.go"}`)))

	assert.Equal(t, "Edit", toolLabel("edit", nil))
	assert.Equal(t, `Edit("main.go")`,
		toolLabel("edit", json.RawMessage(`{"file_path":"main.go"}`)))

	assert.Equal(t, "Replace", toolLabel("content_replace", nil))

	assert.Equal(t, "Bash", toolLabel("execute_command", nil))
	assert.Equal(t, "Bash", toolLabel("shell_execute", nil))
	assert.Equal(t, `Bash("echo hello")`,
		toolLabel("execute_command", json.RawMessage(`{"command":"echo hello"}`)))

	assert.Equal(t, "Search", toolLabel("grep", nil))
	assert.Equal(t, "Search", toolLabel("search_files", nil))

	assert.Equal(t, `ListDirectory(".")`, toolLabel("directory_tree", nil))
	assert.Equal(t, "PulumiPreview", toolLabel("pulumi_preview", nil))
	assert.Equal(t, "PulumiUp", toolLabel("pulumi_up", nil))
	assert.Equal(t, `Bash("git status")`, toolLabel("git_status", nil))
	assert.Equal(t, "some_other_tool", toolLabel("some_other_tool", nil))
}

func TestToolLabelActive(t *testing.T) {
	// toolLabelActive now uses the same format as toolLabel.
	assert.Equal(t, "Read", toolLabelActive("read_file", nil))
	assert.Equal(t, `Read("Pulumi.yaml")`,
		toolLabelActive("read_file", json.RawMessage(`{"path":"Pulumi.yaml"}`)))
	assert.Equal(t, "Write", toolLabelActive("write_file", nil))
	assert.Equal(t, `Write("main.go")`,
		toolLabelActive("write", json.RawMessage(`{"file_path":"main.go"}`)))
	assert.Equal(t, "Edit", toolLabelActive("edit", nil))
	assert.Equal(t, "Bash", toolLabelActive("shell_execute", nil))
	assert.Equal(t, `Bash("echo hello")`,
		toolLabelActive("execute_command", json.RawMessage(`{"command":"echo hello"}`)))
	assert.Equal(t, "Search", toolLabelActive("grep", nil))
	assert.Equal(t, `ListDirectory(".")`, toolLabelActive("directory_tree", nil))
	assert.Equal(t, "PulumiPreview", toolLabelActive("pulumi_preview", nil))
}

func TestExtractArg(t *testing.T) {
	assert.Equal(t, "", extractArg(nil, "path"))
	assert.Equal(t, "", extractArg(json.RawMessage(`{}`), "path"))
	assert.Equal(t, "hello", extractArg(json.RawMessage(`{"path":"hello"}`), "path"))
	assert.Equal(t, "", extractArg(json.RawMessage(`{"path":123}`), "path"))
	assert.Equal(t, "", extractArg(json.RawMessage(`not json`), "path"))
}

func TestRenderTerminal_ToolResponseWithLabel(t *testing.T) {
	d, _, _ := newTestDisplay(false)

	// Start a spinner with args so the label includes the path.
	d.RenderEvent(makeAgentEvent("exec_tool_call", backendEvent{
		Type: "exec_tool_call", Name: "read_file",
		Args: json.RawMessage(`{"path":"Pulumi.yaml"}`),
	}))

	// Complete successfully — tool responses are now silently counted.
	d.RenderEvent(makeAgentEvent("tool_response", backendEvent{
		Type: "tool_response", Name: "read_file",
	}))

	assert.Equal(t, 1, d.toolCount)
	assert.Equal(t, 0, d.toolErrorCount)
}

func TestToolLogFlow_NonTTY(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)

	// Simulate: exec read → response → exec shell_execute → response → final message.
	d.RenderEvent(makeAgentEvent("exec_tool_call", backendEvent{
		Type: "exec_tool_call", Name: "read_file", ToolCallID: "tc1",
		Args: json.RawMessage(`{"path":"Pulumi.yaml"}`),
	}))
	assert.Contains(t, stdout.String(), `* Read("Pulumi.yaml")`)

	d.RenderEvent(makeAgentEvent("tool_response", backendEvent{
		Type: "tool_response", Name: "read_file", ToolCallID: "tc1",
	}))
	assert.Contains(t, stdout.String(), `o Read("Pulumi.yaml")`)

	d.RenderEvent(makeAgentEvent("exec_tool_call", backendEvent{
		Type: "exec_tool_call", Name: "shell_execute", ToolCallID: "tc2",
		Args: json.RawMessage(`{"command":"ls -la"}`),
	}))
	assert.Contains(t, stdout.String(), `* Bash("ls -la")`)

	d.RenderEvent(makeAgentEvent("tool_response", backendEvent{
		Type: "tool_response", Name: "shell_execute", ToolCallID: "tc2",
	}))
	assert.Contains(t, stdout.String(), `o Bash("ls -la")`)

	assert.Equal(t, 2, d.toolCount)
	assert.Equal(t, 0, d.toolErrorCount)

	// Final message renders and resets counters.
	d.RenderEvent(makeAgentEvent("assistant_message", backendEvent{
		Type: "assistant_message", Content: "Here are the results.", IsFinal: true,
	}))
	assert.Contains(t, stdout.String(), "Here are the results.")
	assert.Equal(t, 0, d.toolCount)
}

// --- stripANSI / visibleWidth tests ---

func TestStripANSI(t *testing.T) {
	// Plain text passes through unchanged.
	assert.Equal(t, "hello", stripANSI("hello"))
	// Bold escape is removed.
	assert.Equal(t, "bold", stripANSI("\033[1mbold\033[0m"))
	// 24-bit color codes are removed.
	assert.Equal(t, "color", stripANSI("\033[38;2;255;0;0mcolor\033[0m"))
	// Empty string.
	assert.Equal(t, "", stripANSI(""))
	// Multiple sequences.
	assert.Equal(t, "ab", stripANSI("\033[31ma\033[32mb\033[0m"))
	// OSC 8 hyperlink: only the visible text should remain.
	assert.Equal(t, "click here",
		stripANSI("\033]8;;https://example.com\033\\click here\033]8;;\033\\"))
}

func TestVisibleWidth(t *testing.T) {
	assert.Equal(t, 5, visibleWidth("hello"))
	assert.Equal(t, 4, visibleWidth("\033[1mbold\033[0m"))
	assert.Equal(t, 0, visibleWidth(""))
	assert.Equal(t, 0, visibleWidth("\033[0m"))
	assert.Equal(t, 3, visibleWidth("\033[31ma\033[32mb\033[0mc"))
}

func TestRenderWelcome_NonTTY(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderWelcome("myorg", "/home/user/project", "testuser", "")
	// Non-TTY mode produces no welcome banner.
	assert.Empty(t, stderr.String())
}

func TestRenderSessionStart(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderSessionStart("task-abc-123", "")
	assert.Contains(t, stderr.String(), "Neo session: task-abc-123")
}

func TestRenderSessionStart_WithURL(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderSessionStart("task-abc-123", "https://app.pulumi.com/myorg/neo/tasks/task-abc-123")
	// Non-TTY mode shows the full URL when available.
	assert.Contains(t, stderr.String(), "Neo session: https://app.pulumi.com/myorg/neo/tasks/task-abc-123")
}

func TestRenderSessionAttach(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderSessionAttach("task-abc-123", "")
	assert.Contains(t, stderr.String(), "Attached to session: task-abc-123")
}

func TestRenderSessionAttach_WithURL(t *testing.T) {
	d, _, stderr := newTestDisplay(false)
	d.RenderSessionAttach("task-abc-123", "https://app.pulumi.com/myorg/neo/tasks/task-abc-123")
	// Non-TTY mode always shows the session ID format.
	assert.Contains(t, stderr.String(), "Attached to session: task-abc-123")
}

// --- ConsoleTaskURL tests ---

func TestConsoleTaskURL_PulumiCloud(t *testing.T) {
	c := NewNeoClient("https://api.pulumi.com", "tok", "myorg")
	url := c.ConsoleTaskURL("task-123")
	assert.Equal(t, "https://app.pulumi.com/myorg/neo/tasks/task-123", url)
}

func TestConsoleTaskURL_Localhost(t *testing.T) {
	c := NewNeoClient("http://localhost:8080", "tok", "myorg")
	url := c.ConsoleTaskURL("task-123")
	assert.Equal(t, "http://localhost:3000/myorg/neo/tasks/task-123", url)
}

func TestConsoleTaskURL_UnknownDomain(t *testing.T) {
	c := NewNeoClient("https://custom-api.example.com", "tok", "myorg")
	url := c.ConsoleTaskURL("task-123")
	assert.Equal(t, "", url)
}

func TestConsoleTaskURL_ConsoleDomainEnvVar(t *testing.T) {
	t.Setenv("PULUMI_CONSOLE_DOMAIN", "console.example.com")
	c := NewNeoClient("https://custom-api.example.com", "tok", "myorg")
	url := c.ConsoleTaskURL("task-123")
	assert.Equal(t, "https://console.example.com/myorg/neo/tasks/task-123", url)
}

// --- Approval mode cycling tests ---

func TestNextApprovalMode(t *testing.T) {
	assert.Equal(t, "balanced", nextApprovalMode("manual"))
	assert.Equal(t, "auto", nextApprovalMode("balanced"))
	assert.Equal(t, "manual", nextApprovalMode("auto"))
	// Unknown defaults to first.
	assert.Equal(t, "manual", nextApprovalMode("unknown"))
}

// --- Time-of-day key tests ---

func TestTimeOfDayKey(t *testing.T) {
	assert.Equal(t, "night", timeOfDayKey(0))
	assert.Equal(t, "night", timeOfDayKey(4))
	assert.Equal(t, "morning", timeOfDayKey(5))
	assert.Equal(t, "morning", timeOfDayKey(11))
	assert.Equal(t, "afternoon", timeOfDayKey(12))
	assert.Equal(t, "afternoon", timeOfDayKey(16))
	assert.Equal(t, "evening", timeOfDayKey(17))
	assert.Equal(t, "evening", timeOfDayKey(20))
	assert.Equal(t, "night", timeOfDayKey(21))
	assert.Equal(t, "night", timeOfDayKey(23))
}

// --- buildShimmer tests ---

func TestBuildShimmer_ContainsAllRunes(t *testing.T) {
	text := "Hello"
	result := buildShimmer([]rune(text), 0)
	// The shimmer should contain all original characters (with ANSI codes interspersed).
	for _, r := range text {
		assert.Contains(t, result, string(r))
	}
}

func TestBuildShimmer_ContainsANSI(t *testing.T) {
	text := "Test"
	result := buildShimmer([]rune(text), 0)
	// Should contain ANSI codes.
	assert.Contains(t, result, ansiBold)
	assert.Contains(t, result, ansiMagenta)
	assert.Contains(t, result, ansiReset)
}

// --- Persistent input bar tests ---

// newInputBarDisplay creates a Display with hasTTY=true for testing input bar rendering.
func newInputBarDisplay() (*Display, *bytes.Buffer) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	mode := "balanced"
	d := &Display{
		stdout:      stdout,
		stderr:      stderr,
		stdin:       strings.NewReader(""),
		interactive: true,
		hasTTY:      true,
		termWidth:   80,
		stdinFd:     -1, // No actual FD — we test rendering only.
	}
	d.SetApprovalMode(&mode)
	return d, stdout
}

func TestDrawInputBar(t *testing.T) {
	d, stdout := newInputBarDisplay()

	d.mu.Lock()
	d.inputBarActive = true
	d.drawInputBar()
	d.mu.Unlock()

	out := stdout.String()
	// Should contain separator lines.
	assert.Contains(t, out, "───")
	// Should contain the prompt character.
	assert.Contains(t, out, "❯")
	// Should contain the mode indicator.
	assert.Contains(t, out, "Balanced")
	assert.Contains(t, out, "shift+tab")
}

func TestDrawInputBar_WithText(t *testing.T) {
	d, stdout := newInputBarDisplay()

	d.mu.Lock()
	d.inputBarActive = true
	d.inputText = []rune("hello world")
	d.drawInputBar()
	d.mu.Unlock()

	out := stdout.String()
	assert.Contains(t, out, "hello world")
	assert.Contains(t, out, "❯")
}

func TestDrawInputBar_ApprovalMode(t *testing.T) {
	d, stdout := newInputBarDisplay()

	d.mu.Lock()
	d.inputBarActive = true
	d.pendingApproval = true
	d.approvalMsg = "Neo wants to run: rm -rf /"
	d.drawInputBar()
	d.mu.Unlock()

	out := stdout.String()
	assert.Contains(t, out, "Approve?")
	assert.Contains(t, out, "[y/N]")
}

func TestEraseInputBar(t *testing.T) {
	d, stdout := newInputBarDisplay()

	d.mu.Lock()
	d.inputBarActive = true
	d.drawInputBar()
	stdout.Reset()
	d.eraseInputBar()
	d.mu.Unlock()

	out := stdout.String()
	// Should contain cursor-up and clear-to-end sequences.
	assert.Contains(t, out, "\033[2A")
	assert.Contains(t, out, "\033[J")
}

func TestWriteAboveInputBar(t *testing.T) {
	d, stdout := newInputBarDisplay()

	d.mu.Lock()
	d.inputBarActive = true
	d.drawInputBar()
	stdout.Reset()

	// Write some content above the input bar.
	d.writeAboveInputBar(func() {
		fmt.Fprintf(d.stdout, "tool output here\r\n")
	})
	d.mu.Unlock()

	out := stdout.String()
	// Should contain the erase sequence, the output, and redraw.
	assert.Contains(t, out, "tool output here")
	// Should re-contain the prompt after redraw.
	assert.Contains(t, out, "❯")
	assert.Contains(t, out, "───")
}

func TestWriteAboveInputBar_Inactive(t *testing.T) {
	d, stdout := newInputBarDisplay()

	// Input bar not active — fn called directly.
	d.writeAboveInputBar(func() {
		fmt.Fprintf(d.stdout, "direct output\r\n")
	})

	out := stdout.String()
	assert.Contains(t, out, "direct output")
	// Should not contain input bar elements.
	assert.NotContains(t, out, "❯")
}

func TestInputCh_NilWhenInactive(t *testing.T) {
	d, _ := newInputBarDisplay()
	ch := d.InputCh()
	assert.Nil(t, ch)
}

func TestInputCh_NonNilWhenActive(t *testing.T) {
	d, _ := newInputBarDisplay()
	d.mu.Lock()
	d.inputBarActive = true
	d.inputCh = make(chan string, 1)
	d.mu.Unlock()
	ch := d.InputCh()
	assert.NotNil(t, ch)
}

// --- Diff rendering tests ---

func TestRenderDiff_Edit(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	d.RenderDiff(FileChange{
		Path:       "main.go",
		OldContent: "package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n",
		NewContent: "package main\n\nfunc hello() {\n\tfmt.Println(\"hello world\")\n}\n",
		IsNew:      false,
	})
	d.FlushDiffs()

	out := stdout.String()
	// Non-TTY should produce plain unified-style header.
	assert.Contains(t, out, "--- a/main.go")
	assert.Contains(t, out, "+++ b/main.go")
}

func TestRenderDiff_Edit_TTY(t *testing.T) {
	d, stdout := newInputBarDisplay()
	d.RenderDiff(FileChange{
		Path:       "main.go",
		OldContent: "package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n",
		NewContent: "package main\n\nfunc hello() {\n\tfmt.Println(\"hello world\")\n}\n",
		IsNew:      false,
	})
	d.FlushDiffs()

	out := stdout.String()
	// Should contain filename in header.
	assert.Contains(t, out, "main.go")
	// Should contain box borders.
	assert.Contains(t, out, "╭─")
	assert.Contains(t, out, "╰─")
	// Should contain - and + markers.
	assert.Contains(t, out, "-")
	assert.Contains(t, out, "+")
}

func TestRenderDiff_NewFile(t *testing.T) {
	d, stdout := newInputBarDisplay()
	d.RenderDiff(FileChange{
		Path:       "config.yaml",
		OldContent: "",
		NewContent: "name: my-project\nruntime: go\ndescription: A new project\n",
		IsNew:      true,
	})
	d.FlushDiffs()

	out := stdout.String()
	assert.Contains(t, out, "config.yaml")
	assert.Contains(t, out, "new file")
	assert.Contains(t, out, "3 lines")
	assert.Contains(t, out, "╭─")
	assert.Contains(t, out, "╰─")
}

func TestRenderDiff_NewFile_NonTTY(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	d.RenderDiff(FileChange{
		Path:       "config.yaml",
		OldContent: "",
		NewContent: "name: my-project\nruntime: go\n",
		IsNew:      true,
	})
	d.FlushDiffs()

	out := stdout.String()
	assert.Contains(t, out, "+++ config.yaml")
	assert.Contains(t, out, "new file")
	assert.Contains(t, out, "2 lines")
}

func TestRenderDiff_LargeDiff(t *testing.T) {
	d, stdout := newInputBarDisplay()

	// Create a large file with many changed lines.
	var oldLines, newLines []string
	for i := 0; i < 50; i++ {
		oldLines = append(oldLines, fmt.Sprintf("old line %d", i))
		newLines = append(newLines, fmt.Sprintf("new line %d", i))
	}

	d.RenderDiff(FileChange{
		Path:       "big.txt",
		OldContent: strings.Join(oldLines, "\n") + "\n",
		NewContent: strings.Join(newLines, "\n") + "\n",
		IsNew:      false,
	})
	d.FlushDiffs()

	out := stdout.String()
	assert.Contains(t, out, "big.txt")
	// Should have a truncation indicator.
	assert.Contains(t, out, "more lines")
}

func TestRenderDiff_LargeNewFile(t *testing.T) {
	d, stdout := newInputBarDisplay()

	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}

	d.RenderDiff(FileChange{
		Path:       "large.txt",
		OldContent: "",
		NewContent: strings.Join(lines, "\n") + "\n",
		IsNew:      true,
	})
	d.FlushDiffs()

	out := stdout.String()
	assert.Contains(t, out, "large.txt")
	assert.Contains(t, out, "new file")
	assert.Contains(t, out, "30 lines")
	// Should have a truncation indicator.
	assert.Contains(t, out, "more lines")
}

func TestRenderDiff_IntraLine(t *testing.T) {
	d, stdout := newInputBarDisplay()
	d.RenderDiff(FileChange{
		Path:       "test.go",
		OldContent: "fmt.Println(\"hello\")\n",
		NewContent: "fmt.Println(\"hello world\")\n",
		IsNew:      false,
	})
	d.FlushDiffs()

	out := stdout.String()
	// Should contain ANSI bold codes for intra-line highlighting.
	assert.Contains(t, out, ansiBold)
}

func TestRenderDiff_EmptyChange(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	d.RenderDiff(FileChange{
		Path:       "same.txt",
		OldContent: "unchanged content",
		NewContent: "unchanged content",
		IsNew:      false,
	})

	// Identical content should produce no output.
	assert.Empty(t, stdout.String())
}

func TestRenderDiff_NonTTY(t *testing.T) {
	d, stdout, _ := newTestDisplay(false)
	d.RenderDiff(FileChange{
		Path:       "app.js",
		OldContent: "const x = 1;\n",
		NewContent: "const x = 2;\n",
		IsNew:      false,
	})
	d.FlushDiffs()

	out := stdout.String()
	// Non-TTY should produce simple unified diff header.
	assert.Contains(t, out, "--- a/app.js")
	assert.Contains(t, out, "+++ b/app.js")
	// Should NOT contain box-drawing characters.
	assert.NotContains(t, out, "╭")
	assert.NotContains(t, out, "╰")
}

func TestComputeVisibleLines(t *testing.T) {
	// 10 equal lines, with one change in the middle.
	lines := make([]diffLine, 10)
	for i := range lines {
		lines[i] = diffLine{op: diffmatchpatch.DiffEqual, lineNo: i + 1, text: fmt.Sprintf("line %d", i)}
	}
	lines[5].op = diffmatchpatch.DiffDelete

	visible := computeVisibleLines(lines, 2)

	// Lines 3-7 should be visible (change at 5, context of 2).
	assert.False(t, visible[0])
	assert.False(t, visible[1])
	assert.False(t, visible[2])
	assert.True(t, visible[3])
	assert.True(t, visible[4])
	assert.True(t, visible[5]) // the change
	assert.True(t, visible[6])
	assert.True(t, visible[7])
	assert.False(t, visible[8])
	assert.False(t, visible[9])
}
