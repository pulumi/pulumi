// Copyright 2024, Pulumi Corporation.
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

package docs

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"golang.org/x/term"
)

const defaultWidth = 80

// glamourMargin is the left margin glamour applies to rendered content.
// We match it on box-drawing lines so everything aligns.
const glamourMargin = "  "

func getTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return defaultWidth
}

// segment represents a piece of the document.
type segment struct {
	markdown string // non-empty: render through glamour
	verbatim string // non-empty: emit as-is
	isCode   bool   // if true, trim extra blank lines from glamour output
	// For note boxes: render noteBody through glamour, then wrap in box chrome.
	noteLabel string
	noteBody  string
}

// RenderMarkdown renders the given markdown body for terminal display.
func RenderMarkdown(title, body string) (string, error) {
	fullMD := stripExternalLinks(body)
	if title != "" && !strings.HasPrefix(strings.TrimSpace(fullMD), "# ") {
		fullMD = fmt.Sprintf("# %s\n\n%s", title, fullMD)
	}

	width := getTerminalWidth()
	segments := buildSegments(fullMD, width)

	if !cmdutil.InteractiveTerminal() {
		var buf strings.Builder
		for _, seg := range segments {
			if seg.markdown != "" {
				buf.WriteString(seg.markdown)
			} else {
				buf.WriteString(seg.verbatim)
			}
		}
		return buf.String(), nil
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return fullMD, nil
	}

	var buf strings.Builder
	for _, seg := range segments {
		if seg.verbatim != "" {
			buf.WriteString(seg.verbatim)
		} else if seg.noteLabel != "" {
			buf.WriteString(renderNoteBox(seg.noteLabel, seg.noteBody, width))
		} else if seg.markdown != "" {
			rendered, renderErr := renderer.Render(seg.markdown)
			if renderErr != nil {
				buf.WriteString(seg.markdown)
			} else {
				if seg.isCode {
					// Trim glamour's extra blank lines around code, keep just one
					rendered = trimCodeBlock(rendered)
				}
				buf.WriteString(rendered)
			}
		}
	}

	return buf.String(), nil
}

// renderNoteBox renders a note's body through glamour, then wraps the
// output in box-drawing chrome.
func renderNoteBox(label, body string, width int) string {
	// Box chrome eats: glamourMargin (2) + "│ " (2) + glamour's own margin (2) = 6
	boxWidth := width - len(glamourMargin)
	contentWidth := width - 6

	var buf strings.Builder
	buf.WriteString(glamourMargin + boxHeader(label, boxWidth) + "\n")

	// Render the body with a narrower glamour renderer so it fits inside the box
	rendered := body
	noteRenderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(contentWidth),
	)
	if err == nil {
		if r, err := noteRenderer.Render(body); err == nil {
			rendered = r
		}
	}

	// Trim glamour's extra whitespace, then add controlled padding
	rendered = strings.TrimSpace(rendered)
	buf.WriteString(glamourMargin + "│\n")
	for _, line := range strings.Split(rendered, "\n") {
		buf.WriteString(glamourMargin + "│ " + line + "\n")
	}
	buf.WriteString(glamourMargin + "│\n")

	buf.WriteString(glamourMargin + boxFooter(boxWidth) + "\n")
	return buf.String()
}

// buildSegments splits the markdown into alternating segments of
// regular markdown (for glamour) and verbatim blocks (note boxes,
// code block borders).
func buildSegments(md string, width int) []segment {
	// First pass: extract notes into a structured form
	md = joinSoftWraps(md)
	lines := strings.Split(md, "\n")
	var segments []segment
	var mdBuf strings.Builder
	inCodeBlock := false

	flushCode := false
	flushMD := func() {
		if mdBuf.Len() > 0 {
			segments = append(segments, segment{markdown: mdBuf.String(), isCode: flushCode})
			mdBuf.Reset()
			flushCode = false
		}
	}

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check for note start
		if !inCodeBlock {
			if noteType, prefix, body, ok := parseNoteLine(trimmed); ok {
				flushMD()
				label := noteLabels[noteType]

				var parts []string
				if body != "" {
					parts = append(parts, body)
				}
				i++
				for i < len(lines) {
					next := strings.TrimSpace(lines[i])
					if strings.HasPrefix(next, prefix+" ") || next == prefix {
						content := strings.TrimPrefix(next, prefix+" ")
						content = strings.TrimPrefix(content, prefix)
						content = strings.TrimSpace(content)
						if content != "" {
							parts = append(parts, content)
						}
						i++
					} else {
						break
					}
				}

				fullText := strings.Join(parts, " ")
				segments = append(segments, segment{noteLabel: label, noteBody: fullText})
				continue
			}
		}

		// Track code blocks
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				flushMD()
				lang := strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(lang)
				// Emit top border as verbatim
				segments = append(segments, segment{verbatim: glamourMargin + boxHeader(displayLang(lang), width-len(glamourMargin)) + "\n"})
				// The fence itself and content go through glamour
				flushCode = true
				mdBuf.WriteString(line + "\n")
				inCodeBlock = true
			} else {
				// Closing fence goes through glamour
				mdBuf.WriteString(line + "\n")
				flushMD()
				// Emit bottom border
				segments = append(segments, segment{verbatim: "\n" + glamourMargin + boxFooter(width-len(glamourMargin)) + "\n"})
				inCodeBlock = false
			}
			i++
			continue
		}

		mdBuf.WriteString(line + "\n")
		i++
	}

	flushMD()
	return segments
}

