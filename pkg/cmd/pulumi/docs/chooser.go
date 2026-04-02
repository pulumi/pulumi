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

package docs

import (
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

var (
	chooserOpenRe  = regexp.MustCompile(`<!--\s*chooser:\s*(\w+)\s*-->`)
	optionOpenRe   = regexp.MustCompile(`<!--\s*option:\s*(\w+)\s*-->`)
	optionCloseRe  = regexp.MustCompile(`<!--\s*/option\s*-->`)
	chooserCloseRe = regexp.MustCompile(`<!--\s*/chooser\s*-->`)

	// chooserTagRe matches leftover chooser/option comment tags for cleanup.
	// Only targets chooser-specific tags, not arbitrary HTML comments.
	chooserTagRe = regexp.MustCompile(`^\s*<!--\s*/?(chooser|option)[\s:]*\w*\s*-->\s*$`)

	// codeFenceOpenRe matches opening code fences with a language tag.
	codeFenceOpenRe = regexp.MustCompile("^```(\\w+)\\s*$")
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
		loc := chooserOpenRe.FindStringIndex(markdown[pos:])
		if loc == nil {
			blocks = append(blocks, PlainText{Text: markdown[pos:]})
			break
		}

		absStart := pos + loc[0]
		absEnd := pos + loc[1]
		if absStart > pos {
			blocks = append(blocks, PlainText{Text: markdown[pos:absStart]})
		}

		m := chooserOpenRe.FindStringSubmatch(markdown[absStart:absEnd])
		chooserType := m[1]

		closeLoc := chooserCloseRe.FindStringIndex(markdown[absEnd:])
		if closeLoc == nil {
			blocks = append(blocks, PlainText{Text: markdown[absStart:absEnd]})
			pos = absEnd
			continue
		}

		chooserBody := markdown[absEnd : absEnd+closeLoc[0]]
		chooserEnd := absEnd + closeLoc[1]

		prefix := ""
		suffix := ""
		isInline := !strings.Contains(chooserBody, "\n")

		if isInline {
			lineStart := strings.LastIndex(markdown[pos:absStart], "\n")
			if lineStart >= 0 {
				prefix = markdown[pos+lineStart+1 : absStart]
				if len(blocks) > 0 {
					if pt, ok := blocks[len(blocks)-1].(PlainText); ok {
						pt.Text = strings.TrimSuffix(pt.Text, prefix)
						blocks[len(blocks)-1] = pt
					}
				}
			}
			lineEnd := strings.Index(markdown[chooserEnd:], "\n")
			if lineEnd >= 0 {
				suffix = markdown[chooserEnd : chooserEnd+lineEnd]
				chooserEnd = chooserEnd + lineEnd
			} else {
				suffix = markdown[chooserEnd:]
				chooserEnd = len(markdown)
			}
		}

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

// FilterCodeBlocksByLanguage removes unwanted language code blocks from markdown
// that lacks chooser comments. It detects groups of consecutive fenced code blocks
// (separated only by blank lines) and keeps only the block matching the selected language.
// This handles bundle content where Example Usage sections concatenate all languages
// without chooser wrappers.
func FilterCodeBlocksByLanguage(markdown, language string) string {
	if language == "" {
		return markdown
	}

	lines := strings.Split(markdown, "\n")

	type codeBlock struct {
		lang      string
		startLine int // index of the opening ``` line
		endLine   int // index of the closing ``` line
	}

	var blocks []codeBlock
	i := 0
	for i < len(lines) {
		m := codeFenceOpenRe.FindStringSubmatch(lines[i])
		if m == nil {
			i++
			continue
		}
		lang := m[1]
		start := i
		i++
		for i < len(lines) && lines[i] != "```" {
			i++
		}
		if i < len(lines) {
			blocks = append(blocks, codeBlock{lang: lang, startLine: start, endLine: i})
		}
		i++
	}

	if len(blocks) == 0 {
		return markdown
	}

	type blockGroup struct {
		blocks []codeBlock
	}
	var groups []blockGroup
	current := blockGroup{blocks: []codeBlock{blocks[0]}}

	for j := 1; j < len(blocks); j++ {
		prev := current.blocks[len(current.blocks)-1]
		allBlank := true
		for k := prev.endLine + 1; k < blocks[j].startLine; k++ {
			if strings.TrimSpace(lines[k]) != "" {
				allBlank = false
				break
			}
		}
		if allBlank {
			current.blocks = append(current.blocks, blocks[j])
		} else {
			groups = append(groups, current)
			current = blockGroup{blocks: []codeBlock{blocks[j]}}
		}
	}
	groups = append(groups, current)

	excludeLines := map[int]bool{}
	for _, g := range groups {
		if len(g.blocks) < 2 {
			continue
		}
		langs := map[string]bool{}
		for _, b := range g.blocks {
			langs[b.lang] = true
		}
		if len(langs) < 2 {
			continue
		}

		matchIdx := -1
		for idx, b := range g.blocks {
			if b.lang == language {
				matchIdx = idx
				break
			}
		}
		if matchIdx == -1 {
			continue
		}

		for idx, b := range g.blocks {
			if idx == matchIdx {
				continue
			}
			start := b.startLine
			for start > 0 && strings.TrimSpace(lines[start-1]) == "" {
				start--
				if idx > 0 && start <= g.blocks[idx-1].endLine {
					start = g.blocks[idx-1].endLine + 1
					break
				}
			}
			for k := start; k <= b.endLine; k++ {
				excludeLines[k] = true
			}
		}
	}

	if len(excludeLines) == 0 {
		return markdown
	}

	var result []string
	for idx, line := range lines {
		if !excludeLines[idx] {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func resolveChooser(
	c Chooser, prefs *Preferences, flagLang, flagOS string,
	interactive bool, session map[string]string,
) string {
	if len(c.Options) == 0 {
		return ""
	}

	selection := ""

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

	// Reuse the same selection for all choosers of this type within one render.
	if selection == "" {
		if s, ok := session[c.Type]; ok {
			selection = s
		}
	}

	if selection == "" {
		if pref := prefs.Get(c.Type); pref != "" {
			selection = pref
		}
	}

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

	if selection != "" {
		session[c.Type] = selection
		prefs.Set(c.Type, selection)
		if err := prefs.Save(); err != nil {
			logging.V(7).Infof("failed to save docs preferences: %v", err)
		}
	}

	isInline := c.Prefix != "" || c.Suffix != ""

	if selection != "" {
		for _, opt := range c.Options {
			if opt.Value == selection {
				if isInline {
					return c.Prefix + opt.Content + c.Suffix
				}
				return opt.Content + "\n"
			}
		}
	}

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
