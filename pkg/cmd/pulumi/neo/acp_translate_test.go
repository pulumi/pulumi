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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
)

func TestPromptText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		blocks []acp.ContentBlock
		want   string
	}{
		{
			name:   "empty",
			blocks: nil,
			want:   "",
		},
		{
			name:   "text only",
			blocks: []acp.ContentBlock{{Type: "text", Text: "hello"}},
			want:   "hello",
		},
		{
			name: "concatenates text blocks",
			blocks: []acp.ContentBlock{
				{Type: "text", Text: "hello "},
				{Type: "text", Text: "world"},
			},
			want: "hello world",
		},
		{
			name: "resource link renders its uri",
			blocks: []acp.ContentBlock{
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go"},
			},
			want: "@file:///repo/main.go",
		},
		{
			name: "resource link shows label when it differs from uri",
			blocks: []acp.ContentBlock{
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go", Title: "Entry point"},
			},
			want: "@file:///repo/main.go (Entry point)",
		},
		{
			name: "resource link falls back to name when no title",
			blocks: []acp.ContentBlock{
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go"},
			},
			want: "@file:///repo/main.go",
		},
		{
			name: "text interleaved with a resource link",
			blocks: []acp.ContentBlock{
				{Type: "text", Text: "look at "},
				{Type: "resource_link", URI: "file:///repo/main.go", Name: "main.go"},
				{Type: "text", Text: " please"},
			},
			want: "look at @file:///repo/main.go please",
		},
		{
			name: "capability-gated blocks are ignored",
			blocks: []acp.ContentBlock{
				{Type: "text", Text: "keep"},
				{Type: "image"},
				{Type: "audio"},
				{Type: "resource"},
			},
			want: "keep",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, promptText(tt.blocks))
		})
	}
}

func TestToolTrackerTranslate(t *testing.T) {
	t.Parallel()

	var tr toolTracker

	u, ok := tr.translate(UIAssistantMessage{Content: "hi"})
	require.True(t, ok)
	require.IsType(t, acp.AgentMessageChunk{}, u)
	assert.Equal(t, "hi\n", u.(acp.AgentMessageChunk).Content.Text,
		"each complete message gets a trailing newline so successive messages don't run together")

	_, ok = tr.translate(UIAssistantMessage{})
	assert.False(t, ok, "empty assistant message produces no update")

	u, ok = tr.translate(UIToolStarted{Name: "filesystem__edit", Args: json.RawMessage(`{"x":1}`)})
	require.True(t, ok)
	start := u.(acp.ToolCallStart)
	assert.Equal(t, "tc_1", start.ToolCallID)
	assert.Equal(t, acp.ToolKindEdit, start.Kind)
	assert.Equal(t, acp.ToolStatusInProgress, start.Status)

	u, ok = tr.translate(UIToolCompleted{Name: "filesystem__edit", Result: json.RawMessage(`{"ok":true}`)})
	require.True(t, ok)
	upd := u.(acp.ToolCallProgress)
	assert.Equal(t, "tc_1", upd.ToolCallID, "completed update reuses the started call id")
	assert.Equal(t, acp.ToolStatusCompleted, upd.Status)

	// After completion the current id is cleared, so a stray progress event no
	// longer correlates to the finished call.
	_, ok = tr.translate(UIToolProgress{Message: "late"})
	assert.False(t, ok, "progress after completion produces no update")

	u, ok = tr.translate(UIToolCompleted{IsError: true})
	require.True(t, ok)
	assert.Equal(t, acp.ToolStatusFailed, u.(acp.ToolCallProgress).Status)

	u, ok = tr.translate(UITodoList{Items: []UITodoItem{{Content: "do", Status: "pending", Priority: "high"}}})
	require.True(t, ok)
	plan := u.(acp.PlanUpdate)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, acp.PlanEntry{Content: "do", Status: "pending", Priority: "high"}, plan.Entries[0])
}

