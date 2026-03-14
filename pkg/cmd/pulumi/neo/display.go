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
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/term"
)

//go:embed pulumipus.ans
var pulumipusArt string

// ANSI escape codes.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
	ansiWhite   = "\033[97m"  // bright white foreground
	ansiBgGray  = "\033[100m" // dark grey background
)

// brailleFrames are the animation frames for the TTY spinner.
var brailleFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// toolSpinnerFrames cycle for the in-progress tool marker.
var toolSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// shimmerColors are ANSI 256-color codes used for the shimmering effect on in-progress tools.
var shimmerColors = []string{
	"\033[38;5;245m", // gray
	"\033[38;5;247m", // lighter gray
	"\033[38;5;250m", // bright
	"\033[38;5;253m", // brightest
	"\033[38;5;250m", // bright
	"\033[38;5;247m", // lighter gray
}

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
	turnCount    int // tracks conversation turns for separator rendering

	// Tool log: completed tools are printed permanently via writeAboveInputBar;
	// the in-progress tool animates on the input bar's animation line.

	// Pending file changes queued by onFileChange, rendered after tool completion.
	pendingFileChanges []FileChange

	// Streaming assistant message state.
	streamingText    string // accumulated text from partial assistant_messages
	streamingStarted bool   // whether we've started rendering streaming output
	streamingLines   int    // number of terminal lines written during streaming (for clearing)

	// Thinking shimmer state.
	thinking     bool
	thinkingDone chan struct{}

	// In-flight tool calls indexed by tool_call_id, so tool_response can look
	// up the original args (e.g. to show the shell command that was run).
	pendingTools map[string]*ParsedEvent

	// Tool execution counters for the current agent turn (reset on assistant_message).
	toolCount      int // total tools completed this turn
	toolErrorCount int // tools that returned errors this turn

	// Deduplication: track event IDs we've already processed to avoid re-executing
	// tool calls when the SSE stream replays historical events.
	processedEvents map[string]bool

	// Debug event log file (enabled via PULUMI_NEO_DEBUG_LOG env var).
	debugLog *os.File

	// Welcome box state for redrawing with the session link.
	welcomeOrg     string
	welcomeWorkDir string
	welcomeUser    string
	welcomeLines   int // total lines the welcome box occupies

	// Approval mode (mutable via Shift+Tab).
	approvalMode *string

	// Raw terminal input support.
	stdinFd int // file descriptor for stdin (-1 if not a TTY)

	// Persistent input bar state.
	inputBarActive  bool
	inputText       []rune
	inputCh         chan string // buffered cap 1: queued user message
	approvalCh      chan bool   // unbuffered: approval response
	cancelCh        chan byte   // buffered cap 1: Ctrl+C (0x03) or Ctrl+D (0x04)
	pendingApproval bool
	approvalMsg     string   // message shown during approval prompt
	rawState        *term.State
	inputDone       chan struct{} // closed to stop inputLoop
}

