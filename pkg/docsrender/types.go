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
	"fmt"
	"strings"
	"unicode"

	"github.com/pgavlin/goldmark"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/text"
)

const (
	DefaultDocsBaseURL     = "https://www.pulumi.com"
	DefaultRegistryBaseURL = "https://www.pulumi.com"

	// Chooser type constants.
	ChooserLanguage = "language"
	ChooserOS       = "os"
	ChooserCloud    = "cloud"

	// ANSI escape codes for terminal formatting.
	ANSIBold  = "\033[1m"
	ANSIReset = "\033[0m"

	// Browse mode preferences.
	BrowseModeFull     = "full"
	BrowseModeSections = "sections"

	// Special section slug for the introduction.
	SectionIntroduction = "introduction"
)

// Heading represents an extracted heading from a markdown document.
type Heading struct {
	Slug  string
	Title string
	Level int
}

// Link represents an extracted hyperlink from a markdown document.
type Link struct {
	URL   string
	Title string
}

// SitemapPage represents a page in the docs site navigation.
type SitemapPage struct {
	Title     string        `json:"title"`
	Path      string        `json:"path"`
	SelfLabel string        `json:"selfLabel,omitempty"`
	Children  []SitemapPage `json:"children,omitempty"`
}

// ViewLabel returns the label for viewing this page itself (when it has children).
func (p SitemapPage) ViewLabel() string {
	if p.SelfLabel != "" {
		return p.SelfLabel
	}
	return "Introduction"
}

// RegistryNotAvailableError is returned when a registry page returns 404.
type RegistryNotAvailableError struct {
	Path string
}

func (e *RegistryNotAvailableError) Error() string {
	return "registry docs not available for: " + e.Path
}

// ParseMarkdown parses markdown source into a goldmark AST.
func ParseMarkdown(source []byte) ast.Node {
	p := goldmark.DefaultParser()
	return p.Parse(text.NewReader(source))
}

// Slugify converts heading text to a URL-safe slug.
func Slugify(title string) string {
	return slugify(title)
}

func slugify(title string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevHyphen = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// IsRegistryPath reports whether the path refers to registry content.
func IsRegistryPath(path string) bool {
	p := strings.TrimPrefix(path, "/")
	return strings.HasPrefix(p, "registry/") || p == "registry"
}

// IsAPIDocsPath reports whether the path refers to registry API documentation.
func IsAPIDocsPath(path string) bool {
	p := strings.TrimPrefix(path, "/")
	parts := strings.Split(p, "/")
	// Match both "registry/packages/{pkg}/api-docs" and "registry/packages/{pkg}/api-docs/{key}"
	return len(parts) >= 4 && parts[0] == "registry" && parts[1] == "packages" && parts[3] == "api-docs"
}

// ContentPrefix returns the URL path prefix ("/docs/" or "/registry/") for a given path,
// and the trimmed path with that prefix removed.
func ContentPrefix(path string) (prefix, trimmedPath string) {
	trimmedPath = strings.Trim(path, "/")
	if IsRegistryPath(trimmedPath) {
		after := strings.TrimPrefix(trimmedPath, "registry")
		after = strings.TrimPrefix(after, "/")
		return "/registry/", after
	}
	return "/docs/", trimmedPath
}

// WebURL builds the full web URL for a content path, using the correct prefix.
func WebURL(baseURL, path string) string {
	base := normalizeBaseURL(baseURL)
	prefix, trimmed := ContentPrefix(path)
	if trimmed == "" {
		return base + prefix
	}
	return fmt.Sprintf("%s%s%s/", base, prefix, trimmed)
}
