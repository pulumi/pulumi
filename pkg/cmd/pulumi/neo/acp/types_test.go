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

package acp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionUpdateDiscriminators verifies each update payload injects its
// constant "sessionUpdate" tag on marshal (and still emits its own fields), so
// the wire discriminator stays owned by the type rather than the call site.
func TestSessionUpdateDiscriminators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		update any
		tag    string
		field  string // a payload field that must survive alongside the tag
	}{
		{"agent_message_chunk", AgentMessageChunk{Content: ContentBlock{Type: "text"}}, "agent_message_chunk", "content"},
		{"tool_call", ToolCallStart{ToolCallID: "tc_1", Title: "shell"}, "tool_call", "toolCallId"},
		{"tool_call_update", ToolCallProgress{ToolCallID: "tc_1", Status: "completed"}, "tool_call_update", "toolCallId"},
		{"plan", PlanUpdate{Entries: []PlanEntry{{Content: "do"}}}, "plan", "entries"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(tt.update)
			require.NoError(t, err)

			var m map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &m))
			assert.JSONEq(t, `"`+tt.tag+`"`, string(m["sessionUpdate"]))
			assert.Contains(t, m, tt.field, "payload fields must marshal alongside the discriminator")
		})
	}
}
