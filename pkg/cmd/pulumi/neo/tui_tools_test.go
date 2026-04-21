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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolLabelParts(t *testing.T) {
	t.Parallel()

	// The label parser is the glue between the agent's raw tool names and the
	// human-readable labels the TUI shows. Every branch has a distinct output
	// contract, so the table exercises each switch arm at least once. Entries
	// use json.RawMessage literals rather than a helper so the wire shape the
	// agent actually sends is visible in the test source.
	longCmd := strings.Repeat("x", 80)
	truncatedCmd := longCmd[:60] + "..."

	cases := []struct {
		name     string
		tool     string
		args     json.RawMessage
		wantFunc string
		wantArg  string
	}{
		{"read_file_with_file_path", "read_file", json.RawMessage(`{"file_path":"/tmp/x"}`), "Read", "/tmp/x"},
		{"read_with_path", "read", json.RawMessage(`{"path":"/tmp/x"}`), "Read", "/tmp/x"},
		{"read_no_args", "read_file", nil, "Read", ""},
		{"write_file", "write_file", json.RawMessage(`{"file_path":"/a/b"}`), "Write", "/a/b"},
		{"write_short", "write", nil, "Write", ""},
		{"edit_with_path", "edit", json.RawMessage(`{"file_path":"/x"}`), "Edit", "/x"},
		{"edit_no_path", "edit", json.RawMessage(`{}`), "Edit", ""},
		{"multi_edit_with_path", "multi_edit", json.RawMessage(`{"file_path":"/x"}`), "MultiEdit", "/x"},
		{"multi_edit_no_path", "multi_edit", json.RawMessage(`{}`), "MultiEdit", ""},
		{"content_replace_with_pattern", "content_replace", json.RawMessage(`{"pattern":"needle"}`), "Replace", "needle"},
		{"content_replace_no_pattern", "content_replace", nil, "Replace", ""},
		{"execute_command_short", "execute_command", json.RawMessage(`{"command":"ls -la"}`), "Bash", "ls -la"},
		{
			"execute_command_long_is_truncated",
			"execute_command",
			json.RawMessage(`{"command":"` + longCmd + `"}`),
			"Bash",
			truncatedCmd,
		},
		{"shell_execute_alias", "shell_execute", json.RawMessage(`{"command":"pwd"}`), "Bash", "pwd"},
		{"shell_execute_empty", "shell_execute", nil, "Bash", ""},
		{"search_files_with_pattern", "search_files", json.RawMessage(`{"pattern":"foo"}`), "Search", "foo"},
		{"grep_alias", "grep", json.RawMessage(`{"pattern":"bar"}`), "Search", "bar"},
		{"search_no_pattern", "search_files", nil, "Search", ""},
		{"directory_tree_with_path", "directory_tree", json.RawMessage(`{"path":"/x"}`), "ListDirectory", "/x"},
		// The CWD fallback: when no path is provided we still want the user to know we listed something.
		{"directory_tree_default_cwd", "directory_tree", nil, "ListDirectory", "."},
		{"pulumi_preview", "pulumi_preview", json.RawMessage(`{"stack":"dev"}`), "PulumiPreview", ""},
		{"pulumi_up", "pulumi_up", nil, "PulumiUp", ""},
		// Unknown tools fall through with their raw name — no prefix stripping or styling.
		{"unknown_tool_keeps_name", "mystery_tool", json.RawMessage(`{"a":1}`), "mystery_tool", ""},
		// The "server__method" prefix is stripped before the switch — this is what
		// the backend actually sends (e.g. "filesystem__read", "shell__shell_execute").
		{"server_prefix_stripped_read", "filesystem__read", json.RawMessage(`{"path":"/p"}`), "Read", "/p"},
		{"server_prefix_stripped_shell", "shell__shell_execute", json.RawMessage(`{"command":"echo"}`), "Bash", "echo"},
		{"server_prefix_stripped_unknown", "weird__unknown_method", nil, "unknown_method", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotFunc, gotArg := toolLabelParts(tc.tool, tc.args)
			assert.Equal(t, tc.wantFunc, gotFunc, "func name")
			assert.Equal(t, tc.wantArg, gotArg, "arg")
		})
	}
}

func TestToolLabel(t *testing.T) {
	t.Parallel()

	// toolLabel is the plain-text form used anywhere the TUI shows a tool name
	// without lipgloss styling (e.g. the busy block label).
	cases := []struct {
		name string
		tool string
		args json.RawMessage
		want string
	}{
		{"with_arg_renders_function_call", "read_file", json.RawMessage(`{"file_path":"x"}`), `Read("x")`},
		{"no_arg_renders_bare_name", "read_file", nil, "Read"},
		{"unknown_no_arg", "mystery", nil, "mystery"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, toolLabel(tc.tool, tc.args))
		})
	}
}

func TestStyledToolLabel(t *testing.T) {
	t.Parallel()

	// We don't pin ANSI escape bytes because lipgloss output is terminal-
	// dependent. Instead verify the human-readable runes survive — that's
	// the contract consumers actually rely on.
	withArg := styledToolLabel("read_file", json.RawMessage(`{"file_path":"/tmp/x"}`))
	assert.Contains(t, withArg, "Read")
	assert.Contains(t, withArg, `/tmp/x`)

	noArg := styledToolLabel("pulumi_up", nil)
	assert.Contains(t, noArg, "PulumiUp")
	assert.NotContains(t, noArg, `("`)
}

func TestExtractArg(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args json.RawMessage
		key  string
		want string
	}{
		{"empty_args", nil, "k", ""},
		{"empty_object", json.RawMessage(`{}`), "k", ""},
		{"missing_key", json.RawMessage(`{"other":"v"}`), "k", ""},
		{"string_value", json.RawMessage(`{"k":"v"}`), "k", "v"},
		{"empty_string_value", json.RawMessage(`{"k":""}`), "k", ""},
		// Array values are common for the shell tool's `command` argument when
		// the agent emits argv-style invocations; the TUI joins with spaces so
		// the label reads like a shell command.
		{"string_array_joined", json.RawMessage(`{"k":["a","b","c"]}`), "k", "a b c"},
		{"empty_array", json.RawMessage(`{"k":[]}`), "k", ""},
		// Non-string scalars aren't useful as a label — return empty rather than
		// stringifying (the label would be misleading anyway).
		{"number_returns_empty", json.RawMessage(`{"k":42}`), "k", ""},
		{"bool_returns_empty", json.RawMessage(`{"k":true}`), "k", ""},
		{"null_returns_empty", json.RawMessage(`{"k":null}`), "k", ""},
		{"malformed_json", json.RawMessage(`not-json`), "k", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, extractArg(tc.args, tc.key))
		})
	}
}

func TestExtractFilePathArg(t *testing.T) {
	t.Parallel()

	// file_path takes precedence over path — some tools emit both and file_path
	// is the canonical field. This ordering is load-bearing for the label.
	assert.Equal(t, "/tmp/a", extractFilePathArg(json.RawMessage(`{"file_path":"/tmp/a","path":"/tmp/b"}`)))
	// Falls back to "path" when file_path is absent.
	assert.Equal(t, "/tmp/b", extractFilePathArg(json.RawMessage(`{"path":"/tmp/b"}`)))
	// Neither present → empty.
	assert.Empty(t, extractFilePathArg(json.RawMessage(`{"other":"x"}`)))
	assert.Empty(t, extractFilePathArg(nil))
}
