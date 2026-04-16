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

import "encoding/json"

// UIEvent is the sealed interface for events sent from the Session event loop to the
// bubbletea TUI. Each variant carries just enough information for the TUI to render.
type UIEvent interface {
	uiEvent() // sealed marker
}

// UIAssistantMessage carries streaming or final assistant text.
type UIAssistantMessage struct {
	Content string
	IsFinal bool
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
