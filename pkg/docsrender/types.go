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
	"strings"
	"unicode"

	"github.com/pgavlin/goldmark"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/text"
)

const (
	DefaultDocsBaseURL     = "https://www.pulumi.com"
	DefaultRegistryBaseURL = "https://www.pulumi.com"
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

// ParseMarkdown parses markdown source into a goldmark AST.
func ParseMarkdown(source []byte) ast.Node {
	p := goldmark.DefaultParser()
	return p.Parse(text.NewReader(source))
}

// slugify converts heading text to a URL-safe slug.
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
	// registry/packages/{pkg}/api-docs/...
	if len(parts) >= 5 && parts[0] == "registry" && parts[1] == "packages" && parts[3] == "api-docs" {
		return true
	}
	return false
}
