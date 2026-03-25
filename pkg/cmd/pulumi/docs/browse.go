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
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// browseLinks fetches a page, renders it, then shows its internal docs links
// as a menu. Selecting a link renders that page and shows its links, keeping
// the user in browse mode until they choose "← Done".
func (dc *docsCmd) browseLinks(path string) error {
	for {
		// Fetch the page — it may not exist (container-only node in sitemap)
		rawBody, title, err := FetchDoc(dc.baseURL, path)
		pageExists := err == nil

		if pageExists {
			// Render and display
			rendered, renderErr := dc.renderBody(rawBody, title)
			if renderErr != nil {
				return renderErr
			}
			fmt.Print(rendered)
			fmt.Print(browseFooter(dc.baseURL, path))

			// Save as last viewed page
			prefs, _ := LoadPreferences()
			prefs.LastPage = path
			_ = prefs.Save()
		}

		if !cmdutil.Interactive() {
			if !pageExists {
				fmt.Fprintf(os.Stderr, "  %s\n", err)
			}
			return nil
		}

		// Build navigation options — from page links if the page exists,
		// otherwise fall back to sitemap children
		var navOptions []navOption
		if pageExists {
			navOptions = dc.buildNavOptions(rawBody, path)
		} else {
			// Page doesn't exist — try sitemap children directly
			children := findSitemapChildren(dc.baseURL, path)
			for _, c := range children {
				p := strings.TrimPrefix(c.Path, "/docs/")
				navOptions = append(navOptions, navOption{label: c.Title, path: strings.Trim(p, "/")})
			}
			if len(navOptions) > 0 {
				// Use the path as a title since we have no page title
				title = path
			}
		}
		if len(navOptions) == 0 {
			base := strings.TrimRight(dc.baseURL, "/")
			fmt.Fprintf(os.Stderr, "\nNo links to browse. View in browser: %s/docs/%s/\n", base, path)
			return nil
		}

		// Build prompt
		promptTitle := title
		if promptTitle == "" {
			promptTitle = path
		}

		parts := strings.Split(strings.Trim(path, "/"), "/")
		upLabel := "↑ Back to docs root"
		upAction := "root"
		if len(parts) > 1 {
			upLabel = "↑ Up a level"
			upAction = strings.Join(parts[:len(parts)-1], "/")
		}

		var options []string
		options = append(options, upLabel)
		for _, n := range navOptions {
			options = append(options, n.label)
		}
		options = append(options, "← Done")

		defaultIdx := 1
		if defaultIdx >= len(options) {
			defaultIdx = 0
		}

		selected := ui.PromptUser(
			fmt.Sprintf("Browse %s:", promptTitle),
			options,
			options[defaultIdx],
			cmdutil.GetGlobalColorization(),
		)
		if selected == "" || selected == "← Done" {
			return nil
		}
		if selected == upLabel {
			if upAction == "root" {
				return dc.browseSitemap()
			}
			path = upAction
			continue
		}
		for _, n := range navOptions {
			if n.label == selected {
				path = n.path
				break
			}
		}
	}
}

// navOption represents a navigation choice in browse mode.
type navOption struct {
	label string
	path  string // path without /docs/ prefix
}

// buildNavOptions returns navigation options for a page. It extracts links
// from the raw markdown body, falling back to sitemap children if none found.
func (dc *docsCmd) buildNavOptions(rawBody, path string) []navOption {
	links := extractLinks(rawBody)
	if len(links) > 0 {
		var opts []navOption
		for _, l := range links {
			p := strings.TrimPrefix(l.href, "/docs/")
			opts = append(opts, navOption{label: l.text, path: strings.Trim(p, "/")})
		}
		return opts
	}

	// No inline links — try sitemap children
	children := findSitemapChildren(dc.baseURL, path)
	if len(children) > 0 {
		var opts []navOption
		for _, c := range children {
			p := strings.TrimPrefix(c.Path, "/docs/")
			opts = append(opts, navOption{label: c.Title, path: strings.Trim(p, "/")})
		}
		return opts
	}

	return nil
}