// joinSoftWraps joins soft-wrapped lines within paragraphs.
func joinSoftWraps(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			out = append(out, line)
			continue
		}
		if inCodeBlock {
			out = append(out, line)
			continue
		}
		if isParagraphContinuation(line) && len(out) > 0 && isMergeable(out[len(out)-1]) {
			out[len(out)-1] = out[len(out)-1] + " " + line
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

var noteLabels = map[string]string{
	"note":    "ℹ️  Note",
	"warning": "⚠️  Warning",
	"tip":     "💡 Tip",
}

// langDisplayNames maps markdown fence language identifiers to human-readable names.
var langDisplayNames = map[string]string{
	"csharp":     "C#",
	"bash":       "Bash",
	"typescript": "TypeScript",
	"javascript": "JavaScript",
	"python":     "Python",
	"go":         "Go",
	"yaml":       "YAML",
	"json":       "JSON",
	"java":       "Java",
	"shell":      "Shell",
	"sh":         "Shell",
	"html":       "HTML",
	"css":        "CSS",
	"sql":        "SQL",
	"ts":         "TypeScript",
	"js":         "JavaScript",
	"py":         "Python",
}

func displayLang(lang string) string {
	if name, ok := langDisplayNames[lang]; ok {
		return name
	}
	return lang
}

// isVisuallyBlank returns true if a line has no visible characters
// (accounting for ANSI escape sequences that glamour may inject).
func isVisuallyBlank(line string) bool {
	// Strip ANSI escape sequences
	stripped := stripANSI(line)
	return strings.TrimSpace(stripped) == ""
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// trimCodeBlock trims excessive blank lines from glamour's code block output,
// keeping at most one blank line at the start and end.
func trimCodeBlock(s string) string {
	lines := strings.Split(s, "\n")

	// Trim leading visually blank lines to exactly one
	start := 0
	for start < len(lines) && isVisuallyBlank(lines[start]) {
		start++
	}
	if start > 0 {
		lines = append([]string{""}, lines[start:]...)
	}

	// Trim trailing visually blank lines, then add exactly one
	end := len(lines) - 1
	for end >= 0 && isVisuallyBlank(lines[end]) {
		end--
	}
	lines = append(lines[:end+1], "")

	return strings.Join(lines, "\n")
}

func boxHeader(label string, width int) string {
	if label == "" {
		return "┌" + strings.Repeat("─", width-1)
	}
	inner := "─ " + label + " "
	padding := width - 1 - len([]rune(inner))
	if padding < 1 {
		padding = 1
	}
	return "┌" + inner + strings.Repeat("─", padding)
}

func boxFooter(width int) string {
	return "└" + strings.Repeat("─", width-1)
}

// webURL builds the full web URL for a content path, using the correct prefix.
func webURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	prefix, trimmed := contentPrefix(path)
	return fmt.Sprintf("%s%s%s/", base, prefix, trimmed)
}

// pageFooter returns a formatted footer for standalone page views with
// the web URL and navigation hints.
func pageFooter(baseURL, path string) string {
	width := getTerminalWidth()
	rule := strings.Repeat("─", width-len(glamourMargin))
	url := webURL(baseURL, path)
	bold := "\033[1m"
	reset := "\033[0m"

	var buf strings.Builder
	buf.WriteString("\n")
	buf.WriteString(glamourMargin + rule + "\n")
	buf.WriteString(glamourMargin + "🔗 " + url + "\n")
	buf.WriteString(glamourMargin + "🧭 " + bold + "pulumi docs browse" + reset + "       Browse from here\n")
	buf.WriteString("\n")
	return buf.String()
}

// browseFooter returns a compact footer for browse mode showing the web URL.
func browseFooter(baseURL, path string) string {
	width := getTerminalWidth()
	rule := strings.Repeat("─", width-len(glamourMargin))
	return fmt.Sprintf("\n%s%s\n%s🔗 %s\n", glamourMargin, rule, glamourMargin, webURL(baseURL, path))
}

var noteStartRe = regexp.MustCompile(`^([>|])\s*\*{0,2}(Note|Warning|Tip):\*{0,2}\s*(.*)$`)

func parseNoteLine(trimmed string) (noteType, prefix, body string, ok bool) {
	m := noteStartRe.FindStringSubmatch(trimmed)
	if m == nil {
		return "", "", "", false
	}
	return strings.ToLower(m[2]), m[1], m[3], true
}

func isParagraphContinuation(line string) bool {
	if line == "" {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, ">") ||
		strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "+ ") ||
		strings.HasPrefix(trimmed, "---") ||
		strings.HasPrefix(trimmed, "***") ||
		strings.HasPrefix(trimmed, "___") ||
		strings.HasPrefix(trimmed, "|") ||
		strings.HasPrefix(trimmed, "```") {
		return false
	}
	for j, ch := range trimmed {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && j > 0 && j < len(trimmed)-1 && trimmed[j+1] == ' ' {
			return false
		}
		break
	}
	return true
}

func isMergeable(prev string) bool {
	trimmed := strings.TrimSpace(prev)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, ">") ||
		strings.HasPrefix(trimmed, "---") ||
		strings.HasPrefix(trimmed, "***") ||
		strings.HasPrefix(trimmed, "___") ||
		strings.HasPrefix(trimmed, "```") ||
		strings.HasPrefix(trimmed, "|") {
		return false
	}
	if strings.HasSuffix(prev, "  ") {
		return false
	}
	return true
}
