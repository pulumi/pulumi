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

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"
)

// Filesystem is the local handler for the Neo `filesystem` tool. Method names and argument
// shapes mirror the upstream mcp-claude-code filesystem tools used by the cloud agent — see
// pulumi-service:cmd/agents/vendored/mcp-claude-code/mcp_claude_code/tools/filesystem/.
//
// Supported methods (this slice):
//
//	read           — args: {file_path, offset?, limit?}
//	write          — args: {file_path, content}
//	directory_tree — args: {path, depth?, include_filtered?}
//	edit           — args: {file_path, old_string, new_string, expected_replacements?}
//
// The remaining filesystem methods (grep, grep_ast, content_replace) return a structured
// "not yet implemented" error so the agent can degrade gracefully.
//
// All paths in incoming requests are absolute (the cloud agent assumes a sandboxed VM).
// We resolve them and reject anything that lands outside Root, so the agent can never read
// or write files outside the user's working directory.
type Filesystem struct {
	Root string
}

// NewFilesystem creates a Filesystem handler rooted at the given absolute directory.
func NewFilesystem(root string) (*Filesystem, error) {
	abs, err := canonicalRoot(root)
	if err != nil {
		return nil, fmt.Errorf("resolving filesystem root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("filesystem root %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("filesystem root %q is not a directory", abs)
	}
	return &Filesystem{Root: abs}, nil
}

// Invoke dispatches a single filesystem method call.
func (f *Filesystem) Invoke(_ context.Context, method string, args json.RawMessage) (any, error) {
	switch method {
	case "read":
		var p struct {
			FilePath string `json:"file_path"`
			Offset   int    `json:"offset,omitempty"`
			Limit    int    `json:"limit,omitempty"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding read args: %w", err)
		}
		return f.read(p.FilePath, p.Offset, p.Limit)
	case "write":
		var p struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding write args: %w", err)
		}
		return f.write(p.FilePath, p.Content)
	case "directory_tree":
		var p struct {
			Path  string `json:"path"`
			Depth int    `json:"depth,omitempty"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding directory_tree args: %w", err)
		}
		return f.directoryTree(p.Path, p.Depth)
	case "edit":
		var p editArgs
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding edit args: %w", err)
		}
		return f.edit(p)
	case "grep", "grep_ast", "content_replace":
		return nil, fmt.Errorf("filesystem method %q is not yet implemented in pulumi neo CLI mode", method)
	default:
		return nil, fmt.Errorf("unknown filesystem method %q", method)
	}
}

// resolve safely interprets a path supplied by the agent. The agent sends absolute paths
// (they were absolute inside its sandboxed VM); we accept those but require they resolve
// to a location under Root. Relative paths are resolved against Root.
func (f *Filesystem) resolve(p string) (string, error) {
	if p == "" {
		return "", errors.New("path is required")
	}
	target := p
	if !filepath.IsAbs(target) {
		target = filepath.Join(f.Root, target)
	}
	return resolveUnderRoot(f.Root, target, true)
}

func (f *Filesystem) read(p string, offset, limit int) (any, error) {
	abs, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	content := string(b)
	if offset > 0 || limit > 0 {
		// Apply line-based slicing matching the upstream tool's offset/limit semantics.
		lines := strings.Split(content, "\n")
		if offset >= len(lines) {
			content = ""
		} else {
			if offset > 0 {
				lines = lines[offset:]
			}
			if limit > 0 && limit < len(lines) {
				lines = lines[:limit]
			}
			content = strings.Join(lines, "\n")
		}
	}
	return map[string]any{"content": content}, nil
}

func (f *Filesystem) write(p, content string) (any, error) {
	abs, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		return nil, err
	}
	return map[string]any{"bytes_written": len(content)}, nil
}

// editArgs is the JSON shape for the `edit` tool. ExpectedReplacements is a pointer so
// "omitted" (defaults to 1) is distinguishable from an explicit 0 — the latter is a
// validation error rather than "replace nothing".
type editArgs struct {
	FilePath             string `json:"file_path"`
	OldString            string `json:"old_string"`
	NewString            string `json:"new_string"`
	ExpectedReplacements *int   `json:"expected_replacements,omitempty"`
}

