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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/docsrender"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

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

type browseNode struct {
	path  string
	title string
	body  string
	items []navOption
}

type navOption struct {
	label string
	path  string
	href  string
}

func (dc *docsCmd) browseLoop(startPath string) error {
	path := strings.Trim(startPath, "/")
	var history []string

	for {
		node := dc.resolveNode(path)
		path = node.path

		// Resolve choosers early so that headings and links extracted below
		// only reflect the selected language/OS/cloud option.
		if node.body != "" {
			node.body = dc.resolveContent(node.body)
		}

		headings := docsrender.GetHeadings(node.body)
		hasSections := len(headings) > 0 && node.body != ""

		prefs := docsrender.LoadPreferences()

		showFull := dc.fullPage
		if !showFull && !dc.sectionsView {
			showFull = prefs.BrowseMode == docsrender.BrowseModeFull
		}
		useSections := hasSections && !showFull

		if dc.fullPage {
			prefs.BrowseMode = docsrender.BrowseModeFull
		} else if dc.sectionsView {
			prefs.BrowseMode = docsrender.BrowseModeSections
		}

		var navItems []navOption
		introIncludesFirstSection := false
		if node.body != "" {
			if useSections {
				introIncludesFirstSection = docsrender.IntroContainsFirstHeading(node.body)
				introNav, err := renderIntro(dc, node.body, node.title, path)
				if err != nil {
					return err
				}
				navItems = introNav
			} else {
				displayBody, links := docsrender.NumberLinks(node.body)
				rendered, err := dc.renderBody(displayBody, node.title)
				if err != nil {
					return err
				}
				fmt.Print(rendered)
				fmt.Print(docsrender.BrowseFooter(dc.baseURLForPath(path), path))
				navItems = numberedNavLinks(links)
			}
			prefs.LastPage = path
		}

		docsrender.SavePreferences(prefs)

		if !cmdutil.Interactive() {
			if node.body == "" && len(node.items) > 0 {
				for _, item := range node.items {
					fmt.Printf("  %s  (%s)\n", item.label, item.path)
				}
			}
			return nil
		}

		navItems = append(navItems, node.items...)

		if len(navItems) == 0 && node.body == "" {
			pageURL := docsrender.WebURL(dc.baseURLForPath(path), path)
			fmt.Fprintf(os.Stderr, "\n  This page is not available for terminal viewing.\n\n  🔗 %s\n\n", pageURL)
			if len(history) > 0 {
				path = history[len(history)-1]
				history = history[:len(history)-1]
				continue
			}
			return nil
		}

		activeItems := navItems
		sectionIdx := -1
		if !useSections {
			// Full page mode — signal that all sections are visible
			// so the menu doesn't offer prev/next navigation.
			sectionIdx = len(headings)
		} else if introIncludesFirstSection {
			sectionIdx = 0
		}
		navigated := false
		for !navigated {
			isRoot := path == ""
			hasHeadings := len(headings) > 0
			menuItems := activeItems
			menu := buildBrowseMenu(browseMenuContext{
				items:       menuItems,
				isRoot:      isRoot,
				hasHeadings: hasHeadings,
				hasHistory:  len(history) > 0,
				hasIntro:    !introIncludesFirstSection,
				sectionIdx:  sectionIdx,
				headings:    headings,
			})

			promptTitle := node.title
			if promptTitle == "" {
				promptTitle = path
			}
			if promptTitle == "" {
				promptTitle = "Pulumi"
			}

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
					activeItems = renderSectionByIdx(dc, node.body, headings, idx)
					sectionIdx = idx
				}

			case navFullPage:
				displayBody, links := docsrender.NumberLinks(node.body)
				rendered, err := dc.renderBody(displayBody, node.title)
				if err == nil {
					fmt.Print(rendered)
					fmt.Print(docsrender.BrowseFooter(dc.baseURLForPath(path), path))
				}
				activeItems = numberedNavLinks(links)
				activeItems = append(activeItems, node.items...)
				sectionIdx = len(headings)

			default:
				if strings.HasPrefix(selected, navPrev) && sectionIdx > -1 {
					sectionIdx--
					if sectionIdx == -1 {
						introNav, renderErr := renderIntro(dc, node.body, node.title, path)
						if renderErr == nil {
							activeItems = introNav
						}
					} else {
						activeItems = renderSectionByIdx(dc, node.body, headings, sectionIdx)
					}
				} else if strings.HasPrefix(selected, navNext) && sectionIdx+1 < len(headings) {
					sectionIdx++
					activeItems = renderSectionByIdx(dc, node.body, headings, sectionIdx)
				} else {
					for _, item := range menuItems {
						if item.label == selected {
							if item.href != "" {
								fmt.Fprintf(os.Stderr, "\n  🔗 %s\n\n", item.href)
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

func (dc *docsCmd) resolveNode(path string) browseNode {
	path = strings.Trim(path, "/")

	if path == "" {
		return dc.resolveRoot()
	}
	if path == "docs" {
		return dc.resolveDocsSitemap()
	}
	if path == "registry" || path == "registry/packages" {
		return dc.resolveRegistryList()
	}
	if docsrender.IsRegistryPath(path) {
		return dc.resolveRegistryPage(path)
	}
	return dc.resolveDocsPage(path)
}

func (dc *docsCmd) resolveRoot() browseNode {
	var items []navOption

	docsPages, _ := docsrender.FetchSitemap(dc.baseURL)
	registryPages, _ := docsrender.FetchRegistrySitemap(dc.registryBaseURL)

	if len(docsPages) > 0 {
		items = append(items, navOption{label: "Docs" + navDrill, path: "docs"})
	}
	if len(registryPages) > 0 {
		items = append(items, navOption{label: "Registry" + navDrill, path: "registry"})
	}

	return browseNode{path: "", title: "Pulumi", items: items}
}

func (dc *docsCmd) resolveDocsSitemap() browseNode {
	return dc.resolveSitemap("docs", "Docs", dc.baseURL, docsrender.FetchSitemap)
}

func (dc *docsCmd) resolveRegistryList() browseNode {
	return dc.resolveSitemap("registry", "Registry", dc.registryBaseURL, docsrender.FetchRegistrySitemap)
}

func (dc *docsCmd) resolveSitemap(
	path, title, baseURL string, fetch func(string) ([]docsrender.SitemapPage, error),
) browseNode {
	pages, err := fetch(baseURL)
	if err != nil {
		return browseNode{path: path, title: title}
	}
	return browseNode{path: path, title: title, items: sitemapToNavOptions(pages, baseURL)}
}

func (dc *docsCmd) resolveRegistryPage(path string) browseNode {
	node := browseNode{path: path}

	var pkgName, docKey string
	var bundle *docsrender.CLIDocsBundle
	if docsrender.IsAPIDocsPath(path) {
		var ok bool
		pkgName, docKey, ok = docsrender.ParseAPIDocsPath(path)
		if ok {
			bundle, _ = docsrender.FetchCLIDocsBundle(dc.registryBaseURL, pkgName)
		}
	}

	if bundle != nil {
		if docKey != "" {
			if b, t, found := docsrender.LookupBundleDoc(bundle, docKey); found {
				node.body = b
				node.title = t
			}
		}
		if node.body == "" && docKey == "" && bundle.Overview != "" {
			node.body = docsrender.BuildAPIDocsPage(bundle, docKey, "")
			node.title = bundle.Package
		}
	}

	if node.body == "" {
		body, title, resolvedPath, err := docsrender.FetchDoc(dc.registryBaseURL, path)
		if err == nil {
			if resolvedPath != "" {
				path = resolvedPath
				node.path = path
			}
			node.body = body
			node.title = title
			if bundle != nil {
				intro := docsrender.GetIntro(node.body)
				node.body = docsrender.BuildAPIDocsPage(bundle, docKey, intro)
			} else if !docsrender.IsAPIDocsPath(path) {
				node.body = docsrender.FormatPackageDetails(node.body)
			}
			trimmedPath := strings.Trim(path, "/")
			for _, item := range dc.registryNavItems(path) {
				if strings.Trim(item.path, "/") != trimmedPath {
					node.items = append(node.items, item)
				}
			}
		} else if bundle != nil {
			// FetchDoc failed but we have a bundle: synthesize a page from it so
			// the user still gets numbered links to drill into resources/functions.
			node.body = docsrender.BuildAPIDocsPage(bundle, docKey, "")
			if node.title == "" {
				node.title = pathLastSegment(path)
			}
		} else {
			var regErr *docsrender.RegistryNotAvailableError
			if errors.As(err, &regErr) {
				url := docsrender.WebURL(dc.registryBaseURL, path)
				node.title = pathLastSegment(path)
				fmt.Fprintf(os.Stderr,
					"Registry docs are not yet available for terminal viewing.\nVisit %s instead.\n\n", url)
			}
		}
	}

	if len(node.items) == 0 && node.body == "" {
		node.items = dc.registryNavItems(path)
	}

	if node.title == "" {
		node.title = pathLastSegment(path)
	}

	return node
}

func (dc *docsCmd) resolveDocsPage(path string) browseNode {
	forceContent := strings.HasSuffix(path, "/__view")
	if forceContent {
		path = strings.TrimSuffix(path, "/__view")
	}

	node := browseNode{path: path}

	body, title, resolvedPath, err := docsrender.FetchDoc(dc.baseURL, path)
	if err == nil {
		if resolvedPath != "" {
			path = resolvedPath
			node.path = path
		}
		node.body = body
		node.title = title
	}

	node.items = dc.docsSitemapNavItems(path)

	if len(node.items) > 0 && node.body != "" && !forceContent {
		viewLabel := "Introduction"
		pages, err := docsrender.FetchSitemap(dc.baseURL)
		if err == nil {
			if p := findPage(pages, "/docs/"+strings.Trim(path, "/")+"/"); p != nil {
				viewLabel = p.ViewLabel()
			}
		}
		selfItem := navOption{label: viewLabel, path: path + "/__view"}
		node.items = append([]navOption{selfItem}, node.items...)
		node.body = ""
	}

	if node.title == "" {
		node.title = pathLastSegment(path)
	}

	return node
}

type browseMenuContext struct {
	items       []navOption
	isRoot      bool
	hasHeadings bool
	hasHistory  bool
	hasIntro    bool
	sectionIdx  int
	headings    []docsrender.Heading
}

func buildBrowseMenu(ctx browseMenuContext) []string {
	var menu []string

	if !ctx.isRoot {
		menu = append(menu, navUp)
	}
	if ctx.hasHeadings {
		menu = append(menu, navSections)
	}
	inSectionView := ctx.sectionIdx >= 0 && ctx.sectionIdx < len(ctx.headings)
	minIdx := 0
	if ctx.hasIntro {
		minIdx = -1
	}
	if ctx.sectionIdx > minIdx && (inSectionView || ctx.sectionIdx == -1) {
		prevLabel := "Introduction"
		if ctx.sectionIdx > 0 {
			prevLabel = ctx.headings[ctx.sectionIdx-1].Title
		}
		menu = append(menu, navPrev+" — "+prevLabel)
	}
	if inSectionView || ctx.sectionIdx == -1 {
		nextIdx := ctx.sectionIdx + 1
		if nextIdx < len(ctx.headings) {
			menu = append(menu, navNext+" — "+ctx.headings[nextIdx].Title)
		}
	}
	for _, item := range ctx.items {
		menu = append(menu, item.label)
	}
	if ctx.hasHeadings && ctx.sectionIdx < len(ctx.headings) {
		menu = append(menu, navFullPage)
	}
	if ctx.hasHistory {
		menu = append(menu, navBack)
	}
	if !ctx.isRoot {
		menu = append(menu, navHome)
	}
	menu = append(menu, navDone)

	return menu
}

func showBrowseSectionsIdx(headings []docsrender.Heading, hasIntro bool) int {
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
		indent := strings.Repeat("  ", h.Level-2)
		options = append(options, indent+h.Title)
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

func renderIntro(dc *docsCmd, body, title, path string) ([]navOption, error) {
	intro := docsrender.GetIntro(body)
	displayIntro, links := docsrender.NumberLinks(intro)
	rendered, err := dc.renderBody(displayIntro, title)
	if err != nil {
		return nil, err
	}
	fmt.Print(rendered)
	fmt.Print(docsrender.BrowseFooter(dc.baseURLForPath(path), path))
	return numberedNavLinks(links), nil
}

func renderSectionByIdx(dc *docsCmd, body string, headings []docsrender.Heading, idx int) []navOption {
	heading := headings[idx]

	section := docsrender.GetSection(body, heading.Slug)
	if section == "" {
		return nil
	}
	numbered, sectionLinks := docsrender.NumberLinks(section)
	if rendered, err := dc.renderBody(numbered, ""); err == nil {
		fmt.Print(rendered)
	}
	return numberedNavLinks(sectionLinks)
}

func parentPath(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return ""
	}
	parent := strings.Join(parts[:len(parts)-1], "/")
	if parent == "registry/packages" {
		return "registry"
	}
	return parent
}

func pathLastSegment(path string) string {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func numberedNavLinks(links []docsrender.Link) []navOption {
	if len(links) == 0 {
		return nil
	}
	var opts []navOption
	n := 0
	for _, l := range links {
		text := strings.TrimSpace(strings.ReplaceAll(l.Title, "`", ""))
		if text == "" {
			continue
		}
		n++
		label := fmt.Sprintf("🔗%d %s", n, text)
		opts = append(opts, navOption{label: label, path: docsrender.HrefToPath(l.URL)})
	}
	return opts
}

func sitemapToNavOptions(pages []docsrender.SitemapPage, baseURL string) []navOption {
	opts := make([]navOption, 0, len(pages))
	for _, p := range pages {
		label := p.Title
		if len(p.Children) > 0 {
			label += navDrill
		}
		opt := navOption{label: label, path: docsrender.HrefToPath(p.Path)}
		if strings.HasPrefix(p.Path, "http://") || strings.HasPrefix(p.Path, "https://") {
			opt.href = p.Path
		} else if strings.HasSuffix(p.Path, ".html") {
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

func childNavOptions(pages []docsrender.SitemapPage, targetPath, baseURL string) []navOption {
	children := findExactChildren(pages, targetPath)
	if children == nil {
		var matches []docsrender.SitemapPage
		collectDescendants(pages, targetPath, &matches)
		children = matches
	}
	return sitemapToNavOptions(children, baseURL)
}

func (dc *docsCmd) docsSitemapNavItems(path string) []navOption {
	pages, err := docsrender.FetchSitemap(dc.baseURL)
	if err != nil {
		return nil
	}
	target := "/docs/" + strings.Trim(path, "/") + "/"
	return childNavOptions(pages, target, dc.baseURL)
}

func (dc *docsCmd) registryNavItems(path string) []navOption {
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")

	if len(parts) >= 3 && parts[0] == "registry" && parts[1] == "packages" {
		pkgName := parts[2]
		pages, err := docsrender.FetchPackageSitemap(dc.registryBaseURL, pkgName)
		if err != nil {
			return nil
		}

		if len(parts) == 3 {
			return sitemapToNavOptions(pages, dc.registryBaseURL)
		}

		target := "/" + trimmed + "/"
		return childNavOptions(pages, target, dc.registryBaseURL)
	}

	return nil
}

func findPage(pages []docsrender.SitemapPage, targetPath string) *docsrender.SitemapPage {
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

func findExactChildren(pages []docsrender.SitemapPage, targetPath string) []docsrender.SitemapPage {
	if p := findPage(pages, targetPath); p != nil {
		return p.Children
	}
	return nil
}

func collectDescendants(pages []docsrender.SitemapPage, prefix string, result *[]docsrender.SitemapPage) {
	for _, p := range pages {
		if strings.HasPrefix(p.Path, prefix) && p.Path != prefix {
			*result = append(*result, p)
		} else if len(p.Children) > 0 {
			collectDescendants(p.Children, prefix, result)
		}
	}
}

