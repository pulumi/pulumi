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

// FileChange describes a file modification made by a tool.
type FileChange struct {
	Path       string
	OldContent string // empty for new files
	NewContent string
	IsNew      bool
}

// ToolExecutor executes tool calls locally on the user's machine.
type ToolExecutor struct {
	workDir      string
	approvalFn   func(ApprovalRequest) (bool, error)
	onFileChange func(FileChange)
}

// NewToolExecutor creates a new ToolExecutor rooted at the given working directory.
func NewToolExecutor(workDir string, approvalFn func(ApprovalRequest) (bool, error),
	onFileChange func(FileChange),
) *ToolExecutor {
	return &ToolExecutor{
		workDir:      workDir,
		approvalFn:   approvalFn,
		onFileChange: onFileChange,
	}
}

// Execute runs a tool call and returns the result.
func (e *ToolExecutor) Execute(ctx context.Context, toolCallID, name string, args json.RawMessage) ToolResponseEvent {
	// Defense in depth: if the service strips the args field (e.g. due to a
	// missing field in the typed model), treat nil/empty as an empty object so
	// individual tools get clear "field is required" errors instead of the
	// cryptic "unexpected end of JSON input" from json.Unmarshal(nil, ...).
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}

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

// CanExecute returns true if the executor knows how to handle the named tool.
// Tools not recognized here are executed server-side by the agent's MCP servers.
func (e *ToolExecutor) CanExecute(name string) bool {
	switch name {
	case "read_file", "read",
		"write_file", "write",
		"execute_command", "shell_execute",
		"search_files", "grep",
		"directory_tree",
		"edit", "content_replace",
		"pulumi_preview",
		"pulumi_up",
		"git_diff", "git_status", "git_log", "git_show",
		"ask_user":
		return true
	default:
		return false
	}
}

func (e *ToolExecutor) dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	switch name {
	case "read_file", "read":
		return e.readFile(ctx, args)
	case "write_file", "write":
		return e.writeFile(ctx, args)
	case "execute_command", "shell_execute":
		return e.executeCommand(ctx, args)
	case "search_files", "grep":
		return e.searchFiles(ctx, args)
	case "directory_tree":
		return e.directoryTree(ctx, args)
	case "edit":
		return e.editFile(ctx, args)
	case "content_replace":
		return e.contentReplace(ctx, args)
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
	case "ask_user":
		return e.askUser(ctx, args)
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
	Path     string `json:"path"`
	FilePath string `json:"file_path"` // MCP tool field name (agent sends this)
}

func (a *readFileArgs) path() string {
	if a.FilePath != "" {
		return a.FilePath
	}
	return a.Path
}

func (e *ToolExecutor) readFile(_ context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.path() == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := e.resolvePath(a.path())
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	// If the path is a directory, list its entries (matching the server-side read
	// tool behavior of returning a useful error/listing instead of crashing).
	if info.IsDir() {
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return "", fmt.Errorf("reading directory: %w", err)
		}
		var sb strings.Builder
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			sb.WriteString(name)
			sb.WriteString("\n")
		}
		return sb.String(), nil
	}

	// Check file size before reading to prevent unbounded memory usage.
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
	Path     string `json:"path"`
	FilePath string `json:"file_path"` // MCP tool field name (agent sends this)
	Content  string `json:"content"`
}

func (a *writeFileArgs) path() string {
	if a.FilePath != "" {
		return a.FilePath
	}
	return a.Path
}

