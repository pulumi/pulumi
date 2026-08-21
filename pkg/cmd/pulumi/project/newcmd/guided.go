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

package newcmd

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd/catalog"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

const optionBrowseAll = "Browse all templates"

type selectFunc func(message string, options []string, opts display.Options) (int, error)

var (
	errFallBackToFlatList = errors.New("fall back to the flat template list")
	errBackToProvider     = errors.New("return to the provider prompt")
)

func surveySelect(message string, options []string, opts display.Options) (int, error) {
	return ui.PromptUserIndexErr("\r"+message+"\n", options, opts.Color,
		survey.WithPageSize(cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)})))
}

func pick[T any](
	selector selectFunc, message string, opts display.Options, items []T, name func(T) string,
) (T, error) {
	options := make([]string, len(items))
	counts := make(map[string]int, len(items))
	for i, item := range items {
		label := name(item)
		counts[label]++
		if n := counts[label]; n > 1 {
			label = fmt.Sprintf("%s (%d)", label, n)
		}
		options[i] = label
	}
	i, err := selector(message, options, opts)
	if err != nil {
		var zero T
		return zero, err
	}
	return items[i], nil
}

type fetchTemplatesFunc func() ([]cmdTemplates.Template, error)

// project and database are fast enough to hold up the first prompt; fetchAll blocks on the VCS
// collections the service fetches upstream, so only a row that needs them calls it.
type guidedTemplates struct {
	project  []cmdTemplates.Template
	database []cmdTemplates.Template
	vcsOrgs  []string
	fetchAll fetchTemplatesFunc
}

type guided struct {
	guidedTemplates
	catalog  *catalog.Catalog[cmdTemplates.Template]
	opts     display.Options
	selector selectFunc
}

func newGuided(t guidedTemplates, opts display.Options, selector selectFunc) *guided {
	curated := make([]cmdTemplates.Template, 0, len(t.project))
	for _, template := range t.project {
		// A broken template can't back a provider/language row. Browse all still lists it, marked.
		if template.Error() != nil {
			continue
		}
		curated = append(curated, template)
	}
	return &guided{
		guidedTemplates: t,
		catalog:         catalog.New(curated, cmdTemplates.Template.Name),
		opts:            opts,
		selector:        selector,
	}
}

func (g *guided) backToProvider(format string, a ...any) error {
	fmt.Fprintf(g.opts.Stdout, format+"\n", a...)
	return errBackToProvider
}

func chooseGuidedFromSource(
	src templateSource, opts display.Options, selector selectFunc,
) (cmdTemplates.Template, error) {
	project, err := src.ProjectTemplates()
	if err != nil {
		return nil, err
	}
	database, err := src.DatabaseTemplates()
	if err != nil {
		return nil, err
	}
	fetchAll := sync.OnceValues(func() ([]cmdTemplates.Template, error) {
		spinner, ticker := cmdutil.NewSpinnerAndTicker(
			"Loading templates", nil, opts.Color, 8 /*timesPerSecond*/, !opts.IsInteractive,
		)
		defer cmdutil.SpinUntilStopped(spinner, ticker)()
		return src.Templates()
	})

	template, err := newGuided(guidedTemplates{
		project:  project,
		database: database,
		vcsOrgs:  src.VcsTemplateSourceOrgs(),
		fetchAll: fetchAll,
	}, opts, selector).choose()
	if !errors.Is(err, errFallBackToFlatList) {
		return template, err
	}

	all, err := fetchAll()
	if err != nil {
		return nil, err
	}
	// pickFromSet answers a lone template without a prompt, so there is no switch to announce.
	if len(all) > 1 {
		fmt.Fprintln(opts.Stdout, "Falling back to the full template list.")
	}
	return pickFromSet(all, false /*yes*/, opts, selector)
}

