// Copyright 2016-2025, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// maxFileReadSize is the maximum file size we'll read into memory (10 MB).
	maxFileReadSize = 10 * 1024 * 1024
	// maxCommandOutputSize is the maximum command output we'll capture (5 MB).
	maxCommandOutputSize = 5 * 1024 * 1024
	// maxSearchResults is the maximum number of search results returned.
	maxSearchResults = 100
)

// ApprovalRequest describes an operation that needs user approval before execution.
type ApprovalRequest struct {
	Tool    string
	Command string
	Message string
}

// ToolExecutor executes tool calls locally on the user's machine.
type ToolExecutor struct {
	workDir    string
	approvalFn func(ApprovalRequest) (bool, error)
}

// NewToolExecutor creates a new ToolExecutor rooted at the given working directory.
func NewToolExecutor(workDir string, approvalFn func(ApprovalRequest) (bool, error)) *ToolExecutor {
	return &ToolExecutor{
		workDir:    workDir,
		approvalFn: approvalFn,
	}
}

// Execute runs a tool call and returns the result.
func (e *ToolExecutor) Execute(ctx context.Context, toolCallID, name string, args json.RawMessage) ToolResponseEvent {
	result, err := e.dispatch(ctx, name, args)
	if err != nil {
		return ToolResponseEvent{
			Type:       "tool_response",
			ToolCallID: toolCallID,
			Name:       name,
			Content:    err.Error(),
			IsError:    true,
		}
	}
	return ToolResponseEvent{
		Type:       "tool_response",
		ToolCallID: toolCallID,
		Name:       name,
		Content:    result,
		IsError:    false,
	}
}

func (e *ToolExecutor) dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "read_file":
		return e.readFile(ctx, args)
	case "write_file":
		return e.writeFile(ctx, args)
	case "execute_command":
		return e.executeCommand(ctx, args)
	case "search_files":
		return e.searchFiles(ctx, args)
	case "pulumi_preview":
		return e.pulumiPreview(ctx, args)
	case "pulumi_up":
		return e.pulumiUp(ctx, args)
	case "git_diff":
		return e.gitCommand(ctx, "diff", args)
	case "git_status":
		return e.gitCommand(ctx, "status", args)
	case "git_log":
		return e.gitCommand(ctx, "log", args)
	case "git_show":
		return e.gitCommand(ctx, "show", args)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// resolvePath resolves a path relative to workDir and ensures it doesn't escape.
// It resolves symlinks to prevent symlink-based traversal attacks.
func (e *ToolExecutor) resolvePath(path string) (string, error) {
	var resolved string
	if filepath.IsAbs(path) {
		resolved = filepath.Clean(path)
	} else {
		resolved = filepath.Clean(filepath.Join(e.workDir, path))
	}

	// Resolve symlinks to prevent symlink-based path traversal.
	evaled, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet (e.g., write_file). Walk up to find the
			// nearest existing ancestor and resolve symlinks on that, then
			// append the remaining path components.
			evaled, err = resolveNewPath(resolved)
			if err != nil {
				return "", fmt.Errorf("path %q: %w", path, err)
			}
		} else {
			return "", fmt.Errorf("path %q: %w", path, err)
		}
	}

	workDirEvaled, err := filepath.EvalSymlinks(e.workDir)
	if err != nil {
		workDirEvaled = e.workDir
	}

	if !strings.HasPrefix(evaled, workDirEvaled+string(filepath.Separator)) && evaled != workDirEvaled {
		return "", fmt.Errorf("path %q is outside the working directory", path)
	}
	return evaled, nil
}

// resolveNewPath handles EvalSymlinks for paths where the file (and possibly
// some parent directories) don't yet exist. It walks up until it finds an
// existing ancestor, resolves symlinks on that, and re-appends the tail.
func resolveNewPath(resolved string) (string, error) {
	// Collect path components that don't exist yet.
	current := resolved
	var tail []string
	for {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root without finding an existing dir.
			return resolved, nil
		}
		tail = append([]string{filepath.Base(current)}, tail...)
		evaled, err := filepath.EvalSymlinks(parent)
		if err == nil {
			return filepath.Join(append([]string{evaled}, tail...)...), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		current = parent
	}
}

type readFileArgs struct {
	Path string `json:"path"`
}

