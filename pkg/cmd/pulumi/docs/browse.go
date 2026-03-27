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

	"github.com/pkg/browser"
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
	path        string                 // current location
	title       string                 // display title
	body        string                 // raw markdown body (empty for containers/virtual nodes)
	items       []navOption            // navigation choices (child pages, links, etc.)
	bundleTable string                 // pre-rendered table of resources/functions (shown before menu)
	sectionNav  map[string][]navOption // nav items to inject when viewing specific sections (e.g. "Modules")
	pinnedNav   []navOption            // nav items that always appear in the menu regardless of context
}

// navOption represents a navigation choice in browse mode.
type navOption struct {
	label string
	path  string // content path (e.g. "iac/concepts/stacks" or "registry/packages/aws")
	href  string // original URL for external links (bypasses path-based navigation)
}

// browseLoop is the single interactive browse loop. The user is always at a
// location in the content tree. They can go deeper, go up, go back, or quit.
func (dc *docsCmd) browseLoop(startPath string) error {
	path := strings.Trim(startPath, "/")
	var history []string

	for {
		node := dc.resolveNode(path)
		path = node.path // sync with any path cleanup (e.g. /__view stripping)

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

		// Merge sitemap/bundle nav items with any numbered links from the body.
		navItems = append(navItems, node.items...)

		// Display bundle table (modules/resources/functions) before the menu,
		// but only when the page has no body content (pure listing page).
		if node.bundleTable != "" && node.body == "" {
			fmt.Print(node.bundleTable)
		}

		if len(navItems) == 0 && len(node.pinnedNav) == 0 && node.body == "" {
			// No terminal content — try to open in browser, otherwise show the URL.
			pageURL := webURL(dc.baseURLForPath(path), path)
			if err := browser.OpenURL(pageURL); err != nil {
				fmt.Fprintf(os.Stderr, "\n  🌐 This page is available on the web:\n     %s\n", pageURL)
			} else {
				fmt.Fprintf(os.Stderr, "\n  🌐 Opened in browser: %s\n", pageURL)
			}
			// Go back to previous page
			if len(history) > 0 {
				path = history[len(history)-1]
				history = history[:len(history)-1]
				continue
			}
			return nil
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
			// Always include pinned nav items (e.g. "API Docs" shortcut) even after
			// section navigation replaces activeItems.
			menuItems := append(activeItems, node.pinnedNav...)
			menu := buildBrowseMenu(menuItems, isRoot, hasHeadings, len(history) > 0, !introIncludesFirstSection, sectionIdx, headings)

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
					renderSectionByIdx(dc, node.body, headings, idx, &activeItems, node.sectionNav)
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
				sectionIdx = len(headings) // past all sections — suppresses prev/next

			default:
				if strings.HasPrefix(selected, navPrev) && sectionIdx > -1 {
					sectionIdx--
					if sectionIdx == -1 {
						introNav, renderErr := renderIntro(dc, node.body, node.title, path)
						if renderErr == nil {
							activeItems = introNav
						}
					} else {
						renderSectionByIdx(dc, node.body, headings, sectionIdx, &activeItems, node.sectionNav)
					}
				} else if strings.HasPrefix(selected, navNext) && sectionIdx+1 < len(headings) {
					sectionIdx++
					renderSectionByIdx(dc, node.body, headings, sectionIdx, &activeItems, node.sectionNav)
				} else {
					// Find the selected nav item (search menuItems which includes pinnedNav)
					for _, item := range menuItems {
						if item.label == selected {
							// External link — open in browser instead of navigating
							if item.href != "" {
								if err := browser.OpenURL(item.href); err != nil {
									fmt.Fprintf(os.Stderr, "\n  🌐 This page is available on the web:\n     %s\n", item.href)
								} else {
									fmt.Fprintf(os.Stderr, "\n  🌐 Opened in browser: %s\n", item.href)
								}
								break
							}
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
		items: sitemapToNavOptions(pages, dc.baseURL),
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
		items: sitemapToNavOptions(pages, dc.registryBaseURL),
	}
}

func (dc *docsCmd) resolveRegistryPage(path string) browseNode {
	node := browseNode{path: path}

	// For API docs paths, try the CLI docs bundle for content and navigation
	if isAPIDocsPath(path) {
		pkgName, docKey, ok := ParseAPIDocsPath(path)
		if ok {
			bundle, bundleErr := FetchCLIDocsBundle(dc.registryBaseURL, pkgName)
			if bundleErr == nil {
				// If we have a specific resource/function key, look up its content
				if docKey != "" {
					if b, t, found := LookupBundleDoc(bundle, docKey); found {
						node.body = b
						node.title = t
					}
				}
				// Generate nav items from bundle keys for this module level.
				// If we found content (a resource/function page), skip bundle nav.
				// Otherwise docKey is a module prefix — list its children.
				if node.body == "" {
					bundleNav := BundleNavItems(bundle, docKey, pkgName)
					if len(bundleNav) > 0 {
						node.items = bundleNav
						node.bundleTable = RenderBundleTable(bundle, docKey)
					}
				}
			}
		}
	}

	// Fall back to FetchDoc if bundle didn't provide content
	if node.body == "" {
		body, title, err := FetchDoc(dc.registryBaseURL, path)
		if err == nil {
			node.body = body
			node.title = title
			// Page has its own content — don't flood the menu with bundle items.
			// Users navigate via sections; bundle nav is only for pure listing pages.
			// But replace Modules/Resources/Functions sections with formatted bundle content,
			// and provide per-section nav items so users can drill into modules/resources.
			if isAPIDocsPath(path) {
				pkgName, docKey, ok := ParseAPIDocsPath(path)
				if ok {
					if bundle, bundleErr := FetchCLIDocsBundle(dc.registryBaseURL, pkgName); bundleErr == nil {
						node.body = ReplaceBundleSections(node.body, bundle, docKey)
						node.sectionNav = BundleSectionNav(bundle, docKey, pkgName)
					}
				}
			}
			node.items = nil
			node.bundleTable = ""
		} else {
			var regErr *RegistryNotAvailableError
			if errors.As(err, &regErr) {
				url := webURL(dc.registryBaseURL, path)
				node.title = pathLastSegment(path)
				fmt.Fprintf(os.Stderr,
					"Registry docs are not yet available for terminal viewing.\nVisit %s instead.\n\n", url)
			}
		}
	}

	// Fall back to sitemap nav if no nav items set and no body content.
	// Pages with body content use section-based navigation instead.
	if len(node.items) == 0 && node.body == "" {
		node.items = dc.registryNavItems(path)
	}

	if node.title == "" {
		node.title = pathLastSegment(path)
	}

	// Add an "API Docs" shortcut when viewing any package page that isn't already under api-docs.
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 3 && parts[0] == "registry" && parts[1] == "packages" && !isAPIDocsPath(path) {
		apiDocsPath := fmt.Sprintf("registry/packages/%s/api-docs", parts[2])
		node.pinnedNav = append(node.pinnedNav, navOption{label: "📖 API Docs" + navDrill, path: apiDocsPath})
	}

	return node
}

func (dc *docsCmd) resolveDocsPage(path string) browseNode {
	// The /__view suffix forces content mode (user selected the self-view option).
	forceContent := strings.HasSuffix(path, "/__view")
	if forceContent {
		path = strings.TrimSuffix(path, "/__view")
	}

	node := browseNode{path: path}

	// Try fetching page content
	body, title, err := FetchDoc(dc.baseURL, path)
	if err == nil {
		node.body = body
		node.title = title
	}

	// Sitemap nav items
	node.items = dc.docsSitemapNavItems(path)

	// When a page has both content and children, act as a hub:
	// show children with a self-view option, don't render content immediately.
	if len(node.items) > 0 && node.body != "" && !forceContent {
		viewLabel := dc.docsPageViewLabel(path)
		selfItem := navOption{label: viewLabel, path: path + "/__view"}
		node.items = append([]navOption{selfItem}, node.items...)
		node.body = "" // suppress content rendering — let the user choose
	}

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
	// Show "Previous"/"Next" when viewing a section, not when viewing the full page.
	inSectionView := sectionIdx >= 0 && sectionIdx < len(headings)
	minIdx := 0
	if hasIntro {
		minIdx = -1
	}
	if sectionIdx > minIdx && (inSectionView || sectionIdx == -1) {
		prevLabel := "Introduction"
		if sectionIdx > 0 {
			prevLabel = headings[sectionIdx-1].text
		}
		menu = append(menu, navPrev+" — "+prevLabel)
	}
	if inSectionView || sectionIdx == -1 {
		nextIdx := sectionIdx + 1
		if nextIdx < len(headings) {
			menu = append(menu, navNext+" — "+headings[nextIdx].text)
		}
	}
	for _, item := range items {
		menu = append(menu, item.label)
	}
	if hasHeadings && sectionIdx < len(headings) {
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
// If sectionNav contains items matching the heading text, those are appended
// to activeItems so users can drill into modules/resources/functions.
func renderSectionByIdx(dc *docsCmd, body string, headings []heading, idx int,
	activeItems *[]navOption, sectionNav map[string][]navOption,
) {
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

	// Inject bundle nav items for this section (e.g. module drill items for "Modules")
	if sectionNav != nil {
		if nav, ok := sectionNav[headings[idx].text]; ok {
			*activeItems = append(*activeItems, nav...)
		}
	}
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
// baseURL is used to build full URLs for external/case-sensitive links.
func sitemapToNavOptions(pages []SitemapPage, baseURL string) []navOption {
	var opts []navOption
	for _, p := range pages {
		label := p.Title
		if len(p.Children) > 0 {
			label += navDrill
		}
		opt := navOption{label: label, path: hrefToPath(p.Path)}
		// Preserve original href for external URLs and case-sensitive paths.
		if strings.HasPrefix(p.Path, "http://") || strings.HasPrefix(p.Path, "https://") {
			opt.href = p.Path
		} else if strings.HasSuffix(p.Path, ".html") {
			// Case-sensitive path — build full URL
			base := strings.TrimRight(baseURL, "/")
			if base == "" {
				base = "https://www.pulumi.com"
			}
			opt.href = base + p.Path
		}
		opts = append(opts, opt)
	}
	return opts
}

// childNavOptions finds children for a target path in a sitemap tree,
// falling back to collecting descendants if no exact match exists.
func childNavOptions(pages []SitemapPage, targetPath, baseURL string) []navOption {
	children := findExactChildren(pages, targetPath)
	if children == nil {
		var matches []SitemapPage
		collectDescendants(pages, targetPath, &matches)
		children = matches
	}
	return sitemapToNavOptions(children, baseURL)
}

// docsSitemapNavItems returns nav items from the docs sitemap for a given path.
func (dc *docsCmd) docsSitemapNavItems(path string) []navOption {
	pages, err := FetchSitemap(dc.baseURL)
	if err != nil {
		return nil
	}
	target := "/docs/" + strings.Trim(path, "/") + "/"
	return childNavOptions(pages, target, dc.baseURL)
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
			return sitemapToNavOptions(pages, dc.registryBaseURL)
		}

		// Deeper — find children within package sitemap
		target := "/" + trimmed + "/"
		return childNavOptions(pages, target, dc.registryBaseURL)
	}

	return nil
}

// docsPageViewLabel returns the self-view label for a docs page (e.g. "Overview" or "Introduction").
func (dc *docsCmd) docsPageViewLabel(path string) string {
	pages, err := FetchSitemap(dc.baseURL)
	if err != nil {
		return "Introduction"
	}
	target := "/docs/" + strings.Trim(path, "/") + "/"
	if p := findPage(pages, target); p != nil {
		return p.ViewLabel()
	}
	return "Introduction"
}

// findPage recursively searches for a page with the given path in the sitemap tree.
func findPage(pages []SitemapPage, targetPath string) *SitemapPage {
	for i := range pages {
		if pages[i].Path == targetPath {
			return &pages[i]
		}
		if found := findPage(pages[i].Children, targetPath); found != nil {
			return found
		}
	}
	return nil
}

func findExactChildren(pages []SitemapPage, targetPath string) []SitemapPage {
	if p := findPage(pages, targetPath); p != nil {
		return p.Children
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