// NewDisplay creates a new Display instance.
func NewDisplay(stdout, stderr io.Writer, stdin io.Reader, interactive, jsonMode bool) *Display {
	d := &Display{
		stdout:       stdout,
		stderr:       stderr,
		stdin:        stdin,
		interactive:  interactive,
		jsonMode:     jsonMode,
		termWidth:    80,
		stdinFd:      -1,
		pendingTools:    make(map[string]*ParsedEvent),
		processedEvents: make(map[string]bool),
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

	// Detect stdin TTY for raw input support.
	if f, ok := stdin.(*os.File); ok {
		fd := int(f.Fd())
		if term.IsTerminal(fd) {
			d.stdinFd = fd
		}
	}

	// Open debug log file if PULUMI_NEO_DEBUG_LOG is set.
	if logPath := os.Getenv("PULUMI_NEO_DEBUG_LOG"); logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			d.debugLog = f
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

// SetApprovalMode stores a pointer to the approval mode for Shift+Tab cycling.
func (d *Display) SetApprovalMode(mode *string) {
	d.approvalMode = mode
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
	EventID     string // unique event ID from the server, used for deduplication
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

	// Dedup: skip events we've already processed (SSE replays on reconnection).
	if parsed.EventID != "" {
		if d.processedEvents[parsed.EventID] {
			d.logDebug("DEDUP: skipping event_id=%s type=%s", parsed.EventID, parsed.Type)
			return nil
		}
		d.processedEvents[parsed.EventID] = true
	}

	d.logDebug("RENDER: type=%s tool=%s tool_call_id=%s is_final=%v",
		parsed.Type, parsed.ToolName, parsed.ToolCallID, parsed.IsFinal)

	switch parsed.Type {
	case "assistant_message":
		d.stopThinking()
		d.stopSpinner()
		if !parsed.IsFinal {
			// Streaming partial: print raw text delta for incremental display.
			if d.hasTTY && parsed.Message != "" {
				if d.inputBarActive {
					// When input bar is active, accumulate streaming text silently.
					// The thinking shimmer provides visual feedback; the final
					// glamour-rendered message will be displayed in one shot.
					d.streamingText = parsed.Message
					d.streamingStarted = true
				} else {
					delta := parsed.Message[len(d.streamingText):]
					if !d.streamingStarted {
						fmt.Fprintf(d.stdout, "\r\n  ")
						d.streamingStarted = true
						d.streamingLines = 1
					}
					if delta != "" {
						// Print delta and track lines for later clearing.
						fmt.Fprint(d.stdout, delta)
						d.streamingLines = d.countTerminalLines(parsed.Message, 2)
					}
					d.streamingText = parsed.Message
				}
			}
			// Non-TTY: silently consume partials, only show the final message.
		} else if parsed.Message != "" {
			// Final message: re-render complete text with glamour.
			d.stopThinking()
			d.writeAboveInputBar(func() {
				// Print a compact tool summary if any tools ran this turn.
				d.renderToolSummary()

				if d.hasTTY && d.streamingStarted {
					// Clear the raw streamed text, then render with markdown formatting.
					d.clearStreamedOutput()
					d.renderAssistantMessage(parsed.Message)
				} else if d.hasTTY {
					d.renderAssistantMessage(parsed.Message)
				} else {
					fmt.Fprintf(d.stdout, "\nNeo: %s\n", parsed.Message)
				}
			})
			d.streamingText = ""
			d.streamingStarted = false
			d.streamingLines = 0
		}

	case "exec_tool_call":
		d.stopThinking()
		// Track the tool call so we can look up args at response time.
		// Note: d.mu is already held by renderTerminal — do not re-lock.
		if parsed.ToolCallID != "" {
			d.pendingTools[parsed.ToolCallID] = parsed
		}
		d.startToolSpinner(parsed.ToolName, parsed.ToolArgs)

	case "exec_tool_call_progress":
		if parsed.Message != "" {
			d.updateSpinner(parsed.ToolName, parsed.Message)
		}

	case "tool_response":
		// Look up the original exec_tool_call args by tool_call_id.
		// If the tool_call_id is not in pendingTools, this is a server echo
		// of a tool we already completed — skip it to avoid duplicate lines.
		if parsed.ToolCallID != "" {
			pt, ok := d.pendingTools[parsed.ToolCallID]
			if !ok {
				d.logDebug("TOOL_RESPONSE_SKIP: tool_call_id=%s tool=%s (already completed)",
					parsed.ToolCallID, parsed.ToolName)
				break // Already completed — skip duplicate.
			}
			d.completeToolSpinner(parsed.ToolName, pt.ToolArgs, parsed.IsError)
			delete(d.pendingTools, parsed.ToolCallID)
		} else {
			d.completeToolSpinner(parsed.ToolName, nil, parsed.IsError)
		}
		d.toolCount++
		if parsed.IsError {
			d.toolErrorCount++
		}

	case "user_approval_request":
		d.stopSpinner()
		d.writeAboveInputBar(func() {
			fmt.Fprintf(d.stdout, "\r\n  %s\r\n", d.yellow("⚠ Approval required"))
			fmt.Fprintf(d.stdout, "    %s\r\n", parsed.Message)
		})

	case "error":
		d.stopSpinner()
		d.writeAboveInputBar(func() {
			fmt.Fprintf(d.stderr, "\r\n  %s %s\r\n", d.red("✗ Error:"), parsed.Message)
		})

	case "warning":
		d.writeAboveInputBar(func() {
			fmt.Fprintf(d.stderr, "  %s %s\r\n", d.yellow("⚠"), parsed.Message)
		})

	case "set_task_name":
		// Quietly absorbed — the task name is visible in the session list.

	case "cancelled":
		d.stopSpinner()
		d.writeAboveInputBar(func() {
			fmt.Fprintf(d.stdout, "\r\n  %s\r\n", d.dim("Session cancelled."))
		})
	}

	return parsed
}

func (d *Display) logDebug(format string, args ...interface{}) {
	if d.debugLog != nil {
		fmt.Fprintf(d.debugLog, "[%s] ", time.Now().Format("15:04:05.000"))
		fmt.Fprintf(d.debugLog, format, args...)
		fmt.Fprintln(d.debugLog)
	}
}

// LogDebug writes a debug log entry if PULUMI_NEO_DEBUG_LOG is set.
func (d *Display) LogDebug(format string, args ...interface{}) {
	d.logDebug(format, args...)
}

func (d *Display) parseEvent(event SSEEvent) *ParsedEvent {
	d.logDebug("SSE event=%s id=%s data=%s", event.Event, event.ID, string(event.Data))

	if event.Event == "heartbeat" {
		return nil
	}

	if event.Event == "task_status" {
		var status struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(event.Data, &status); err != nil {
			return nil
		}
		d.logDebug("SSE task_status=%s", status.Status)
		// Surface terminal/idle states so the event loop can unblock.
		switch status.Status {
		case "failed":
			return &ParsedEvent{Type: "error", Message: "Agent task failed.", IsError: true}
		case "idle":
			// Agent finished its turn without sending a final assistant_message.
			return &ParsedEvent{Type: "task_idle"}
		default:
			return nil
		}
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
		parsed := d.parseBackendEvent(be)
		if parsed != nil {
			parsed.EventID = consoleEvt.ID
		}
		return parsed
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
			Type:       "tool_response",
			ToolCallID: be.ToolCallID,
			ToolName:   be.Name,
			Message:    be.Content,
			IsError:    be.IsError,
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

	fmt.Fprintf(d.stdout, "    Approve? [y/N]: ")
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
	if d.hasTTY {
		if d.turnCount > 0 {
			// Dim separator between conversation turns (full width).
			w := d.termWidth - 4
			if w < 20 {
				w = 20
			}
			fmt.Fprintf(d.stdout, "\n  %s\n", d.dim(strings.Repeat("─", w)))
		}
		d.turnCount++

		// Use raw terminal input when we have a TTY stdin and an approval mode to cycle.
		if d.stdinFd >= 0 && d.approvalMode != nil {
			return d.rawInput()
		}

		fmt.Fprintf(d.stdout, "\n%s ", d.style(ansiCyan+ansiBold, "❯"))
	} else {
		fmt.Fprintf(d.stdout, "\n> ")
		d.turnCount++
	}

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

// stripANSI removes ANSI escape sequences from a string using a simple state machine.
// Handles both CSI sequences (\033[...letter) and OSC sequences (\033]...ST).
func stripANSI(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\033' {
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == '[' {
					// CSI sequence: skip until terminating letter.
					i += 2
					for i < len(runes) {
						c := runes[i]
						if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
							break
						}
						i++
					}
					continue
				}
				if next == ']' {
					// OSC sequence: skip until ST (\033\\ or BEL).
					i += 2
					for i < len(runes) {
						if runes[i] == '\a' {
							break
						}
						if runes[i] == '\033' && i+1 < len(runes) && runes[i+1] == '\\' {
							i++ // skip the backslash too
							break
						}
						i++
					}
					continue
				}
			}
			// Bare ESC — skip just this character.
			continue
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

// visibleWidth returns the number of visible runes in a string after stripping ANSI codes.
func visibleWidth(s string) int {
	return len([]rune(stripANSI(s)))
}

// boxLine renders one content line inside the box with correct padding.
func (d *Display) boxLine(w io.Writer, content string, inner int) {
	vis := visibleWidth(content)
	pad := inner - vis
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(w, "  %s%s%s%s\n",
		d.magenta("│"), content, strings.Repeat(" ", pad), d.magenta("│"))
}

// RenderWelcome displays the startup banner for interactive sessions (TTY only).
// If consoleURL is non-empty it is rendered as a clickable link inside the box.
func (d *Display) RenderWelcome(org, workDir, username, consoleURL string) {
	if !d.hasTTY {
		return
	}

	// Store params for potential redraw with URL.
	d.welcomeOrg = org
	d.welcomeWorkDir = workDir
	d.welcomeUser = username

	// Shorten home directory to ~ for display.
	displayDir := workDir
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, workDir); err == nil && !strings.HasPrefix(rel, "..") {
			displayDir = "~/" + rel
		}
	}

	// The Pulumipus art is pre-rendered ANSI from pulumipus.ans (generated by chafa).
	artLines := strings.Split(strings.TrimRight(pulumipusArt, "\n"), "\n")

	// Box width: 2-space indent + │ + inner + │. Use full terminal width.
	boxWidth := d.termWidth
	boxInner := boxWidth - 4
	if boxInner < 20 {
		boxInner = 20
	}

	w := d.stderr
	title := " Pulumi Neo "

	// Top border: ╭──── Pulumi Neo ────────────────────╮
	titleLen := len([]rune(title))
	leftDash := 4
	rightDash := boxInner - leftDash - titleLen
	if rightDash < 1 {
		rightDash = 1
	}
	fmt.Fprintf(w, "\n  %s%s%s%s%s\n",
		d.magenta("╭"), d.magenta(strings.Repeat("─", leftDash)),
		d.style(ansiBold+ansiMagenta, title),
		d.magenta(strings.Repeat("─", rightDash)), d.magenta("╮"))

	// Blank line.
	d.boxLine(w, "", boxInner)

	// Greeting line (above the art).
	greeting := d.greeting(username)
	greetContent := "  " + greeting
	d.boxLine(w, greetContent, boxInner)

	// Blank line.
	d.boxLine(w, "", boxInner)

	// Art lines, left-aligned with a small indent.
	artIndent := 4
	for _, line := range artLines {
		vis := visibleWidth(line)
		rightPad := boxInner - artIndent - vis
		if rightPad < 0 {
			rightPad = 0
		}
		content := strings.Repeat(" ", artIndent) + line + strings.Repeat(" ", rightPad)
		fmt.Fprintf(w, "  %s%s%s\n",
			d.magenta("│"), content, d.magenta("│"))
	}

	// Blank line.
	d.boxLine(w, "", boxInner)

	// Info line: path · org
	infoText := displayDir
	if org != "" {
		infoText += " · org: " + org
	}
	// Truncate path if info line is too long.
	maxInfo := boxInner - 4 // 2 spaces padding on each side
	if len([]rune(infoText)) > maxInfo && org != "" {
		orgSuffix := " · org: " + org
		maxPath := maxInfo - len([]rune(orgSuffix))
		if maxPath > 3 {
			pathRunes := []rune(displayDir)
			if len(pathRunes) > maxPath {
				displayDir = string(pathRunes[:maxPath-3]) + "..."
			}
			infoText = displayDir + orgSuffix
		}
	}
	infoContent := "  " + d.dim(infoText)
	d.boxLine(w, infoContent, boxInner)

	// Session link line (OSC 8 hyperlink for terminals that support it).
	if consoleURL != "" {
		// Use OSC 8 escape to make the URL a clickable hyperlink in supported terminals.
		linkText := consoleURL
		// Truncate the visible link text if needed, but keep the full URL in the hyperlink.
		prefix := "⟡ "
		maxLink := boxInner - 4 - len([]rune(prefix))
		if len([]rune(linkText)) > maxLink {
			linkText = string([]rune(linkText)[:maxLink-3]) + "..."
		}
		hyperlink := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", consoleURL, linkText)
		linkContent := "  " + d.dim(prefix+hyperlink)
		d.boxLine(w, linkContent, boxInner)
	}

	// Blank line.
	d.boxLine(w, "", boxInner)

	// Bottom border: ╰──────────────────────────────────╯
	fmt.Fprintf(w, "  %s%s%s\n",
		d.magenta("╰"), d.magenta(strings.Repeat("─", boxInner)), d.magenta("╯"))

	// Count lines: leading blank(1) + top border(1) + blank(1) + greeting(1)
	// + blank(1) + art(N) + blank(1) + info(1) + [link(0 or 1)] + blank(1) + bottom(1)
	d.welcomeLines = 9 + len(artLines)
	if consoleURL != "" {
		d.welcomeLines++
	}
}

// RedrawWelcome overwrites the previously rendered welcome box in-place, adding
// the session link. Called after task creation when the session URL is available.
// The cursor is expected to be right below the welcome box (rawInput erased its
// prompt frame, leaving the cursor at the line after the box's bottom border).
func (d *Display) RedrawWelcome(consoleURL string) {
	if !d.hasTTY || d.welcomeLines == 0 {
		return
	}

	w := d.stderr
	// Move cursor up to the line with \n before the top border.
	// RenderWelcome wrote welcomeLines lines; cursor is on the line after the last one.
	fmt.Fprintf(w, "\033[%dA\r", d.welcomeLines)

	// Clear from here to end of screen, then redraw the box with the URL.
	fmt.Fprintf(w, "\033[J")
	d.RenderWelcome(d.welcomeOrg, d.welcomeWorkDir, d.welcomeUser, consoleURL)
}

// RenderSessionStart displays the session link after task creation.
func (d *Display) RenderSessionStart(taskID, consoleURL string) {
	if d.hasTTY {
		if consoleURL != "" {
			// Render an OSC 8 hyperlink so the URL is clickable in supporting terminals.
			hyperlink := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", consoleURL, consoleURL)
			fmt.Fprintf(d.stderr, "\n  %s %s\n", d.dim("⟡"), d.dim(hyperlink))
		} else {
			// No console URL available; show just the task ID dimmed.
			fmt.Fprintf(d.stderr, "\n  %s %s\n", d.dim("⟡"), d.dim(taskID))
		}
	} else {
		if consoleURL != "" {
			fmt.Fprintf(d.stderr, "Neo session: %s\n", consoleURL)
		} else {
			fmt.Fprintf(d.stderr, "Neo session: %s\n", taskID)
		}
	}
}

// RenderWarning displays a non-fatal warning message.
func (d *Display) RenderWarning(msg string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writeAboveInputBar(func() {
		fmt.Fprintf(d.stderr, "  %s %s\r\n", d.yellow("⚠"), d.dim(msg))
	})
}

// RenderSessionAttach displays the banner when attaching to an existing session.
func (d *Display) RenderSessionAttach(taskID, consoleURL string) {
	if d.hasTTY {
		fmt.Fprintf(d.stderr, "\n  %s %s\n", d.style(ansiCyan, "✻"), d.bold("Pulumi Neo"))
		if consoleURL != "" {
			fmt.Fprintf(d.stderr, "  %s %s\n\n", d.dim("View on Web:"), consoleURL)
		} else {
			fmt.Fprintf(d.stderr, "  %s %s\n\n", d.dim("attached to session"), d.dim(taskID))
		}
	} else {
		fmt.Fprintf(d.stderr, "Attached to session: %s\n", taskID)
	}
}

// RenderTeleportToCloud prints a banner when handing the session off to the cloud.
func (d *Display) RenderTeleportToCloud(consoleURL string) {
	if d.hasTTY {
		d.writeAboveInputBar(func() {
			fmt.Fprintf(d.stderr,
				"\n  %s %s\n  %s\n  %s %s\n  %s\n  %s\n\n",
				d.style(ansiCyan, "╭─"),
				d.bold("Teleported to Cloud"),
				d.style(ansiCyan, "│")+"  Session handed off to Pulumi Cloud.",
				d.style(ansiCyan, "│")+"  Continue at:",
				consoleURL,
				d.style(ansiCyan, "│"),
				d.style(ansiCyan, "╰─"),
			)
		})
	} else {
		fmt.Fprintf(d.stderr, "Session handed off to Pulumi Cloud.\nContinue at: %s\n", consoleURL)
	}
}

// --- Time-of-day greeting ---

// timeOfDayKey returns a greeting bucket for the given hour (0-23).
func timeOfDayKey(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

var greetingTemplates = map[string][]string{
	"morning": {
		"Morning, %s. What are we working on?",
		"Good morning, %s. What can I build for you?",
		"Morning, %s. Ready to ship something?",
		"Rise and ship, %s. What are we building?",
	},
	"afternoon": {
		"Afternoon, %s. What can I help with?",
		"Good afternoon, %s. What are we building?",
		"Hey %s, what can I help you with?",
		"Afternoon, %s. What should we work on?",
	},
	"evening": {
		"Evening, %s. What can I help with?",
		"Good evening, %s. What are we working on?",
		"Evening, %s. What should we build?",
		"Hey %s, what can I help with tonight?",
	},
	"night": {
		"Late one, %s? What can I help with?",
		"Burning the midnight oil, %s? What are we building?",
		"Night owl mode, %s. What can I help with?",
		"Up late, %s? Let's build something.",
	},
}

func (d *Display) greeting(name string) string {
	if name == "" {
		return "What do you want to build today?"
	}
	key := timeOfDayKey(time.Now().Hour())
	templates := greetingTemplates[key]
	return fmt.Sprintf(templates[rand.Intn(len(templates))], d.bold(name)) //nolint:gosec
}

// --- Thinking shimmer + spinner ---

// thinkingVerbs contains Pulumi-themed verbs for the thinking indicator.
var thinkingVerbs = []string{
	"Puluminating", "Cloudforming", "Driftifying", "Ephemerizing",
	"Stacking", "Reconcifying", "Planifesting", "Speculating",
	"Dreamforming", "Outputting", "Resourcifying", "Providering",
	"Previewizing", "Pipelining", "Summoning", "Materializing", "Crunching",
}

func pickThinkingVerb() string {
	// 60% "Thinking", 40% random from thinkingVerbs.
	if rand.Intn(5) < 3 { //nolint:gosec
		return "Thinking"
	}
	return thinkingVerbs[rand.Intn(len(thinkingVerbs))] //nolint:gosec
}

// StartThinking starts the thinking shimmer animation. Called from session.go.
func (d *Display) StartThinking() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.startThinking()
}

// renderToolSummary resets tool counters for the turn. Individual tool lines are
// already printed by startToolSpinner/completeToolSpinner, so no summary line needed.
// Caller must hold d.mu.
func (d *Display) renderToolSummary() {
	d.toolCount = 0
	d.toolErrorCount = 0
}

// RenderUserMessage displays the submitted user message above the input bar
// with bright white text on a dark grey background.
func (d *Display) RenderUserMessage(msg string) {
	d.writeAboveInputBar(func() {
		styled := d.style(ansiWhite+ansiBgGray, " "+msg+" ")
		fmt.Fprintf(d.stdout, "\r\n%s %s\r\n", d.style(ansiCyan+ansiBold, "❯"), styled)
	})
}

// startThinking is the internal version (caller must hold mutex).
func (d *Display) startThinking() {
	if d.thinking {
		d.stopThinking()
	}
	d.thinking = true
	if d.hasTTY {
		d.thinkingDone = make(chan struct{})
		verb := pickThinkingVerb()
		go d.animateThinking(verb + "...")
	}
}

// stopThinking stops the thinking shimmer (caller must hold mutex).
func (d *Display) stopThinking() {
	if d.thinking {
		d.thinking = false
		if d.thinkingDone != nil {
			close(d.thinkingDone)
			d.thinkingDone = nil
		}
		if d.hasTTY {
			if d.inputBarActive {
				// Clear the animation line above the input bar.
				fmt.Fprintf(d.stdout, "\0337\033[2A\r\033[K\0338")
			} else {
				fmt.Fprintf(d.stdout, "\r\033[K")
			}
		}
	}
}

func (d *Display) animateThinking(text string) {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	runes := []rune(text)
	pos := 0
	frame := 0
	for {
		select {
		case <-d.thinkingDone:
			return
		case <-ticker.C:
			d.mu.Lock()
			if !d.thinking {
				d.mu.Unlock()
				return
			}
			shimmer := buildShimmer(runes, pos)
			spinner := brailleFrames[frame]
			if d.inputBarActive {
				// Write to the animation line above the input bar.
				// Save cursor, move up 2 to animation line, overwrite, restore.
				fmt.Fprintf(d.stdout, "\0337\033[2A\r  %s %s\033[K\0338",
					ansiCyan+spinner+ansiReset, shimmer)
			} else {
				fmt.Fprintf(d.stdout, "\r\033[K  %s %s", ansiCyan+spinner+ansiReset, shimmer)
			}
			d.mu.Unlock()
			pos = (pos + 1) % len(runes)
			frame = (frame + 1) % len(brailleFrames)
		}
	}
}

// buildShimmer creates a shimmer effect: one character is bold+magenta, its
// neighbors are normal magenta, and the rest are dim magenta.
func buildShimmer(runes []rune, pos int) string {
	n := len(runes)
	var buf strings.Builder
	for i, r := range runes {
		diff := i - pos
		if diff < 0 {
			diff = -diff
		}
		if wrapDiff := n - diff; wrapDiff < diff {
			diff = wrapDiff
		}

		switch {
		case diff == 0:
			buf.WriteString(ansiBold + ansiMagenta)
			buf.WriteRune(r)
			buf.WriteString(ansiReset)
		case diff == 1:
			buf.WriteString(ansiMagenta)
			buf.WriteRune(r)
			buf.WriteString(ansiReset)
		default:
			buf.WriteString(ansiDim + ansiMagenta)
			buf.WriteRune(r)
			buf.WriteString(ansiReset)
		}
	}
	return buf.String()
}

// --- Approval mode cycling ---

var approvalModeLabels = map[string]string{
	"manual":   "Suggest",
	"balanced": "Balanced",
	"auto":     "Auto-approve",
}

var approvalModeCycle = []string{"manual", "balanced", "auto"}

func nextApprovalMode(current string) string {
	for i, mode := range approvalModeCycle {
		if mode == current {
			return approvalModeCycle[(i+1)%len(approvalModeCycle)]
		}
	}
	return approvalModeCycle[0]
}

// --- Raw terminal input (Shift+Tab support) ---

func (d *Display) rawInput() (string, error) {
	oldState, err := term.MakeRaw(d.stdinFd)
	if err != nil {
		// Fall back to line-buffered input.
		fmt.Fprintf(d.stdout, "\n%s ", d.style(ansiCyan+ansiBold, "❯"))
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
	defer term.Restore(d.stdinFd, oldState)

	label := approvalModeLabels[*d.approvalMode]
	if label == "" {
		label = *d.approvalMode
	}

	// Render the prompt frame (in raw mode, \n doesn't do CR, so use \r\n):
	//   ─────────────────────
	// ❯ [cursor]
	//   ─────────────────────
	//   ⏵⏵ Balanced (shift+tab to cycle) · esc to interrupt
	sepW := d.termWidth - 4
	if sepW < 20 {
		sepW = 20
	}
	sep := strings.Repeat("─", sepW)
	fmt.Fprintf(d.stdout, "\r\n  %s\r\n%s \r\n  %s\r\n  %s",
		d.dim(sep),
		d.style(ansiCyan+ansiBold, "❯"),
		d.dim(sep),
		d.dim(fmt.Sprintf("⏵⏵ %s (shift+tab to cycle) · esc to interrupt", label)))
	// Move cursor back up to the prompt line (2 lines up from mode indicator line).
	fmt.Fprintf(d.stdout, "\033[2A\r%s ", d.style(ansiCyan+ansiBold, "❯"))

	var line []rune
	reader := bufio.NewReader(d.stdin)

	for {
		buf := make([]byte, 1)
		_, err := reader.Read(buf)
		if err != nil {
			fmt.Fprintf(d.stdout, "\r\n")
			return "", err
		}

		b := buf[0]
		switch {
		case b == 0x0d: // Enter (raw mode sends CR)
			// Erase the prompt frame (top sep, prompt, bottom sep, mode indicator)
			// so the caller can re-render the message styled via RenderUserMessage.
			fmt.Fprintf(d.stdout, "\033[A\r\033[J")
			return strings.TrimSpace(string(line)), nil

		case b == 0x03: // Ctrl+C
			fmt.Fprintf(d.stdout, "\r\n\033[K\r\n\033[K")
			return "", fmt.Errorf("interrupted")

		case b == 0x04: // Ctrl+D
			fmt.Fprintf(d.stdout, "\r\n\033[K\r\n\033[K")
			return "", io.EOF

		case b == 0x7f || b == 0x08: // Backspace / BS
			if len(line) > 0 {
				line = line[:len(line)-1]
				d.renderPromptLine(string(line))
			}

		case b == 0x1b: // ESC — start of escape sequence
			b2, err := reader.ReadByte()
			if err != nil {
				continue
			}
			if b2 == '[' {
				b3, err := reader.ReadByte()
				if err != nil {
					continue
				}
				if b3 == 'Z' { // Shift+Tab (ESC [ Z)
					if d.approvalMode != nil {
						*d.approvalMode = nextApprovalMode(*d.approvalMode)
						d.renderModeIndicator()
						d.renderPromptLine(string(line))
					}
				}
				// Other escape sequences (arrows etc.) are silently consumed.
			}

		case b >= 0x20 && b <= 0x7e: // Printable ASCII
			line = append(line, rune(b))
			d.renderPromptLine(string(line))
		}
	}
}

func (d *Display) renderPromptLine(text string) {
	fmt.Fprintf(d.stdout, "\r%s %s\033[K", d.style(ansiCyan+ansiBold, "❯"), text)
}

func (d *Display) renderModeIndicator() {
	if d.approvalMode == nil {
		return
	}
	label := approvalModeLabels[*d.approvalMode]
	if label == "" {
		label = *d.approvalMode
	}
	// Save cursor, move down 2 lines (past bottom separator to mode indicator), rewrite, restore cursor.
	fmt.Fprintf(d.stdout, "\0337\033[2B\r  %s\033[K\0338",
		d.dim(fmt.Sprintf("⏵⏵ %s (shift+tab to cycle) · esc to interrupt", label)))
}

// --- Persistent input bar ---

// InputCh returns the channel for receiving user messages when the input bar is active.
// Returns nil when the input bar is not active (non-TTY, non-interactive).
func (d *Display) InputCh() <-chan string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.inputBarActive {
		return d.inputCh
	}
	return nil
}

// CancelCh returns the channel for receiving cancel signals (Ctrl+C = 0x03, Ctrl+D = 0x04).
// Always selected on in the event loop so the user can interrupt even when the agent is busy.
// Returns nil when the input bar is not active.
func (d *Display) CancelCh() <-chan byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.inputBarActive {
		return d.cancelCh
	}
	return nil
}

