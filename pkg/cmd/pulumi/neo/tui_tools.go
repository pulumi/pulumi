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
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	toolFuncStyle = lipgloss.NewStyle().Bold(true)
	toolArgStyle  = lipgloss.NewStyle().Faint(true)
)

// toolLabelParts returns the display function name and argument for a tool call,
// formatted in function-call style like Read("file") or Bash("cmd").
func toolLabelParts(toolName string, args json.RawMessage) (funcName, arg string) {
	// Strip the "server__" prefix if present (e.g. "filesystem__read" -> "read").
	if _, method, ok := strings.Cut(toolName, "__"); ok {
		toolName = method
	}

	switch toolName {
	case "read_file", "read":
		if p := extractFilePathArg(args); p != "" {
			return "Read", p
		}
		return "Read", ""
	case "write_file", "write":
		if p := extractFilePathArg(args); p != "" {
			return "Write", p
		}
		return "Write", ""
	case "edit":
		if p := extractFilePathArg(args); p != "" {
			return "Edit", p
		}
		return "Edit", ""
	case "content_replace":
		if p := extractArg(args, "pattern"); p != "" {
			return "Replace", p
		}
		return "Replace", ""
	case "execute_command", "shell_execute":
		if cmd := extractArg(args, "command"); cmd != "" {
			if len(cmd) > 60 {
				cmd = cmd[:60] + "..."
			}
			return "Bash", cmd
		}
		return "Bash", ""
	case "search_files", "grep":
		if p := extractArg(args, "pattern"); p != "" {
			return "Search", p
		}
		return "Search", ""
	case "directory_tree":
		if p := extractArg(args, "path"); p != "" {
			return "ListDirectory", p
		}
		return "ListDirectory", "."
	case "pulumi_preview":
		return "PulumiPreview", ""
	case "pulumi_up":
		return "PulumiUp", ""
	default:
		return toolName, ""
	}
}

// toolLabel returns a compact plain-text label: FuncName("arg") or FuncName.
func toolLabel(toolName string, args json.RawMessage) string {
	funcName, arg := toolLabelParts(toolName, args)
	if arg != "" {
		return funcName + "(\"" + arg + "\")"
	}
	return funcName
}

// styledToolLabel returns a lipgloss-styled label: bold func name + dim args.
func styledToolLabel(toolName string, args json.RawMessage) string {
	funcName, arg := toolLabelParts(toolName, args)
	label := toolFuncStyle.Render(funcName)
	if arg != "" {
		label += toolArgStyle.Render(fmt.Sprintf("(\"%s\")", arg))
	}
	return label
}

// extractFilePathArg extracts a file path from args, checking both "file_path" and "path".
func extractFilePathArg(args json.RawMessage) string {
	if p := extractArg(args, "file_path"); p != "" {
		return p
	}
	return extractArg(args, "path")
}

// extractArg extracts a string argument from a JSON args object.
func extractArg(args json.RawMessage, key string) string {
	if len(args) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	var arr []string
	if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
		return strings.Join(arr, " ")
	}
	return ""
}
