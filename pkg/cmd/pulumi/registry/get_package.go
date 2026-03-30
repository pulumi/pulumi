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

package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/blang/semver"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	cmdcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemarender"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	commonregistry "github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/spf13/cobra"
)

type packageInfoJSON struct {
	Name              string   `json:"name"`
	Publisher         string   `json:"publisher"`
	Source            string   `json:"source"`
	Version           string   `json:"version"`
	Title             string   `json:"title,omitempty"`
	Description       string   `json:"description,omitempty"`
	Category          string   `json:"category,omitempty"`
	LogoURL           string   `json:"logoURL,omitempty"`
	RepositoryURL     string   `json:"repositoryURL,omitempty"`
	IsFeatured        bool     `json:"isFeatured"`
	Types             []string `json:"types,omitempty"`
	Status            string   `json:"status"`
	SchemaURL         string   `json:"schemaURL,omitempty"`
	PluginDownloadURL string   `json:"pluginDownloadURL,omitempty"`
	Visibility        string   `json:"visibility"`
	CreatedAt         string   `json:"createdAt"`
}

func newRegistryPackageGetCmd() *cobra.Command {
	var jsonOut bool
	var versionStr string
	var showSchema bool

	cmd := &cobra.Command{
		Use:   "get <package>",
		Short: "Get metadata about a package",
		Long: `Get detailed metadata about a specific package in the Pulumi Registry.

The package can be specified as <name>, <publisher>/<name>, or <source>/<publisher>/<name>.
Use --schema to output the full package schema JSON.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			reg := cmdcmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())

			var version *semver.Version
			if versionStr != "" {
				v, err := semver.Parse(versionStr)
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", versionStr, err)
				}
				version = &v
			}

			if showSchema {
				spec, err := loadSchemaForPackage(ctx, reg, args[0], version)
				if err != nil {
					return err
				}
				return ui.PrintJSON(spec)
			}

			meta, err := commonregistry.ResolvePackageFromName(ctx, reg, args[0], version)
			if err != nil {
				return fmt.Errorf("could not resolve package %q: %w", args[0], err)
			}

			if jsonOut {
				return formatPackageInfoJSON(meta)
			}
			return formatPackageInfoConsole(meta)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package", Usage: "<name|publisher/name|source/publisher/name>"},
		},
		Required: 1,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific version to fetch")
	cmd.PersistentFlags().BoolVar(&showSchema, "schema", false, "Output the full package schema JSON")

	return cmd
}

func formatPackageInfoJSON(meta apitype.PackageMetadata) error {
	var types []string
	for _, t := range meta.PackageTypes {
		types = append(types, string(t))
	}
	info := packageInfoJSON{
		Name:              meta.Name,
		Publisher:         meta.Publisher,
		Source:            meta.Source,
		Version:           meta.Version.String(),
		Title:             meta.Title,
		Description:       meta.Description,
		Category:          meta.Category,
		LogoURL:           meta.LogoURL,
		RepositoryURL:     meta.RepoURL,
		IsFeatured:        meta.IsFeatured,
		Types:             types,
		Status:            meta.PackageStatus.String(),
		SchemaURL:         meta.SchemaURL,
		PluginDownloadURL: meta.PluginDownloadURL,
		Visibility:        meta.Visibility.String(),
		CreatedAt:         cmdcmd.FormatTime(meta.CreatedAt),
	}
	return ui.PrintJSON(info)
}

func formatPackageInfoConsole(meta apitype.PackageMetadata, inline ...bool) error {
	var b strings.Builder

	// Header: Title (name) or just name.
	if meta.Title != "" {
		fmt.Fprintf(&b, "# %s (%s)\n\n", meta.Title, meta.Name)
	} else {
		fmt.Fprintf(&b, "# %s\n\n", meta.Name)
	}

	// Description.
	if meta.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", schemarender.SummaryFromDescription(meta.Description))
	}

	// Metadata table.
	b.WriteString("| | |\n")
	b.WriteString("|---|---|\n")
	fmt.Fprintf(&b, "| **Publisher** | %s |\n", meta.Publisher)
	fmt.Fprintf(&b, "| **Version** | %s |\n", meta.Version.String())
	if meta.Category != "" {
		fmt.Fprintf(&b, "| **Category** | %s |\n", meta.Category)
	}
	if len(meta.PackageTypes) > 0 {
		var types []string
		for _, t := range meta.PackageTypes {
			types = append(types, string(t))
		}
		fmt.Fprintf(&b, "| **Types** | %s |\n", strings.Join(types, ", "))
	}
	fmt.Fprintf(&b, "| **Status** | %s |\n", meta.PackageStatus.String())
	fmt.Fprintf(&b, "| **Visibility** | %s |\n", meta.Visibility.String())
	if meta.IsFeatured {
		b.WriteString("| **Featured** | yes |\n")
	}
	if meta.RepoURL != "" {
		fmt.Fprintf(&b, "| **Repository** | %s |\n", meta.RepoURL)
	}
	if meta.SchemaURL != "" {
		fmt.Fprintf(&b, "| **Schema URL** | %s |\n", meta.SchemaURL)
	}
	if !meta.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "| **Created At** | %s |\n", cmdcmd.FormatTime(meta.CreatedAt))
	}

	isInline := len(inline) > 0 && inline[0]

	if !isInline {
		// Non-interactive / direct invocation: show quick links as text.
		b.WriteString("\n## Quick Links\n\n")
		fmt.Fprintf(&b, "- `pulumi registry resource ls %s` — browse resources\n", meta.Name)
		fmt.Fprintf(&b, "- `pulumi registry package config %s` — configuration reference\n", meta.Name)
		fmt.Fprintf(&b, "- `pulumi registry package install-guide %s` — setup guide\n", meta.Name)
		fmt.Fprintf(&b, "- `pulumi registry example ls %s` — code examples\n", meta.Name)
		return ui.RenderMarkdown(b.String())
	}

	// Inline: just render without pager.
	return ui.RenderMarkdownInline(b.String())
}

// buildPackageOverviewMarkdown generates the rendered markdown for the package overview.
func buildPackageOverviewMarkdown(meta apitype.PackageMetadata) string {
	var b strings.Builder

	if meta.Title != "" {
		fmt.Fprintf(&b, "# %s (%s)\n\n", meta.Title, meta.Name)
	} else {
		fmt.Fprintf(&b, "# %s\n\n", meta.Name)
	}

	if meta.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", schemarender.SummaryFromDescription(meta.Description))
	}

	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| **Publisher** | %s |\n", meta.Publisher)
	fmt.Fprintf(&b, "| **Version** | %s |\n", meta.Version.String())
	if meta.Category != "" {
		fmt.Fprintf(&b, "| **Category** | %s |\n", meta.Category)
	}
	if len(meta.PackageTypes) > 0 {
		var types []string
		for _, t := range meta.PackageTypes {
			types = append(types, string(t))
		}
		fmt.Fprintf(&b, "| **Kind** | %s |\n", strings.Join(types, ", "))
	}
	fmt.Fprintf(&b, "| **Status** | %s |\n", meta.PackageStatus.String())
	if meta.RepoURL != "" {
		fmt.Fprintf(&b, "| **Repository** | %s |\n", meta.RepoURL)
	}

	// Render with glamour.
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return b.String()
	}
	rendered, err := renderer.Render(b.String())
	if err != nil {
		return b.String()
	}
	return rendered
}

// browsePackage shows package info and lets the user navigate into sub-views,
// then back to the package overview. Returns when the user quits.
func browsePackage(ctx context.Context, reg commonregistry.Registry, meta apitype.PackageMetadata) error {
	// Build the package overview markdown (without quick links).
	overview := buildPackageOverviewMarkdown(meta)

	for {
		// Show package overview + navigation menu together in alt-screen.
		items := []registryItem{
			{title: "Resources", value: 0},
			{title: "Functions", value: 1},
			{title: "Installation", value: 2},
			{title: "Configuration", value: 3},
			{title: "Examples", value: 4},
		}
		idx, err := runInteractiveListWithBanner(overview, items)
		if err != nil || idx < 0 {
			return err // user quit
		}

		// Clear screen and show loading indicator while fetching data.
		fmt.Print("\033[2J\033[H\033[36m⏳ Loading...\033[0m\n")

		switch idx {
		case 0: // resources
			spec, err := loadSchemaForPackage(ctx, reg, meta.Name, nil)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			browseResources(ctx, spec, meta.Name)
		case 1: // functions
			spec, err := loadSchemaForPackage(ctx, reg, meta.Name, nil)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			browseFunctions(ctx, spec, meta.Name)
		case 2: // install guide
			content, err := fetchInstallGuideMarkdown(ctx, meta.Name)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			showInViewport(content)
		case 3: // config
			spec, err := loadSchemaForPackage(ctx, reg, meta.Name, nil)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			content := buildConfigMarkdown(spec)
			showInViewport(content)
		case 4: // examples
			examples, err := resolveExamples(ctx, reg, meta.Name, nil)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			browseExamples(ctx, examples, meta.Name)
		}
		// Loop back to package overview.
	}
}

func browseResources(_ context.Context, spec *schema.PackageSpec, pkgName string) {
	items, err := collectResources(spec, "")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if len(items) == 0 {
		fmt.Println("No resources found")
		return
	}

	listItems := make([]registryItem, len(items))
	for i, item := range items {
		parts := strings.Split(item.Token, ":")
		listItems[i] = registryItem{
			title:      fmt.Sprintf("%-12s %-40s %s", parts[0], parts[1], parts[2]),
			filterText: parts[1] + " " + parts[2],
			value:      i,
		}
	}

	for {
		idx, err := runInteractiveList(
			fmt.Sprintf("%d resources in %s", len(items), pkgName),
			fmt.Sprintf("%-12s %-40s %s", "PACKAGE", "MODULE", "TYPE"),
			listItems)
		if err != nil || idx < 0 {
			return // user quit — back to package menu
		}
		token := items[idx].Token
		resolvedToken, res, err := findResource(spec, token)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		examples := parseExamples(res.Description)
		if len(examples) > 0 {
			showResourceWithTabs(spec, resolvedToken, res, examples[0])
		} else {
			content := buildResourceDetailMarkdown(spec, resolvedToken, res)
			showInViewport(content)
		}
		// Loop back to the resource list.
	}
}

func browseFunctions(_ context.Context, spec *schema.PackageSpec, pkgName string) {
	items, err := collectFunctions(spec, "")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if len(items) == 0 {
		fmt.Println("No functions found")
		return
	}

	listItems := make([]registryItem, len(items))
	for i, item := range items {
		parts := strings.Split(item.Token, ":")
		listItems[i] = registryItem{
			title:      fmt.Sprintf("%-12s %-40s %s", parts[0], parts[1], parts[2]),
			filterText: parts[1] + " " + parts[2],
			value:      i,
		}
	}

	for {
		idx, err := runInteractiveList(
			fmt.Sprintf("%d functions in %s", len(items), pkgName),
			fmt.Sprintf("%-12s %-40s %s", "PACKAGE", "MODULE", "FUNCTION"),
			listItems)
		if err != nil || idx < 0 {
			return
		}
		token := items[idx].Token
		resolvedToken, fn, err := findFunction(spec, token)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		content := buildFunctionDetailMarkdown(spec, resolvedToken, fn)
		showInViewport(content)
	}
}

func browseExamples(_ context.Context, examples []parsedExample, target string) {
	if len(examples) == 0 {
		fmt.Println("No code examples found")
		return
	}
	listItems := make([]registryItem, len(examples))
	for i, ex := range examples {
		listItems[i] = registryItem{
			title:      fmt.Sprintf("%-60s  %s", ex.title, strings.Join(ex.languages, ", ")),
			filterText: ex.title,
			value:      i,
		}
	}

	for {
		idx, err := runInteractiveList(
			fmt.Sprintf("%d examples", len(examples)),
			fmt.Sprintf("%-60s  %s", "TITLE", "LANGUAGES"),
			listItems)
		if err != nil || idx < 0 {
			return
		}
		ex := examples[idx]
		showTabbedCode(ex.title, ex.languages, ex.codeByLang)
	}
}

// showResourceWithTabs shows resource detail with a language-switchable example code section.
func showResourceWithTabs(spec *schema.PackageSpec, token string, res schema.ResourceSpec, example parsedExample) {
	m := resourceDetailModel{
		spec:      spec,
		token:     token,
		res:       res,
		example:   example,
		languages: example.languages,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, _ = p.Run()
}

type resourceDetailModel struct {
	spec      *schema.PackageSpec
	token     string
	res       schema.ResourceSpec
	example   parsedExample
	languages []string
	activeLang int
	viewport  viewport.Model
	ready     bool
}

func (m resourceDetailModel) Init() tea.Cmd { return nil }

func (m resourceDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "tab", "right":
			if m.activeLang < len(m.languages)-1 {
				m.activeLang++
				m.rebuildContent()
			}
		case "shift+tab", "left":
			if m.activeLang > 0 {
				m.activeLang--
				m.rebuildContent()
			}
		}
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-2)
			m.rebuildContent()
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *resourceDetailModel) rebuildContent() {
	lang := m.languages[m.activeLang]
	md := buildResourceDetailMarkdownWithLang(m.spec, m.token, m.res, lang, m.example)

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(120),
		glamour.WithStylesFromJSONBytes([]byte(`{"document":{"margin":0}}`)),
	)
	var content string
	if err == nil {
		content, err = renderer.Render(md)
		if err != nil {
			content = md
		}
	} else {
		content = md
	}
	m.viewport.SetContent(content)
}

func (m resourceDetailModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Tab bar for language selection.
	var tabs strings.Builder
	for i, lang := range m.languages {
		if i == m.activeLang {
			tab := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("14")).
				Padding(0, 1).
				Render(lang)
			tabs.WriteString(tab)
		} else {
			tab := lipgloss.NewStyle().
				Foreground(lipgloss.Color("243")).
				Padding(0, 1).
				Render(lang)
			tabs.WriteString(tab)
		}
		tabs.WriteString(" ")
	}

	help := countStyle.Render("  ←/→ language • ↑/↓ scroll • q back")
	return tabs.String() + "\n" + m.viewport.View() + "\n" + help
}

// buildResourceDetailMarkdownWithLang builds the resource detail markdown with
// the example code in a specific language.
func buildResourceDetailMarkdownWithLang(
	spec *schema.PackageSpec, token string, res schema.ResourceSpec,
	lang string, example parsedExample,
) string {
	var md strings.Builder
	fmt.Fprintf(&md, "# %s\n\n", token)
	if res.IsComponent {
		md.WriteString("**Component resource**\n\n")
	}
	if res.Description != "" {
		desc := descriptionBeforeExamples(res.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}
	}

	// Example in the selected language.
	if code, ok := example.codeByLang[lang]; ok {
		fmt.Fprintf(&md, "## Example Usage\n\n```%s\n%s\n```\n\n", lang, code)
	}

	if len(res.InputProperties) > 0 {
		md.WriteString("## Inputs\n\n| Name | Type | Required | Description |\n|------|------|----------|-------------|\n")
		for _, name := range maputil.SortedKeys(res.InputProperties) {
			prop := res.InputProperties[name]
			typ, _ := schemarender.GetType(spec, prop.TypeSpec)
			req := ""
			if slices.Contains(res.RequiredInputs, name) {
				req = "yes"
			}
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, req, truncateDesc(prop.Description, 50))
		}
		md.WriteString("\n")
	}
	if len(res.Properties) > 0 {
		md.WriteString("## Outputs\n\n| Name | Type | Always | Description |\n|------|------|--------|-------------|\n")
		for _, name := range maputil.SortedKeys(res.Properties) {
			prop := res.Properties[name]
			typ, _ := schemarender.GetType(spec, prop.TypeSpec)
			always := ""
			if slices.Contains(res.Required, name) {
				always = "yes"
			}
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, always, truncateDesc(prop.Description, 50))
		}
		md.WriteString("\n")
	}
	return md.String()
}

func fetchInstallGuideMarkdown(ctx context.Context, name string) (string, error) {
	url := fmt.Sprintf(
		"https://raw.githubusercontent.com/pulumi/registry/master/themes/default/content/registry/packages/%s/installation-configuration.md",
		name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "pulumi-cli")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Sprintf("No installation guide found for %q\n", name), nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return stripFrontmatter(string(body)), nil
}

func buildConfigMarkdown(spec *schema.PackageSpec) string {
	displayName := spec.DisplayName
	if displayName == "" {
		displayName = spec.Name
	}

	var md strings.Builder
	fmt.Fprintf(&md, "# %s Provider Configuration\n\n", displayName)
	fmt.Fprintf(&md, "Set configuration with: `pulumi config set %s:<key> <value>`\n\n", spec.Name)

	for _, name := range maputil.SortedKeys(spec.Config.Variables) {
		prop := spec.Config.Variables[name]
		typ, err := schemarender.GetType(spec, prop.TypeSpec)
		if err != nil {
			typ = "unknown"
		}
		badges := ""
		if slices.Contains(spec.Config.Required, name) {
			badges += " **required**"
		}
		if prop.Secret {
			badges += " **secret**"
		}
		fmt.Fprintf(&md, "### `%s` (`%s`)%s\n\n", name, typ, badges)
		desc := schemarender.SummaryFromDescription(prop.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}
	}
	return md.String()
}

func buildResourceDetailMarkdown(spec *schema.PackageSpec, token string, res schema.ResourceSpec) string {
	var md strings.Builder
	fmt.Fprintf(&md, "# %s\n\n", token)
	if res.IsComponent {
		md.WriteString("**Component resource**\n\n")
	}
	if res.Description != "" {
		desc := descriptionBeforeExamples(res.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}
		code := firstExampleCode(res.Description, "typescript")
		if code != "" {
			fmt.Fprintf(&md, "## Example Usage\n\n```typescript\n%s\n```\n\n", code)
		}
	}
	if len(res.InputProperties) > 0 {
		md.WriteString("## Inputs\n\n| Name | Type | Required | Description |\n|------|------|----------|-------------|\n")
		for _, name := range maputil.SortedKeys(res.InputProperties) {
			prop := res.InputProperties[name]
			typ, _ := schemarender.GetType(spec, prop.TypeSpec)
			req := ""
			if slices.Contains(res.RequiredInputs, name) {
				req = "yes"
			}
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, req, truncateDesc(prop.Description, 50))
		}
		md.WriteString("\n")
	}
	if len(res.Properties) > 0 {
		md.WriteString("## Outputs\n\n| Name | Type | Always | Description |\n|------|------|--------|-------------|\n")
		for _, name := range maputil.SortedKeys(res.Properties) {
			prop := res.Properties[name]
			typ, _ := schemarender.GetType(spec, prop.TypeSpec)
			always := ""
			if slices.Contains(res.Required, name) {
				always = "yes"
			}
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, always, truncateDesc(prop.Description, 50))
		}
		md.WriteString("\n")
	}
	return md.String()
}

func buildFunctionDetailMarkdown(spec *schema.PackageSpec, token string, fn schema.FunctionSpec) string {
	var md strings.Builder
	fmt.Fprintf(&md, "# %s\n\n", token)
	if fn.Description != "" {
		desc := descriptionBeforeExamples(fn.Description)
		if desc != "" {
			fmt.Fprintf(&md, "%s\n\n", desc)
		}
		code := firstExampleCode(fn.Description, "typescript")
		if code != "" {
			fmt.Fprintf(&md, "## Example Usage\n\n```typescript\n%s\n```\n\n", code)
		}
	}
	if fn.Inputs != nil && len(fn.Inputs.Properties) > 0 {
		md.WriteString("## Inputs\n\n| Name | Type | Required | Description |\n|------|------|----------|-------------|\n")
		for _, name := range maputil.SortedKeys(fn.Inputs.Properties) {
			prop := fn.Inputs.Properties[name]
			typ, _ := schemarender.GetType(spec, prop.TypeSpec)
			req := ""
			if slices.Contains(fn.Inputs.Required, name) {
				req = "yes"
			}
			fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, req, truncateDesc(prop.Description, 50))
		}
		md.WriteString("\n")
	}
	returnType := fn.ReturnType
	if returnType == nil && fn.Outputs != nil {
		returnType = &schema.ReturnTypeSpec{ObjectTypeSpec: fn.Outputs}
	}
	if returnType != nil && returnType.ObjectTypeSpec != nil {
		obj := returnType.ObjectTypeSpec
		if len(obj.Properties) > 0 {
			md.WriteString("## Outputs\n\n| Name | Type | Always | Description |\n|------|------|--------|-------------|\n")
			for _, name := range maputil.SortedKeys(obj.Properties) {
				prop := obj.Properties[name]
				typ, _ := schemarender.GetType(spec, prop.TypeSpec)
				always := ""
				if slices.Contains(obj.Required, name) {
					always = "yes"
				}
				fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", name, typ, always, truncateDesc(prop.Description, 50))
			}
			md.WriteString("\n")
		}
	} else if returnType != nil && returnType.TypeSpec != nil {
		typ, _ := schemarender.GetType(spec, *returnType.TypeSpec)
		fmt.Fprintf(&md, "## Outputs\n\nReturns: `%s`\n\n", typ)
	}
	return md.String()
}