func (g *guided) choose() (cmdTemplates.Template, error) {
	rows := g.choiceRows()
	if len(rows) == 0 {
		return nil, errFallBackToFlatList
	}
	rows = append(rows, g.browseAllRow())

	for {
		row, err := pick(g.selector, "Which provider would you like to use?", g.opts, rows,
			func(r providerRow) string { return r.label })
		if err != nil {
			return nil, err
		}
		template, err := row.choose()
		// Ctrl-C steps back to this prompt, as it does in `pulumi state rename`; at this prompt
		// there is nothing to step back to, so it leaves the flow.
		if errors.Is(err, errBackToProvider) || errors.Is(err, terminal.InterruptErr) {
			continue
		}
		return template, err
	}
}

// choose returns [errBackToProvider] if the row turned out to lead nowhere.
type providerRow struct {
	label  string
	choose func() (cmdTemplates.Template, error)
}

func registryPublisher(t cmdTemplates.Template) (string, bool) {
	if t.Error() != nil {
		return "", false
	}
	publisher := t.Publisher()
	return publisher, publisher != ""
}

// VCS template source orgs and registry publishers only tell us an organization exists, not that it
// has templates the guided flow can offer, so a row here can still resolve to an empty list.
func (t guidedTemplates) orgRows() []string {
	names := slices.Clone(t.vcsOrgs)
	for _, template := range t.database {
		if publisher, ok := registryPublisher(template); ok {
			names = append(names, publisher)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (g *guided) choiceRows() []providerRow {
	providers, orgs := g.catalog.Providers(), g.orgRows()
	rows := make([]providerRow, 0, len(providers)+len(orgs))
	for _, p := range providers {
		rows = append(rows, providerRow{p.DisplayName, func() (cmdTemplates.Template, error) {
			return g.chooseLanguage(p)
		}})
	}
	for _, org := range orgs {
		rows = append(rows, providerRow{org + " templates", func() (cmdTemplates.Template, error) {
			return g.chooseOrgTemplates(org)
		}})
	}
	return rows
}

// browseAllRow is always offered, so it never counts towards there being a choice worth prompting
// for.
func (g *guided) browseAllRow() providerRow {
	return providerRow{optionBrowseAll, g.chooseAllTemplates}
}

func (g *guided) chooseLanguage(p catalog.Provider) (cmdTemplates.Template, error) {
	language, err := pick(g.selector, "Which language would you like to use?", g.opts,
		p.Languages, func(l catalog.Language) string { return l.DisplayName })
	if err != nil {
		return nil, err
	}
	// The prompts only offer values the catalog can resolve, so a miss here is a broken invariant.
	template, ok := g.catalog.Resolve(p.ID, language.ID)
	if !ok {
		return nil, fmt.Errorf("no template for provider %q and language %q", p.ID, language.ID)
	}
	return template, nil
}

// chooseAllTemplates and chooseOrgTemplates return to the provider prompt rather than abandoning
// the flow, since the provider and language rows still work without the full template set.
func (g *guided) chooseAllTemplates() (cmdTemplates.Template, error) {
	all, err := g.fetchAll()
	if err != nil {
		return nil, g.backToProvider("Could not load the full template list: %v", err)
	}
	if len(all) == 0 {
		return nil, g.backToProvider("No templates found.")
	}
	return chooseTemplateFromList(sortedForDisplay(all), g.opts, g.selector)
}

func (g *guided) chooseOrgTemplates(org string) (cmdTemplates.Template, error) {
	available := g.database
	if slices.Contains(g.vcsOrgs, org) {
		all, err := g.fetchAll()
		if err != nil {
			return nil, g.backToProvider("Could not load templates for organization %q: %v", org, err)
		}
		available = all
	}

	var orgTemplates []cmdTemplates.Template
	for _, template := range available {
		if publisher, ok := registryPublisher(template); ok && publisher == org {
			orgTemplates = append(orgTemplates, template)
		}
	}
	if len(orgTemplates) == 0 {
		return nil, g.backToProvider("No templates found for organization %q.", org)
	}
	return chooseTemplateFromList(orgTemplates, g.opts, g.selector)
}

func chooseTemplateFromList(
	templates []cmdTemplates.Template, opts display.Options, selector selectFunc,
) (cmdTemplates.Template, error) {
	message := fmt.Sprintf("Please choose a template (%d total):", len(templates))
	return pick(selector, message, opts, templates, templateLabeler(templates))
}
