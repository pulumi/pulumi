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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// Navigation label constants used across all browse menus.
const (
	navUp       = "↑ Up a level"
	navBack     = "← Back"
	navHome     = "🏠 Home"
	navDone     = "🛑 Done"
	navSections = "📑 Sections"
	navFullPage = "📄 View full page"
	navDrill    = " ▸"
	navNext     = "⏩ Next section"
	navPrev     = "⏪ Previous section"
)

// browseNode represents what to display at a given location in the content tree.
type browseNode struct {
	path  string      // current location
	title string      // display title
	body  string      // raw markdown body (empty for containers/virtual nodes)
	items []navOption // navigation choices (child pages, links, etc.)
}

// navOption represents a navigation choice in browse mode.
type navOption struct {
	label string
	path  string // content path (e.g. "iac/concepts/stacks" or "registry/packages/aws")
}

// browseLoop is the single interactive browse loop. The user is always at a
// location in the content tree. They can go deeper, go up, go back, or quit.
func (dc *docsCmd) browseLoop(startPath string) error {
	path := strings.Trim(startPath, "/")
	var history []string

	for {
		node := dc.resolveNode(path)

		headings := extractHeadings(node.body)
		hasSections := len(headings) > 0 && node.body != ""

		// Determine whether to default to sections view or full page.
		// Flags override saved preference; default is sections.
		showFull := dc.fullPage
		if !showFull && !dc.sectionsView {
			prefs, _ := LoadPreferences()
			showFull = prefs.BrowseMode == "full"
		}
		useSections := hasSections && !showFull

		// Save the preference when a flag is explicitly used
		if dc.fullPage || dc.sectionsView {
			prefs, _ := LoadPreferences()
			if dc.fullPage {
				prefs.BrowseMode = "full"
			} else {
				prefs.BrowseMode = "sections"
			}
			_ = prefs.Save()
		}

		var navItems []navOption
		introIncludesFirstSection := false
		if node.body != "" {
			if useSections {
				introIncludesFirstSection = introContainsFirstHeading(node.body)
				introNav, err := renderIntro(dc, node.body, node.title, path)
				if err != nil {
					return err
				}
				navItems = introNav
			} else {
				displayBody, links := numberLinks(node.body)
				rendered, err := dc.renderBody(displayBody, node.title)
				if err != nil {
					return err
				}
				fmt.Print(rendered)
				fmt.Print(browseFooter(dc.baseURLForPath(path), path))
				navItems = numberedNavLinks(links)
			}
		}

		if node.body != "" {
			prefs, _ := LoadPreferences()
			prefs.LastPage = path
			_ = prefs.Save()
		}

		if !cmdutil.Interactive() {
			if node.body == "" && len(node.items) > 0 {
				for _, item := range node.items {
					fmt.Printf("  %s  (%s)\n", item.label, item.path)
				}
			}
			return nil
		}

		if len(navItems) == 0 {
			navItems = node.items
		}

		if len(navItems) == 0 && node.body == "" {
			url := webURL(dc.baseURLForPath(path), path)
			fmt.Fprintf(os.Stderr, "No content available. Visit %s\n", url)
		}

		activeItems := navItems
		sectionIdx := -1
		if introIncludesFirstSection {
			sectionIdx = 0
		}
		navigated := false
		for !navigated {
			isRoot := path == ""
			hasHeadings := len(headings) > 0
			menu := buildBrowseMenu(activeItems, isRoot, hasHeadings, len(history) > 0, !introIncludesFirstSection, sectionIdx, headings)

			promptTitle := node.title
			if promptTitle == "" {
				promptTitle = path
			}
			if promptTitle == "" {
				promptTitle = "Pulumi"
			}

			// Default to first nav item (skip Up if present)
			defaultIdx := 0
			if !isRoot && len(menu) > 1 {
				defaultIdx = 1
			}

			fmt.Println()
			selected := ui.PromptUser(
				fmt.Sprintf("Browse %s:", promptTitle),
				menu,
				menu[defaultIdx],
				cmdutil.GetGlobalColorization(),
			)

			switch selected {
			case "", navDone:
				return nil

			case navBack:
				path = history[len(history)-1]
				history = history[:len(history)-1]
				navigated = true

			case navUp:
				history = append(history, path)
				path = parentPath(path)
				navigated = true

			case navHome:
				history = append(history, path)
				path = ""
				navigated = true

			case navSections:
				hasIntro := !introIncludesFirstSection
				idx := showBrowseSectionsIdx(headings, hasIntro)
				if idx == -1 {
					introNav, renderErr := renderIntro(dc, node.body, node.title, path)
					if renderErr == nil {
						activeItems = introNav
					}
					sectionIdx = -1
				} else if idx >= 0 {
					renderSectionByIdx(dc, node.body, headings, idx, &activeItems)
					sectionIdx = idx
				}

			case navFullPage:
				displayBody, links := numberLinks(node.body)
				rendered, err := dc.renderBody(displayBody, node.title)
				if err == nil {
					fmt.Print(rendered)
					fmt.Print(browseFooter(dc.baseURLForPath(path), path))
				}
				activeItems = numberedNavLinks(links)
				sectionIdx = -1

			default:
				if strings.HasPrefix(selected, navPrev) && sectionIdx > -1 {
					sectionIdx--
					if sectionIdx == -1 {
						introNav, renderErr := renderIntro(dc, node.body, node.title, path)
						if renderErr == nil {
							activeItems = introNav
						}
					} else {
						renderSectionByIdx(dc, node.body, headings, sectionIdx, &activeItems)
					}
				} else if strings.HasPrefix(selected, navNext) && sectionIdx+1 < len(headings) {
					sectionIdx++
					renderSectionByIdx(dc, node.body, headings, sectionIdx, &activeItems)
				} else {
					// Find the selected nav item
					for _, item := range activeItems {
						if item.label == selected {
							history = append(history, path)
							path = item.path
							navigated = true
							break
						}
					}
				}
			}
		}
	}
}

