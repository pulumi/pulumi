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

// Package neo implements the `pulumi neo` command and the local tool-execution loop that
// pairs with Pulumi Cloud's Neo agent when a task is created in `cli` tool execution mode.
//
// All wire types in this file mirror the IDL-generated apitype.* shapes in
// pulumi/pulumi-service. The JSON tags must match exactly — see:
//   - pkg/apitype/agent_console_event_*.go (SSE envelope)
//   - pkg/apitype/agent_backend_event_tool_call_.go (assistantMessage tool calls)
//   - pkg/apitype/agent_user_event_tool_result_.go (generic tool_result reply)
package neo

import (
	"encoding/json"
)

// Discriminator values for the AgentConsoleEvent envelope and the inner backend/user
// events we care about.
const (
	consoleEventAgentResponse    = "agentResponse"
	consoleEventUserInput        = "userInput"
	backendEventAssistantMessage = "assistant_message"
	userEventToolResult          = "tool_result"
	userEventExecToolCall        = "exec_tool_call"

	// toolExecutionModeCLI marks an individual tool call inside an AssistantMessage
	// that the CLI client must execute locally. Cloud-marked or unset calls are
	// handled by the agent runtime and must not be touched by the CLI.
	toolExecutionModeCLI = "cli"
)

// ConsoleEventEnvelope is the SSE-streamed AgentConsoleEvent. The agent-side has two
// subtypes (agentResponse, userInput); we only act on agentResponse and even then only
// when the inner eventBody is an assistantMessage with cli-marked tool calls.
type ConsoleEventEnvelope struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	EventBody json.RawMessage `json:"eventBody,omitempty"`
}

// BackendEventHeader peeks at the inner eventBody to dispatch on the discriminator without
// fully decoding the payload.
type BackendEventHeader struct {
	Type string `json:"type"`
}

// AssistantMessage is the agent → user backend event that may carry tool_calls. When any
// tool call has execution_mode == "cli", the message is sent with is_final: true and the
// agent pauses until the CLI replies with a ToolResultEvent. We do not model the
// conversational text fields — the CLI only acts on tool_calls.
type AssistantMessage struct {
	Type      string     `json:"type"` // always "assistantMessage"
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	IsFinal   bool       `json:"is_final,omitempty"`
}

// ToolCall mirrors apitype.AgentBackendEventToolCall.
type ToolCall struct {
	// ToolCallID correlates a request call to its result item. The wire field is
	// "id" because the service reuses AgentBackendEventToolCall here; only the
	// outbound AgentUserEventToolResultItem uses the longer "tool_call_id" key.
	ToolCallID string `json:"id"`
	// Name is the full tool name as understood by the agent — "<server>__<method>"
	// (e.g. "filesystem__read"). The CLI dispatches by splitting on "__".
	Name string `json:"name"`
	// Args is the raw JSON arguments object for the call. Kept as RawMessage so each
	// handler can decode into its own typed struct.
	Args json.RawMessage `json:"args"`
	// ExecutionMode is "cloud", "cli", or "" (treated as cloud). Only "cli" calls are
	// executed by this client; everything else is handled by the agent runtime.
	ExecutionMode string `json:"execution_mode,omitempty"`
}

// ToolResultEvent is the generic user event the CLI posts to resume an agent turn after
// executing one or more cli-marked tool calls. It mirrors apitype.AgentUserEventToolResult.
type ToolResultEvent struct {
	Type        string           `json:"type"` // always "tool_result"
	ToolResults []ToolResultItem `json:"tool_results"`
}

// ExecToolCallEvent is the user event the CLI posts just before executing a
// cli-marked tool call, so the service can emit a backend event that lights
// up the "running" state in the UI.
type ExecToolCallEvent struct {
	Type       string `json:"type"`         // always "exec_tool_call"
	ToolCallID string `json:"tool_call_id"` // from the inbound ToolCall.ToolCallID
	Name       string `json:"name"`         // full tool name, e.g. "filesystem__read"
}

// ToolResultItem mirrors apitype.AgentUserEventToolResultItem.
type ToolResultItem struct {
	// ToolCallID echoes the request's tool_call_id so the agent can pair them up.
	ToolCallID string `json:"tool_call_id"`
	// Name echoes the request's full tool name (the agent strips the server prefix
	// itself when materializing the ToolResponse).
	Name string `json:"name"`
	// Content is the result payload. Any JSON-marshalable value is accepted; handlers
	// typically return an object describing what they did.
	Content any `json:"content"`
	// IsError is true when the tool failed. The agent uses this to decide whether to
	// retry or report the failure to the user. Failed items must still be sent — omitting
	// an item leaves the agent waiting forever.
	IsError bool `json:"is_error,omitempty"`
}
