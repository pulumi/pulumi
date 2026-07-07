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
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/neo/acp"
)

// toolTracker assigns synthetic tool-call ids and carries the "current" id used
// to correlate a UIToolStarted with the progress/completed events that follow.
// This relies on the Session dispatching tool calls serially (the same
// assumption the TUI's pulumi blocks make). cwd is the session working
// directory, used to render file paths in tool titles relative to it.
type toolTracker struct {
	seq     int
	current string
	cwd     string
}

// translate maps a streaming UIEvent to the ACP session/update payload to send,
// reporting false for events that produce no notification. Turn-boundary events
// (idle/cancelled/error) and approvals are handled by the pump, not here. The
// payload types inject their own "sessionUpdate" discriminator on marshal.
func (t *toolTracker) translate(evt UIEvent) (acp.SessionUpdate, bool) {
	switch e := evt.(type) {
	case UIAssistantMessage:
		if e.Content == "" {
			return nil, false
		}
		// Each backend assistant_message is a complete message, not a token
		// delta, and the editor concatenates the chunks it receives. Append a
		// newline so successive messages in a turn don't run together (a trailing
		// newline on the final message is invisible once rendered).
		return acp.AgentMessageChunk{Content: acp.ContentBlock{Type: "text", Text: e.Content + "\n"}}, true
	case UIToolStarted:
		t.seq++
		t.current = fmt.Sprintf("tc_%d", t.seq)
		args := parseToolArgs(e.Args)
		return acp.ToolCallStart{
			ToolCallID: t.current,
			Title:      toolTitle(e.Name, args, t.cwd),
			Kind:       toolKind(e.Name),
			Status:     acp.ToolStatusInProgress,
			Locations:  toolLocations(args, t.cwd),
			RawInput:   e.Args,
		}, true
	case UIToolProgress:
		if t.current == "" {
			return nil, false
		}
		return acp.ToolCallProgress{
			ToolCallID: t.current,
			Status:     acp.ToolStatusInProgress,
			Content:    textContent(e.Message),
		}, true
	case UIToolCompleted:
		// A completion with no tracked start (e.g. events replayed across a
		// reconnect) has no call id to attach to, and ACP requires one.
		if t.current == "" {
			return nil, false
		}
		status := acp.ToolStatusCompleted
		if e.IsError {
			status = acp.ToolStatusFailed
		}
		update := acp.ToolCallProgress{
			ToolCallID: t.current,
			Status:     status,
			Content:    textContent(string(e.Result)),
			RawOutput:  e.Result,
		}
		// The call is finished; clear current so a stray progress event after
		// completion doesn't attach to it (tool calls dispatch serially).
		t.current = ""
		return update, true
	case UITodoList:
		return acp.PlanUpdate{Entries: planEntries(e.Items)}, true
	}
	return nil, false
}

// promptText renders a prompt's content blocks into the plain text forwarded to
// Neo. Text blocks pass through verbatim. Resource links — which a client may
// send regardless of prompt capabilities, typically for an @-mentioned file —
// are materialized inline as their URI so the reference reaches the model rather
// than being silently dropped. Capability-gated blocks (image/audio/embedded
// resource) are ignored, since we don't advertise those capabilities.
func promptText(blocks []acp.ContentBlock) string {
	var b strings.Builder
	for _, blk := range blocks {
		switch blk.Type {
		case "text":
			b.WriteString(blk.Text)
		case "resource_link":
			b.WriteString(resourceLinkText(blk))
		}
	}
	return b.String()
}

// resourceLinkText renders a resource_link block as text. The URI is what lets
// the model (and its tools) locate the resource, so it always appears. The
// optional Title — a human-readable description distinct from the file path — is
// appended for context when set; Name is intentionally not appended, as it's
// typically just the basename already present in the URI.
func resourceLinkText(blk acp.ContentBlock) string {
	if blk.Title == "" || blk.Title == blk.URI {
		return "@" + blk.URI
	}
	return fmt.Sprintf("@%s (%s)", blk.URI, blk.Title)
}

// textContent wraps a non-empty string as a single ACP tool-call content block.
func textContent(s string) []acp.ToolCallContent {
	if s == "" {
		return nil
	}
	return []acp.ToolCallContent{{Type: "content", Content: acp.ContentBlock{Type: "text", Text: s}}}
}

// planEntries maps Neo todo items to ACP plan entries. Neo's priority and status
// vocabularies match ACP today, but ACP requires a valid value from its enums on
// every entry, so we clamp on egress: a backend that drifts (an empty priority, a
// new status) can't make us emit a spec-invalid plan that the editor may reject.
func planEntries(items []UITodoItem) []acp.PlanEntry {
	out := make([]acp.PlanEntry, 0, len(items))
	for _, it := range items {
		out = append(out, acp.PlanEntry{
			Content:  it.Content,
			Priority: clampPlanPriority(it.Priority),
			Status:   clampPlanStatus(it.Status),
		})
	}
	return out
}

