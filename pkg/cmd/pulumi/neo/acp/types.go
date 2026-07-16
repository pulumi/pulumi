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

// Package acp is a self-contained implementation of the Agent Client Protocol
// (https://agentclientprotocol.com) wire layer: the JSON-RPC plumbing, the
// method router, the protocol types, and the fs/terminal callback hooks an
// agent exposes to its editor. It is application-neutral: the embedding
// application supplies its identity (see NewAgent) and behavior (see Delegate).
//
// It has no dependencies on Pulumi or Neo — its only external dependency is the
// jsonrpc2 transport. The Neo-specific glue that wires this protocol to Neo's
// session, config, and backend lives in the parent neo package (the acp_*.go
// files), which imports this package; the dependency only ever points inward.
// Keep it that way: nothing here should import anything under pkg/cmd/pulumi/neo.
package acp

import "encoding/json"

// ProtocolVersion is the ACP MAJOR version this agent implements. The wire field
// is a single integer; on initialize we negotiate down to a version the client
// also understands (see Agent.initialize).
//
// https://agentclientprotocol.com/protocol/initialization
const ProtocolVersion = 1

// Implementation identifies a client or agent: free-form name/title/version
// metadata exchanged on initialize.
type Implementation struct {
	Name    string `json:"name,omitempty"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

// InitializeParams is the client→agent `initialize` request. protocolVersion is
// the latest MAJOR version the client supports.
type InitializeParams struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
	ClientInfo         *Implementation    `json:"clientInfo,omitempty"`
}

// ClientCapabilities describes the optional features the editor (client) offers
// the agent. We consult these in session/new to decide whether filesystem and
// shell tool calls are routed to the editor or executed locally.
type ClientCapabilities struct {
	// FS reports the editor's filesystem read/write support.
	FS FileSystemCapability `json:"fs"`
	// Terminal reports whether the editor can run commands via the terminal/*
	// methods on the agent's behalf. Distinct from Auth.Terminal, which is about
	// running the agent's login flow in an editor terminal.
	Terminal bool `json:"terminal,omitempty"`
	// Auth reports which typed auth methods the editor can execute.
	Auth AuthCapability `json:"auth"`
}

// AuthCapability reports the editor's support for typed authentication methods
// (https://agentclientprotocol.com/rfds/auth-methods).
type AuthCapability struct {
	// Terminal reports whether the editor can run a terminal-typed auth method:
	// re-launching the agent binary in a real terminal with the method's
	// args/env for an interactive setup flow.
	Terminal bool `json:"terminal,omitempty"`
}

// FileSystemCapability reports which filesystem operations the editor exposes to
// the agent. Each gates the correspondingly-named fs/* client method.
type FileSystemCapability struct {
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

// InitializeResult is the agent→client `initialize` response. protocolVersion is
// the agreed version (the same value the client sent when we support it, else
// our latest).
type InitializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         *Implementation   `json:"agentInfo,omitempty"`
	AuthMethods       []AuthMethod      `json:"authMethods,omitempty"`
}

// AgentCapabilities describes the optional features this agent supports. For the
// initial slice we support neither session loading nor non-text prompt content.
type AgentCapabilities struct {
	// LoadSession reports support for the session/load method (resumption).
	LoadSession bool `json:"loadSession,omitempty"`
	// PromptCapabilities reports which non-text content blocks may appear in a
	// session/prompt request.
	PromptCapabilities PromptCapabilities `json:"promptCapabilities"`
}

// PromptCapabilities reports which optional content block types the agent
// accepts in a prompt. All default false: text-only for now.
type PromptCapabilities struct {
	Image           bool `json:"image,omitempty"`
	Audio           bool `json:"audio,omitempty"`
	EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

// AuthMethodTypeTerminal marks an AuthMethod whose setup is an interactive
// terminal flow: the client re-launches the agent binary in a real terminal,
// with the method's Args and Env replacing the configured launch values for
// that run only. Only advertised to clients that declare the auth.terminal
// capability (see AuthCapability).
const AuthMethodTypeTerminal = "terminal"

// AuthMethod is one authentication option the agent advertises on initialize.
type AuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Type identifies how the client executes the method
	// (https://agentclientprotocol.com/rfds/auth-methods). Empty means the
	// spec-default "agent": the agent authenticates by itself when the client
	// calls `authenticate`.
	Type string `json:"type,omitempty"`
	// Args and Env apply to terminal-typed methods: they replace the agent's
	// configured launch arguments and extend its environment for the setup run.
	Args []string          `json:"args,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
	// Meta is the spec's implementation-specific extension point; clients
	// ignore keys they don't recognize. See MetaKeyTerminalAuth.
	Meta map[string]any `json:"_meta,omitempty"`
}