// resolveNode determines what to display at a given path in the content tree.
func (dc *docsCmd) resolveNode(path string) browseNode {
	path = strings.Trim(path, "/")

	// Root — synthetic Docs/Registry menu
	if path == "" {
		return dc.resolveRoot()
	}

	// "docs" — docs sitemap top level
	if path == "docs" {
		return dc.resolveDocsSitemap()
	}

	// "registry" or "registry/packages" — registry package list
	if path == "registry" || path == "registry/packages" {
		return dc.resolveRegistryList()
	}

	// Registry package paths — try content + lazy-load sitemap
	if isRegistryPath(path) {
		return dc.resolveRegistryPage(path)
	}

	// Docs paths — try content, fall back to sitemap for containers
	return dc.resolveDocsPage(path)
}

func (dc *docsCmd) resolveRoot() browseNode {
	var items []navOption

	// Try to fetch both sitemaps to determine what's available
	docsPages, _ := FetchSitemap(dc.baseURL)
	registryPages, _ := FetchRegistrySitemap(dc.registryBaseURL)

	if len(docsPages) > 0 {
		items = append(items, navOption{label: "Docs" + navDrill, path: "docs"})
	}
	if len(registryPages) > 0 {
		items = append(items, navOption{label: "Registry" + navDrill, path: "registry"})
	}

	return browseNode{path: "", title: "Pulumi", items: items}
}

func (dc *docsCmd) resolveDocsSitemap() browseNode {
	pages, err := FetchSitemap(dc.baseURL)
	if err != nil {
		return browseNode{path: "docs", title: "Docs"}
	}
	return browseNode{
		path:  "docs",
		title: "Docs",
		items: sitemapToNavOptions(pages),
	}
}

func (dc *docsCmd) resolveRegistryList() browseNode {
	pages, err := FetchRegistrySitemap(dc.registryBaseURL)
	if err != nil {
		return browseNode{path: "registry", title: "Registry"}
	}
	return browseNode{
		path:  "registry",
		title: "Registry",
		items: sitemapToNavOptions(pages),
	}
}

