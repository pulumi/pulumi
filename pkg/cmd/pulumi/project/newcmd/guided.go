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
	"sort"

	survey "github.com/AlecAivazis/survey/v2"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd/catalog"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
)

const (
	optionOther     = "Other"
	optionBrowseAll = "Browse all templates"
)

type selectFunc func(message string, options []string, opts display.Options) (string, error)

var errFallBackToFlatList = errors.New("fall back to the flat template list")

func surveySelect(message string, options []string, opts display.Options) (string, error) {
	return ui.PromptUserErr("\r"+message+"\n", options, "", opts.Color,
		survey.WithPageSize(cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)})))
}

// pick prompts for one of items. Duplicate display names are suffixed so the label-to-item lookup
// stays unambiguous.
func pick[T any](
	sel selectFunc, message string, opts display.Options, items []T, name func(T) string,
) (T, error) {
	var zero T
	options := make([]string, len(items))
	byLabel := make(map[string]T, len(items))
	counts := make(map[string]int, len(items))
	for i, item := range items {
		label := name(item)
		counts[label]++
		if n := counts[label]; n > 1 {
			label = fmt.Sprintf("%s (%d)", label, n)
		}
		options[i] = label
		byLabel[label] = item
	}
	answer, err := sel(message, options, opts)
	if err != nil {
		return zero, err
	}
	chosen, ok := byLabel[answer]
	if !ok {
		return zero, fmt.Errorf("no such option: %q", answer)
	}
	return chosen, nil
}

func chooseGuided(
	templates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	byName := make(map[string]cmdTemplates.Template, len(templates))
	var registryTemplates []cmdTemplates.Template
	curatedNames := make([]string, 0, len(templates))
	for _, t := range templates {
		// Broken templates are only offered by the flat chooser.
		if t.Error() != nil {
			continue
		}
		if t.FromRegistry() {
			registryTemplates = append(registryTemplates, t)
		} else {
			byName[t.Name()] = t
			curatedNames = append(curatedNames, t.Name())
		}
	}

	cat := catalog.New(curatedNames)
	if cat.Empty() && len(registryTemplates) == 0 {
		fmt.Fprintln(opts.Stdout, "Falling back to the full template list.")
		return nil, errFallBackToFlatList
	}

	rows := cloudRows(cat, registryTemplates, templates)
	var choice guidedChoice
	var language string
	if err := ui.SurveyStack(
		func() (err error) {
			choice, err = chooseCloud(rows, opts, sel)
			return err
		},
		func() (err error) {
			if choice.template != nil {
				return nil
			}
			l, err := pick(sel, "Which language would you like to use?", opts,
				choice.provider.Languages, func(l catalog.Language) string { return l.DisplayName })
			language = l.ID
			return err
		},
	); err != nil {
		return nil, err
	}
	if choice.template != nil {
		return choice.template, nil
	}

	// The prompts only offer values the catalog can resolve, so a miss here is a broken invariant.
	name, ok := cat.Resolve(choice.provider.ID, language)
	if !ok {
		return nil, fmt.Errorf("no template for provider %q and language %q", choice.provider.ID, language)
	}
	template, ok := byName[name]
	if !ok {
		return nil, fmt.Errorf("template %q is missing from the available set", name)
	}
	return template, nil
}

// guidedChoice is either a provider that still needs a language, or a registry template chosen
// directly.
type guidedChoice struct {
	provider catalog.Provider
	template cmdTemplates.Template
}

type rowKind int

const (
	rowProvider rowKind = iota
	rowRegistry
	rowOther
	rowBrowseAll
)

type cloudRow struct {
	kind      rowKind
	label     string
	provider  catalog.Provider        // rowProvider
	providers []catalog.Provider      // rowOther
	templates []cmdTemplates.Template // rowRegistry and rowBrowseAll
}

func cloudRows(cat *catalog.Catalog, registryTemplates, all []cmdTemplates.Template) []cloudRow {
	featured := cat.Featured()
	rows := make([]cloudRow, 0, len(featured)+3)
	for _, p := range featured {
		rows = append(rows, cloudRow{kind: rowProvider, label: p.DisplayName, provider: p})
	}
	if others := cat.Others(); len(others) > 0 {
		rows = append(rows, cloudRow{kind: rowOther, label: optionOther, providers: others})
	}
	if none, ok := cat.None(); ok {
		rows = append(rows, cloudRow{kind: rowProvider, label: none.DisplayName, provider: none})
	}
	rows = append(rows, registryRows(registryTemplates)...)
	rows = append(rows, cloudRow{kind: rowBrowseAll, label: optionBrowseAll, templates: sortedForDisplay(all)})
	return rows
}

func registryRows(registryTemplates []cmdTemplates.Template) []cloudRow {
	byPublisher := map[string][]cmdTemplates.Template{}
	for _, t := range registryTemplates {
		byPublisher[t.Publisher()] = append(byPublisher[t.Publisher()], t)
	}
	publishers := make([]string, 0, len(byPublisher))
	for publisher := range byPublisher {
		publishers = append(publishers, publisher)
	}
	sort.Strings(publishers)

	rows := make([]cloudRow, 0, len(byPublisher))
	for _, publisher := range publishers {
		group := byPublisher[publisher]
		label := fmt.Sprintf("%s templates (%d)", publisher, len(group))
		if publisher == "" {
			label = fmt.Sprintf("Registry templates (%d)", len(group))
		}
		rows = append(rows, cloudRow{kind: rowRegistry, label: label, templates: group})
	}
	return rows
}

// chooseCloud runs the cloud prompt and its dispatch in a SurveyStack of their own, nested inside
// chooseGuided's outer stack: an interrupt in a dispatch sub-prompt (a template list, the Other
// providers) steps back to the cloud prompt here, while an interrupt at the language step lands on
// this whole function, skipping over the non-prompting dispatch step that would otherwise bounce
// the interrupt around.
func chooseCloud(rows []cloudRow, opts display.Options, sel selectFunc) (guidedChoice, error) {
	var row cloudRow
	var choice guidedChoice
	err := ui.SurveyStack(
		func() (err error) {
			row, err = pick(sel, "Which cloud would you like to use?", opts, rows,
				func(r cloudRow) string { return r.label })
			return err
		},
		func() (err error) {
			choice = guidedChoice{}
			switch row.kind {
			case rowProvider:
				choice.provider = row.provider
			case rowOther:
				choice.provider, err = pick(sel, "Which provider would you like to use?", opts, row.providers,
					func(p catalog.Provider) string { return p.DisplayName })
			case rowRegistry, rowBrowseAll:
				choice.template, err = chooseTemplateFromList(row.templates, opts, sel)
			}
			return err
		},
	)
	return choice, err
}

func chooseTemplateFromList(
	templates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	message := fmt.Sprintf("Please choose a template (%d total):", len(templates))
	return pick(sel, message, opts, templates, templateLabeler(templates))
}
