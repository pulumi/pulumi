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

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type docsCmd struct {
	baseURL  string
	language string
	osFlag   string
	raw      bool
	toc      bool
}

// NewDocsCmd creates the `pulumi docs` command.
func NewDocsCmd() *cobra.Command {
	dc := &docsCmd{}

	cmd := &cobra.Command{
		Use:   "docs [path[#section]]",
		Short: "View Pulumi documentation in the terminal",
		Long: "Fetch and display Pulumi documentation pages with formatted " +
			"terminal output.\n\nUse #section to jump to a specific section:\n" +
			"  pulumi docs iac/concepts/stacks#stack-tags\n\n" +
			"Use --toc to list available sections.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// --toc with no path: use the last viewed page
			if dc.toc && len(args) == 0 {
				prefs, _ := LoadPreferences()
				if prefs.LastPage == "" {
					return fmt.Errorf("no page specified and no previously viewed page — provide a path")
				}
				return dc.fetchAndRender(prefs.LastPage)
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			return dc.fetchAndRender(args[0])
		},
	}

	dc.baseURL = "https://www.pulumi.com"
	if envURL := os.Getenv("PULUMI_DOCS_BASE_URL"); envURL != "" {
		dc.baseURL = envURL
	}
	cmd.PersistentFlags().StringVar(&dc.language, "language", "",
		"Pre-select language for code examples (e.g., python, typescript, go)")
	cmd.PersistentFlags().StringVar(&dc.osFlag, "os", "",
		"Pre-select operating system (e.g., macos, linux, windows)")
	cmd.Flags().BoolVar(&dc.raw, "raw", false,
		"Output raw markdown without formatting or chooser resolution")
	cmd.Flags().BoolVar(&dc.toc, "toc", false,
		"Show table of contents (list of sections)")
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "path"}},
	})

	// Subcommands
	cmd.AddCommand(dc.newSearchCmd())
	cmd.AddCommand(dc.newBrowseCmd())

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
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "query"}},
		Required:  1,
		Variadic:  true,
	})
	return cmd
}

func (dc *docsCmd) newBrowseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browse [path]",
		Short: "Browse Pulumi documentation interactively",
		Long: "Browse docs by following links between pages.\n\n" +
			"  pulumi docs browse              Browse links on the last viewed page\n" +
			"  pulumi docs browse <path>        Browse links on a specific page\n" +
			"  pulumi docs browse /             Browse from the docs site map",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "/" {
				return dc.browseSitemap()
			}
			path := ""
			if len(args) > 0 {
				path = args[0]
			} else {
				prefs, _ := LoadPreferences()
				path = prefs.LastPage
			}
			if path == "" {
				return dc.browseSitemap()
			}
			return dc.browseLinks(path)
		},
	}
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "path"}},
	})
	return cmd
}

// fetchAndRender is the main pipeline: fetch → parse → resolve choosers → render → print.
func (dc *docsCmd) fetchAndRender(path string) error {
	// Split path#section
	section := ""
	if idx := strings.Index(path, "#"); idx >= 0 {
		section = path[idx+1:]
		path = path[:idx]
	}

	body, title, err := FetchDoc(dc.baseURL, path)
	if err != nil {
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
		_ = prefs.Save()
	}

	// Show navigation footer when viewing a full page
	if section == "" {
		fmt.Print(pageFooter(dc.baseURL, path))
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
	return RenderMarkdown(title, resolved)
}

// viewPage fetches and renders a page given its href path (e.g. /docs/iac/concepts/stacks/).
func (dc *docsCmd) viewPage(href string) error {
	path := strings.TrimPrefix(href, "/docs/")
	path = strings.Trim(path, "/")
	if path == "" {
		return fmt.Errorf("invalid page path: %s", href)
	}
	return dc.fetchAndRender(path)
}

// showTOC displays the table of contents for a page, either as an interactive
// picker or a plain list.
func (dc *docsCmd) showTOC(body string, section *string) error {
	headings := extractHeadings(body)
	if len(headings) == 0 {
		fmt.Println("No sections found.")
		return nil
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
