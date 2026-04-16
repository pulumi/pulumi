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
// This file only defines the string discriminator values we filter on.
package neo

// Discriminator values for the AgentConsoleEvent envelope and the inner backend/user
// events we care about.
const (
	consoleEventAgentResponse    = "agentResponse"
	backendEventAssistantMessage = "assistant_message"
	userEventToolResult          = "tool_result"
	userEventExecToolCall        = "exec_tool_call"

	// toolExecutionModeCLI marks an individual tool call inside an assistant_message
	// that the CLI client must execute locally. Cloud-marked or unset calls are
	// handled by the agent runtime and must not be touched by the CLI.
	toolExecutionModeCLI = "cli"
)
