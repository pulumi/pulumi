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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// HTTPClient is the shared HTTP client for doc fetches. Tests can replace it.
var HTTPClient = &http.Client{Timeout: 10 * time.Second}

// BundleHTTPClient uses a longer timeout for large bundle downloads.
var BundleHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// redirectClient follows no redirects so we can inspect 301/302 responses.
var redirectClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// sitemapCache provides session-level caching for sitemap fetches, keyed by URL.
var sitemapCache sync.Map

var metaRefreshRe = regexp.MustCompile(`(?i)url=([^"'>]+)`)

func normalizeBaseURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/")
}

func userAgent() string {
	return fmt.Sprintf("pulumi-cli/1 (%s; %s)", runtime.GOOS, runtime.GOARCH)
}

// FetchDoc fetches a markdown doc page from the docs or registry site.
// Returns the body (with frontmatter stripped) and the title.
// If the markdown 404s, it tries the HTML page to find redirects or meta refreshes.
// For registry paths that 404, returns a RegistryNotAvailableError.
func FetchDoc(baseURL, path string) (body string, title string, err error) {
	base := normalizeBaseURL(baseURL)
	prefix, trimmedPath := ContentPrefix(path)
	mdURL := fmt.Sprintf("%s%s%s/index.md", base, prefix, trimmedPath)

	req, err := http.NewRequest(http.MethodGet, mdURL, nil) //nolint:gosec // URL from user base URL
	if err != nil {
		return "", "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent())
	resp, err := HTTPClient.Do(req)
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

	if IsRegistryPath(path) {
		return "", "", &RegistryNotAvailableError{Path: path}
	}

	// 404 on .md — try the HTML page to find a redirect
	redirectPath, err := resolveRedirect(base, path)
	if err != nil || redirectPath == "" || redirectPath == strings.Trim(path, "/") {
		return "", "", fmt.Errorf("documentation page not found: %s", path)
	}

	return FetchDoc(baseURL, redirectPath)
}

func resolveRedirect(base, path string) (string, error) {
	prefix, trimmedPath := ContentPrefix(path)
	htmlURL := fmt.Sprintf("%s%s%s/", base, prefix, trimmedPath)

	//nolint:gosec // URL is constructed from user-provided base URL and path
	resp, err := redirectClient.Get(htmlURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			return extractContentPath(loc, base), nil
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

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

func extractContentPath(rawURL, base string) string {
	rawURL = strings.TrimPrefix(rawURL, base)
	if idx := strings.Index(rawURL, "/registry/"); idx >= 0 {
		path := rawURL[idx+1:]
		return strings.Trim(path, "/")
	}
	if idx := strings.Index(rawURL, "/docs/"); idx >= 0 {
		rawURL = rawURL[idx:]
	}
	path := strings.TrimPrefix(rawURL, "/docs/")
	return strings.Trim(path, "/")
}

// StripFrontmatter removes YAML frontmatter delimited by --- and extracts the title.
func StripFrontmatter(raw string) (body string, title string) {
	if !strings.HasPrefix(raw, "---") {
		return raw, ""
	}

	rest := raw[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return raw, ""
	}

	frontmatter := rest[:idx]
	body = strings.TrimLeft(rest[idx+4:], "\n")

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
			title = strings.Trim(title, "\"'")
			break
		}
	}

	return body, title
}

type sitemapResponse struct {
	Pages []SitemapPage `json:"pages"`
}

// FetchSitemap fetches the docs site navigation structure.
func FetchSitemap(baseURL string) ([]SitemapPage, error) {
	url := normalizeBaseURL(baseURL) + "/docs/cli-sitemap.json"
	return fetchSitemapJSON(url, "sitemap")
}

// FetchRegistrySitemap fetches the top-level registry navigation (list of all packages).
func FetchRegistrySitemap(baseURL string) ([]SitemapPage, error) {
	url := normalizeBaseURL(baseURL) + "/registry/cli-sitemap.json"
	return fetchSitemapJSON(url, "registry sitemap")
}

// FetchPackageSitemap fetches the per-package navigation for a registry package.
func FetchPackageSitemap(baseURL, packageName string) ([]SitemapPage, error) {
	url := fmt.Sprintf("%s/registry/packages/%s/cli-sitemap.json",
		normalizeBaseURL(baseURL), packageName)
	return fetchSitemapJSON(url, "package sitemap for "+packageName)
}

func fetchSitemapJSON(url, label string) ([]SitemapPage, error) {
	if cached, ok := sitemapCache.Load(url); ok {
		return cached.([]SitemapPage), nil
	}

	//nolint:gosec // URL is constructed from user-provided base URL
	resp, err := HTTPClient.Get(url)
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

// HrefToPath strips /docs/ or /registry/ prefix from an href, returning a clean path.
func HrefToPath(href string) string {
	if idx := strings.IndexAny(href, "?#"); idx >= 0 {
		href = href[:idx]
	}
	href = strings.Trim(href, "/")
	if strings.HasPrefix(href, "registry/") {
		return href
	}
	path := strings.TrimPrefix(href, "docs/")
	return strings.Trim(path, "/")
}
