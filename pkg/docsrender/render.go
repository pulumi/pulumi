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

// renderBlock represents a classified AST node group for rendering.
type renderBlock struct {
	kind     blockKind
	nodes    []ast.Node // AST nodes to render
	lang     string     // language for code blocks
	noteType string     // "note", "warning", "tip" for note blocks
}

type blockKind int

const (
	blockMarkdown blockKind = iota
	blockCode
	blockNote
)

var noteLabels = map[string]string{
	"note":    "ℹ️  Note",
	"warning": "⚠️  Warning",
	"tip":     "💡 Tip",
}

var noteKeywords = map[string]bool{"note": true, "warning": true, "tip": true}

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

// RenderMarkdown renders the given markdown body for terminal display
// using goldmark AST for structural analysis.
func RenderMarkdown(title, body string) (string, error) {
	fullMD := body
	if title != "" && !strings.HasPrefix(strings.TrimSpace(fullMD), "# ") {
		fullMD = fmt.Sprintf("# %s\n\n%s", title, fullMD)
	}

	source := []byte(fullMD)
	tree := ParseMarkdown(source)
	width := GetTerminalWidth()
	blocks := classifyBlocks(source, tree)

	if !cmdutil.InteractiveTerminal() {
		var buf strings.Builder
		for _, b := range blocks {
			buf.Write(renderNodesToMarkdown(source, b.nodes))
			buf.WriteByte('\n')
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
	for _, b := range blocks {
		switch b.kind {
		case blockCode:
			buf.WriteString(pageMargin + boxHeader(displayLang(b.lang), width-len(pageMargin)) + "\n")
			codeMD := string(renderNodesToMarkdown(source, b.nodes))
			rendered, renderErr := glamourRenderer.Render(codeMD)
			if renderErr != nil {
				buf.WriteString(codeMD)
			} else {
				buf.WriteString(trimCodeBlock(rendered))
			}
			buf.WriteString("\n" + pageMargin + boxFooter(width-len(pageMargin)) + "\n")

		case blockNote:
			label := noteLabels[b.noteType]
			noteBody := extractNoteBody(source, b.nodes[0])
			buf.WriteString(renderNoteBox(label, noteBody, width))

		case blockMarkdown:
			md := string(renderNodesToMarkdown(source, b.nodes))
			rendered, renderErr := glamourRenderer.Render(md)
			if renderErr != nil {
				buf.WriteString(md)
			} else {
				buf.WriteString(rendered)
			}
		}
	}

	return buf.String(), nil
}

// classifyBlocks walks top-level AST nodes and groups them into render blocks.
func classifyBlocks(source []byte, tree ast.Node) []renderBlock {
	var blocks []renderBlock
	var mdNodes []ast.Node

	flushMD := func() {
		if len(mdNodes) > 0 {
			blocks = append(blocks, renderBlock{kind: blockMarkdown, nodes: mdNodes})
			mdNodes = nil
		}
	}

	for c := tree.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.FencedCodeBlock:
			flushMD()
			lang := string(n.Language(source))
			blocks = append(blocks, renderBlock{kind: blockCode, nodes: []ast.Node{c}, lang: lang})

		case *ast.Blockquote:
			if noteType := detectNoteType(source, n); noteType != "" {
				flushMD()
				blocks = append(blocks, renderBlock{kind: blockNote, nodes: []ast.Node{c}, noteType: noteType})
			} else {
				mdNodes = append(mdNodes, c)
			}

		default:
			mdNodes = append(mdNodes, c)
		}
	}

	flushMD()
	return blocks
}

// detectNoteType checks if a blockquote is a note/warning/tip box by walking
// the AST. Returns the note type ("note", "warning", "tip") or "" if not a note.
func detectNoteType(source []byte, bq *ast.Blockquote) string {
	first := bq.FirstChild()
	if first == nil {
		return ""
	}

	// The first child should be a paragraph. Look at its inline children.
	child := first.FirstChild()
	if child == nil {
		return ""
	}

	// The note keyword may be wrapped in emphasis (**Note:** or *Note:*) or plain (Note:).
	var keyword string
	switch n := child.(type) {
	case *ast.Emphasis:
		// Check text inside emphasis node for "Note:", "Warning:", "Tip:"
		keyword = extractNoteKeyword(source, n)
	case *ast.Text:
		// Check plain text for "Note:", "Warning:", "Tip:"
		keyword = extractNoteKeywordFromText(string(n.Segment.Value(source)))
	}

	return keyword
}

// extractNoteKeyword checks if an emphasis node contains a note keyword like "Note:".
func extractNoteKeyword(source []byte, em *ast.Emphasis) string {
	text := plainText(source, em)
	return extractNoteKeywordFromText(text)
}

// extractNoteKeywordFromText checks if text starts with "Note:", "Warning:", or "Tip:".
func extractNoteKeywordFromText(text string) string {
	text = strings.TrimSpace(text)
	// Look for "Keyword:" at the start
	idx := strings.Index(text, ":")
	if idx < 0 {
		return ""
	}
	word := strings.ToLower(text[:idx])
	if noteKeywords[word] {
		return word
	}
	return ""
}