// ApprovalCh returns the channel for receiving approval responses.
// Returns nil when no approval is pending.
func (d *Display) ApprovalCh() <-chan bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pendingApproval {
		return d.approvalCh
	}
	return nil
}

// EnableInputBar enters raw mode, draws the input bar, and starts the input goroutine.
// No-op if not a TTY or stdinFd is unavailable.
func (d *Display) EnableInputBar() {
	if !d.hasTTY || d.stdinFd < 0 || d.approvalMode == nil {
		return
	}

	oldState, err := term.MakeRaw(d.stdinFd)
	if err != nil {
		return
	}

	d.mu.Lock()
	d.rawState = oldState
	d.inputBarActive = true
	d.inputText = nil
	d.inputCh = make(chan string, 1)
	d.approvalCh = make(chan bool, 1)
	d.cancelCh = make(chan byte, 1)
	d.inputDone = make(chan struct{})
	d.drawInputBar()
	d.mu.Unlock()

	go d.inputLoop()
}

// DisableInputBar erases the input bar, restores the terminal, and stops the input goroutine.
func (d *Display) DisableInputBar() {
	d.mu.Lock()
	if !d.inputBarActive {
		d.mu.Unlock()
		return
	}
	d.inputBarActive = false
	d.eraseInputBar()
	if d.inputDone != nil {
		close(d.inputDone)
		d.inputDone = nil
	}
	d.mu.Unlock()

	if d.rawState != nil {
		term.Restore(d.stdinFd, d.rawState)
		d.rawState = nil
	}
}

