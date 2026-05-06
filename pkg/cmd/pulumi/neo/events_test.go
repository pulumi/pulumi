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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// TestHasPendingCLIToolCalls is thin but explicit — the helper drives the
// HasPendingCLIWork flag on UIAssistantMessage, which the TUI busy rule uses
// to keep the spinner on through a CLI-work hand-off.
func TestHasPendingCLIToolCalls(t *testing.T) {
	t.Parallel()

	assert.False(t, hasPendingCLIToolCalls(nil))
	assert.False(t, hasPendingCLIToolCalls([]apitype.AgentBackendEventToolCall{}))
	assert.False(t, hasPendingCLIToolCalls([]apitype.AgentBackendEventToolCall{
		{ExecutionMode: "cloud"},
		{ExecutionMode: ""},
	}))
	assert.True(t, hasPendingCLIToolCalls([]apitype.AgentBackendEventToolCall{
		{ExecutionMode: toolExecutionModeCLI},
	}))
	assert.True(t, hasPendingCLIToolCalls([]apitype.AgentBackendEventToolCall{
		{ExecutionMode: "cloud"},
		{ExecutionMode: toolExecutionModeCLI},
	}))
}

func TestIsAskUserToolName(t *testing.T) {
	t.Parallel()

	// Bare method form and the namespaced "<server>__<method>" form must
	// both match — server prefixes can change without a CLI rebuild.
	assert.True(t, isAskUserToolName("ask_user"))
	assert.True(t, isAskUserToolName("ux__ask_user"))
	assert.True(t, isAskUserToolName("anything__ask_user"))

	// Unrelated tool names must not match.
	assert.False(t, isAskUserToolName(""))
	assert.False(t, isAskUserToolName("ask_user_other"))
	assert.False(t, isAskUserToolName("filesystem__read"))
	assert.False(t, isAskUserToolName("ux__notify"))
	// A tool whose method merely contains "ask_user" must not match —
	// suffix means the full method, not a substring.
	assert.False(t, isAskUserToolName("ux__ask_user_v2"))
}
