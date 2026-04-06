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
//   - pkg/apitype/agent_backend_event_cli_tool_request_.go (request payload)
//   - pkg/apitype/agent_user_event_cli_tool_result_.go (result payload)
package neo

import (
	"encoding/json"
	"time"
)

// Discriminator values for the AgentConsoleEvent envelope and the inner backend/user
// events we care about.
const (
	consoleEventAgentResponse  = "agentResponse"
	consoleEventUserInput      = "userInput"
	backendEventCliToolRequest = "cli_tool_request"
	userEventCliToolResult     = "cli_tool_result"
)

// ConsoleEventEnvelope is the SSE-streamed AgentConsoleEvent. The agent-side has two
// subtypes (agentResponse, userInput); we only act on agentResponse and even then only
// when the inner eventBody is a cli_tool_request.
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

// CliToolRequest matches apitype.AgentBackendEventCliToolRequest. The agent yields its
// turn after emitting one of these and resumes once a CliToolResult arrives.
type CliToolRequest struct {
	Type      string        `json:"type"` // always "cli_tool_request"
	Timestamp time.Time     `json:"timestamp"`
	ToolCalls []CliToolCall `json:"tool_calls"`
}

// CliToolCall matches apitype.AgentBackendEventCliToolCall.
type CliToolCall struct {
	// ToolCallID correlates a request call to its result item.
	ToolCallID string `json:"tool_call_id"`
	// Name is the full tool name as understood by the agent — "<server>__<method>"
	// (e.g. "filesystem__read"). The CLI dispatches by splitting on "__".
	Name string `json:"name"`
	// Args is the raw JSON arguments object for the call. Kept as RawMessage so each
	// handler can decode into its own typed struct.
	Args json.RawMessage `json:"args"`
}

// CliToolResult matches apitype.AgentUserEventCliToolResult and is posted back to the Neo
// task via POST /api/preview/agents/{org}/tasks/{taskID}/events.
type CliToolResult struct {
	Type        string              `json:"type"` // always "cli_tool_result"
	Timestamp   time.Time           `json:"timestamp"`
	EntityDiff  EntityDiff          `json:"entity_diff"`
	ToolResults []CliToolResultItem `json:"tool_results"`
}

// CliToolResultItem matches apitype.AgentUserEventCliToolResultItem.
type CliToolResultItem struct {
	// ToolCallID echoes the request's tool_call_id so the agent can pair them up.
	ToolCallID string `json:"tool_call_id"`
	// Name echoes the request's full tool name (the agent strips the server prefix
	// itself when materializing the ToolResponse).
	Name string `json:"name"`
	// Content is the result payload. Any JSON-marshalable value is accepted; handlers
	// typically return an object describing what they did.
	Content any `json:"content"`
	// IsError is true when the tool failed. The agent uses this to decide whether to
	// retry or report the failure to the user.
	IsError bool `json:"is_error,omitempty"`
}

// EntityDiff matches apitype.AgentEntityDiff. The CLI does not currently extract entities
// from tool output, so this is always sent empty — the field is required by the IDL
// marshaler though, so we serialize it as an empty object rather than omitting it.
type EntityDiff struct {
	Add    []any `json:"add"`
	Remove []any `json:"remove"`
}
