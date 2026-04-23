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
// The wire types consumed by this loop live in sdk/go/common/apitype (see neo.go there).
// This file defines the string discriminator values we filter on and small CLI-side
// helpers that operate on those wire types.
package neo

import "github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

// Discriminator values for the AgentConsoleEvent envelope and the inner backend/user
// events we care about.
const (
	consoleEventAgentResponse    = "agentResponse"
	consoleEventUserInput        = "userInput"
	backendEventAssistantMessage = "assistant_message"
	userEventToolResult          = "tool_result"
	userEventExecToolCall        = "exec_tool_call"

	// toolExecutionModeCLI marks an individual tool call inside an assistant_message
	// that the CLI client must execute locally. Cloud-marked or unset calls are
	// handled by the agent runtime and must not be touched by the CLI.
	toolExecutionModeCLI = "cli"

	userEventUserMessage      = "user_message"
	userEventUserConfirmation = "user_confirmation"

	// userEventUserCancel is the user event the CLI posts when the user asks to
	// abort the current turn (ESC in the TUI). The agent acknowledges by emitting
	// a cancelled backend event.
	userEventUserCancel = "user_cancel"

	// Additional backend event types forwarded to the TUI. Note that
	// "exec_tool_call" is used both by the CLI (posted upstream, see
	// userEventExecToolCall above) and by the service (echoed back in the event
	// stream when a server-side tool starts running).
	backendEventExecToolCallProgress = "exec_tool_call_progress"
	backendEventToolResponse         = "tool_response"
	backendEventError                = "error"
	backendEventWarning              = "warning"
	backendEventCancelled            = "cancelled"
	backendEventUserApprovalRequest  = "user_approval_request"
	backendEventAwaitingApprovals    = "awaiting_approvals"
	backendEventContextCompression   = "context_compression_event"
	backendEventChangeEntities       = "change_entities"
	backendEventSetTaskName          = "set_task_name"

	// approvalTypePlanExit is the approval_type value the service sets on a
	// user_approval_request that gates an exit_plan_mode tool call. The TUI
	// renders context.plan_description with markdown and, on user approval,
	// auto-clears the local plan-mode indicator (the server-side PlanModeTracker
	// exits in lockstep). Any other value (today: "general") takes the regular
	// tool-approval rendering path.
	approvalTypePlanExit = "plan_exit"
)

// hasPendingCLIToolCalls reports whether any tool call in the list has
// execution_mode=="cli", meaning the CLI must run it locally before the agent
// turn can resume. The TUI-side busy rule uses this to keep the spinner on
// through an is_final=true assistant_message that hands work off to the CLI.
func hasPendingCLIToolCalls(calls []apitype.AgentBackendEventToolCall) bool {
	for _, tc := range calls {
		if tc.ExecutionMode == toolExecutionModeCLI {
			return true
		}
	}
	return false
}
