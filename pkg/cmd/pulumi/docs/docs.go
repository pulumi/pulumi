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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/docsrender"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type docsCmd struct {
	baseURL         string
	registryBaseURL string
	language        string
	osFlag          string
	raw             bool
	toc             bool
	tocJSON         bool
	jsonOutput      bool
	fullPage        bool
	sectionsView    bool

	// session tracks chooser selections within a single CLI invocation
	// so the user isn't prompted repeatedly for the same chooser type.
	session map[string]string
}

// NewDocsCmd creates the `pulumi docs` command.
func NewDocsCmd() *cobra.Command {
	dc := &docsCmd{session: map[string]string{}}

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "View Pulumi documentation in the terminal",
		Long: "Read and browse Pulumi documentation in the terminal.\n\n" +
			"  pulumi docs                    Browse interactively\n" +
			"  pulumi docs read <path>        Read a specific page\n" +
			"  pulumi docs browse [path]      Browse interactively\n" +
			"  pulumi docs registry <pkg>     Read a registry package\n" +
			"  pulumi docs search <query>     Search documentation\n" +
			"  pulumi docs sitemap            List available pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmdutil.Interactive() {
				return dc.browseLoop("")
			}
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVar(&dc.baseURL, "base-url", "https://www.pulumi.com",
		"Base URL for Pulumi documentation")
	cmd.PersistentFlags().StringVar(&dc.registryBaseURL, "registry-base-url", "https://www.pulumi.com",
		"Base URL for Pulumi registry")
	cmd.PersistentFlags().MarkHidden("base-url")          //nolint:errcheck
	cmd.PersistentFlags().MarkHidden("registry-base-url") //nolint:errcheck
	cmd.PersistentFlags().StringVar(&dc.language, "language", "",
		"Filter code examples in docs by language (e.g., python, typescript, go); choice is remembered")
	cmd.PersistentFlags().StringVar(&dc.osFlag, "os", "",
		"Filter OS-specific content in docs (e.g., macos, linux, windows); choice is remembered")

	cmd.AddCommand(dc.newReadCmd())
	cmd.AddCommand(dc.newBrowseCmd())
	cmd.AddCommand(dc.newSearchCmd())
	cmd.AddCommand(dc.newRegistryCmd())
	cmd.AddCommand(dc.newSitemapCmd())

	return cmd
}

func (dc *docsCmd) newReadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <path[#section]>",
		Short: "Read a documentation page",
		Long: "Fetch and display a Pulumi documentation page.\n\n" +
			"  pulumi docs read iac/concepts/stacks               Read a docs page\n" +
			"  pulumi docs read iac/concepts/stacks#stack-tags    Jump to a section\n" +
			"  pulumi docs read registry/packages/aws             Read a registry page\n" +
			"  pulumi docs read --toc                             Show sections on last viewed page",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dc.tocJSON {
				dc.toc = true
			}
			if dc.toc && len(args) == 0 {
				prefs := docsrender.LoadPreferences()
				if prefs.LastPage == "" {
					return errors.New("no page specified and no previously viewed page — provide a path")
				}
				return dc.fetchAndRender(prefs.LastPage)
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			return dc.fetchAndRender(args[0])
		},
	}
	cmd.Flags().BoolVar(&dc.raw, "raw", false,
		"Output raw markdown without formatting or chooser resolution")
	cmd.Flags().BoolVar(&dc.toc, "toc", false,
		"Show table of contents (list of sections)")
	cmd.Flags().BoolVar(&dc.tocJSON, "toc-json", false,
		"Output table of contents as JSON")
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "path", Usage: "path[#section]"}},
	})
	return cmd
}

func (dc *docsCmd) newBrowseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browse [path]",
		Short: "Browse Pulumi documentation interactively",
		Long: "Browse docs and registry by following links between pages.\n\n" +
			"  pulumi docs browse             Browse from last viewed page or root\n" +
			"  pulumi docs browse <path>      Start browsing at a specific page\n" +
			"  pulumi docs browse /           Browse from the site map root\n" +
			"  pulumi docs browse registry/   Browse all registry packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) > 0 && args[0] != "/" {
				path = args[0]
			} else if len(args) == 0 {
				prefs := docsrender.LoadPreferences()
				path = prefs.LastPage
			}
			return dc.browseLoop(path)
		},
	}
	cmd.Flags().BoolVar(&dc.fullPage, "full", false,
		"Show full page instead of sections view (saved as default when used)")
	cmd.Flags().BoolVar(&dc.sectionsView, "sections", false,
		"Show sections view instead of full page (saved as default when used)")
	cmd.MarkFlagsMutuallyExclusive("full", "sections")
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "path"}},
	})
	return cmd
}