// RequestApproval shows an approval prompt in the input bar and blocks until answered.
// Falls back to PromptApproval when the input bar is not active.
func (d *Display) RequestApproval(message string) (bool, error) {
	d.mu.Lock()
	if !d.inputBarActive {
		d.mu.Unlock()
		return d.PromptApproval(message)
	}
	d.pendingApproval = true
	d.approvalMsg = message
	d.inputText = nil
	d.drawInputBar()
	d.mu.Unlock()

	approved := <-d.approvalCh

	d.mu.Lock()
	d.pendingApproval = false
	d.approvalMsg = ""
	d.inputText = nil
	d.drawInputBar()
	d.mu.Unlock()

	return approved, nil
}

// drawInputBar renders the 5-line input bar at the current cursor position.
// Caller must hold d.mu. Uses \r\n since we're in raw mode.
//
// Layout:
//
//	  ⠋ Thinking...              ← animation line (blank placeholder)
//	  ─────────────────────────── ← top separator
//	❯ user types here             ← prompt line
//	  ─────────────────────────── ← bottom separator
//	  ⏵⏵ Balanced (shift+tab)    ← mode indicator
func (d *Display) drawInputBar() {
	sepW := d.termWidth - 4
	if sepW < 20 {
		sepW = 20
	}
	sep := d.dim(strings.Repeat("─", sepW))

	label := ""
	if d.approvalMode != nil {
		label = approvalModeLabels[*d.approvalMode]
		if label == "" {
			label = *d.approvalMode
		}
	}

	// Line 1: animation placeholder (blank).
	fmt.Fprintf(d.stdout, "\r\n\033[K")
	// Line 2: top separator.
	fmt.Fprintf(d.stdout, "\r\n  %s\033[K", sep)
	// Line 3: prompt line.
	if d.pendingApproval {
		fmt.Fprintf(d.stdout, "\r\n  %s \033[K", d.yellow("Approve? [y/N]:"))
	} else {
		fmt.Fprintf(d.stdout, "\r\n%s %s\033[K", d.style(ansiCyan+ansiBold, "❯"), string(d.inputText))
	}
	// Line 4: bottom separator.
	fmt.Fprintf(d.stdout, "\r\n  %s\033[K", sep)
	// Line 5: mode indicator.
	fmt.Fprintf(d.stdout, "\r\n  %s\033[K",
		d.dim(fmt.Sprintf("⏵⏵ %s (shift+tab to cycle) · esc to interrupt", label)))
	// Move cursor back to prompt line (up 2 from mode indicator).
	if d.pendingApproval {
		// Position after "Approve? [y/N]: "
		fmt.Fprintf(d.stdout, "\033[2A\r  %s ", d.yellow("Approve? [y/N]:"))
	} else {
		fmt.Fprintf(d.stdout, "\033[2A\r%s %s", d.style(ansiCyan+ansiBold, "❯"), string(d.inputText))
	}
}