func (e *ToolExecutor) writeFile(_ context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.path() == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := e.resolvePath(a.path())
	if err != nil {
		return "", err
	}

	// Read existing content before writing (for diff rendering).
	oldContent, readErr := os.ReadFile(resolved)
	isNew := os.IsNotExist(readErr)

	// Create parent directories if needed.
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(resolved, []byte(a.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	if e.onFileChange != nil {
		e.onFileChange(FileChange{
			Path:       a.path(),
			OldContent: string(oldContent),
			NewContent: a.Content,
			IsNew:      isNew,
		})
	}

	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.path()), nil
}

// executeCommandArgs handles both formats:
//   - execute_command: {"command": "echo hello"} or {"command": "echo", "args": ["hello"]}
//   - shell_execute (MCP): {"command": ["echo", "hello"], "directory": "/path"}
//
// The "command" field is polymorphic: it can be a string or an array of strings.
// We use json.RawMessage to handle this, then parse accordingly.
type executeCommandArgs struct {
	RawCommand json.RawMessage `json:"command"`
	Args       []string        `json:"args,omitempty"`
	Directory  string          `json:"directory,omitempty"` // MCP shell_execute field
	Timeout    *int            `json:"timeout,omitempty"`   // MCP shell_execute field (seconds)
}

func (e *ToolExecutor) executeCommand(ctx context.Context, args json.RawMessage) (string, error) {
	var a executeCommandArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	// Parse the polymorphic "command" field: string or []string.
	var command string
	var cmdArgs []string
	if len(a.RawCommand) == 0 {
		return "", fmt.Errorf("command is required")
	}
	// Try as string first.
	var cmdStr string
	if err := json.Unmarshal(a.RawCommand, &cmdStr); err == nil {
		command = cmdStr
		cmdArgs = a.Args
	} else {
		// Try as array of strings (MCP shell_execute format).
		var cmdArr []string
		if err := json.Unmarshal(a.RawCommand, &cmdArr); err != nil {
			return "", fmt.Errorf("command must be a string or array of strings")
		}
		if len(cmdArr) == 0 {
			return "", fmt.Errorf("command is required")
		}
		command = cmdArr[0]
		cmdArgs = cmdArr[1:]
	}

	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := 5 * time.Minute
	if a.Timeout != nil && *a.Timeout > 0 {
		timeout = time.Duration(*a.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if len(cmdArgs) > 0 {
		cmd = exec.CommandContext(ctx, command, cmdArgs...)
	} else {
		// If no args provided, run via shell.
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	// Use the MCP directory field if provided, otherwise workDir.
	cmd.Dir = e.workDir
	if a.Directory != "" {
		resolved, err := e.resolvePath(a.Directory)
		if err != nil {
			return "", err
		}
		cmd.Dir = resolved
	}

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

type directoryTreeArgs struct {
	Path  string `json:"path,omitempty"`
	Depth int    `json:"depth,omitempty"`
}

const maxTreeEntries = 500

func (e *ToolExecutor) directoryTree(_ context.Context, args json.RawMessage) (string, error) {
	var a directoryTreeArgs
	if len(args) > 0 {
		if err := json.Unmarshal(args, &a); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
	}

	maxDepth := a.Depth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	rootDir := e.workDir
	if a.Path != "" {
		resolved, err := e.resolvePath(a.Path)
		if err != nil {
			return "", err
		}
		rootDir = resolved
	}

	var lines []string
	count := 0
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, _ := filepath.Rel(rootDir, path)
		if rel == "." {
			return nil
		}

		depth := strings.Count(rel, string(filepath.Separator))
		if depth >= maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden directories and common non-essential dirs.
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
		}

		count++
		if count > maxTreeEntries {
			return fmt.Errorf("truncated")
		}

		indent := strings.Repeat("  ", depth)
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		lines = append(lines, indent+name)
		return nil
	})

	if err != nil && err.Error() != "truncated" {
		return "", fmt.Errorf("listing directory: %w", err)
	}

	if len(lines) == 0 {
		return "empty directory", nil
	}

	result := strings.Join(lines, "\n")
	if count > maxTreeEntries {
		result += fmt.Sprintf("\n... (truncated, showing first %d entries)", maxTreeEntries)
	}
	return result, nil
}

type editFileArgs struct {
	Path      string `json:"path"`
	FilePath  string `json:"file_path"`  // MCP tool field name (agent sends this)
	OldStr    string `json:"old_str"`
	NewStr    string `json:"new_str"`
	OldString string `json:"old_string"` // MCP edit tool field name
	NewString string `json:"new_string"` // MCP edit tool field name
}

func (a *editFileArgs) path() string {
	if a.FilePath != "" {
		return a.FilePath
	}
	return a.Path
}

func (e *ToolExecutor) editFile(_ context.Context, args json.RawMessage) (string, error) {
	var a editFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.path() == "" {
		return "", fmt.Errorf("path is required")
	}

	// Support both field name conventions.
	oldStr := a.OldStr
	if oldStr == "" {
		oldStr = a.OldString
	}
	newStr := a.NewStr
	if newStr == "" {
		newStr = a.NewString
	}

	if oldStr == "" {
		return "", fmt.Errorf("old_str is required")
	}

	resolved, err := e.resolvePath(a.path())
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, oldStr) {
		return "", fmt.Errorf("old_str not found in file")
	}

	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	if e.onFileChange != nil {
		e.onFileChange(FileChange{
			Path:       a.path(),
			OldContent: content,
			NewContent: newContent,
			IsNew:      false,
		})
	}

	return fmt.Sprintf("edited %s", a.path()), nil
}

// contentReplaceArgs matches the MCP content_replace tool schema.
// This is a multi-file text replacement tool (different from edit).
type contentReplaceArgs struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Path        string `json:"path"`
	FilePattern string `json:"file_pattern,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

func (e *ToolExecutor) contentReplace(_ context.Context, args json.RawMessage) (string, error) {
	var a contentReplaceArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if a.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := e.resolvePath(a.Path)
	if err != nil {
		return "", err
	}

	filePattern := a.FilePattern
	if filePattern == "" {
		filePattern = "*"
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", a.Path, err)
	}

	// Collect files to process.
	var files []string
	if info.IsDir() {
		err = filepath.WalkDir(resolved, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			matched, _ := filepath.Match(filePattern, d.Name())
			if matched {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("walking directory: %w", err)
		}
	} else {
		files = []string{resolved}
	}

	var results []string
	totalReplacements := 0
	filesModified := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content := string(data)
		count := strings.Count(content, a.Pattern)
		if count == 0 {
			continue
		}

		totalReplacements += count
		filesModified++
		relPath, _ := filepath.Rel(e.workDir, file)
		results = append(results, fmt.Sprintf("%s: %d replacements", relPath, count))

		if !a.DryRun {
			oldContent := content
			newContent := strings.ReplaceAll(content, a.Pattern, a.Replacement)
			if err := os.WriteFile(file, []byte(newContent), 0o644); err != nil {
				return "", fmt.Errorf("writing %q: %w", file, err)
			}
			if e.onFileChange != nil {
				e.onFileChange(FileChange{
					Path:       relPath,
					OldContent: oldContent,
					NewContent: newContent,
					IsNew:      false,
				})
			}
		}
	}

	if totalReplacements == 0 {
		return fmt.Sprintf("no occurrences of pattern %q found", a.Pattern), nil
	}

	prefix := "Made"
	if a.DryRun {
		prefix = "Dry run:"
	}
	return fmt.Sprintf("%s %d replacements in %d files:\n%s",
		prefix, totalReplacements, filesModified, strings.Join(results, "\n")), nil
}

type askUserArgs struct {
	Message            string `json:"message"`
	ShowApprovalButton bool   `json:"show_approval_button"`
}

func (e *ToolExecutor) askUser(_ context.Context, args json.RawMessage) (string, error) {
	var a askUserArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if a.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	approved, err := e.approvalFn(ApprovalRequest{
		Tool:    "ask_user",
		Message: a.Message,
	})
	if err != nil {
		return "", fmt.Errorf("prompting user: %w", err)
	}
	if !approved {
		return "User declined the request.", nil
	}
	return "True", nil
}

type pulumiArgs struct {
	// CLI-native field names.
	Stack string `json:"stack,omitempty"`
	Cwd   string `json:"cwd,omitempty"`
	// MCP tool field names (agent sends these).
	StackName      string `json:"stack_name,omitempty"`
	LocalPulumiDir string `json:"local_pulumi_dir,omitempty"`
}

func (a *pulumiArgs) stack() string {
	if a.Stack != "" {
		return a.Stack
	}
	return a.StackName
}

func (a *pulumiArgs) dir(workDir string, resolve func(string) (string, error)) (string, error) {
	cwd := a.Cwd
	if cwd == "" {
		cwd = a.LocalPulumiDir
	}
	if cwd == "" {
		return workDir, nil
	}
	return resolve(cwd)
}

func (e *ToolExecutor) pulumiPreview(ctx context.Context, args json.RawMessage) (string, error) {
	var a pulumiArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	cmdArgs := []string{"preview", "--json"}
	if s := a.stack(); s != "" {
		cmdArgs = append(cmdArgs, "--stack", s)
	}

	dir, err := a.dir(e.workDir, e.resolvePath)
	if err != nil {
		return "", err
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

	stack := a.stack()

	// Always require approval for pulumi up.
	msg := "Neo wants to deploy changes with 'pulumi up'"
	if stack != "" {
		msg = fmt.Sprintf("Neo wants to deploy changes with 'pulumi up' on stack %q", stack)
	}
	approved, err := e.approvalFn(ApprovalRequest{
		Tool:    "pulumi_up",
		Command: "pulumi up",
		Message: msg,
	})
	if err != nil {
		return "", err
	}
	if !approved {
		return "", fmt.Errorf("deployment rejected by user")
	}

	cmdArgs := []string{"up", "--yes", "--json"}
	if stack != "" {
		cmdArgs = append(cmdArgs, "--stack", stack)
	}

	dir, err := a.dir(e.workDir, e.resolvePath)
	if err != nil {
		return "", err
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