func (dc *docsCmd) newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>...",
		Short: "Search Pulumi documentation",
		Long:  "Search Pulumi documentation using Algolia and display results.",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runSearch(dc, query)
		},
	}
	cmd.Flags().BoolVar(&dc.jsonOutput, "json", false,
		"Output results as JSON")
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "query"}},
		Required:  1,
		Variadic:  true,
	})
	return cmd
}

func (dc *docsCmd) baseURLForPath(path string) string {
	if docsrender.IsRegistryPath(path) {
		return dc.registryBaseURL
	}
	return dc.baseURL
}

// fetchAndRender is the main pipeline: fetch → parse → resolve choosers → render → print.
func (dc *docsCmd) fetchAndRender(path string) error {
	section := ""
	if idx := strings.Index(path, "#"); idx >= 0 {
		section = path[idx+1:]
		path = path[:idx]
	}

	fetchBase := dc.baseURLForPath(path)

	var body, title string
	var err error
	var bundle *docsrender.CLIDocsBundle
	if docsrender.IsAPIDocsPath(path) {
		pkgName, docKey, ok := docsrender.ParseAPIDocsPath(path)
		if ok {
			bundle, _ = docsrender.FetchCLIDocsBundle(fetchBase, pkgName)
			if bundle != nil && docKey != "" {
				if b, t, found := docsrender.LookupBundleDoc(bundle, docKey); found {
					body, title = b, t
				}
			}
		}
	}

	if body == "" {
		body, title, err = docsrender.FetchDoc(fetchBase, path)
	}
	if err != nil {
		var regErr *docsrender.RegistryNotAvailableError
		if errors.As(err, &regErr) {
			return dc.handleRegistryFallback(path)
		}
		return err
	}

	// For API docs pages with a bundle, build the page from bundle data
	// instead of using the raw index.md (which has poorly formatted content).
	if bundle != nil {
		if _, docKey, ok := docsrender.ParseAPIDocsPath(path); ok {
			intro := docsrender.GetIntro(body)
			body = docsrender.BuildAPIDocsPage(bundle, docKey, intro)
		}
	} else if docsrender.IsRegistryPath(path) {
		body = docsrender.FormatPackageDetails(body)
	}

	if dc.toc {
		if err := dc.showTOC(body, &section); err != nil {
			return err
		}
		if section == "" {
			return nil
		}
	}

	if section == docsrender.SectionIntroduction {
		body = docsrender.GetIntro(body)
	} else if section != "" {
		extracted := docsrender.GetSection(body, section)
		if extracted == "" {
			return fmt.Errorf("section %q not found — use --toc to list sections", section)
		}
		body = extracted
		title = ""
	}

	if dc.raw {
		if title != "" {
			fmt.Printf("# %s\n\n", title)
		}
		fmt.Print(body)
		return nil
	}

	// For API docs pages with a bundle and a Modules section, render the
	// module list as a column table instead of going through glamour.
	if bundle != nil && strings.Contains(body, "## "+docsrender.SectionModules) {
		if _, docKey, ok := docsrender.ParseAPIDocsPath(path); ok {
			modulesTable := docsrender.RenderBundleSingleSection(bundle, docKey, docsrender.SectionModules)
			if modulesTable != "" {
				dc.renderWithModulesTable(body, title, modulesTable)
				if section == "" {
					prefs := docsrender.LoadPreferences()
					prefs.LastPage = path
					docsrender.SavePreferences(prefs)
					fmt.Print(docsrender.PageFooter(fetchBase, path))
				}
				return nil
			}
		}
	}

	rendered, err := dc.renderBody(body, title)
	if err != nil {
		return err
	}
	fmt.Print(rendered)

	if section == "" {
		prefs := docsrender.LoadPreferences()
		prefs.LastPage = path
		docsrender.SavePreferences(prefs)

		fmt.Print(docsrender.PageFooter(fetchBase, path))
	}

	return nil
}

// renderWithModulesTable splits the body at the Modules heading, renders the
// parts before and after through glamour, and prints the modules column table directly.
func (dc *docsCmd) renderWithModulesTable(body, title, modulesTable string) {
	heading := "## " + docsrender.SectionModules
	start, _, endIdx := docsrender.FindSectionBounds(body, heading)
	if start < 0 {
		return
	}

	before := body[:start]
	after := body[endIdx:]

	if strings.TrimSpace(before) != "" {
		rendered, err := dc.renderBody(before, title)
		if err == nil {
			fmt.Print(rendered)
		}
	}

	docsrender.PrintHeadingWithTable(docsrender.SectionModules, modulesTable)

	if strings.TrimSpace(after) != "" {
		rendered, err := dc.renderBody(after, "")
		if err == nil {
			fmt.Print(rendered)
		}
	}
}