// eraseInputBar clears the 5-line input bar. Caller must hold d.mu.
// Moves up 2 lines from prompt to animation line, then clears to end of screen.
func (d *Display) eraseInputBar() {
	// From the prompt line, go up 2 to the animation line, then clear everything below.
	fmt.Fprintf(d.stdout, "\r\033[2A\r\033[J")
}

// writeAboveInputBar erases the input bar, calls fn (which writes output), then redraws.
// If the input bar is not active, fn is called directly.
func (d *Display) writeAboveInputBar(fn func()) {
	if !d.inputBarActive {
		fn()
		return
	}
	d.eraseInputBar()
	fn()
	// fn() typically ends with \r\n, leaving cursor at a blank line. Move up
	// so drawInputBar's leading \r\n doesn't produce a double-spaced gap.
	fmt.Fprintf(d.stdout, "\033[A")
	d.drawInputBar()
}

// inputLoop reads raw keystrokes in a goroutine and dispatches them.
func (d *Display) inputLoop() {
	reader := bufio.NewReader(d.stdin)
	buf := make([]byte, 4) // enough for multi-byte UTF-8

	for {
		select {
		case <-d.inputDone:
			return
		default:
		}

		n, err := reader.Read(buf[:1])
		if err != nil || n == 0 {
			return
		}

		b := buf[0]

		// Check if this is a multi-byte UTF-8 sequence.
		if b >= 0x80 && b < 0xC0 {
			// Continuation byte on its own — skip.
			continue
		}

		switch {
		case b == 0x0d: // Enter
			d.mu.Lock()
			if d.pendingApproval {
				// Treat as 'n' (default deny).
				d.mu.Unlock()
				select {
				case d.approvalCh <- false:
				default:
				}
			} else {
				text := strings.TrimSpace(string(d.inputText))
				d.inputText = nil
				// Clear the typed text on the prompt line in-place (don't
				// redraw the full bar — RenderUserMessage will do that).
				fmt.Fprintf(d.stdout, "\r%s \033[K", d.style(ansiCyan+ansiBold, "❯"))
				d.mu.Unlock()
				if text != "" {
					select {
					case d.inputCh <- text:
					default:
						// Channel full — message already queued.
					}
				}
			}

		case b == 0x03: // Ctrl+C
			// Signal cancel FIRST, before acquiring any locks, so the event
			// loop can exit even if the shimmer goroutine holds d.mu.
			select {
			case d.cancelCh <- 0x03:
			default:
			}
			select {
			case d.inputCh <- "":
			default:
			}
			// Best-effort UI cleanup (non-blocking lock attempt).
			if d.mu.TryLock() {
				d.inputText = nil
				fmt.Fprintf(d.stdout, "\r%s \033[K", d.style(ansiCyan+ansiBold, "❯"))
				d.mu.Unlock()
			}

		case b == 0x1a: // Ctrl+Z — suspend
			d.mu.Lock()
			// Restore terminal to cooked mode so the shell works normally.
			if d.rawState != nil {
				term.Restore(d.stdinFd, d.rawState)
			}
			d.eraseInputBar()
			d.mu.Unlock()

			// Send SIGTSTP to our own process group to actually suspend.
			_ = syscall.Kill(0, syscall.SIGTSTP)

			// When resumed (SIGCONT), re-enter raw mode and redraw.
			d.mu.Lock()
			if newState, err := term.MakeRaw(d.stdinFd); err == nil {
				d.rawState = newState
			}
			d.drawInputBar()
			d.mu.Unlock()

		case b == 0x04: // Ctrl+D
			// Always signal cancel — Ctrl+D means "exit" regardless of agent state.
			select {
			case d.cancelCh <- 0x04:
			default:
			}
			return

		case b == 0x7f || b == 0x08: // Backspace
			d.mu.Lock()
			if len(d.inputText) > 0 {
				d.inputText = d.inputText[:len(d.inputText)-1]
				if d.pendingApproval {
					// No text editing during approval.
				} else {
					fmt.Fprintf(d.stdout, "\r%s %s\033[K",
						d.style(ansiCyan+ansiBold, "❯"), string(d.inputText))
				}
			}
			d.mu.Unlock()

		case b == 0x1b: // ESC sequence
			b2, err := reader.ReadByte()
			if err != nil {
				continue
			}
			if b2 == '[' {
				b3, err := reader.ReadByte()
				if err != nil {
					continue
				}
				if b3 == 'Z' { // Shift+Tab
					d.mu.Lock()
					if d.approvalMode != nil {
						*d.approvalMode = nextApprovalMode(*d.approvalMode)
						d.renderInputBarModeIndicator()
					}
					d.mu.Unlock()
				}
				// Other sequences (arrows etc.) silently consumed.
			}

		default:
			// Handle printable characters including multi-byte UTF-8.
			if b >= 0xC0 {
				// Multi-byte: determine how many more bytes to read.
				size := 2
				if b >= 0xE0 {
					size = 3
				}
				if b >= 0xF0 {
					size = 4
				}
				buf[0] = b
				for i := 1; i < size; i++ {
					nb, err := reader.ReadByte()
					if err != nil {
						break
					}
					buf[i] = nb
				}
				r, _ := utf8.DecodeRune(buf[:size])
				if r != utf8.RuneError {
					d.mu.Lock()
					if d.pendingApproval {
						// During approval, only accept y/n.
						if r == 'y' || r == 'Y' {
							d.mu.Unlock()
							select {
							case d.approvalCh <- true:
							default:
							}
						} else if r == 'n' || r == 'N' {
							d.mu.Unlock()
							select {
							case d.approvalCh <- false:
							default:
							}
						} else {
							d.mu.Unlock()
						}
					} else {
						d.inputText = append(d.inputText, r)
						fmt.Fprintf(d.stdout, "\r%s %s\033[K",
							d.style(ansiCyan+ansiBold, "❯"), string(d.inputText))
						d.mu.Unlock()
					}
				}
			} else if b >= 0x20 && b <= 0x7e {
				// Printable ASCII.
				d.mu.Lock()
				if d.pendingApproval {
					r := rune(b)
					if r == 'y' || r == 'Y' {
						d.mu.Unlock()
						select {
						case d.approvalCh <- true:
						default:
						}
					} else if r == 'n' || r == 'N' {
						d.mu.Unlock()
						select {
						case d.approvalCh <- false:
						default:
						}
					} else {
						d.mu.Unlock()
					}
				} else {
					d.inputText = append(d.inputText, rune(b))
					fmt.Fprintf(d.stdout, "\r%s %s\033[K",
						d.style(ansiCyan+ansiBold, "❯"), string(d.inputText))
					d.mu.Unlock()
				}
			}
		}
	}
}

