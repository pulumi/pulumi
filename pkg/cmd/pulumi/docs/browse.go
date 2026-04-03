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

	"github.com/pkg/browser"
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
	path         string
	title        string
	body         string
	items        []navOption
	bundleTable  string
	bundle       *docsrender.CLIDocsBundle
	bundlePrefix string
	sectionNav   map[string][]navOption
	pinnedNav    []navOption
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

		headings := docsrender.GetHeadings(node.body)
		hasSections := len(headings) > 0 && node.body != ""

		showFull := dc.fullPage
		if !showFull && !dc.sectionsView {
			prefs := docsrender.LoadPreferences()
			showFull = prefs.BrowseMode == "full"
		}
		useSections := hasSections && !showFull

		if dc.fullPage || dc.sectionsView {
			prefs := docsrender.LoadPreferences()
			if dc.fullPage {
				prefs.BrowseMode = "full"
			} else {
				prefs.BrowseMode = "sections"
			}
			docsrender.SavePreferences(prefs)
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
		}

		if node.body != "" {
			prefs := docsrender.LoadPreferences()
			prefs.LastPage = path
			docsrender.SavePreferences(prefs)
		}

		if !cmdutil.Interactive() {
			if node.body == "" && len(node.items) > 0 {
				for _, item := range node.items {
					fmt.Printf("  %s  (%s)\n", item.label, item.path)
				}
			}
			return nil
		}

		navItems = append(navItems, node.items...)

		if node.bundleTable != "" && node.body == "" {
			fmt.Print(node.bundleTable)
		}

		if len(navItems) == 0 && len(node.pinnedNav) == 0 && node.body == "" {
			pageURL := docsrender.WebURL(dc.baseURLForPath(path), path)
			if err := browser.OpenURL(pageURL); err != nil {
				fmt.Fprintf(os.Stderr, "\n  🌐 This page is available on the web:\n     %s\n", pageURL)
			} else {
				fmt.Fprintf(os.Stderr, "\n  🌐 Opened in browser: %s\n", pageURL)
			}
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
			menuItems := append(activeItems, node.pinnedNav...)
			menu := buildBrowseMenu(menuItems, isRoot, hasHeadings,
				len(history) > 0, !introIncludesFirstSection, sectionIdx, headings)

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
					renderSectionByIdx(dc, node.body, headings, idx, &activeItems, &node)
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
						renderSectionByIdx(dc, node.body, headings, sectionIdx, &activeItems, &node)
					}
				} else if strings.HasPrefix(selected, navNext) && sectionIdx+1 < len(headings) {
					sectionIdx++
					renderSectionByIdx(dc, node.body, headings, sectionIdx, &activeItems, &node)
				} else {
					for _, item := range menuItems {
						if item.label == selected {
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
	pages, err := docsrender.FetchSitemap(dc.baseURL)
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
	pages, err := docsrender.FetchRegistrySitemap(dc.registryBaseURL)
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

	if docsrender.IsAPIDocsPath(path) {
		pkgName, docKey, ok := docsrender.ParseAPIDocsPath(path)
		if ok {
			bundle, bundleErr := docsrender.FetchCLIDocsBundle(dc.registryBaseURL, pkgName)
			if bundleErr == nil {
				if docKey != "" {
					if b, t, found := docsrender.LookupBundleDoc(bundle, docKey); found {
						node.body = b
						node.title = t
					}
				}
				if node.body == "" {
					bundleNav := BundleNavItems(bundle, docKey, pkgName)
					if len(bundleNav) > 0 {
						node.items = bundleNav
						node.bundleTable = docsrender.RenderBundleTable(bundle, docKey)
					}
				}
			}
		}
	}

	if node.body == "" {
		body, title, err := docsrender.FetchDoc(dc.registryBaseURL, path)
		if err == nil {
			node.body = body
			node.title = title
			if docsrender.IsAPIDocsPath(path) {
				pkgName, docKey, ok := docsrender.ParseAPIDocsPath(path)
				if ok {
					if bundle, bundleErr := docsrender.FetchCLIDocsBundle(dc.registryBaseURL, pkgName); bundleErr == nil {
						intro := docsrender.GetIntro(node.body)
						node.body = docsrender.BuildAPIDocsPage(bundle, docKey, intro)
						node.sectionNav = BundleSectionNav(bundle, docKey, pkgName)
						node.bundleTable = docsrender.RenderBundleTable(bundle, docKey)
						node.bundle = bundle
						node.bundlePrefix = docKey
					}
				}
			} else {
				node.body = docsrender.FormatPackageDetails(node.body)
			}
			node.items = nil
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

	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) >= 3 && parts[0] == "registry" && parts[1] == "packages" && !docsrender.IsAPIDocsPath(path) {
		apiDocsPath := fmt.Sprintf("registry/packages/%s/api-docs", parts[2])
		node.pinnedNav = append(node.pinnedNav, navOption{label: "📖 API Docs" + navDrill, path: apiDocsPath})
	}

	return node
}

func (dc *docsCmd) resolveDocsPage(path string) browseNode {
	forceContent := strings.HasSuffix(path, "/__view")
	if forceContent {
		path = strings.TrimSuffix(path, "/__view")
	}

	node := browseNode{path: path}

	body, title, err := docsrender.FetchDoc(dc.baseURL, path)
	if err == nil {
		node.body = body
		node.title = title
	}

	node.items = dc.docsSitemapNavItems(path)

	if len(node.items) > 0 && node.body != "" && !forceContent {
		viewLabel := dc.docsPageViewLabel(path)
		selfItem := navOption{label: viewLabel, path: path + "/__view"}
		node.items = append([]navOption{selfItem}, node.items...)
		node.body = ""
	}

	if node.title == "" {
		node.title = pathLastSegment(path)
	}

	return node
}

func buildBrowseMenu(
	items []navOption, isRoot, hasHeadings, hasHistory, hasIntro bool,
	sectionIdx int, headings []docsrender.Heading,
) []string {
	var menu []string

	if !isRoot {
		menu = append(menu, navUp)
	}
	if hasHeadings {
		menu = append(menu, navSections)
	}
	inSectionView := sectionIdx >= 0 && sectionIdx < len(headings)
	minIdx := 0
	if hasIntro {
		minIdx = -1
	}
	if sectionIdx > minIdx && (inSectionView || sectionIdx == -1) {
		prevLabel := "Introduction"
		if sectionIdx > 0 {
			prevLabel = headings[sectionIdx-1].Title
		}
		menu = append(menu, navPrev+" — "+prevLabel)
	}
	if inSectionView || sectionIdx == -1 {
		nextIdx := sectionIdx + 1
		if nextIdx < len(headings) {
			menu = append(menu, navNext+" — "+headings[nextIdx].Title)
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

func renderSectionByIdx(dc *docsCmd, body string, headings []docsrender.Heading, idx int,
	activeItems *[]navOption, node *browseNode,
) {
	heading := headings[idx]

	// For the Modules section, render the column table directly instead of
	// going through glamour (which can't preserve columnar whitespace).
	// Resources and Functions use bullet lists that glamour handles fine.
	if node.sectionNav != nil && node.bundle != nil && heading.Title == docsrender.SectionModules {
		if nav, ok := node.sectionNav[heading.Title]; ok {
			sectionTable := docsrender.RenderBundleSingleSection(node.bundle, node.bundlePrefix, heading.Title)
			if sectionTable != "" {
				docsrender.PrintHeadingWithTable(heading.Title, sectionTable)
				*activeItems = nav
				return
			}
		}
	}

	section := docsrender.GetSection(body, heading.Slug)
	if section == "" {
		return
	}
	numbered, sectionLinks := docsrender.NumberLinks(section)
	rendered, err := dc.renderBody(numbered, "")
	if err == nil {
		fmt.Print(rendered)
	}
	*activeItems = numberedNavLinks(sectionLinks)

	if node.sectionNav != nil {
		if nav, ok := node.sectionNav[headings[idx].Title]; ok {
			*activeItems = append(*activeItems, nav...)
		}
	}
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

func (dc *docsCmd) docsPageViewLabel(path string) string {
	pages, err := docsrender.FetchSitemap(dc.baseURL)
	if err != nil {
		return "Introduction"
	}
	target := "/docs/" + strings.Trim(path, "/") + "/"
	if p := findPage(pages, target); p != nil {
		return p.ViewLabel()
	}
	return "Introduction"
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

// bundleModuleNav builds nav options for sub-modules.
func bundleModuleNav(ck docsrender.ClassifiedKeys, basePath, modulePrefix string) []navOption {
	opts := make([]navOption, 0, len(ck.SubModules))
	for _, mod := range ck.SubModules {
		modPath := basePath
		if modulePrefix != "" {
			modPath += "/" + modulePrefix
		}
		modPath += "/" + mod
		opts = append(opts, navOption{label: "🔗 " + mod + navDrill, path: modPath})
	}
	return opts
}

// bundleEntryNav builds nav options for a slice of classified entries.
func bundleEntryNav(entries []docsrender.ClassifiedEntry, basePath string) []navOption {
	opts := make([]navOption, 0, len(entries))
	for _, e := range entries {
		opts = append(opts, navOption{label: "🔗 " + e.Title, path: basePath + "/" + e.Key})
	}
	return opts
}

// BundleNavItems generates a flat list of navigation items from bundle keys.
func BundleNavItems(bundle *docsrender.CLIDocsBundle, modulePrefix string, pkgName string) []navOption {
	basePath := fmt.Sprintf("registry/packages/%s/api-docs", pkgName)
	ck := docsrender.ClassifyBundleKeys(bundle, modulePrefix)

	opts := make([]navOption, 0, len(ck.SubModules)+len(ck.Resources)+len(ck.Functions))
	opts = append(opts, bundleModuleNav(ck, basePath, modulePrefix)...)
	opts = append(opts, bundleEntryNav(ck.Resources, basePath)...)
	opts = append(opts, bundleEntryNav(ck.Functions, basePath)...)
	return opts
}

// BundleSectionNav returns per-section nav items for drilling into modules, resources, and functions.
func BundleSectionNav(bundle *docsrender.CLIDocsBundle, modulePrefix string, pkgName string) map[string][]navOption {
	basePath := fmt.Sprintf("registry/packages/%s/api-docs", pkgName)
	ck := docsrender.ClassifyBundleKeys(bundle, modulePrefix)

	result := map[string][]navOption{}
	if len(ck.SubModules) > 0 {
		result[docsrender.SectionModules] = bundleModuleNav(ck, basePath, modulePrefix)
	}
	if len(ck.Resources) > 0 {
		result[docsrender.SectionResources] = bundleEntryNav(ck.Resources, basePath)
	}
	if len(ck.Functions) > 0 {
		result[docsrender.SectionFunctions] = bundleEntryNav(ck.Functions, basePath)
	}
	return result
}