// findSitemapChildren fetches the sitemap and returns children for the given path.
// If the exact path isn't in the sitemap, it looks for pages whose paths
// start with the target (for intermediate container nodes that aren't sitemap entries).
func findSitemapChildren(baseURL, path string) []SitemapPage {
	pages, err := FetchSitemap(baseURL)
	if err != nil {
		return nil
	}
	target := "/docs/" + strings.Trim(path, "/") + "/"

	// First try: exact match
	if found := findExactChildren(pages, target); found != nil {
		return found
	}

	// Second try: collect all direct descendants whose paths start with target
	var matches []SitemapPage
	collectDescendants(pages, target, &matches)
	return matches
}

func findExactChildren(pages []SitemapPage, targetPath string) []SitemapPage {
	for _, p := range pages {
		if p.Path == targetPath {
			return p.Children
		}
		if len(p.Children) > 0 {
			if found := findExactChildren(p.Children, targetPath); found != nil {
				return found
			}
		}
	}
	return nil
}

// collectDescendants finds pages whose path starts with the target prefix.
// It only collects the shallowest matches (direct children, not deep descendants).
func collectDescendants(pages []SitemapPage, prefix string, result *[]SitemapPage) {
	for _, p := range pages {
		if strings.HasPrefix(p.Path, prefix) && p.Path != prefix {
			*result = append(*result, p)
		} else if len(p.Children) > 0 {
			collectDescendants(p.Children, prefix, result)
		}
	}
}

// browseSitemap fetches the docs sitemap and lets the user interactively
// browse and select a page to view.
func (dc *docsCmd) browseSitemap() error {
	pages, err := FetchSitemap(dc.baseURL)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		fmt.Println("No pages found in sitemap.")
		return nil
	}

	if !cmdutil.Interactive() {
		printSitemapTree(pages, 0)
		return nil
	}

	current := pages
	for {
		options := make([]string, 0, len(current)+1)
		for _, p := range current {
			label := p.Title
			if len(p.Children) > 0 {
				label += " ▸"
			}
			options = append(options, label)
		}
		options = append(options, "← Back")

		selected := ui.PromptUser(
			"Browse docs:",
			options,
			options[0],
			cmdutil.GetGlobalColorization(),
		)
		if selected == "" || selected == "← Back" {
			return nil
		}

		for _, p := range current {
			label := p.Title
			if len(p.Children) > 0 {
				label += " ▸"
			}
			if label != selected {
				continue
			}
			if len(p.Children) == 0 {
				// Leaf page — enter browse mode on it
				return dc.browseLinks(pathFromHref(p.Path))
			}
			// Drill into children, offering to view the current page too
			childOptions := []string{p.ViewLabel()}
			for _, c := range p.Children {
				cl := c.Title
				if len(c.Children) > 0 {
					cl += " ▸"
				}
				childOptions = append(childOptions, cl)
			}
			childOptions = append(childOptions, "← Back")

			childSelected := ui.PromptUser(
				p.Title+":",
				childOptions,
				childOptions[0],
				cmdutil.GetGlobalColorization(),
			)
			if childSelected == "" || childSelected == "← Back" {
				break
			}
			if childSelected == p.ViewLabel() {
				return dc.browseLinks(pathFromHref(p.Path))
			}
			for _, c := range p.Children {
				cl := c.Title
				if len(c.Children) > 0 {
					cl += " ▸"
				}
				if cl == childSelected {
					if len(c.Children) > 0 {
						current = c.Children
					} else {
						return dc.browseLinks(pathFromHref(c.Path))
					}
					break
				}
			}
			break
		}
	}
}

// pathFromHref strips the /docs/ prefix and trailing slash from a sitemap href.
func pathFromHref(href string) string {
	p := strings.TrimPrefix(href, "/docs/")
	return strings.Trim(p, "/")
}

func printSitemapTree(pages []SitemapPage, depth int) {
	for _, p := range pages {
		indent := strings.Repeat("  ", depth)
		fmt.Printf("%s%s  (%s)\n", indent, p.Title, p.Path)
		if len(p.Children) > 0 {
			printSitemapTree(p.Children, depth+1)
		}
	}
}