func (e *ToolExecutor) readFile(_ context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := e.resolvePath(a.Path)
	if err != nil {
		return "", err
	}

	// Check file size before reading to prevent unbounded memory usage.
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	if info.Size() > maxFileReadSize {
		return "", fmt.Errorf("file is too large (%d bytes, max %d)", info.Size(), maxFileReadSize)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	return string(data), nil
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (e *ToolExecutor) writeFile(_ context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := e.resolvePath(a.Path)
	if err != nil {
		return "", err
	}

	// Create parent directories if needed.
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(resolved, []byte(a.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.Path), nil
}

type executeCommandArgs struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

func (e *ToolExecutor) executeCommand(ctx context.Context, args json.RawMessage) (string, error) {
	var a executeCommandArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// All shell commands go through approval -- this is the user's machine.
	cmdStr := a.Command
	if len(a.Args) > 0 {
		cmdStr += " " + strings.Join(a.Args, " ")
	}
	approved, err := e.approvalFn(ApprovalRequest{
		Tool:    "execute_command",
		Command: cmdStr,
		Message: fmt.Sprintf("Neo wants to run: %s", cmdStr),
	})
	if err != nil {
		return "", err
	}
	if !approved {
		return "", fmt.Errorf("command rejected by user")
	}

	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if len(a.Args) > 0 {
		cmd = exec.CommandContext(ctx, a.Command, a.Args...)
	} else {
		// If no args provided, run via shell.
		cmd = exec.CommandContext(ctx, "sh", "-c", a.Command)
	}
	cmd.Dir = e.workDir

	output, err := e.runCommandWithLimit(cmd)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after %s", timeout)
		}
		return output, fmt.Errorf("command failed: %w\nOutput: %s", err, output)
	}
	return output, nil
}

// runCommandWithLimit runs a command and captures output with a size limit.
func (e *ToolExecutor) runCommandWithLimit(cmd *exec.Cmd) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	limited := io.LimitReader(stdout, maxCommandOutputSize+1)
	data, readErr := io.ReadAll(limited)

	waitErr := cmd.Wait()

	result := string(data)
	if len(data) > maxCommandOutputSize {
		result = result[:maxCommandOutputSize] + "\n... (output truncated)"
	}

	if readErr != nil {
		return result, readErr
	}
	return result, waitErr
}

type searchFilesArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Include string `json:"include,omitempty"`
}

func (e *ToolExecutor) searchFiles(_ context.Context, args json.RawMessage) (string, error) {
	var a searchFilesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Validate the pattern is well-formed.
	if _, err := filepath.Match(a.Pattern, ""); err != nil {
		return "", fmt.Errorf("invalid pattern %q: %w", a.Pattern, err)
	}
	if a.Include != "" {
		if _, err := filepath.Match(a.Include, ""); err != nil {
			return "", fmt.Errorf("invalid include pattern %q: %w", a.Include, err)
		}
	}

	searchDir := e.workDir
	if a.Path != "" {
		resolved, err := e.resolvePath(a.Path)
		if err != nil {
			return "", err
		}
		searchDir = resolved
	}

	var matches []string
	err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() {
			// Skip hidden directories and common non-essential dirs.
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check include pattern.
		if a.Include != "" {
			matched, _ := filepath.Match(a.Include, d.Name())
			if !matched {
				return nil
			}
		}

		// Check name pattern.
		matched, _ := filepath.Match(a.Pattern, d.Name())
		if matched {
			relPath, _ := filepath.Rel(e.workDir, path)
			matches = append(matches, relPath)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("searching files: %w", err)
	}

	if len(matches) == 0 {
		return "no matching files found", nil
	}

	// Limit results -- capture total before truncating.
	totalCount := len(matches)
	if totalCount > maxSearchResults {
		matches = matches[:maxSearchResults]
		matches = append(matches, fmt.Sprintf("... and more (showing first %d of %d)", maxSearchResults, totalCount))
	}
	return strings.Join(matches, "\n"), nil
}

type pulumiArgs struct {
	Stack string `json:"stack,omitempty"`
	Cwd   string `json:"cwd,omitempty"`
}

func (e *ToolExecutor) pulumiPreview(ctx context.Context, args json.RawMessage) (string, error) {
	var a pulumiArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	cmdArgs := []string{"preview", "--json"}
	if a.Stack != "" {
		cmdArgs = append(cmdArgs, "--stack", a.Stack)
	}

	dir := e.workDir
	if a.Cwd != "" {
		resolved, err := e.resolvePath(a.Cwd)
		if err != nil {
			return "", err
		}
		dir = resolved
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pulumi", cmdArgs...)
	cmd.Dir = dir
	output, err := e.runCommandWithLimit(cmd)
	if err != nil {
		return output, fmt.Errorf("pulumi preview failed: %w", err)
	}
	return output, nil
}

func (e *ToolExecutor) pulumiUp(ctx context.Context, args json.RawMessage) (string, error) {
	var a pulumiArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	// Always require approval for pulumi up.
	approved, err := e.approvalFn(ApprovalRequest{
		Tool:    "pulumi_up",
		Command: "pulumi up",
		Message: "Neo wants to deploy changes with 'pulumi up'",
	})
	if err != nil {
		return "", err
	}
	if !approved {
		return "", fmt.Errorf("deployment rejected by user")
	}

	cmdArgs := []string{"up", "--yes", "--json"}
	if a.Stack != "" {
		cmdArgs = append(cmdArgs, "--stack", a.Stack)
	}

	dir := e.workDir
	if a.Cwd != "" {
		resolved, err := e.resolvePath(a.Cwd)
		if err != nil {
			return "", err
		}
		dir = resolved
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pulumi", cmdArgs...)
	cmd.Dir = dir
	output, err := e.runCommandWithLimit(cmd)
	if err != nil {
		return output, fmt.Errorf("pulumi up failed: %w", err)
	}
	return output, nil
}

type gitArgs struct {
	Args []string `json:"args,omitempty"`
}

func (e *ToolExecutor) gitCommand(ctx context.Context, subcommand string, args json.RawMessage) (string, error) {
	var a gitArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	cmdArgs := append([]string{subcommand}, a.Args...)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = e.workDir
	output, err := e.runCommandWithLimit(cmd)
	if err != nil {
		return output, fmt.Errorf("git %s failed: %w", subcommand, err)
	}
	return output, nil
}
