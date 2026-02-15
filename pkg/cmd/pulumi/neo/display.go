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
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

// ANSI escape codes.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// brailleFrames are the animation frames for the TTY spinner.
var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Display handles rendering of Neo agent events to the terminal.
type Display struct {
	stdout       io.Writer
	stderr       io.Writer
	stdin        io.Reader
	interactive  bool
	jsonMode     bool
	hasTTY       bool
	termWidth    int
	mdRenderer   *glamour.TermRenderer
	mu           sync.Mutex
	spinning     bool
	currentTool  string
	spinnerLabel string
	spinnerDone  chan struct{}
}

// NewDisplay creates a new Display instance.
func NewDisplay(stdout, stderr io.Writer, stdin io.Reader, interactive, jsonMode bool) *Display {
	d := &Display{
		stdout:      stdout,
		stderr:      stderr,
		stdin:       stdin,
		interactive: interactive,
		jsonMode:    jsonMode,
		termWidth:   80,
	}

	// Detect TTY by checking if stdout is a terminal.
	if f, ok := stdout.(*os.File); ok {
		fd := int(f.Fd())
		if term.IsTerminal(fd) {
			d.hasTTY = true
			if w, _, err := term.GetSize(fd); err == nil && w > 0 {
				d.termWidth = w
			}
		}
	}

	// Initialize glamour markdown renderer for TTY mode.
	if d.hasTTY && !d.jsonMode {
		if r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(d.termWidth-4),
		); err == nil {
			d.mdRenderer = r
		}
	}

	return d
}

// agentEvent represents the inner structure of an SSE agent_event.
type agentEvent struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	EventBody json.RawMessage `json:"eventBody,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
}

// backendEvent represents an agent backend event (inside agentResponse).
type backendEvent struct {
	Type        string          `json:"type"`
	Content     string          `json:"content,omitempty"`
	IsFinal     bool            `json:"is_final,omitempty"`
	ToolCallID  string          `json:"tool_call_id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Args        json.RawMessage `json:"args,omitempty"`
	Message     string          `json:"message,omitempty"`
	ApprovalID  string          `json:"approval_id,omitempty"`
	Sensitivity string          `json:"sensitivity,omitempty"`
	IsError     bool            `json:"is_error,omitempty"`
	TaskName    string          `json:"task_name,omitempty"`
}

// ParsedEvent contains the parsed result of an event for the main loop.
type ParsedEvent struct {
	Type        string // "assistant_message", "exec_tool_call", "tool_response", "user_approval_request", "error", etc.
	IsFinal     bool
	ToolCallID  string
	ToolName    string
	ToolArgs    json.RawMessage
	ApprovalID  string
	Sensitivity string
	Message     string
	IsError     bool
}

// RenderEvent processes an SSE event and renders it to the terminal.
// Returns parsed event info for the main loop to act on.
func (d *Display) RenderEvent(event SSEEvent) *ParsedEvent {
	if d.jsonMode {
		return d.renderJSON(event)
	}
	return d.renderTerminal(event)
}

func (d *Display) renderJSON(event SSEEvent) *ParsedEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	// In JSON mode, output each event as a JSON line.
	fmt.Fprintf(d.stdout, "%s\n", string(event.Data))
	return d.parseEvent(event)
}