// renderInputBarModeIndicator redraws just the mode indicator line. Caller must hold d.mu.
func (d *Display) renderInputBarModeIndicator() {
	if d.approvalMode == nil {
		return
	}
	label := approvalModeLabels[*d.approvalMode]
	if label == "" {
		label = *d.approvalMode
	}
	// Save cursor, move down 2 to mode indicator, rewrite, restore.
	fmt.Fprintf(d.stdout, "\0337\033[2B\r  %s\033[K\0338",
		d.dim(fmt.Sprintf("⏵⏵ %s (shift+tab to cycle) · esc to interrupt", label)))
}

// --- Rich text diff rendering ---

const (
	diffContextLines   = 2  // unchanged lines before/after each hunk
	diffMaxOutputLines = 30 // truncate diff output beyond this
	diffTruncateAt     = 25 // show this many lines before truncation message
	newFileMaxLines    = 20 // new files beyond this get summarized
	newFileTruncateAt  = 15 // show this many lines for large new files
)

// RenderDiff queues a file change to be rendered after the tool completes.
// This ensures the diff appears after the ⏺ completion marker, not before it.
// Thread-safe; called from the executor goroutine.
func (d *Display) RenderDiff(change FileChange) {
	if change.OldContent == change.NewContent {
		return // no change
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.pendingFileChanges = append(d.pendingFileChanges, change)
}

// FlushDiffs renders any queued file change diffs. Thread-safe.
func (d *Display) FlushDiffs() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.flushPendingDiffs()
}

// flushPendingDiffs renders any queued file change diffs. Caller must hold d.mu.
func (d *Display) flushPendingDiffs() {
	if len(d.pendingFileChanges) == 0 {
		return
	}
	changes := d.pendingFileChanges
	d.pendingFileChanges = nil

	d.writeAboveInputBar(func() {
		for _, change := range changes {
			if d.hasTTY {
				d.renderDiffTTY(change)
			} else {
				d.renderDiffPlain(change)
			}
		}
	})
}

// renderDiffTTY renders a rich diff with box borders, colors, and line numbers.
func (d *Display) renderDiffTTY(change FileChange) {
	w := d.stdout

	// Header line.
	header := change.Path
	if change.IsNew {
		newLines := strings.Count(change.NewContent, "\n")
		if !strings.HasSuffix(change.NewContent, "\n") && change.NewContent != "" {
			newLines++
		}
		header += fmt.Sprintf(" (new file, %d lines)", newLines)
	}
	fmt.Fprintf(w, "  %s %s\r\n", d.dim("╭─"), d.bold(header))

	if change.IsNew {
		d.renderNewFileDiff(w, change.NewContent)
	} else {
		d.renderEditDiff(w, change.OldContent, change.NewContent)
	}

	// Footer line.
	fmt.Fprintf(w, "  %s\r\n", d.dim("╰─"))
}

// renderNewFileDiff renders the content of a newly created file.
func (d *Display) renderNewFileDiff(w io.Writer, content string) {
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	totalLines := len(lines)
	showLines := totalLines
	truncated := false
	if totalLines > newFileMaxLines {
		showLines = newFileTruncateAt
		truncated = true
	}

	lineNoWidth := len(fmt.Sprintf("%d", totalLines))
	if lineNoWidth < 4 {
		lineNoWidth = 4
	}

	for i := 0; i < showLines; i++ {
		lineNo := d.dim(fmt.Sprintf("%*d", lineNoWidth, i+1))
		fmt.Fprintf(w, "  %s %s %s  %s\r\n",
			d.dim("│"), d.green("+"), lineNo, d.green(lines[i]))
	}

	if truncated {
		remaining := totalLines - showLines
		fmt.Fprintf(w, "  %s   %s\r\n",
			d.dim("│"), d.dim(fmt.Sprintf("... (%d more lines)", remaining)))
	}
}

// diffLine represents a single line in the diff output.
type diffLine struct {
	op     diffmatchpatch.Operation // DiffEqual, DiffDelete, DiffInsert
	lineNo int                      // 1-based line number (old for deletes, new for inserts, old for equal)
	text   string
	// For intra-line highlighting (only set on paired delete/insert lines):
	styledText string // pre-rendered with intra-line ANSI codes
}

