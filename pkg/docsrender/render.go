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

package docsrender

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/renderer"
	"github.com/pgavlin/goldmark/renderer/markdown"
	"github.com/pgavlin/goldmark/util"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"golang.org/x/term"
)

const defaultWidth = 80

// pageMargin is the left margin glamour applies to rendered content.
const pageMargin = "  "

// GetTerminalWidth returns the current terminal width, defaulting to 80.
func GetTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 { //nolint:gosec // fd fits int
		return w
	}
	return defaultWidth
}

// segment represents a piece of the document for rendering.
type segment struct {
	markdown  string
	verbatim  string
	isCode    bool
	noteLabel string
	noteBody  string
}

// RenderMarkdown renders the given markdown body for terminal display
// with note boxes, code block chrome, and glamour formatting.
func RenderMarkdown(title, body string) (string, error) {
	fullMD := body
	if title != "" && !strings.HasPrefix(strings.TrimSpace(fullMD), "# ") {
		fullMD = fmt.Sprintf("# %s\n\n%s", title, fullMD)
	}

	width := GetTerminalWidth()
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

	glamourRenderer, err := glamour.NewTermRenderer(
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
			rendered, renderErr := glamourRenderer.Render(seg.markdown)
			if renderErr != nil {
				buf.WriteString(seg.markdown)
			} else {
				if seg.isCode {
					rendered = trimCodeBlock(rendered)
				}
				buf.WriteString(rendered)
			}
		}
	}

	return buf.String(), nil
}

func renderNoteBox(label, body string, width int) string {
	boxWidth := width - len(pageMargin)
	contentWidth := width - 6

	var buf strings.Builder
	buf.WriteString(pageMargin + boxHeader(label, boxWidth) + "\n")

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

	rendered = strings.TrimSpace(rendered)
	buf.WriteString(pageMargin + "│\n")
	for _, line := range strings.Split(rendered, "\n") {
		buf.WriteString(pageMargin + "│ " + line + "\n")
	}
	buf.WriteString(pageMargin + "│\n")
	buf.WriteString(pageMargin + boxFooter(boxWidth) + "\n")
	return buf.String()
}

