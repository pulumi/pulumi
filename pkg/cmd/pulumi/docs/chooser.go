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
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

var (
	chooserOpenRe  = regexp.MustCompile(`<!--\s*chooser:\s*(\w+)\s*-->`)
	optionOpenRe   = regexp.MustCompile(`<!--\s*option:\s*(\w+)\s*-->`)
	optionCloseRe  = regexp.MustCompile(`<!--\s*/option\s*-->`)
	chooserCloseRe = regexp.MustCompile(`<!--\s*/chooser\s*-->`)

	// chooserTagRe matches leftover chooser/option comment tags for cleanup.
	// Only targets chooser-specific tags, not arbitrary HTML comments.
	chooserTagRe = regexp.MustCompile(`^\s*<!--\s*/?(chooser|option)[\s:]*\w*\s*-->\s*$`)
)

// ContentBlock represents either plain text or a chooser in the document.
type ContentBlock interface {
	isContentBlock()
}

// PlainText is a block of text with no chooser.
type PlainText struct {
	Text string
}

func (PlainText) isContentBlock() {}

// Chooser represents a set of selectable options (language, OS, cloud).
type Chooser struct {
	Type    string
	Options []Option
	// Prefix is text before an inline chooser on the same line.
	Prefix string
	// Suffix is text after an inline chooser on the same line.
	Suffix string
}

func (Chooser) isContentBlock() {}

// Option is a single choice within a chooser.
type Option struct {
	Value   string
	Content string
}

// ParseChoosers parses the markdown content into a sequence of PlainText and Chooser blocks.
// It operates on the full string using position-based matching to handle multi-line content
// and inline choosers correctly.
func ParseChoosers(markdown string) []ContentBlock {
	var blocks []ContentBlock
	pos := 0

	for pos < len(markdown) {
		// Find the next chooser open tag
		loc := chooserOpenRe.FindStringIndex(markdown[pos:])
		if loc == nil {
			// No more choosers — rest is plain text
			blocks = append(blocks, PlainText{Text: markdown[pos:]})
			break
		}

		// Everything before this chooser is plain text
		absStart := pos + loc[0]
		absEnd := pos + loc[1]
		if absStart > pos {
			blocks = append(blocks, PlainText{Text: markdown[pos:absStart]})
		}

		m := chooserOpenRe.FindStringSubmatch(markdown[absStart:absEnd])
		chooserType := m[1]

		// Find the matching <!-- /chooser --> from absEnd onward
		closeLoc := chooserCloseRe.FindStringIndex(markdown[absEnd:])
		if closeLoc == nil {
			// Unmatched chooser open — treat the tag as plain text
			blocks = append(blocks, PlainText{Text: markdown[absStart:absEnd]})
			pos = absEnd
			continue
		}

		chooserBody := markdown[absEnd : absEnd+closeLoc[0]]
		chooserEnd := absEnd + closeLoc[1]

		// Check for inline chooser: no newlines in the body
		prefix := ""
		suffix := ""
		isInline := !strings.Contains(chooserBody, "\n")

		if isInline {
			// For inline, capture surrounding text on the same line
			// Find start of line for prefix
			lineStart := strings.LastIndex(markdown[pos:absStart], "\n")
			if lineStart >= 0 {
				prefix = markdown[pos+lineStart+1 : absStart]
				// Re-adjust: replace the plain text block to exclude the prefix
				if len(blocks) > 0 {
					if pt, ok := blocks[len(blocks)-1].(PlainText); ok {
						pt.Text = strings.TrimSuffix(pt.Text, prefix)
						blocks[len(blocks)-1] = pt
					}
				}
			}
			// Find end of line for suffix
			lineEnd := strings.Index(markdown[chooserEnd:], "\n")
			if lineEnd >= 0 {
				suffix = markdown[chooserEnd : chooserEnd+lineEnd]
				chooserEnd = chooserEnd + lineEnd
			} else {
				suffix = markdown[chooserEnd:]
				chooserEnd = len(markdown)
			}
		}

		// Parse options within the chooser body
		chooser := Chooser{Type: chooserType, Prefix: prefix, Suffix: suffix}
		parseOptions(chooserBody, &chooser)

		blocks = append(blocks, chooser)
		pos = chooserEnd
	}

	return blocks
}