// renderBody processes markdown through the chooser and rendering pipeline
// using the docsrender library for AST-based chooser resolution and code block filtering.
func (dc *docsCmd) renderBody(body, title string) (string, error) {
	prefs := docsrender.LoadPreferences()

	// Build selections and resolve choosers via goldmark AST.
	selections := buildChooserSelections(body, prefs, dc.language, dc.osFlag, dc.session)
	source := []byte(body)
	tree := docsrender.ParseMarkdown(source)
	resolved := docsrender.ResolveChoosers(source, tree, selections)

	// Filter code blocks by language via goldmark AST.
	lang := dc.language
	if lang == "" {
		lang = prefs.Language
	}
	tree2 := docsrender.ParseMarkdown(resolved)
	filtered := docsrender.FilterCodeBlocksByLanguage(resolved, tree2, lang)

	return docsrender.RenderMarkdown(title, string(filtered))
}

func (dc *docsCmd) showTOC(body string, section *string) error {
	headings := docsrender.GetHeadings(body)
	if len(headings) == 0 {
		fmt.Println("No sections found.")
		return nil
	}

	hasIntro := !docsrender.IntroContainsFirstHeading(body)

	if dc.tocJSON {
		type tocEntry struct {
			Title string `json:"title"`
			Slug  string `json:"slug"`
			Depth int    `json:"depth"`
		}
		var entries []tocEntry
		if hasIntro {
			entries = append(entries, tocEntry{Title: "Introduction", Slug: docsrender.SectionIntroduction, Depth: 2})
		}
		for _, h := range headings {
			entries = append(entries, tocEntry{Title: h.Title, Slug: h.Slug, Depth: h.Level})
		}
		return ui.PrintJSON(entries)
	}

	if !cmdutil.Interactive() {
		if hasIntro {
			fmt.Println("Introduction  (#introduction)")
		}
		for _, h := range headings {
			indent := strings.Repeat("  ", h.Level-2)
			fmt.Printf("%s%s  (#%s)\n", indent, h.Title, h.Slug)
		}
		return nil
	}

	var options []string
	if hasIntro {
		options = append(options, "Introduction")
	}
	for _, h := range headings {
		options = append(options, h.Title)
	}
	selected := ui.PromptUser(
		"Select a section:",
		options,
		options[0],
		cmdutil.GetGlobalColorization(),
	)
	if selected == "" {
		return nil
	}
	if selected == "Introduction" {
		*section = docsrender.SectionIntroduction
		return nil
	}
	for _, h := range headings {
		if h.Title == selected {
			*section = h.Slug
			break
		}
	}
	return nil
}

func (dc *docsCmd) viewPage(href string) error {
	path := docsrender.HrefToPath(href)
	if path == "" {
		return fmt.Errorf("invalid page path: %s", href)
	}
	return dc.fetchAndRender(path)
}

func (dc *docsCmd) handleRegistryFallback(path string) error {
	pageWebURL := docsrender.WebURL(dc.registryBaseURL, path)

	var overviewPath string
	trimmed := strings.Trim(path, "/")
	if idx := strings.Index(trimmed, "/api-docs"); idx >= 0 {
		overviewPath = trimmed[:idx]
	}

	fmt.Fprintln(os.Stderr)
	if overviewPath != "" {
		fmt.Fprintln(os.Stderr, "Registry API docs are not available for terminal viewing.")
	} else {
		fmt.Fprintln(os.Stderr, "This registry page is not available for terminal viewing.")
	}

	if cmdutil.Interactive() && overviewPath != "" {
		optBrowseAPI := "Browse API docs"
		optOverview := "View package overview"
		optBrowser := "Open in web browser"
		options := []string{optBrowseAPI, optOverview, optBrowser}

		fmt.Fprintln(os.Stderr)
		selected := ui.PromptUser(
			"What would you like to do?",
			options,
			optBrowseAPI,
			cmdutil.GetGlobalColorization(),
		)

		switch selected {
		case optBrowseAPI:
			return dc.browseLoop(overviewPath + "/api-docs")
		case optOverview:
			return dc.fetchAndRender(overviewPath)
		case optBrowser:
			fmt.Fprintf(os.Stderr, "Opening %s\n\n", pageWebURL)
			return browser.OpenURL(pageWebURL)
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "Visit: %s\n", pageWebURL)
	if overviewPath != "" {
		fmt.Fprintf(os.Stderr, "Or view the package overview: pulumi docs read %s\n", overviewPath)
	}
	fmt.Fprintln(os.Stderr)
	return nil
}