func buildSegments(md string, width int) []segment {
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

		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				flushMD()
				lang := strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(lang)
				border := pageMargin + boxHeader(displayLang(lang), width-len(pageMargin)) + "\n"
				segments = append(segments, segment{verbatim: border})
				flushCode = true
				mdBuf.WriteString(line + "\n")
				inCodeBlock = true
			} else {
				mdBuf.WriteString(line + "\n")
				flushMD()
				segments = append(segments, segment{verbatim: "\n" + pageMargin + boxFooter(width-len(pageMargin)) + "\n"})
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

var langDisplayNames = map[string]string{
	"csharp": "C#", "bash": "Bash", "typescript": "TypeScript",
	"javascript": "JavaScript", "python": "Python", "go": "Go",
	"yaml": "YAML", "json": "JSON", "java": "Java",
	"shell": "Shell", "sh": "Shell", "html": "HTML",
	"css": "CSS", "sql": "SQL", "ts": "TypeScript",
	"js": "JavaScript", "py": "Python",
}

func displayLang(lang string) string {
	if name, ok := langDisplayNames[lang]; ok {
		return name
	}
	return lang
}

var (
	ansiRe         = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	internalLinkRe = regexp.MustCompile(`\[([^\]\n]+)\]\((/(docs|registry)/[^\)\n]+)\)`)
)

func isVisuallyBlank(line string) bool {
	return strings.TrimSpace(ansiRe.ReplaceAllString(line, "")) == ""
}

func trimCodeBlock(s string) string {
	lines := strings.Split(s, "\n")

	start := 0
	for start < len(lines) && isVisuallyBlank(lines[start]) {
		start++
	}
	if start > 0 {
		lines = append([]string{""}, lines[start:]...)
	}

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

func termRule() string {
	return strings.Repeat("─", GetTerminalWidth()-len(pageMargin))
}

// PageFooter returns a formatted footer for standalone page views.
func PageFooter(baseURL, path string) string {
	url := WebURL(baseURL, path)
	var buf strings.Builder
	buf.WriteString("\n")
	buf.WriteString(pageMargin + termRule() + "\n")
	buf.WriteString(pageMargin + "🔗 " + url + "\n")
	buf.WriteString(pageMargin + "🧭 " + ANSIBold + "pulumi docs browse" + ANSIReset + "       Browse from here\n")
	buf.WriteString("\n")
	return buf.String()
}

// BrowseFooter returns a compact footer for browse mode showing the web URL.
func BrowseFooter(baseURL, path string) string {
	return fmt.Sprintf("\n%s%s\n%s🔗 %s\n", pageMargin, termRule(), pageMargin, WebURL(baseURL, path))
}

// PrintHeadingWithTable renders a heading through glamour, then prints
// pre-formatted table lines with the glamour margin. This is used when
// table content must bypass glamour to preserve columnar formatting.
func PrintHeadingWithTable(heading, table string) {
	headingMD := "## " + heading
	rendered, err := RenderMarkdown("", headingMD)
	if err == nil {
		fmt.Print(rendered)
	} else {
		fmt.Printf("\n%s## %s\n\n", pageMargin, heading)
	}
	for _, line := range strings.Split(strings.TrimRight(table, "\n"), "\n") {
		fmt.Println(pageMargin + line)
	}
	fmt.Println()
}

// FindSectionBounds returns the start index, the index after the heading,
// and the end index of a section identified by its heading text.
// Returns -1, -1, -1 if the heading is not found.
func FindSectionBounds(body, heading string) (start, afterHeading, end int) {
	idx := strings.Index(body, heading)
	if idx < 0 {
		return -1, -1, -1
	}
	after := idx + len(heading)
	endIdx := len(body)
	if nextH := strings.Index(body[after:], "\n## "); nextH >= 0 {
		endIdx = after + nextH
	}
	return idx, after, endIdx
}

var noteStartRe = regexp.MustCompile(`^([>|])\s*\*{0,2}(Note|Warning|Tip):\*{0,2}\s*(.*)$`)

func parseNoteLine(trimmed string) (noteType, prefix, body string, ok bool) {
	m := noteStartRe.FindStringSubmatch(trimmed)
	if m == nil {
		return "", "", "", false
	}
	return strings.ToLower(m[2]), m[1], m[3], true
}

func isBlockStart(trimmed string) bool {
	return strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, ">") ||
		strings.HasPrefix(trimmed, "---") ||
		strings.HasPrefix(trimmed, "***") ||
		strings.HasPrefix(trimmed, "___") ||
		strings.HasPrefix(trimmed, "```") ||
		strings.HasPrefix(trimmed, "|")
}

func isParagraphContinuation(line string) bool {
	if line == "" {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if isBlockStart(trimmed) ||
		strings.HasPrefix(trimmed, "- ") ||
		strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "+ ") {
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
	if isBlockStart(trimmed) {
		return false
	}
	return !strings.HasSuffix(prev, "  ")
}

// FilterCodeBlocksByLanguage filters fenced code blocks to keep only those
// matching lang (or "sh"). Isolated code blocks not adjacent to other code
// blocks are always preserved.
func FilterCodeBlocksByLanguage(source []byte, tree ast.Node, lang string) []byte {
	if lang == "" {
		return renderTree(source, tree)
	}
	filterCodeBlocks(source, tree, lang)
	return renderTree(source, tree)
}

func filterCodeBlocks(source []byte, node ast.Node, lang string) {
	var c, next ast.Node
	for c = node.FirstChild(); c != nil; c = next {
		filterCodeBlocks(source, c, lang)
		next = c.NextSibling()

		cb, ok := c.(*ast.FencedCodeBlock)
		if !ok {
			continue
		}

		cbLang := string(cb.Language(source))
		if cbLang == "" || cbLang == lang || cbLang == "sh" || cbLang == "bash" || cbLang == "shell" {
			continue
		}

		if !hasAdjacentCodeBlock(c) {
			continue
		}

		node.RemoveChild(node, c)
	}
}

func hasAdjacentCodeBlock(node ast.Node) bool {
	if prev := node.PreviousSibling(); prev != nil {
		if _, ok := prev.(*ast.FencedCodeBlock); ok {
			return true
		}
	}
	if next := node.NextSibling(); next != nil {
		if _, ok := next.(*ast.FencedCodeBlock); ok {
			return true
		}
	}
	return false
}

// ExtractLinks extracts all hyperlinks from the document, deduplicating by URL.
func ExtractLinks(source []byte, tree ast.Node) []Link {
	seen := map[string]bool{}
	var links []Link

	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}

		url := string(link.Destination)
		if seen[url] {
			return ast.WalkSkipChildren, nil
		}
		seen[url] = true

		title := linkPlainText(source, link)
		links = append(links, Link{URL: url, Title: title})
		return ast.WalkSkipChildren, nil
	})

	return links
}

// ExtractInternalLinks extracts internal doc/registry links, deduplicated by URL.
func ExtractInternalLinks(md string) []Link {
	source := []byte(md)
	tree := ParseMarkdown(source)
	all := ExtractLinks(source, tree)

	var internal []Link
	for _, l := range all {
		if strings.HasPrefix(l.URL, "/docs/") || strings.HasPrefix(l.URL, "/registry/") {
			internal = append(internal, l)
		}
	}
	return internal
}

// NumberLinks replaces internal doc/registry links in the markdown with numbered
// references (e.g. "🔗1 [Link text](url)") and returns the annotated markdown along
// with the ordered list of links.
func NumberLinks(md string) (annotated string, links []Link) {
	internal := ExtractInternalLinks(md)
	if len(internal) == 0 {
		return md, nil
	}

	linkNum := make(map[string]int, len(internal))
	for i, l := range internal {
		linkNum[l.URL] = i + 1
	}

	annotated = internalLinkRe.ReplaceAllStringFunc(md, func(match string) string {
		m := internalLinkRe.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		href := m[2]
		num, ok := linkNum[href]
		if !ok {
			return match
		}
		return fmt.Sprintf("🔗%d [%s](%s)", num, m[1], href)
	})
	return annotated, internal
}

func linkPlainText(source []byte, link *ast.Link) string {
	var buf bytes.Buffer
	_ = ast.Walk(link, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := node.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return buf.String()
}

func renderTree(source []byte, tree ast.Node) []byte {
	md := &markdown.Renderer{}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(md, 100)))
	var buf bytes.Buffer
	err := r.Render(&buf, source, tree)
	contract.AssertNoErrorf(err, "rendering markdown tree")
	return bytes.TrimRight(buf.Bytes(), "\n")
}