// renderEditDiff computes and renders a line-level diff with context.
func (d *Display) renderEditDiff(w io.Writer, oldContent, newContent string) {
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 0

	// Compute line-level diff.
	a, b, lineArray := dmp.DiffLinesToChars(oldContent, newContent)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	// Build list of diff lines with line numbers.
	var allLines []diffLine
	oldLineNo := 1
	newLineNo := 1

	for _, diff := range diffs {
		text := diff.Text
		// Split into individual lines, preserving the content.
		lines := strings.Split(text, "\n")
		// The last element after split on trailing \n is empty; remove it.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		for _, line := range lines {
			switch diff.Type {
			case diffmatchpatch.DiffEqual:
				allLines = append(allLines, diffLine{
					op: diffmatchpatch.DiffEqual, lineNo: oldLineNo, text: line,
				})
				oldLineNo++
				newLineNo++
			case diffmatchpatch.DiffDelete:
				allLines = append(allLines, diffLine{
					op: diffmatchpatch.DiffDelete, lineNo: oldLineNo, text: line,
				})
				oldLineNo++
			case diffmatchpatch.DiffInsert:
				allLines = append(allLines, diffLine{
					op: diffmatchpatch.DiffInsert, lineNo: newLineNo, text: line,
				})
				newLineNo++
			}
		}
	}

	// Apply intra-line highlighting to adjacent delete/insert pairs.
	d.applyIntraLineHighlighting(allLines)

	// Compute which lines to show using context windows around changes.
	visible := computeVisibleLines(allLines, diffContextLines)

	// Determine line number width.
	maxLineNo := oldLineNo
	if newLineNo > maxLineNo {
		maxLineNo = newLineNo
	}
	lineNoWidth := len(fmt.Sprintf("%d", maxLineNo))
	if lineNoWidth < 4 {
		lineNoWidth = 4
	}

	// Render visible lines with truncation.
	outputCount := 0
	lastVisibleIdx := -1

	for i, dl := range allLines {
		if !visible[i] {
			continue
		}

		// Insert hunk separator if there's a gap.
		if lastVisibleIdx >= 0 && i > lastVisibleIdx+1 {
			if outputCount >= diffMaxOutputLines {
				break
			}
			fmt.Fprintf(w, "  %s   %s\r\n", d.dim("│"), d.dim("···"))
			outputCount++
		}
		lastVisibleIdx = i

		if outputCount >= diffTruncateAt && outputCount < diffMaxOutputLines {
			// Count remaining visible lines.
			remaining := 0
			for j := i; j < len(allLines); j++ {
				if visible[j] {
					remaining++
				}
			}
			if remaining+outputCount > diffMaxOutputLines {
				fmt.Fprintf(w, "  %s   %s\r\n",
					d.dim("│"), d.dim(fmt.Sprintf("... (%d more lines)", remaining)))
				break
			}
		}

		lineNo := d.dim(fmt.Sprintf("%*d", lineNoWidth, dl.lineNo))
		displayText := dl.text
		if dl.styledText != "" {
			displayText = dl.styledText
		}

		switch dl.op {
		case diffmatchpatch.DiffEqual:
			fmt.Fprintf(w, "  %s   %s  %s\r\n", d.dim("│"), lineNo, displayText)
		case diffmatchpatch.DiffDelete:
			if dl.styledText != "" {
				fmt.Fprintf(w, "  %s %s %s  %s\r\n", d.dim("│"), d.red("-"), lineNo, displayText)
			} else {
				fmt.Fprintf(w, "  %s %s %s  %s\r\n", d.dim("│"), d.red("-"), lineNo, d.red(displayText))
			}
		case diffmatchpatch.DiffInsert:
			if dl.styledText != "" {
				fmt.Fprintf(w, "  %s %s %s  %s\r\n", d.dim("│"), d.green("+"), lineNo, displayText)
			} else {
				fmt.Fprintf(w, "  %s %s %s  %s\r\n", d.dim("│"), d.green("+"), lineNo, d.green(displayText))
			}
		}
		outputCount++
	}
}

// applyIntraLineHighlighting finds adjacent delete/insert pairs and adds
// character-level highlighting.
func (d *Display) applyIntraLineHighlighting(lines []diffLine) {
	for i := 0; i < len(lines)-1; i++ {
		if lines[i].op == diffmatchpatch.DiffDelete && lines[i+1].op == diffmatchpatch.DiffInsert {
			oldStyled, newStyled := d.renderIntraLineDiff(lines[i].text, lines[i+1].text)
			lines[i].styledText = oldStyled
			lines[i+1].styledText = newStyled
			i++ // skip the insert, already processed
		}
	}
}

// renderIntraLineDiff computes character-level diff between two lines and returns
// styled versions: deleted chars bold+red, inserted chars bold+green, equal chars
// in the base color (red for old line, green for new line).
func (d *Display) renderIntraLineDiff(oldLine, newLine string) (styledOld, styledNew string) {
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 0
	charDiffs := dmp.DiffMain(oldLine, newLine, false)
	charDiffs = dmp.DiffCleanupSemantic(charDiffs)

	var oldBuf, newBuf strings.Builder
	for _, cd := range charDiffs {
		switch cd.Type {
		case diffmatchpatch.DiffEqual:
			oldBuf.WriteString(d.style(ansiRed, cd.Text))
			newBuf.WriteString(d.style(ansiGreen, cd.Text))
		case diffmatchpatch.DiffDelete:
			oldBuf.WriteString(d.style(ansiBold+ansiRed, cd.Text))
		case diffmatchpatch.DiffInsert:
			newBuf.WriteString(d.style(ansiBold+ansiGreen, cd.Text))
		}
	}
	return oldBuf.String(), newBuf.String()
}

// computeVisibleLines determines which lines should be shown based on context
// windows around changed lines.
func computeVisibleLines(lines []diffLine, contextSize int) []bool {
	visible := make([]bool, len(lines))

	// Mark all changed lines and their context.
	for i, dl := range lines {
		if dl.op != diffmatchpatch.DiffEqual {
			// Mark this line and surrounding context.
			start := i - contextSize
			if start < 0 {
				start = 0
			}
			end := i + contextSize
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for j := start; j <= end; j++ {
				visible[j] = true
			}
		}
	}
	return visible
}

// renderDiffPlain renders a simple unified-style diff summary for non-TTY.
func (d *Display) renderDiffPlain(change FileChange) {
	w := d.stdout
	if change.IsNew {
		lineCount := strings.Count(change.NewContent, "\n")
		if !strings.HasSuffix(change.NewContent, "\n") && change.NewContent != "" {
			lineCount++
		}
		fmt.Fprintf(w, "+++ %s (new file, %d lines)\n", change.Path, lineCount)
	} else {
		fmt.Fprintf(w, "--- a/%s\n+++ b/%s\n", change.Path, change.Path)
	}
}

// Formatting helpers.

func (d *Display) style(codes, s string) string {
	if d.hasTTY {
		return codes + s + ansiReset
	}
	return s
}

func (d *Display) bold(s string) string    { return d.style(ansiBold, s) }
func (d *Display) dim(s string) string     { return d.style(ansiDim, s) }
func (d *Display) red(s string) string     { return d.style(ansiRed, s) }
func (d *Display) green(s string) string   { return d.style(ansiGreen, s) }
func (d *Display) yellow(s string) string  { return d.style(ansiYellow, s) }
func (d *Display) cyan(s string) string    { return d.style(ansiCyan, s) }
func (d *Display) magenta(s string) string { return d.style(ansiMagenta, s) }

func (d *Display) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// countTerminalLines estimates the number of terminal lines occupied by text with the given indent.
func (d *Display) countTerminalLines(text string, indent int) int {
	lines := 0
	for _, line := range strings.Split(text, "\n") {
		lineLen := len(line) + indent
		if lineLen == 0 {
			lines++
		} else {
			lines += (lineLen + d.termWidth - 1) / d.termWidth
		}
	}
	return lines
}

// clearStreamedOutput clears previously streamed raw text using ANSI escape codes.
func (d *Display) clearStreamedOutput() {
	if d.streamingLines > 0 {
		// Move cursor up to the start of streamed output and clear to end of screen.
		fmt.Fprintf(d.stdout, "\r\033[%dA\033[J", d.streamingLines)
	}
}