func (d *Display) renderTerminal(event SSEEvent) *ParsedEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	parsed := d.parseEvent(event)
	if parsed == nil {
		return nil
	}

	switch parsed.Type {
	case "assistant_message":
		d.stopSpinner()
		if parsed.Message != "" {
			if d.hasTTY {
				fmt.Fprintf(d.stdout, "\n%s\n", d.style(ansiBold+ansiCyan, "Neo"))
				rendered := d.renderMarkdown(parsed.Message)
				for _, line := range strings.Split(rendered, "\n") {
					fmt.Fprintf(d.stdout, "  %s\n", line)
				}
			} else {
				fmt.Fprintf(d.stdout, "\nNeo: %s\n", parsed.Message)
			}
		}

	case "exec_tool_call":
		d.startSpinner(parsed.ToolName, parsed.ToolArgs)

	case "exec_tool_call_progress":
		if parsed.Message != "" {
			d.updateSpinner(parsed.ToolName, parsed.Message)
		}

	case "tool_response":
		label := d.spinnerLabel
		d.stopSpinner()
		if label == "" {
			label = d.toolLabel(parsed.ToolName, nil)
		}
		if parsed.IsError {
			fmt.Fprintf(d.stderr, "  %s %s\n", d.red("✗"), parsed.ToolName)
			if parsed.Message != "" {
				fmt.Fprintf(d.stderr, "    %s\n", d.dim(d.truncate(parsed.Message, 200)))
			}
		} else {
			fmt.Fprintf(d.stdout, "  %s %s\n", d.green("✓"), label)
		}

	case "user_approval_request":
		d.stopSpinner()
		fmt.Fprintf(d.stdout, "\n%s\n", d.yellow("⚠ Approval required"))
		fmt.Fprintf(d.stdout, "  %s\n", parsed.Message)

	case "error":
		d.stopSpinner()
		fmt.Fprintf(d.stderr, "\n%s %s\n", d.red("✗ Error:"), parsed.Message)

	case "warning":
		fmt.Fprintf(d.stderr, "%s %s\n", d.yellow("⚠ Warning:"), parsed.Message)

	case "set_task_name":
		if parsed.Message != "" {
			fmt.Fprintf(d.stdout, "%s %s\n", d.dim("Session:"), parsed.Message)
		}

	case "cancelled":
		d.stopSpinner()
		fmt.Fprintf(d.stdout, "\n%s\n", d.yellow("Task cancelled."))
	}

	return parsed
}

func (d *Display) parseEvent(event SSEEvent) *ParsedEvent {
	if event.Event == "heartbeat" || event.Event == "task_status" {
		return nil
	}

	if event.Event != "agent_event" {
		return nil
	}

	var consoleEvt agentEvent
	if err := json.Unmarshal(event.Data, &consoleEvt); err != nil {
		return nil
	}

	if consoleEvt.Type == "agentResponse" {
		var be backendEvent
		if err := json.Unmarshal(consoleEvt.EventBody, &be); err != nil {
			return nil
		}
		return d.parseBackendEvent(be)
	}

	// Don't re-render our own input.
	return nil
}

func (d *Display) parseBackendEvent(be backendEvent) *ParsedEvent {
	switch be.Type {
	case "assistant_message":
		return &ParsedEvent{
			Type:    "assistant_message",
			Message: be.Content,
			IsFinal: be.IsFinal,
		}
	case "exec_tool_call":
		return &ParsedEvent{
			Type:       "exec_tool_call",
			ToolCallID: be.ToolCallID,
			ToolName:   be.Name,
			ToolArgs:   be.Args,
		}
	case "exec_tool_call_progress":
		return &ParsedEvent{
			Type:     "exec_tool_call_progress",
			ToolName: be.Name,
			Message:  be.Content,
		}
	case "tool_response":
		return &ParsedEvent{
			Type:     "tool_response",
			ToolName: be.Name,
			Message:  be.Content,
			IsError:  be.IsError,
		}
	case "user_approval_request":
		return &ParsedEvent{
			Type:        "user_approval_request",
			ApprovalID:  be.ApprovalID,
			Message:     be.Message,
			Sensitivity: be.Sensitivity,
		}
	case "error":
		return &ParsedEvent{
			Type:    "error",
			Message: be.Message,
			IsError: true,
		}
	case "warning":
		return &ParsedEvent{
			Type:    "warning",
			Message: be.Message,
		}
	case "set_task_name":
		return &ParsedEvent{
			Type:    "set_task_name",
			Message: be.TaskName,
		}
	case "cancelled":
		return &ParsedEvent{
			Type:    "cancelled",
			IsFinal: true,
		}
	default:
		return nil
	}
}

