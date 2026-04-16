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

package apitype

import "encoding/json"

// Wire types for the Pulumi Cloud Neo agent's task event stream. These mirror the
// IDL-generated shapes in pulumi/pulumi-service:
//   - cmd/agents/src/agents_py/events/agent_console_event_*.go (SSE envelope)
//   - cmd/agents/src/agents_py/events/agent_backend_event_*.go (assistantMessage tool calls)
//   - cmd/agents/src/agents_py/events/agent_user_event_*.go    (tool_result, exec_tool_call)
//
// The JSON tags must match the service IDL exactly.

// AgentConsoleEvent is the SSE-streamed envelope wrapping every Neo task event. The
// agent-side has two subtypes (agentResponse, userInput); CLI clients only act on
// agentResponse and even then only when the inner EventBody is an
// AgentBackendEventAssistantMessage with cli-marked tool calls.
type AgentConsoleEvent struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	EventBody json.RawMessage `json:"eventBody,omitempty"`
}

// AgentBackendEventHeader peeks at the inner eventBody to dispatch on the type
// discriminator without fully decoding the payload.
type AgentBackendEventHeader struct {
	Type string `json:"type"`
}

// AgentBackendEventAssistantMessage is the agent → user backend event that may carry
// tool_calls. When any tool call has execution_mode == "cli", the message is sent with
// is_final: true and the agent pauses until the CLI replies with an
// AgentUserEventToolResult. Conversational text fields are intentionally omitted — CLI
// clients only act on tool_calls.
type AgentBackendEventAssistantMessage struct {
	Type      string                      `json:"type"` // always "assistant_message"
	ToolCalls []AgentBackendEventToolCall `json:"tool_calls,omitempty"`
	IsFinal   bool                        `json:"is_final,omitempty"`
}

// AgentBackendEventToolCall is one tool call inside an AgentBackendEventAssistantMessage.
type AgentBackendEventToolCall struct {
	// ToolCallID correlates a request call to its result item. The wire field is
	// "id" because the service reuses this type here; only the outbound
	// AgentUserEventToolResultItem uses the longer "tool_call_id" key.
	ToolCallID string `json:"id"`
	// Name is the full tool name as understood by the agent — "<server>__<method>"
	// (e.g. "filesystem__read"). Clients dispatch by splitting on "__".
	Name string `json:"name"`
	// Args is the raw JSON arguments object for the call. Kept as RawMessage so each
	// handler can decode into its own typed struct.
	Args json.RawMessage `json:"args"`
	// ExecutionMode is "cloud", "cli", or "" (treated as cloud). Only "cli" calls are
	// executed by the CLI; everything else is handled by the agent runtime.
	ExecutionMode string `json:"execution_mode,omitempty"`
}

// AgentUserEventToolResult is the user event a CLI client posts to resume an agent turn
// after executing one or more cli-marked tool calls.
type AgentUserEventToolResult struct {
	Type        string                         `json:"type"` // always "tool_result"
	ToolResults []AgentUserEventToolResultItem `json:"tool_results"`
}

// AgentUserEventToolResultItem is a single tool result inside an AgentUserEventToolResult.
type AgentUserEventToolResultItem struct {
	// ToolCallID echoes the request's tool_call_id so the agent can pair request and
	// response.
	ToolCallID string `json:"tool_call_id"`
	// Name echoes the request's full tool name (the agent strips the server prefix
	// itself when materializing the ToolResponse).
	Name string `json:"name"`
	// Content is the result payload. Any JSON-marshalable value is accepted; handlers
	// typically return an object describing what they did.
	Content any `json:"content"`
	// IsError is true when the tool failed. The agent uses this to decide whether to
	// retry or report the failure to the user. Failed items must still be sent —
	// omitting an item leaves the agent waiting forever.
	IsError bool `json:"is_error,omitempty"`
}

// AgentUserEventExecToolCall is the user event a CLI client posts just before executing
// a cli-marked tool call, so the service can emit a backend event that lights up the
// "running" state in the UI.
type AgentUserEventExecToolCall struct {
	Type       string `json:"type"`         // always "exec_tool_call"
	ToolCallID string `json:"tool_call_id"` // from the inbound AgentBackendEventToolCall.ToolCallID
	Name       string `json:"name"`         // full tool name, e.g. "filesystem__read"
}
