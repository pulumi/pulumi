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
	"strings"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
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
}

// NewDocsCmd creates the `pulumi docs` command.
func NewDocsCmd() *cobra.Command {
	dc := &docsCmd{}

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "View Pulumi documentation in the terminal",
		Long: "Read and browse Pulumi documentation in the terminal.\n\n" +
			"  pulumi docs                    Show help\n" +
			"  pulumi docs read <path>        Read a specific page\n" +
			"  pulumi docs registry <pkg>     Read a registry package\n" +
			"  pulumi docs search <query>     Search documentation\n" +
			"  pulumi docs sitemap            List available pages",
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
				prefs, _ := LoadPreferences()
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

// baseURLForPath returns the appropriate base URL for the given content path.
func (dc *docsCmd) baseURLForPath(path string) string {
	if isRegistryPath(path) {
		return dc.registryBaseURL
	}
	return dc.baseURL
}

// fetchAndRender is the main pipeline: fetch → parse → resolve choosers → render → print.
func (dc *docsCmd) fetchAndRender(path string) error {
	// Split path#section
	section := ""
	if idx := strings.Index(path, "#"); idx >= 0 {
		section = path[idx+1:]
		path = path[:idx]
	}

	fetchBase := dc.baseURLForPath(path)

	body, title, err := FetchDoc(fetchBase, path)
	if err != nil {
		// Graceful fallback for registry pages that aren't available in the terminal
		var regErr *RegistryNotAvailableError
		if errors.As(err, &regErr) {
			return dc.handleRegistryFallback(path)
		}
		return err
	}

	// --toc: interactive section picker or plain list
	if dc.toc {
		if err := dc.showTOC(body, &section); err != nil {
			return err
		}
		// If no section was selected (user cancelled), we're done
		if section == "" {
			return nil
		}
	}

	// If a section was specified, extract just that section
	if section != "" {
		extracted := extractSection(body, section)
		if extracted == "" {
			return fmt.Errorf("section %q not found — use --toc to list sections", section)
		}
		body = extracted
		title = ""
	}

	// Raw mode: dump the markdown as fetched (with frontmatter already stripped)
	if dc.raw {
		if title != "" {
			fmt.Printf("# %s\n\n", title)
		}
		fmt.Print(body)
		return nil
	}

	rendered, err := dc.renderBody(body, title)
	if err != nil {
		return err
	}
	fmt.Print(rendered)

	// Save last viewed page for --toc and browse shortcuts
	prefs, _ := LoadPreferences()
	if section == "" {
		prefs.LastPage = path
		if err := prefs.Save(); err != nil {
			logging.V(7).Infof("failed to save docs preferences: %v", err)
		}
	}

	// Show navigation footer when viewing a full page
	if section == "" {
		fmt.Print(pageFooter(fetchBase, path))
	}

	return nil
}

// renderBody processes markdown through the chooser and rendering pipeline.
func (dc *docsCmd) renderBody(body, title string) (string, error) {
	prefs, err := LoadPreferences()
	if err != nil {
		prefs = &Preferences{}
	}

	interactive := cmdutil.Interactive()
	blocks := ParseChoosers(body)
	resolved := ResolveChoosers(blocks, prefs, dc.language, dc.osFlag, interactive)

	// Filter unwrapped code blocks by language (handles bundle content without chooser comments).
	lang := dc.language
	if lang == "" {
		lang = prefs.Language
	}
	resolved = FilterCodeBlocksByLanguage(resolved, lang)

	return RenderMarkdown(title, resolved)
}

// hrefToPath strips /docs/ or /registry/ prefix from an href, returning a clean path.
// For /registry/ hrefs, the returned path retains the "registry/" prefix.
// Query parameters and fragments are stripped.
func hrefToPath(href string) string {
	// Strip query parameters and fragments
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

// showTOC displays the table of contents for a page, either as an interactive
// picker or a plain list.
func (dc *docsCmd) showTOC(body string, section *string) error {
	headings := extractHeadings(body)
	if len(headings) == 0 {
		fmt.Println("No sections found.")
		return nil
	}

	if dc.tocJSON {
		type tocEntry struct {
			Title string `json:"title"`
			Slug  string `json:"slug"`
			Depth int    `json:"depth"`
		}
		entries := make([]tocEntry, len(headings))
		for i, h := range headings {
			entries[i] = tocEntry{Title: h.text, Slug: h.slug, Depth: h.level}
		}
		return ui.PrintJSON(entries)
	}

	if !cmdutil.Interactive() {
		for _, h := range headings {
			indent := strings.Repeat("  ", h.level-2)
			fmt.Printf("%s%s  (#%s)\n", indent, h.text, h.slug)
		}
		return nil
	}

	options := make([]string, len(headings))
	for i, h := range headings {
		indent := strings.Repeat("  ", h.level-2)
		options[i] = indent + h.text
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
	for i, opt := range options {
		if opt == selected {
			*section = headings[i].slug
			break
		}
	}
	return nil
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

// viewPage fetches and renders a page given its href path (e.g. /docs/iac/concepts/stacks/ or /registry/packages/aws/).
func (dc *docsCmd) viewPage(href string) error {
	path := hrefToPath(href)
	if path == "" {
		return fmt.Errorf("invalid page path: %s", href)
	}
	return dc.fetchAndRender(path)
}

// handleRegistryFallback provides a graceful fallback when a registry page (typically
// an API docs page) isn't available as markdown for terminal viewing.
func (dc *docsCmd) handleRegistryFallback(path string) error {
	pageWebURL := webURL(dc.registryBaseURL, path)

	// Check if this is an API docs page and extract the package overview path.
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
		optOverview := "View package overview"
		optBrowser := "Open in web browser"
		options := []string{optOverview, optBrowser}

		fmt.Fprintln(os.Stderr)
		selected := ui.PromptUser(
			"What would you like to do?",
			options,
			optOverview,
			cmdutil.GetGlobalColorization(),
		)

		switch selected {
		case optOverview:
			return dc.fetchAndRender(overviewPath)
		case optBrowser:
			fmt.Fprintf(os.Stderr, "Opening %s\n\n", pageWebURL)
			return browser.OpenURL(pageWebURL)
		}
		return nil
	}

	// Non-interactive or no overview available
	fmt.Fprintf(os.Stderr, "Visit: %s\n", pageWebURL)
	if overviewPath != "" {
		fmt.Fprintf(os.Stderr, "Or view the package overview: pulumi docs read %s\n", overviewPath)
	}
	fmt.Fprintln(os.Stderr)
	return nil
}