func (dc *docsCmd) resolveRegistryPage(path string) browseNode {
	node := browseNode{path: path}

	// Try fetching page content
	body, title, err := FetchDoc(dc.registryBaseURL, path)
	if err == nil {
		node.body = body
		node.title = title
	} else {
		var regErr *RegistryNotAvailableError
		if errors.As(err, &regErr) {
			url := webURL(dc.registryBaseURL, path)
			node.title = pathLastSegment(path)
			// Show fallback as a note, but don't block navigation
			fmt.Fprintf(os.Stderr,
				"Registry docs are not yet available for terminal viewing.\nVisit %s instead.\n\n", url)
		}
	}

	// Sitemap nav items (links from body are extracted with numbering in the loop)
	node.items = dc.registryNavItems(path)

	if node.title == "" {
		node.title = pathLastSegment(path)
	}

	return node
}

func (dc *docsCmd) resolveDocsPage(path string) browseNode {
	node := browseNode{path: path}

	// Try fetching page content
	body, title, err := FetchDoc(dc.baseURL, path)
	if err == nil {
		node.body = body
		node.title = title
	}

	// Sitemap nav items (links from body are extracted with numbering in the loop)
	node.items = dc.docsSitemapNavItems(path)

	if node.title == "" {
		node.title = pathLastSegment(path)
	}

	return node
}

// buildBrowseMenu constructs the menu options for a browse prompt.
// sectionIdx is the current section index (-1 if not viewing a section).
func buildBrowseMenu(items []navOption, isRoot, hasHeadings, hasHistory, hasIntro bool, sectionIdx int, headings []heading) []string {
	var menu []string

	if !isRoot {
		menu = append(menu, navUp)
	}
	if hasHeadings {
		menu = append(menu, navSections)
	}
	// Show "Previous" when viewing a section. Allow going back to intro only if there is one.
	minIdx := 0
	if hasIntro {
		minIdx = -1
	}
	if sectionIdx > minIdx {
		prevLabel := "Introduction"
		if sectionIdx > 0 {
			prevLabel = headings[sectionIdx-1].text
		}
		menu = append(menu, navPrev+" — "+prevLabel)
	}
	if hasHeadings {
		nextIdx := sectionIdx + 1
		if nextIdx < len(headings) {
			menu = append(menu, navNext+" — "+headings[nextIdx].text)
		}
	}
	for _, item := range items {
		menu = append(menu, item.label)
	}
	if hasHeadings {
		menu = append(menu, navFullPage)
	}
	if hasHistory {
		menu = append(menu, navBack)
	}
	if !isRoot {
		menu = append(menu, navHome)
	}
	menu = append(menu, navDone)

	return menu
}

// showBrowseSectionsIdx displays a TOC picker. Returns the selected heading
// index, or -1 for "Introduction" (when hasIntro is true), or -2 if cancelled.
func showBrowseSectionsIdx(headings []heading, hasIntro bool) int {
	if len(headings) == 0 {
		return -2
	}

	var options []string
	introOffset := 0
	if hasIntro {
		options = append(options, "Introduction")
		introOffset = 1
	}
	for _, h := range headings {
		indent := strings.Repeat("  ", h.level-2)
		options = append(options, indent+h.text)
	}
	options = append(options, navBack)

	selected := ui.PromptUser(
		"Jump to section:",
		options,
		options[0],
		cmdutil.GetGlobalColorization(),
	)
	if selected == "" || selected == navBack {
		return -2
	}
	if hasIntro && selected == "Introduction" {
		return -1
	}

	for i, opt := range options {
		idx := i - introOffset
		if opt == selected && idx >= 0 && idx < len(headings) {
			return idx
		}
	}
	return -2
}

// renderIntro renders the page introduction (content before the first heading),
// prints it with the footer, and returns the nav options from any links found.
func renderIntro(dc *docsCmd, body, title, path string) ([]navOption, error) {
	intro := extractIntro(body)
	displayIntro, links := numberLinks(intro)
	rendered, err := dc.renderBody(displayIntro, title)
	if err != nil {
		return nil, err
	}
	fmt.Print(rendered)
	fmt.Print(browseFooter(dc.baseURLForPath(path), path))
	return numberedNavLinks(links), nil
}