// renderAssistantMessage renders a final assistant message with a white ⏺ marker
// on the first non-empty line and 2-space indent on subsequent lines.
func (d *Display) renderAssistantMessage(text string) {
	rendered := d.renderMarkdown(text)
	lines := strings.Split(rendered, "\n")
	markerPrinted := false
	for _, line := range lines {
		stripped := strings.TrimSpace(stripANSI(line))
		if !markerPrinted {
			if stripped == "" {
				continue // Skip leading blank lines from glamour.
			}
			// Trim leading whitespace from glamour output so the marker aligns cleanly.
			trimmed := strings.TrimLeft(line, " ")
			fmt.Fprintf(d.stdout, "\r\n  %s %s\r\n", d.style(ansiWhite, "⏺"), trimmed)
			markerPrinted = true
		} else {
			fmt.Fprintf(d.stdout, "    %s\r\n", line)
		}
	}
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

// toolLabelParts returns the function name and argument for a tool label,
// formatted as FuncName("arg") in the style of Claude Code.
func toolLabelParts(toolName string, args json.RawMessage) (funcName, arg string) {
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
	case "git_status":
		return "Bash", "git status"
	case "git_diff":
		return "Bash", "git diff"
	case "git_log":
		return "Bash", "git log"
	case "git_show":
		return "Bash", "git show"
	case "ask_user":
		if q := extractArg(args, "question"); q != "" {
			if len(q) > 60 {
				q = q[:60] + "..."
			}
			return "AskUser", q
		}
		return "AskUser", ""
	case "TodoWrite":
		// TodoWrite is a server-side tool for managing a task checklist.
		// Try to extract a count of todo items.
		if count := countArrayArg(args, "todos"); count > 0 {
			return "TodoWrite", fmt.Sprintf("%d items", count)
		}
		return "TodoWrite", ""
	default:
		return toolName, ""
	}
}

// toolLabel returns a compact, plain-text label for a tool invocation (non-TTY use).
// Format: FuncName("arg") or just FuncName.
func toolLabel(toolName string, args json.RawMessage) string {
	funcName, arg := toolLabelParts(toolName, args)
	if arg != "" {
		return funcName + "(\"" + arg + "\")"
	}
	return funcName
}

// styledToolLabel returns a TTY-styled label: bold white FuncName + dim ("arg").
func (d *Display) styledToolLabel(toolName string, args json.RawMessage) string {
	funcName, arg := toolLabelParts(toolName, args)
	if arg != "" {
		return d.bold(funcName) + d.dim("(\""+arg+"\")")
	}
	return d.bold(funcName)
}

// toolLabelActive returns the in-progress label using the same function-call format.
func toolLabelActive(toolName string, args json.RawMessage) string {
	return toolLabel(toolName, args)
}

// extractFilePathArg extracts a file path from a JSON args object,
// checking both "file_path" (MCP tools) and "path" (legacy tools).
func extractFilePathArg(args json.RawMessage) string {
	if p := extractArg(args, "file_path"); p != "" {
		return p
	}
	return extractArg(args, "path")
}

// extractArg extracts a string argument from a JSON args object.
// Handles both string values ("command": "pwd") and array values ("command": ["pwd"]).
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
	// Try as string first.
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	// Try as array of strings (e.g. ["bash", "-c", "pwd"]).
	var arr []string
	if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
		return strings.Join(arr, " ")
	}
	return ""
}

// countArrayArg returns the length of an array argument, or 0 if not found/not an array.
func countArrayArg(args json.RawMessage, key string) int {
	if len(args) == 0 {
		return 0
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(v, &arr); err != nil {
		return 0
	}
	return len(arr)
}

// Tool log implementation: completed tools are printed permanently via writeAboveInputBar
// (pushing the input bar down). The in-progress tool animates on the input bar's animation
// line, just like the thinking shimmer.

// startToolSpinner stops any existing spinner and starts animating the in-progress tool
// on the animation line. Caller must hold d.mu.
func (d *Display) startToolSpinner(toolName string, args json.RawMessage) {
	if d.spinning {
		d.stopSpinner()
	}
	d.spinning = true
	d.currentTool = toolName
	d.spinnerLabel = toolLabelActive(toolName, args)

	if d.hasTTY {
		// Render the first frame immediately on the animation line.
		spinner := d.style(ansiCyan, toolSpinnerFrames[0])
		label := d.shimmerText(d.spinnerLabel+" ...", 0)
		if d.inputBarActive {
			fmt.Fprintf(d.stdout, "\0337\033[2A\r  %s %s\033[K\0338", spinner, label)
		} else {
			fmt.Fprintf(d.stdout, "\r  %s %s\033[K", spinner, label)
		}
		d.spinnerDone = make(chan struct{})
		go d.animateToolSpinner()
	} else {
		fmt.Fprintf(d.stdout, "  * %s ...\n", d.spinnerLabel)
	}
}

// completeToolSpinner stops the spinner and prints the completed tool line permanently
// above the input bar. Caller must hold d.mu.
func (d *Display) completeToolSpinner(toolName string, args json.RawMessage, isError bool) {
	// Stop the spinner goroutine (writeAboveInputBar will clear the animation line).
	if d.spinning {
		d.spinning = false
		if d.spinnerDone != nil {
			close(d.spinnerDone)
			d.spinnerDone = nil
		}
	}

	if d.hasTTY {
		d.writeAboveInputBar(func() {
			styled := d.styledToolLabel(toolName, args)
			if isError {
				fmt.Fprintf(d.stdout, "  %s %s\r\n", d.red("⏺"), styled)
			} else {
				fmt.Fprintf(d.stdout, "  %s %s\r\n", d.green("⏺"), styled)
			}
		})
	} else {
		label := toolLabel(toolName, args)
		if isError {
			fmt.Fprintf(d.stdout, "  x %s (error)\n", label)
		} else {
			fmt.Fprintf(d.stdout, "  o %s\n", label)
		}
	}

	// Render any file diffs queued during this tool's execution (after the ⏺ marker).
	d.flushPendingDiffs()

	d.currentTool = ""
	d.spinnerLabel = ""
}

// animateToolSpinner runs in a goroutine, animating the in-progress tool
// on the input bar's animation line (same position as the thinking shimmer).
func (d *Display) animateToolSpinner() {
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
			spinner := d.style(ansiCyan, toolSpinnerFrames[frame%len(toolSpinnerFrames)])
			label := d.shimmerText(d.spinnerLabel+" ...", frame)
			if d.inputBarActive {
				// Write to the animation line above the input bar.
				// Save cursor, move up 2 to animation line, overwrite, restore.
				fmt.Fprintf(d.stdout, "\0337\033[2A\r  %s %s\033[K\0338",
					spinner, label)
			} else {
				fmt.Fprintf(d.stdout, "\r  %s %s\033[K", spinner, label)
			}
			d.mu.Unlock()
			frame++
		}
	}
}

// shimmerText applies a traveling highlight effect across the text.
func (d *Display) shimmerText(text string, frame int) string {
	if !d.hasTTY {
		return text
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	var sb strings.Builder
	// The shimmer is a bright "wave" that moves across the text.
	wavePos := frame % (len(runes) + len(shimmerColors))
	for i, r := range runes {
		dist := wavePos - i
		if dist >= 0 && dist < len(shimmerColors) {
			sb.WriteString(shimmerColors[dist])
			sb.WriteRune(r)
			sb.WriteString(ansiReset)
		} else {
			sb.WriteString(ansiDim)
			sb.WriteRune(r)
			sb.WriteString(ansiReset)
		}
	}
	return sb.String()
}

func (d *Display) updateSpinner(toolName, progress string) {
	if d.spinning && progress != "" {
		label := toolLabelActive(toolName, nil)
		d.spinnerLabel = label + ": " + d.truncate(progress, 60)
	}
}

// stopSpinner stops the in-progress animation and clears the animation line.
// Used when the event loop transitions away from tool calls (e.g. to assistant_message).
// Caller must hold d.mu.
func (d *Display) stopSpinner() {
	if d.spinning {
		d.spinning = false
		if d.spinnerDone != nil {
			close(d.spinnerDone)
			d.spinnerDone = nil
		}
		if d.hasTTY {
			if d.inputBarActive {
				// Clear the animation line above the input bar.
				fmt.Fprintf(d.stdout, "\0337\033[2A\r\033[K\0338")
			} else {
				fmt.Fprintf(d.stdout, "\r\033[K")
			}
		}
		d.currentTool = ""
		d.spinnerLabel = ""
	}
}