// plainText extracts visible text from an AST node, stripping formatting.
func plainText(source []byte, node ast.Node) string {
	var buf bytes.Buffer
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := n.(*ast.Text); ok {
			buf.Write(t.Segment.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return buf.String()
}

// extractNoteBody extracts the body content from a blockquote note by walking
// the AST and removing the leading note label (e.g. "**Note:** ") from the
// first paragraph, then rendering the remaining content as markdown.
func extractNoteBody(source []byte, bqNode ast.Node) string {
	bq, ok := bqNode.(*ast.Blockquote)
	if !ok {
		return string(renderNodesToMarkdown(source, []ast.Node{bqNode}))
	}

	// Extract the blockquote's children into a new document (without the > wrapper).
	doc := ast.NewDocument()
	var children []ast.Node
	for c := bq.FirstChild(); c != nil; c = c.NextSibling() {
		children = append(children, c)
	}
	for _, c := range children {
		bq.RemoveChild(bq, c)
		doc.AppendChild(doc, c)
	}

	// Remove the note label from the first paragraph's inline children.
	first := doc.FirstChild()
	if first != nil {
		removeNoteLabel(source, first)
	}

	md := &markdown.Renderer{}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(md, 100)))
	var buf bytes.Buffer
	err := r.Render(&buf, source, doc)
	contract.AssertNoErrorf(err, "rendering note body")
	return strings.TrimSpace(buf.String())
}

// removeNoteLabel removes the leading note label nodes (e.g. "**Note:** " or
// "Note: ") from a paragraph node's inline children.
func removeNoteLabel(source []byte, para ast.Node) {
	child := para.FirstChild()
	if child == nil {
		return
	}

	switch n := child.(type) {
	case *ast.Emphasis:
		// **Note:** or *Note:* — remove the emphasis node and any following space text.
		text := plainText(source, n)
		if extractNoteKeywordFromText(text) != "" {
			para.RemoveChild(para, n)
			trimLeadingSpace(source, para)
		}
	case *ast.Text:
		// "Note: some text" — trim the "Note: " prefix from the text segment.
		seg := n.Segment
		text := string(seg.Value(source))
		kw := extractNoteKeywordFromText(text)
		if kw != "" {
			// Skip past "Keyword: " (keyword + colon + optional space)
			prefix := text[:len(kw)+1] // "Note:"
			rest := strings.TrimPrefix(text[len(prefix):], " ")
			if rest != "" {
				// Adjust the segment to skip the prefix.
				advance := len(text) - len(rest)
				n.Segment = n.Segment.WithStart(n.Segment.Start + advance)
			} else {
				para.RemoveChild(para, n)
				trimLeadingSpace(source, para)
			}
		}
	}
}

// trimLeadingSpace removes leading whitespace from the first remaining text child.
func trimLeadingSpace(source []byte, para ast.Node) {
	next := para.FirstChild()
	if next == nil {
		return
	}
	if t, ok := next.(*ast.Text); ok {
		text := string(t.Segment.Value(source))
		trimmed := strings.TrimLeft(text, " ")
		if trimmed == "" {
			para.RemoveChild(para, next)
		} else {
			advance := len(text) - len(trimmed)
			t.Segment = t.Segment.WithStart(t.Segment.Start + advance)
		}
	}
}

// renderNodesToMarkdown renders a slice of AST nodes back to markdown text.
func renderNodesToMarkdown(source []byte, nodes []ast.Node) []byte {
	doc := ast.NewDocument()
	for _, n := range nodes {
		if n.Parent() != nil {
			n.Parent().RemoveChild(n.Parent(), n)
		}
		doc.AppendChild(doc, n)
	}

	md := &markdown.Renderer{}
	r := renderer.NewRenderer(renderer.WithNodeRenderers(util.Prioritized(md, 100)))
	var buf bytes.Buffer
	err := r.Render(&buf, source, doc)
	contract.AssertNoErrorf(err, "rendering nodes to markdown")
	return bytes.TrimRight(buf.Bytes(), "\n")
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

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func isVisuallyBlank(line string) bool {
	return strings.TrimSpace(ansiRegex.ReplaceAllString(line, "")) == ""
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

// FormatHeadingWithTable renders a heading through glamour and appends
// pre-formatted table lines with the page margin.
func FormatHeadingWithTable(heading, table string) string {
	var buf strings.Builder
	headingMD := "## " + heading
	rendered, err := RenderMarkdown("", headingMD)
	if err == nil {
		buf.WriteString(rendered)
	} else {
		fmt.Fprintf(&buf, "\n%s## %s\n\n", pageMargin, heading)
	}
	for _, line := range strings.Split(strings.TrimRight(table, "\n"), "\n") {
		buf.WriteString(pageMargin + line + "\n")
	}
	buf.WriteString("\n")
	return buf.String()
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
// with the ordered list of links. Uses goldmark AST to find and modify links.
func NumberLinks(md string) (annotated string, links []Link) {
	source := []byte(md)
	tree := ParseMarkdown(source)

	// First pass: collect internal links and assign numbers.
	seen := map[string]bool{}
	linkNum := map[string]int{}
	n := 0

	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		url := string(link.Destination)
		if !strings.HasPrefix(url, "/docs/") && !strings.HasPrefix(url, "/registry/") {
			return ast.WalkSkipChildren, nil
		}
		if !seen[url] {
			seen[url] = true
			n++
			linkNum[url] = n
			title := linkPlainText(source, link)
			links = append(links, Link{URL: url, Title: title})
		}
		return ast.WalkSkipChildren, nil
	})

	if len(links) == 0 {
		return md, nil
	}

	// Second pass: prepend 🔗N to each internal link's text.
	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		num, ok := linkNum[string(link.Destination)]
		if !ok {
			return ast.WalkSkipChildren, nil
		}
		// Prepend a text node with the numbered reference before existing children.
		prefix := ast.NewString([]byte(fmt.Sprintf("🔗%d ", num)))
		prefix.SetRaw(true)
		first := link.FirstChild()
		if first != nil {
			link.InsertBefore(link, first, prefix)
		} else {
			link.AppendChild(link, prefix)
		}
		return ast.WalkSkipChildren, nil
	})

	return string(renderTree(source, tree)), links
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