// renderSectionByIdx renders the section at the given heading index and
// updates activeItems with any links found in that section.
func renderSectionByIdx(dc *docsCmd, body string, headings []heading, idx int, activeItems *[]navOption) {
	section := extractSection(body, headings[idx].slug)
	if section == "" {
		return
	}
	numbered, sectionLinks := numberLinks(section)
	rendered, err := dc.renderBody(numbered, "")
	if err == nil {
		fmt.Print(rendered)
	}
	*activeItems = numberedNavLinks(sectionLinks)
}

// parentPath returns the parent of a path by removing the last segment.
// Returns "" (root) for single-segment paths.
// Collapses dead intermediate levels (e.g. "registry/packages" → "registry").
func parentPath(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	parent := strings.Join(parts[:len(parts)-1], "/")
	// "registry/packages" is not a real level — the package list lives at "registry"
	if parent == "registry/packages" {
		return "registry"
	}
	return parent
}

// pathLastSegment returns the last segment of a path for use as a fallback title.
func pathLastSegment(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// --- Data helpers (unchanged logic, cleaned up) ---

// numberedNavLinks returns navigation options from internal links, with numbered
// labels matching the **[N]** markers in the rendered output.
func numberedNavLinks(links []docLink) []navOption {
	if len(links) == 0 {
		return nil
	}
	var opts []navOption
	for i, l := range links {
		text := strings.ReplaceAll(l.text, "`", "")
		label := fmt.Sprintf("🔗%d %s", i+1, text)
		opts = append(opts, navOption{label: label, path: hrefToPath(l.href)})
	}
	return opts
}

// sitemapToNavOptions converts sitemap pages to nav options.
func sitemapToNavOptions(pages []SitemapPage) []navOption {
	var opts []navOption
	for _, p := range pages {
		label := p.Title
		if len(p.Children) > 0 {
			label += navDrill
		}
		opts = append(opts, navOption{label: label, path: hrefToPath(p.Path)})
	}
	return opts
}

// childNavOptions finds children for a target path in a sitemap tree,
// falling back to collecting descendants if no exact match exists.
func childNavOptions(pages []SitemapPage, targetPath string) []navOption {
	children := findExactChildren(pages, targetPath)
	if children == nil {
		var matches []SitemapPage
		collectDescendants(pages, targetPath, &matches)
		children = matches
	}
	return sitemapToNavOptions(children)
}

// docsSitemapNavItems returns nav items from the docs sitemap for a given path.
func (dc *docsCmd) docsSitemapNavItems(path string) []navOption {
	pages, err := FetchSitemap(dc.baseURL)
	if err != nil {
		return nil
	}
	target := "/docs/" + strings.Trim(path, "/") + "/"
	return childNavOptions(pages, target)
}

// registryNavItems returns nav items for a registry path from sitemaps.
func (dc *docsCmd) registryNavItems(path string) []navOption {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")

	// "registry/packages/<pkg>" or deeper — lazy-load per-package sitemap
	if len(parts) >= 3 && parts[0] == "registry" && parts[1] == "packages" {
		pkgName := parts[2]
		pages, err := FetchPackageSitemap(dc.registryBaseURL, pkgName)
		if err != nil {
			return nil
		}

		// At package root, return top-level pages
		if len(parts) == 3 {
			return sitemapToNavOptions(pages)
		}

		// Deeper — find children within package sitemap
		target := "/" + trimmed + "/"
		return childNavOptions(pages, target)
	}

	return nil
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
func collectDescendants(pages []SitemapPage, prefix string, result *[]SitemapPage) {
	for _, p := range pages {
		if strings.HasPrefix(p.Path, prefix) && p.Path != prefix {
			*result = append(*result, p)
		} else if len(p.Children) > 0 {
			collectDescendants(p.Children, prefix, result)
		}
	}
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
