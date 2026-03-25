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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var metaRefreshRe = regexp.MustCompile(`(?i)url=([^"'>]+)`)

// FetchDoc fetches a markdown doc page from the docs site.
// Returns the body (with frontmatter stripped) and the title.
// If the markdown 404s, it tries the HTML page to find redirects or meta refreshes.
func FetchDoc(baseURL, path string) (body string, title string, err error) {
	base := strings.TrimRight(baseURL, "/")
	trimmedPath := strings.Trim(path, "/")
	mdURL := fmt.Sprintf("%s/docs/%s/index.md", base, trimmedPath)

	//nolint:gosec // URL is constructed from user-provided base URL and path
	resp, err := http.Get(mdURL)
	if err != nil {
		return "", "", fmt.Errorf("fetching docs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", fmt.Errorf("reading docs response: %w", err)
		}
		raw := string(data)
		body, title = StripFrontmatter(raw)
		return body, title, nil
	}

	if resp.StatusCode != http.StatusNotFound {
		return "", "", fmt.Errorf("unexpected status %d fetching docs page: %s", resp.StatusCode, path)
	}

	// 404 on .md — try the HTML page to find a redirect
	redirectPath, err := resolveRedirect(base, trimmedPath)
	if err != nil || redirectPath == "" {
		return "", "", fmt.Errorf("documentation page not found: %s", path)
	}

	// Try fetching the markdown at the redirected path
	return FetchDoc(baseURL, redirectPath)
}

// resolveRedirect tries to find a redirect for a missing page by checking
// the HTML version for HTTP redirects or meta refresh tags.
func resolveRedirect(base, path string) (string, error) {
	htmlURL := fmt.Sprintf("%s/docs/%s/", base, path)

	// Use a client that doesn't follow redirects so we can see 301/302
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	//nolint:gosec // URL is constructed from user-provided base URL and path
	resp, err := client.Get(htmlURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for HTTP redirect
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			return extractDocsPath(loc, base), nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	// Check for meta refresh in HTML
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(data)
	if m := metaRefreshRe.FindStringSubmatch(html); m != nil {
		return extractDocsPath(m[1], base), nil
	}

	return "", nil
}

// extractDocsPath extracts a /docs/... path from a full URL or absolute path.
func extractDocsPath(rawURL, base string) string {
	// Strip base URL if present
	rawURL = strings.TrimPrefix(rawURL, base)
	// Strip scheme+host if it's a full URL
	if idx := strings.Index(rawURL, "/docs/"); idx >= 0 {
		rawURL = rawURL[idx:]
	}
	// Strip the /docs/ prefix and trailing slash
	path := strings.TrimPrefix(rawURL, "/docs/")
	path = strings.Trim(path, "/")
	return path
}

// StripFrontmatter removes YAML frontmatter delimited by --- and extracts the title.
func StripFrontmatter(raw string) (body string, title string) {
	if !strings.HasPrefix(raw, "---") {
		return raw, ""
	}

	// Find the closing ---
	rest := raw[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return raw, ""
	}

	frontmatter := rest[:idx]
	body = strings.TrimLeft(rest[idx+4:], "\n")

	// Extract title from frontmatter
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			// Strip surrounding quotes
			title = strings.Trim(title, "\"'")
			break
		}
	}

	return body, title
}

// SitemapPage represents a page in the docs site navigation.
type SitemapPage struct {
	Title     string        `json:"title"`
	Path      string        `json:"path"`
	SelfLabel string        `json:"selfLabel,omitempty"`
	Children  []SitemapPage `json:"children"`
}

// ViewLabel returns the label for viewing this page itself (when it has children).
// Defaults to "Introduction" if not set.
func (p SitemapPage) ViewLabel() string {
	if p.SelfLabel != "" {
		return p.SelfLabel
	}
	return "Introduction"
}

type sitemapResponse struct {
	Pages []SitemapPage `json:"pages"`
}

// FetchSitemap fetches the docs site navigation structure.
func FetchSitemap(baseURL string) ([]SitemapPage, error) {
	url := fmt.Sprintf("%s/docs/cli-sitemap.json", strings.TrimRight(baseURL, "/"))

	//nolint:gosec // URL is constructed from user-provided base URL
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching sitemap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap not available (status %d)", resp.StatusCode)
	}

	var result sitemapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding sitemap: %w", err)
	}
	return result.Pages, nil
}