// edit performs a single exact-string replacement. The response string and error wording
// are deliberately kept byte-identical to the upstream mcp-claude-code `edit` tool so the
// agent sees the same output whether the call ran on Cloud or CLI.
func (f *Filesystem) edit(p editArgs) (any, error) {
	abs, err := f.resolve(p.FilePath)
	if err != nil {
		return nil, err
	}

	expected := 1
	if p.ExpectedReplacements != nil {
		expected = *p.ExpectedReplacements
	}
	if expected < 0 {
		return "Error: Parameter 'expected_replacements' must be a non-negative number", nil
	}

	info, statErr := os.Stat(abs)
	fileExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}

	// Creation mode: file doesn't exist and old_string is empty.
	if !fileExists && p.OldString == "" {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(abs, []byte(p.NewString), 0o600); err != nil {
			return nil, err
		}
		return fmt.Sprintf("Successfully created file: %s (%d bytes)", p.FilePath, len(p.NewString)), nil
	}

	if !fileExists {
		return nil, statErr
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path %q is a directory, not a file", p.FilePath)
	}

	// Whitespace-only old_string on an existing file is ambiguous — reject it rather
	// than silently matching every run of whitespace in the file.
	if strings.TrimSpace(p.OldString) == "" {
		return "Error: Parameter 'old_string' cannot be empty for existing files", nil
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(raw) {
		return "Error: Cannot edit binary file: " + p.FilePath, nil
	}
	original := string(raw)

	occurrences := strings.Count(original, p.OldString)
	if occurrences == 0 {
		return "Error: The specified old_string was not found in the file content. " +
			"Please check that it matches exactly, including all whitespace and indentation.", nil
	}
	if occurrences != expected {
		return fmt.Sprintf(
			"Error: Found %d occurrences of the specified old_string, but expected %d. "+
				"Change your old_string to uniquely identify the target text, "+
				"or set expected_replacements=%d to replace all occurrences.",
			occurrences, expected, occurrences), nil
	}

	modified := strings.ReplaceAll(original, p.OldString, p.NewString)

	diff, err := renderUnifiedDiff(p.FilePath, original, modified)
	if err != nil {
		return nil, err
	}
	if diff == "" {
		return "No changes made to file: " + p.FilePath, nil
	}

	if err := os.WriteFile(abs, []byte(modified), 0o600); err != nil {
		return nil, err
	}
	return fmt.Sprintf(
		"Successfully edited file: %s (%d replacements applied)\n\n%s",
		p.FilePath, expected, diff,
	), nil
}

// renderUnifiedDiff produces a unified diff formatted the same way as the upstream Python
// tool: `--- <path> (original)` / `+++ <path> (modified)` headers, 3 lines of context,
// and the body fenced with the smallest backtick run that doesn't collide with the diff
// contents. Returns an empty string when the two inputs are identical.
func renderUnifiedDiff(path, original, modified string) (string, error) {
	if original == modified {
		return "", nil
	}
	body, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(modified),
		FromFile: path + " (original)",
		ToFile:   path + " (modified)",
		Context:  3,
	})
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", nil
	}
	ticks := 3
	for strings.Contains(body, strings.Repeat("`", ticks)) {
		ticks++
	}
	fence := strings.Repeat("`", ticks)
	return fmt.Sprintf("%sdiff\n%s%s\n", fence, body, fence), nil
}

// directoryTree returns a flat listing of the directory at p, optionally bounded by depth.
// Depth 0 (or unset) defaults to 1 (immediate children only).
func (f *Filesystem) directoryTree(p string, depth int) (any, error) {
	abs, err := f.resolve(p)
	if err != nil {
		return nil, err
	}
	if depth <= 0 {
		depth = 1
	}
	var entries []map[string]any
	err = filepath.Walk(abs, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(abs, path)
		if rel == "." {
			return nil
		}
		if strings.Count(rel, string(filepath.Separator))+1 > depth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entries = append(entries, map[string]any{"name": rel, "is_dir": info.IsDir()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i]["name"].(string) < entries[j]["name"].(string)
	})
	return map[string]any{"entries": entries}, nil
}
