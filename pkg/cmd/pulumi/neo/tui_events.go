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
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/display"
)

// UIEvent is the sealed interface for events sent from the Session event loop to the
// bubbletea TUI. Each variant carries just enough information for the TUI to render.
type UIEvent interface {
	uiEvent() // sealed marker
}

// UIAssistantMessage carries streaming or final assistant text.
type UIAssistantMessage struct {
	Content string
	IsFinal bool
	// HasPendingCLIWork is true when IsFinal is true AND the original
	// assistant_message backend event had at least one tool_call with
	// execution_mode=="cli". In that case the agent is paused handing control to
	// the CLI to run those tools locally; the declarative busy rule treats the
	// event as non-final and keeps the spinner on until the CLI has replied and
	// the agent emits a truly-final message.
	HasPendingCLIWork bool
}

func (UIAssistantMessage) uiEvent() {}

// UIToolStarted signals that a CLI tool call is about to be invoked.
type UIToolStarted struct {
	Name string
	Args json.RawMessage
}

func (UIToolStarted) uiEvent() {}

// UIToolProgress carries a progress update for a running tool.
type UIToolProgress struct {
	Name    string
	Message string
}

func (UIToolProgress) uiEvent() {}

// UIToolCompleted signals that a CLI tool call has finished.
type UIToolCompleted struct {
	Name    string
	Args    json.RawMessage
	IsError bool
}

func (UIToolCompleted) uiEvent() {}

// UIError carries a fatal or non-fatal error from the agent.
type UIError struct {
	Message string
}

func (UIError) uiEvent() {}

// UIWarning carries a non-fatal warning from the agent.
type UIWarning struct {
	Message string
}

func (UIWarning) uiEvent() {}

// UICancelled signals the session was cancelled.
type UICancelled struct{}

func (UICancelled) uiEvent() {}

// UITaskIdle signals the agent finished its turn.
type UITaskIdle struct{}

func (UITaskIdle) uiEvent() {}

// UISessionURL carries the console URL once the task is created.
type UISessionURL struct {
	URL string
}

func (UISessionURL) uiEvent() {}

// UIUserMessage carries a user message to display in the TUI.
type UIUserMessage struct {
	Content string
}

func (UIUserMessage) uiEvent() {}

// UIApprovalRequest signals that the agent needs user approval for an operation.
type UIApprovalRequest struct {
	ApprovalID  string
	Message     string
	Sensitivity string
	// ApprovalType is the wire discriminator that tells the TUI which rendering
	// path to use. "plan_exit" triggers the plan rendering (markdown body, plan
	// header, auto-exit on approval); any other value (today: "general") uses
	// the regular tool-approval rendering.
	ApprovalType string
	// PlanDescription is the markdown plan body, populated only for plan_exit
	// approvals. The TUI renders it through the glamour markdown renderer.
	PlanDescription string
}

func (UIApprovalRequest) uiEvent() {}

// UIAwaitingApprovals is an interim backend signal that the agent is pausing before
// it will emit a UIApprovalRequest. The declarative busy rule treats it as non-final
// and shows an "Awaiting approvals" label until the approval request arrives.
type UIAwaitingApprovals struct{}

func (UIAwaitingApprovals) uiEvent() {}

// UIContextCompression signals that the agent is compressing its context window.
// Non-final; the TUI surfaces it as a "Compressing context" label on the busy
// indicator and otherwise doesn't render anything.
type UIContextCompression struct {
	Status string
}

func (UIContextCompression) uiEvent() {}

// UIPulumiStart opens a persistent preview/up block in the TUI for a pulumi tool
// call. The block accumulates resource and diagnostic rows as they stream in,
// and is finalized by UIPulumiEnd.
//
// The open block is keyed on ToolName: subsequent UIPulumi{Resource,Diag,End}
// events for the in-flight call update the same block. Once UIPulumiEnd fires
// the block is finalized; the next UIPulumiStart with the same ToolName starts
// a fresh block. Relies on Session.runBatch dispatching tool calls serially —
// concurrent calls would clobber each other.
type UIPulumiStart struct {
	ToolName  string
	StackName string
	// IsPreview is true for pulumi_preview, false for pulumi_up. Used by the
	// renderer to title the block (PulumiPreview vs PulumiUp) and to pick the
	// "planned changes" vs "applied changes" wording.
	IsPreview bool
}

func (UIPulumiStart) uiEvent() {}

// UIPulumiResource reports one resource the engine is acting on. URN is used
// as the dedup key — duplicate events for the same URN update the row in place
// (e.g. status transitions from "planned" to "running" to "done").
type UIPulumiResource struct {
	ToolName string
	// Op is the typed StepOp from the engine: create, update, delete,
	// replace, read, refresh, etc.
	Op   display.StepOp
	URN  string
	Type string
	// Status is "planned" (preview only), "running" (up, pre-event), "done"
	// (up, outputs-event), or "failed" (up, operation-failed event).
	Status string
}

func (UIPulumiResource) uiEvent() {}

// UIPulumiDiag appends one diagnostic row to the open pulumi block. URN may be
// empty for stack-level diagnostics.
type UIPulumiDiag struct {
	ToolName string
	Severity string
	Message  string
	URN      string
}

func (UIPulumiDiag) uiEvent() {}

// UIPulumiEnd finalizes the open pulumi block. Err is empty on success. Counts
// is the engine's ResourceChanges map; the TUI consumes it as-is so we don't
// pay a flatten/unflatten round-trip just to cross the package boundary.
type UIPulumiEnd struct {
	ToolName string
	Err      string
	Counts   display.ResourceChanges
	// Elapsed is the duration the backend call took, pre-formatted so the TUI
	// doesn't need to care about time types.
	Elapsed string
}

func (UIPulumiEnd) uiEvent() {}