// parseOptions extracts all <!-- option: X --> ... <!-- /option --> pairs from the chooser body.
func parseOptions(body string, chooser *Chooser) {
	pos := 0
	for pos < len(body) {
		openLoc := optionOpenRe.FindStringSubmatchIndex(body[pos:])
		if openLoc == nil {
			break
		}
		optValue := body[pos+openLoc[2] : pos+openLoc[3]]
		contentStart := pos + openLoc[1]

		closeLoc := optionCloseRe.FindStringIndex(body[contentStart:])
		if closeLoc == nil {
			break
		}

		content := body[contentStart : contentStart+closeLoc[0]]
		// Trim leading/trailing blank lines
		content = strings.TrimLeft(content, "\n")
		content = strings.TrimRight(content, "\n")

		chooser.Options = append(chooser.Options, Option{
			Value:   optValue,
			Content: content,
		})

		pos = contentStart + closeLoc[1]
	}
}

// ResolveChoosers processes content blocks, resolving choosers based on flags, preferences,
// or interactive prompts. Returns the final markdown string.
func ResolveChoosers(blocks []ContentBlock, prefs *Preferences, flagLang, flagOS string, interactive bool) string {
	var result strings.Builder

	// Track selections made during this render so all choosers of the same type
	// use the same selection.
	sessionSelections := map[string]string{}

	for _, block := range blocks {
		switch b := block.(type) {
		case PlainText:
			result.WriteString(b.Text)
		case Chooser:
			selected := resolveChooser(b, prefs, flagLang, flagOS, interactive, sessionSelections)
			result.WriteString(selected)
		}
	}

	// Final cleanup: strip any leftover chooser/option HTML comment tags that
	// the parser may have missed (e.g. due to nesting or unusual formatting).
	output := result.String()
	output = stripLeftoverTags(output)
	return output
}

// stripLeftoverTags removes lines that consist entirely of a chooser/option HTML comment tag.
// It does NOT modify lines that mix tag and non-tag content to avoid mangling the output.
func stripLeftoverTags(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		if chooserTagRe.MatchString(line) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func resolveChooser(
	c Chooser, prefs *Preferences, flagLang, flagOS string,
	interactive bool, session map[string]string,
) string {
	if len(c.Options) == 0 {
		return ""
	}

	// Determine which option to show
	selection := ""

	// 1. Check flags
	switch c.Type {
	case "language":
		if flagLang != "" {
			selection = flagLang
		}
	case "os":
		if flagOS != "" {
			selection = flagOS
		}
	}

	// 2. Check session (same type already selected this render)
	if selection == "" {
		if s, ok := session[c.Type]; ok {
			selection = s
		}
	}

	// 3. Check stored preferences
	if selection == "" {
		if pref := prefs.Get(c.Type); pref != "" {
			selection = pref
		}
	}

	// 4. If interactive and still no selection, prompt
	if selection == "" && interactive {
		options := make([]string, len(c.Options))
		for i, opt := range c.Options {
			options[i] = opt.Value
		}
		defaultOpt := options[0]
		if pref := prefs.Get(c.Type); pref != "" {
			defaultOpt = pref
		}
		selection = ui.PromptUser(
			"Select "+c.Type+":",
			options,
			defaultOpt,
			cmdutil.GetGlobalColorization(),
		)
	}

	// Save selection
	if selection != "" {
		session[c.Type] = selection
		prefs.Set(c.Type, selection)
		// Best-effort save
		_ = prefs.Save()
	}

	// Build output
	isInline := c.Prefix != "" || c.Suffix != ""

	if selection != "" {
		// Find the matching option
		for _, opt := range c.Options {
			if opt.Value == selection {
				if isInline {
					return c.Prefix + opt.Content + c.Suffix
				}
				return opt.Content + "\n"
			}
		}
	}

	// No selection or no match — show all options
	var buf strings.Builder
	if isInline {
		buf.WriteString(c.Prefix)
		for i, opt := range c.Options {
			if i > 0 {
				buf.WriteString(" / ")
			}
			buf.WriteString(opt.Content)
		}
		buf.WriteString(c.Suffix)
	} else {
		for _, opt := range c.Options {
			buf.WriteString("**" + opt.Value + "**\n\n")
			buf.WriteString(opt.Content)
			buf.WriteString("\n\n")
		}
	}
	return buf.String()
}