// TestPlanEntriesClampVocabulary verifies the ACP egress path keeps plan entries
// within ACP's enums even if the backend drifts: empty or unknown priority falls
// back to "medium", empty or unknown status falls back to "pending".
func TestPlanEntriesClampVocabulary(t *testing.T) {
	t.Parallel()

	var tr toolTracker
	u, ok := tr.translate(UITodoList{Items: []UITodoItem{
		{Content: "valid", Status: "in_progress", Priority: "low"},
		{Content: "empty", Status: "", Priority: ""},
		{Content: "unknown", Status: "cancelled", Priority: "urgent"},
	}})
	require.True(t, ok)
	plan := u.(acp.PlanUpdate)
	require.Len(t, plan.Entries, 3)
	assert.Equal(t, acp.PlanEntry{Content: "valid", Status: "in_progress", Priority: "low"}, plan.Entries[0])
	assert.Equal(t, acp.PlanEntry{Content: "empty", Status: "pending", Priority: "medium"}, plan.Entries[1])
	assert.Equal(t, acp.PlanEntry{Content: "unknown", Status: "pending", Priority: "medium"}, plan.Entries[2])
}

func TestToolStartHasReadableTitleAndLocation(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	main := filepath.Join(cwd, "main.go")

	tr := toolTracker{cwd: cwd}
	u, ok := tr.translate(UIToolStarted{
		Name: "filesystem__read",
		Args: mustJSON(t, map[string]string{"file_path": main}),
	})
	require.True(t, ok)
	start := u.(acp.ToolCallStart)

	assert.Equal(t, "Read ./main.go", start.Title, "title should name the file, relative to cwd")
	assert.Equal(t, acp.ToolKindRead, start.Kind)
	require.Len(t, start.Locations, 1)
	assert.Equal(t, main, start.Locations[0].Path)
}

func TestToolTitleAndLocations(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	fsArgs := func(field, path string) toolArgs { return parseToolArgs(mustJSON(t, map[string]string{field: path})) }

	// Filesystem calls name the file they touch, relative to cwd when possible.
	assert.Equal(t, "Read ./pyproject.toml",
		toolTitle("filesystem__read", fsArgs("file_path", filepath.Join(cwd, "pyproject.toml")), cwd))
	assert.Equal(t, "Content replace ./src/a.go",
		toolTitle("filesystem__content_replace", fsArgs("path", filepath.Join(cwd, "src", "a.go")), cwd))
	assert.Equal(t, "Read ./src/a.go",
		toolTitle("filesystem__read", fsArgs("file_path", "src/a.go"), cwd),
		"relative paths absolutize against cwd, then relativize — matching the location")
	outside := filepath.Join(filepath.Dir(cwd), "elsewhere", "hosts")
	assert.Equal(t, "Read "+outside,
		toolTitle("filesystem__read", fsArgs("file_path", outside), cwd),
		"paths outside cwd stay absolute")
	assert.Equal(t, "Read",
		toolTitle("filesystem__read", toolArgs{}, cwd), "no path falls back to the verb")

	// Shell calls render the command itself, flattened to one line.
	assert.Equal(t, "git status",
		toolTitle("shell__shell_execute", parseToolArgs(json.RawMessage(`{"command":"git status"}`)), cwd))
	assert.Equal(t, "go build ./...",
		toolTitle("shell__shell_execute", parseToolArgs(json.RawMessage(`{"command":"go build\n  ./..."}`)), cwd),
		"multi-line commands collapse to a single line")
	assert.Equal(t, "Shell execute",
		toolTitle("shell__shell_execute", toolArgs{}, cwd), "no command falls back to the verb")

	assert.Equal(t, "weird", toolTitle("weird", toolArgs{}, ""), "names without a server prefix pass through")

	assert.Nil(t, toolLocations(toolArgs{}, cwd))
	assert.Nil(t, toolLocations(parseToolArgs(json.RawMessage(`{"command":"ls"}`)), cwd), "no file target -> no location")
	locs := toolLocations(parseToolArgs(json.RawMessage(`{"path":"/work/src"}`)), cwd)
	require.Len(t, locs, 1)
	assert.Equal(t, "/work/src", locs[0].Path, "absolute paths pass through unchanged")

	// Relative arguments are absolutized against cwd: ACP locations must be
	// absolute or the editor can't resolve the file.
	rel := toolLocations(fsArgs("file_path", "src/a.go"), cwd)
	require.Len(t, rel, 1)
	assert.Equal(t, filepath.Join(cwd, "src", "a.go"), rel[0].Path)
}

// mustJSON marshals v to a json.RawMessage, failing the test on error. Used to
// build tool-call args with OS-correct (cross-platform) file paths.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
