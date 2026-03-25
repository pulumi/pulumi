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
)

var linkRe = regexp.MustCompile(`\[([^\]]+)\]\((/docs/[^)]+)\)`)

// docLink represents a markdown link to an internal docs page.
type docLink struct {
	text string
	href string
}

// extractLinks finds all internal docs links (pointing to /docs/...) in the markdown.
// Links are deduplicated by href and returned in order of first appearance.
func extractLinks(md string) []docLink {
	matches := linkRe.FindAllStringSubmatch(md, -1)
	seen := map[string]bool{}
	var links []docLink
	for _, m := range matches {
		href := m[2]
		if seen[href] {
			continue
		}
		seen[href] = true
		links = append(links, docLink{text: m[1], href: href})
	}
	return links
}

// heading represents a markdown heading with its nesting level and URL slug.
type heading struct {
	level int
	text  string
	slug  string
}

// extractHeadings returns all ## and deeper headings from the markdown.
func extractHeadings(md string) []heading {
	var headings []heading
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "## ") {
			continue
		}
		level := 0
		for _, ch := range trimmed {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		text := strings.TrimSpace(trimmed[level:])
		headings = append(headings, heading{
			level: level,
			text:  text,
			slug:  slugify(text),
		})
	}
	return headings
}

// extractSection returns the content from the matching heading through
// the line before the next heading of the same or higher level.
func extractSection(md, slug string) string {
	lines := strings.Split(md, "\n")
	startIdx := -1
	startLevel := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "## ") {
			continue
		}
		level := 0
		for _, ch := range trimmed {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		text := strings.TrimSpace(trimmed[level:])
		if startIdx < 0 {
			if slugify(text) == slug {
				startIdx = i
				startLevel = level
			}
		} else if level <= startLevel {
			return strings.Join(lines[startIdx:i], "\n")
		}
	}

	if startIdx >= 0 {
		return strings.Join(lines[startIdx:], "\n")
	}
	return ""
}

// slugify converts heading text to a URL-friendly slug,
// matching Hugo/GitHub's anchor generation.
func slugify(text string) string {
	text = strings.ReplaceAll(text, "`", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")

	var buf strings.Builder
	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			buf.WriteRune(r)
		} else if r == ' ' || r == '-' {
			buf.WriteByte('-')
		}
	}

	result := buf.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
