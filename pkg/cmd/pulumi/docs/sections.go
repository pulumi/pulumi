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
	"regexp"
	"strings"
)

var linkRe = regexp.MustCompile(`\[([^\]]+)\]\((/(docs|registry)/[^)]+)\)`)

// allLinkRe matches any markdown link.
var allLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)

// stripExternalLinks replaces non-internal markdown links with just their
// display text. Internal links (/docs/... and /registry/...) are preserved.
// This prevents glamour from rendering unhelpful raw URLs inline.
func stripExternalLinks(md string) string {
	return allLinkRe.ReplaceAllStringFunc(md, func(match string) string {
		// Keep internal links intact
		if linkRe.MatchString(match) {
			return match
		}
		m := allLinkRe.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		return m[1]
	})
}

// docLink represents a markdown link to an internal docs page.
type docLink struct {
	text string
	href string
}

// extractLinks finds all internal links (pointing to /docs/... or /registry/...) in the markdown.
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

// numberLinks replaces internal doc/registry links in the markdown with numbered
// references (e.g. "[1] Link text") and returns the annotated markdown along with
// the ordered list of links. This makes links easy to identify in rendered output.
func numberLinks(md string) (annotated string, links []docLink) {
	links = extractLinks(md)
	if len(links) == 0 {
		return md, nil
	}

	// Build a map from href to number (1-based)
	linkNum := make(map[string]int, len(links))
	for i, l := range links {
		linkNum[l.href] = i + 1
	}

	annotated = linkRe.ReplaceAllStringFunc(md, func(match string) string {
		m := linkRe.FindStringSubmatch(match)
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
	return annotated, links
}

// heading represents a markdown heading with its nesting level and URL slug.
type heading struct {
	level int
	text  string
	slug  string
}

// extractIntro returns the content before the first ## heading.
func extractIntro(md string) string {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			intro := strings.TrimSpace(strings.Join(lines[:i], "\n"))
			if intro != "" {
				return intro
			}
			// No text before first ##, include the first section too.
			// Find the end of this section (next ## at same or higher level).
			for j := i + 1; j < len(lines); j++ {
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "## ") {
					return strings.TrimSpace(strings.Join(lines[:j], "\n"))
				}
			}
			// Only one section — return the whole thing.
			return md
		}
	}
	return md
}

// introContainsFirstHeading returns true if the intro (as returned by extractIntro)
// includes the first ## heading because there was no text before it.
func introContainsFirstHeading(md string) bool {
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		return strings.HasPrefix(trimmed, "## ")
	}
	return false
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
