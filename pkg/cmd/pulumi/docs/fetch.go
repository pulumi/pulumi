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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// httpClient is a shared HTTP client with a reasonable timeout to prevent
// hanging on slow or unresponsive servers (e.g. external SDK docs redirects).
var httpClient = &http.Client{Timeout: 10 * time.Second}

// bundleHTTPClient uses a longer timeout for large bundle downloads.
var bundleHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// sitemapCache provides session-level caching for sitemap fetches, keyed by URL.
var sitemapCache sync.Map

var metaRefreshRe = regexp.MustCompile(`(?i)url=([^"'>]+)`)

// RegistryNotAvailableError is returned when a registry page returns 404,
// indicating the registry hasn't deployed markdown support yet.
type RegistryNotAvailableError struct {
	Path string
}

func (e *RegistryNotAvailableError) Error() string {
	return "registry docs not available for: " + e.Path
}

// isRegistryPath returns true if the path refers to registry content.
func isRegistryPath(path string) bool {
	return strings.HasPrefix(strings.Trim(path, "/"), "registry/") || strings.Trim(path, "/") == "registry"
}

// contentPrefix returns the URL path prefix ("/docs/" or "/registry/") for a given path,
// and the trimmed path with that prefix removed.
func contentPrefix(path string) (prefix, trimmedPath string) {
	trimmedPath = strings.Trim(path, "/")
	if isRegistryPath(trimmedPath) {
		// Strip the "registry/" prefix (or bare "registry") from the path
		after := strings.TrimPrefix(trimmedPath, "registry")
		after = strings.TrimPrefix(after, "/")
		return "/registry/", after
	}
	return "/docs/", trimmedPath
}

// isAPIDocsPath returns true if the path refers to a registry API docs page
// (e.g. "registry/packages/aws/api-docs/provider").
func isAPIDocsPath(path string) bool {
	return strings.Contains(path, "/api-docs")
}

// FetchDoc fetches a markdown doc page from the docs or registry site.
// Returns the body (with frontmatter stripped) and the title.
// If the markdown 404s, it tries the HTML page to find redirects or meta refreshes.
// For registry paths that 404, returns a RegistryNotAvailableError.
func FetchDoc(baseURL, path string) (body string, title string, err error) {
	base := strings.TrimRight(baseURL, "/")
	prefix, trimmedPath := contentPrefix(path)
	mdURL := fmt.Sprintf("%s%s%s/index.md", base, prefix, trimmedPath)

	//nolint:gosec // URL is constructed from user-provided base URL and path
	resp, err := httpClient.Get(mdURL)
	if err != nil {
		return "", "", fmt.Errorf("fetching docs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", fmt.Errorf("reading docs response: %w", err)
		}
		raw := strings.ReplaceAll(string(data), "\r\n", "\n")
		raw = strings.ReplaceAll(raw, "\t", "    ")
		body, title = StripFrontmatter(raw)
		return body, title, nil
	}

	// CloudFront returns 403 for missing content, so treat it the same as 404.
	notFound := resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden

	if !notFound {
		return "", "", fmt.Errorf("unexpected status %d fetching docs page: %s", resp.StatusCode, path)
	}

	// Registry pages without markdown get a specific error for graceful fallback
	if isRegistryPath(path) {
		return "", "", &RegistryNotAvailableError{Path: path}
	}

	// 404 on .md — try the HTML page to find a redirect
	redirectPath, err := resolveRedirect(base, path)
	if err != nil || redirectPath == "" || redirectPath == strings.Trim(path, "/") {
		return "", "", fmt.Errorf("documentation page not found: %s", path)
	}

	// Try fetching the markdown at the redirected path (one level only)
	return FetchDoc(baseURL, redirectPath)
}

// resolveRedirect tries to find a redirect for a missing page by checking
// the HTML version for HTTP redirects or meta refresh tags.
func resolveRedirect(base, path string) (string, error) {
	prefix, trimmedPath := contentPrefix(path)
	htmlURL := fmt.Sprintf("%s%s%s/", base, prefix, trimmedPath)

	// Use a client that doesn't follow redirects so we can see 301/302
	client := &http.Client{
		Timeout: 10 * time.Second,
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
			return extractContentPath(loc, base), nil
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
		return extractContentPath(m[1], base), nil
	}

	return "", nil
}

// extractContentPath extracts a content path from a full URL or absolute path,
// handling both /docs/... and /registry/... prefixes.
func extractContentPath(rawURL, base string) string {
	// Strip base URL if present
	rawURL = strings.TrimPrefix(rawURL, base)
	// Check for /registry/ first
	if idx := strings.Index(rawURL, "/registry/"); idx >= 0 {
		path := rawURL[idx+1:] // keep "registry/..."
		return strings.Trim(path, "/")
	}
	// Check for /docs/
	if idx := strings.Index(rawURL, "/docs/"); idx >= 0 {
		rawURL = rawURL[idx:]
	}
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
	Children  []SitemapPage `json:"children,omitempty"`
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
	url := strings.TrimRight(baseURL, "/") + "/docs/cli-sitemap.json"
	return fetchSitemapJSON(url, "sitemap")
}

// FetchRegistrySitemap fetches the top-level registry navigation (list of all packages).
func FetchRegistrySitemap(baseURL string) ([]SitemapPage, error) {
	url := strings.TrimRight(baseURL, "/") + "/registry/cli-sitemap.json"
	return fetchSitemapJSON(url, "registry sitemap")
}

// FetchPackageSitemap fetches the per-package navigation for a registry package.
func FetchPackageSitemap(baseURL, packageName string) ([]SitemapPage, error) {
	url := fmt.Sprintf("%s/registry/packages/%s/cli-sitemap.json",
		strings.TrimRight(baseURL, "/"), packageName)
	return fetchSitemapJSON(url, "package sitemap for "+packageName)
}

func fetchSitemapJSON(url, label string) ([]SitemapPage, error) {
	if cached, ok := sitemapCache.Load(url); ok {
		return cached.([]SitemapPage), nil
	}

	//nolint:gosec // URL is constructed from user-provided base URL
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s not available (status %d)", label, resp.StatusCode)
	}

	var result sitemapResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", label, err)
	}
	sitemapCache.Store(url, result.Pages)
	return result.Pages, nil
}