// MetaKeyTerminalAuth is the AuthMethod.Meta key carrying a TerminalAuthMeta.
const MetaKeyTerminalAuth = "terminal-auth"

// TerminalAuthMeta is the pre-stabilization form of terminal auth, honored by
// editors (e.g. current stable Zed) that predate the typed AuthMethod fields:
// the client runs Command with Args/Env in a real terminal. Unlike the typed
// form, Command names the binary explicitly — the client substitutes nothing
// into its configured launch command. Advertise it under MetaKeyTerminalAuth
// alongside the typed fields until the RFD stabilizes.
type TerminalAuthMeta struct {
	Label   string            `json:"label"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// AuthenticateParams is the client→agent `authenticate` request, naming the
// method (by AuthMethod.ID) the client chose.
type AuthenticateParams struct {
	MethodID string `json:"methodId"`
}

// NewSessionParams is the client→agent `session/new` request. Cwd is the
// absolute working directory the session is rooted at; tool file and shell
// operations resolve against it. Other fields the spec defines (e.g.
// mcpServers) are accepted but ignored for now.
type NewSessionParams struct {
	Cwd string `json:"cwd"`
}

// NewSessionResult is the agent→client `session/new` response carrying the id
// the client passes on subsequent session/prompt and session/cancel requests.
// ConfigOptions advertises the session settings the editor may change (see
// ConfigOption); it is omitted when the agent exposes none.
type NewSessionResult struct {
	SessionID     string         `json:"sessionId"`
	ConfigOptions []ConfigOption `json:"configOptions,omitempty"`
}

// ConfigOptionTypeSelect is the only ConfigOption.Type the spec defines so far: a
// choice among a fixed set of values.
const ConfigOptionTypeSelect = "select"

// ConfigCategoryMode is the ConfigOption.Category hint for options that change how
// the agent behaves (vs. e.g. "model"). Editors may surface these prominently.
const ConfigCategoryMode = "mode"

// ConfigOption is one session configuration setting the agent advertises and the
// editor can change via session/set_config_option. It supersedes the older
// Session Modes API and, unlike modes, lets the agent expose several independent
// settings at once. Today every option is a "select" (Type) over Options, with
// CurrentValue naming the active one.
//
// https://agentclientprotocol.com/protocol/v1/session-config-options
type ConfigOption struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Category     string              `json:"category,omitempty"`
	Type         string              `json:"type"`
	CurrentValue string              `json:"currentValue"`
	Options      []ConfigOptionValue `json:"options"`
}

// ConfigOptionValue is one selectable value of a select ConfigOption.
type ConfigOptionValue struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SetConfigOptionParams is the client→agent `session/set_config_option` request:
// set the option ConfigID to Value for SessionID.
type SetConfigOptionParams struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
}

// SetConfigOptionResult is the agent→client response: the complete, updated option
// list (current values reflect the change, including any the agent clamped or
// adjusted as a side effect).
type SetConfigOptionResult struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// ContentBlock is a single piece of prompt or message content. We produce text
// blocks and consume "text" and "resource_link" blocks: text and resource links
// are baseline content a client may send regardless of prompt capabilities (an
// editor emits a resource_link when the user @-mentions a file). The capability-
// gated blocks (image/audio and the embedded "resource") are not yet advertised,
// so a conforming client won't send them.
type ContentBlock struct {
	Type string `json:"type"` // "text" | "resource_link"
	Text string `json:"text,omitempty"`

	// Resource link fields, set when Type == "resource_link". URI and Name are
	// required by the spec for that type; the rest are optional metadata we keep
	// so the reference survives translation to Neo's text-only prompt input.
	URI         string `json:"uri,omitempty"`
	Name        string `json:"name,omitempty"`
	Title       string `json:"title,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
	Description string `json:"description,omitempty"`
}