// PromptApproval asks the user to approve or reject an operation.
func (d *Display) PromptApproval(message string) (bool, error) {
	if !d.interactive {
		return false, fmt.Errorf("approval required in non-interactive mode")
	}

	fmt.Fprintf(d.stdout, "  Approve? [y/N]: ")
	scanner := bufio.NewScanner(d.stdin)
	if scanner.Scan() {
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return response == "y" || response == "yes", nil
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, fmt.Errorf("no input received")
}

// PromptUserInput prompts the user for a message in interactive mode.
func (d *Display) PromptUserInput() (string, error) {
	fmt.Fprintf(d.stdout, "\n%s ", d.bold("You:"))
	scanner := bufio.NewScanner(d.stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

// RenderSessionStart displays the session banner when starting a new session.
func (d *Display) RenderSessionStart(taskID string) {
	if d.hasTTY {
		fmt.Fprintf(d.stderr, "\n  %s %s\n\n", d.cyan("▸"), d.bold("Neo session "+taskID))
	} else {
		fmt.Fprintf(d.stderr, "Neo session: %s\n", taskID)
	}
}

// RenderSessionAttach displays the session banner when attaching to a session.
func (d *Display) RenderSessionAttach(taskID string) {
	if d.hasTTY {
		fmt.Fprintf(d.stderr, "\n  %s %s\n\n", d.cyan("▸"), d.bold("Attached to Neo session "+taskID))
	} else {
		fmt.Fprintf(d.stderr, "Attached to session: %s\n", taskID)
	}
}

// Formatting helpers.

func (d *Display) style(codes, s string) string {
	if d.hasTTY {
		return codes + s + ansiReset
	}
	return s
}

func (d *Display) bold(s string) string   { return d.style(ansiBold, s) }
func (d *Display) dim(s string) string    { return d.style(ansiDim, s) }
func (d *Display) red(s string) string    { return d.style(ansiRed, s) }
func (d *Display) green(s string) string  { return d.style(ansiGreen, s) }
func (d *Display) yellow(s string) string { return d.style(ansiYellow, s) }
func (d *Display) cyan(s string) string   { return d.style(ansiCyan, s) }

func (d *Display) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// renderMarkdown renders markdown text using glamour, falling back to plain text.
func (d *Display) renderMarkdown(text string) string {
	if d.mdRenderer == nil {
		return text
	}
	rendered, err := d.mdRenderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(rendered, "\n")
}

// toolLabel returns a human-readable label for a tool invocation.
func (d *Display) toolLabel(toolName string, args json.RawMessage) string {
	switch toolName {
	case "read_file":
		if p := extractArg(args, "path"); p != "" {
			return "Reading " + p
		}
		return "Reading file"
	case "write_file":
		if p := extractArg(args, "path"); p != "" {
			return "Writing " + p
		}
		return "Writing file"
	case "execute_command":
		if cmd := extractArg(args, "command"); cmd != "" {
			return "Running `" + d.truncate(cmd, 60) + "`"
		}
		return "Running command"
	case "search_files":
		if p := extractArg(args, "pattern"); p != "" {
			return "Searching for " + p
		}
		return "Searching files"
	case "pulumi_preview":
		return "Running pulumi preview"
	case "pulumi_up":
		return "Running pulumi up"
	case "git_status":
		return "Checking git status"
	case "git_diff":
		return "Running git diff"
	case "git_log":
		return "Running git log"
	case "git_show":
		return "Running git show"
	default:
		return toolName
	}
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
	if err := json.Unmarshal(v, &s); err != nil {
		return ""
	}
	return s
}

// Spinner implementation.

func (d *Display) startSpinner(toolName string, args json.RawMessage) {
	if d.spinning {
		d.stopSpinner()
	}
	d.spinning = true
	d.currentTool = toolName
	d.spinnerLabel = d.toolLabel(toolName, args)

	if d.hasTTY {
		d.spinnerDone = make(chan struct{})
		go d.animateSpinner()
	} else {
		fmt.Fprintf(d.stdout, "  ⠿ %s...\n", d.spinnerLabel)
	}
}

func (d *Display) animateSpinner() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-d.spinnerDone:
			return
		case <-ticker.C:
			d.mu.Lock()
			if !d.spinning {
				d.mu.Unlock()
				return
			}
			fmt.Fprintf(d.stdout, "\r  %s %s\033[K", d.style(ansiCyan, brailleFrames[frame]), d.spinnerLabel)
			d.mu.Unlock()
			frame = (frame + 1) % len(brailleFrames)
		}
	}
}

func (d *Display) updateSpinner(toolName, progress string) {
	if d.spinning && progress != "" {
		label := d.toolLabel(toolName, nil)
		d.spinnerLabel = label + ": " + d.truncate(progress, 60)
	}
}

func (d *Display) stopSpinner() {
	if d.spinning {
		d.spinning = false
		if d.spinnerDone != nil {
			close(d.spinnerDone)
			d.spinnerDone = nil
		}
		if d.hasTTY {
			fmt.Fprintf(d.stdout, "\r\033[K")
		}
		d.currentTool = ""
		d.spinnerLabel = ""
	}
}