// clampPlanPriority maps a Neo todo priority to an ACP-allowed priority,
// defaulting unknown or empty values to "medium" (the neutral middle).
func clampPlanPriority(priority string) string {
	switch priority {
	case acp.PlanPriorityHigh, acp.PlanPriorityMedium, acp.PlanPriorityLow:
		return priority
	default:
		return acp.PlanPriorityMedium
	}
}

// clampPlanStatus maps a Neo todo status to an ACP-allowed status, defaulting
// unknown or empty values to "pending" so an unrecognized status reads as
// not-yet-done rather than silently completed.
func clampPlanStatus(status string) string {
	switch status {
	case acp.PlanStatusPending, acp.PlanStatusInProgress, acp.PlanStatusCompleted:
		return status
	default:
		return acp.PlanStatusPending
	}
}

// toolArgs is the subset of a tool call's JSON arguments used to enrich its ACP
// presentation. The three fields cover every shape the title/location helpers
// care about: filesystem calls carry file_path (read/write/edit) or path
// (grep/content_replace/directory_tree); shell calls carry command. A tool call
// is decoded once (see parseToolArgs) and the decoded value flows to both the
// title and location helpers, rather than each re-parsing the raw arguments.
type toolArgs struct {
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
	Command  string `json:"command"`
}

// parseToolArgs decodes a tool call's raw arguments into the display subset.
// Decoding is best-effort: the result is presentation-only, so malformed or
// absent arguments yield the zero value rather than an error.
func parseToolArgs(raw json.RawMessage) toolArgs {
	var a toolArgs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &a)
	}
	return a
}

// file returns the file a filesystem tool call touches, preferring file_path
// over path; tools without a file target (shell, pulumi) yield "".
func (a toolArgs) file() string {
	if a.FilePath != "" {
		return a.FilePath
	}
	return a.Path
}

// command returns a shell call's command flattened to a single line so it reads
// cleanly as a tool-call title; non-shell calls yield "".
func (a toolArgs) command() string {
	return strings.Join(strings.Fields(a.Command), " ")
}

// toolTitle turns a Neo tool name ("<server>__<method>") into a human-readable
// tool-call title for the editor, enriched from the call's arguments so the
// editor shows what the call operates on rather than a bare verb:
//
//   - filesystem calls append the file they touch, e.g.
//     "filesystem__read" with {"file_path":"/work/pyproject.toml"} ->
//     "Read ./pyproject.toml" (relative to cwd when possible).
//   - shell calls render the command itself, e.g.
//     "shell__shell_execute" with {"command":"git status"} -> "git status",
//     so the editor shows the command instead of just "Shell execute".
//
// Falls back to the humanized method name (and then the raw name) when the
// arguments carry nothing useful.
func toolTitle(name string, args toolArgs, cwd string) string {
	server, method, ok := strings.Cut(name, "__")
	if !ok || method == "" {
		return name
	}
	base := strings.ReplaceAll(method, "_", " ")
	base = strings.ToUpper(base[:1]) + base[1:]

	switch server {
	case "shell":
		// The command is the useful thing to show; the execute icon already
		// conveys that it's a shell call.
		if cmd := args.command(); cmd != "" {
			return cmd
		}
	case "filesystem":
		if path := args.file(); path != "" {
			return base + " " + displayPath(path, cwd)
		}
	}
	return base
}

// absToolPath renders a tool-call path as absolute: relative arguments are joined
// against cwd (the session working directory), matching how the filesystem tool
// resolves them before it reads or writes. Absolute arguments, or any path when cwd
// is unknown, are returned unchanged. Both the title and the location derive from
// this single normalized value so they can't disagree about the same file.
func absToolPath(path, cwd string) string {
	if cwd == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}

// displayPath renders a tool-call path for a title: relative to cwd with a
// leading "./" (and forward slashes) when it sits inside the working directory,
// otherwise the path as given. Paths outside cwd, or any path when cwd is unknown,
// are returned as the absolute path.
func displayPath(path, cwd string) string {
	path = absToolPath(path, cwd)
	if cwd == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return "./" + filepath.ToSlash(rel)
}

// toolLocations reports the file a tool call touches, so the editor can render
// the call against that file natively — a clickable target or follow-along
// highlighting. ACP locations must be absolute, so the path is normalized through
// absToolPath; the editor can only resolve a path that points at a real on-disk
// location. Tools without a file target (shell, pulumi) yield none.
func toolLocations(args toolArgs, cwd string) []acp.ToolCallLocation {
	path := args.file()
	if path == "" {
		return nil
	}
	return []acp.ToolCallLocation{{Path: absToolPath(path, cwd)}}
}

// toolKind maps a Neo tool name ("<server>__<method>") to an ACP ToolKind for
// display. The method names mirror tools.Filesystem.Invoke; they're display-only
// and any unrecognized name falls back to "other", so drift is harmless.
func toolKind(name string) string {
	server, method, _ := strings.Cut(name, "__")
	switch server {
	case "shell", "pulumi":
		return acp.ToolKindExecute
	case "filesystem":
		switch method {
		case "read", "directory_tree":
			return acp.ToolKindRead
		case "write", "edit", "content_replace":
			return acp.ToolKindEdit
		case "grep":
			return acp.ToolKindSearch
		}
	}
	return acp.ToolKindOther
}