// PromptParams is the client→agent `session/prompt` request: the user's message
// for sessionId, as an ordered list of content blocks.
type PromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

// PromptResult is the agent→client `session/prompt` response, returned when the
// turn ends.
type PromptResult struct {
	StopReason StopReason `json:"stopReason"`
}

// StopReason explains why a prompt turn ended.
type StopReason string

const (
	// StopEndTurn is the normal completion: the model finished without
	// requesting more tools.
	StopEndTurn StopReason = "end_turn"
	// StopCancelled is returned when the client cancelled the turn.
	StopCancelled StopReason = "cancelled"
	// StopRefusal is returned when the agent declined to continue.
	StopRefusal StopReason = "refusal"
	// StopMaxTokens is returned when the token limit was reached.
	StopMaxTokens StopReason = "max_tokens"
	// StopMaxTurnRequests is returned when the model-call limit was reached.
	StopMaxTurnRequests StopReason = "max_turn_requests"
)

// CancelParams is the client→agent `session/cancel` notification, asking the
// agent to stop the in-flight turn for sessionId.
type CancelParams struct {
	SessionID string `json:"sessionId"`
}

// SessionNotification is the params of a `session/update` notification. Update
// is one of the SessionUpdate payload types below, distinguished by its
// sessionUpdate discriminator injected on marshal.
type SessionNotification struct {
	SessionID string        `json:"sessionId"`
	Update    SessionUpdate `json:"update"`
}

// SessionUpdate is the sealed set of session/update payloads. The marker method
// is unexported so only this package's payload types satisfy it, keeping the
// notification stream typed end to end rather than passing an open any.
type SessionUpdate interface {
	isSessionUpdate()
}

func (AgentMessageChunk) isSessionUpdate()  {}
func (ToolCallStart) isSessionUpdate()      {}
func (ToolCallProgress) isSessionUpdate()   {}
func (PlanUpdate) isSessionUpdate()         {}
func (ConfigOptionUpdate) isSessionUpdate() {}

// The session/update payload types below each marshal with a constant
// "sessionUpdate" discriminator injected by MarshalJSON, so the wire tag is
// owned here (by the type) rather than set by hand at every call site.

// AgentMessageChunk is the session/update payload carrying a piece of the
// agent's response text.
type AgentMessageChunk struct {
	Content ContentBlock `json:"content"`
}

// MarshalJSON tags the payload as "agent_message_chunk".
func (u AgentMessageChunk) MarshalJSON() ([]byte, error) {
	type alias AgentMessageChunk
	return json.Marshal(struct {
		SessionUpdate string `json:"sessionUpdate"`
		alias
	}{"agent_message_chunk", alias(u)})
}

// ToolCallStart is the session/update payload announcing a new tool call.
type ToolCallStart struct {
	ToolCallID string             `json:"toolCallId"`
	Title      string             `json:"title"`
	Kind       string             `json:"kind,omitempty"`
	Status     string             `json:"status,omitempty"`
	Locations  []ToolCallLocation `json:"locations,omitempty"`
	RawInput   json.RawMessage    `json:"rawInput,omitempty"`
}

// ToolCallLocation is a file (optionally a line) a tool call touches. Editors
// use it to render the call against the file natively — a clickable path, a
// read/edit affordance, follow-along highlighting.
type ToolCallLocation struct {
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
}

// MarshalJSON tags the payload as "tool_call".
func (u ToolCallStart) MarshalJSON() ([]byte, error) {
	type alias ToolCallStart
	return json.Marshal(struct {
		SessionUpdate string `json:"sessionUpdate"`
		alias
	}{"tool_call", alias(u)})
}

// ToolCallProgress is the session/update payload reporting a status or output
// change for an in-flight tool call.
type ToolCallProgress struct {
	ToolCallID string            `json:"toolCallId"`
	Status     string            `json:"status,omitempty"`
	Content    []ToolCallContent `json:"content,omitempty"`
	RawOutput  json.RawMessage   `json:"rawOutput,omitempty"`
}

// MarshalJSON tags the payload as "tool_call_update".
func (u ToolCallProgress) MarshalJSON() ([]byte, error) {
	type alias ToolCallProgress
	return json.Marshal(struct {
		SessionUpdate string `json:"sessionUpdate"`
		alias
	}{"tool_call_update", alias(u)})
}

// ToolCallContent wraps a content block produced by a tool call. Only the
// "content" form is produced for now (diff and terminal forms exist in ACP).
type ToolCallContent struct {
	Type    string       `json:"type"` // "content"
	Content ContentBlock `json:"content"`
}

// PlanUpdate is the session/update payload carrying the agent's task list.
type PlanUpdate struct {
	Entries []PlanEntry `json:"entries"`
}

// MarshalJSON tags the payload as "plan".
func (u PlanUpdate) MarshalJSON() ([]byte, error) {
	type alias PlanUpdate
	return json.Marshal(struct {
		SessionUpdate string `json:"sessionUpdate"`
		alias
	}{"plan", alias(u)})
}

// ConfigOptionUpdate is the session/update payload announcing that one or more
// config options changed value out of band (e.g. the agent left plan mode on its
// own). It carries the complete, current option list, mirroring the
// session/set_config_option response.
type ConfigOptionUpdate struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// MarshalJSON tags the payload as "config_option_update".
func (u ConfigOptionUpdate) MarshalJSON() ([]byte, error) {
	type alias ConfigOptionUpdate
	return json.Marshal(struct {
		SessionUpdate string `json:"sessionUpdate"`
		alias
	}{"config_option_update", alias(u)})
}

// PlanEntry is one item in a PlanUpdate. Field values match ACP: priority is
// "high"|"medium"|"low"; status is "pending"|"in_progress"|"completed".
type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// Plan entry priorities and statuses (ACP enums). ACP requires a valid value
// from these sets on every entry, so the adapter clamps to them on egress.
const (
	PlanPriorityHigh   = "high"
	PlanPriorityMedium = "medium"
	PlanPriorityLow    = "low"

	PlanStatusPending    = "pending"
	PlanStatusInProgress = "in_progress"
	PlanStatusCompleted  = "completed"
)

// Tool call kinds and statuses (ACP enums).
const (
	ToolKindRead    = "read"
	ToolKindEdit    = "edit"
	ToolKindSearch  = "search"
	ToolKindExecute = "execute"
	ToolKindOther   = "other"

	ToolStatusInProgress = "in_progress"
	ToolStatusCompleted  = "completed"
	ToolStatusFailed     = "failed"
)

// RequestPermissionParams is the agent→client `session/request_permission`
// request, asking the user to approve a tool call. The client replies by
// selecting one of options.
type RequestPermissionParams struct {
	SessionID string             `json:"sessionId"`
	ToolCall  PermissionToolCall `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

// PermissionToolCall identifies (and briefly describes) the tool call awaiting
// approval.
type PermissionToolCall struct {
	ToolCallID string `json:"toolCallId"`
	Title      string `json:"title,omitempty"`
}

// PermissionOption is one choice the user can pick in a permission request. Kind
// is one of "allow_once"|"allow_always"|"reject_once"|"reject_always".
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// RequestPermissionResult is the client→agent response to a permission request.
type RequestPermissionResult struct {
	Outcome PermissionOutcome `json:"outcome"`
}

// PermissionOutcome reports the user's decision. Outcome is "selected" (with
// OptionID set) or "cancelled".
type PermissionOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

// Permission option ids and kinds we offer for each approval request.
const (
	permissionOptionAllow  = "allow"
	permissionOptionReject = "reject"
	permissionKindAllow    = "allow_once"
	permissionKindReject   = "reject_once"
	permissionSelected     = "selected"
)

// ApprovalOptions returns the standard allow/reject choices to present in a
// permission request. Callers interpret the response with
// RequestPermissionResult.Approved.
func ApprovalOptions() []PermissionOption {
	return []PermissionOption{
		{OptionID: permissionOptionAllow, Name: "Allow", Kind: permissionKindAllow},
		{OptionID: permissionOptionReject, Name: "Reject", Kind: permissionKindReject},
	}
}

// Approved reports whether the user selected the allow option. A cancelled
// outcome, a missing selection, or the reject option all count as not approved.
func (r RequestPermissionResult) Approved() bool {
	return r.Outcome.Outcome == permissionSelected && r.Outcome.OptionID == permissionOptionAllow
}
