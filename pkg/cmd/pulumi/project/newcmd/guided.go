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

// errFallBackToFlatList signals that the guided flow cannot structure the available templates and
// the caller should run the flat chooser instead.
var errFallBackToFlatList = errors.New("fall back to the flat template list")

func surveySelect(message string, options []string, opts display.Options) (string, error) {
	return ui.PromptUserErr("\r"+message+"\n", options, "", opts.Color,
		survey.WithPageSize(cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)})))
}

// pick prompts for one of items, presenting each by its display name, and returns the chosen item.
// Duplicate display names (possible for registry/org templates) are suffixed so every option is
// distinct: showing two identical rows would be ambiguous both to the user and to the reverse lookup.
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
		byName[t.Name()] = t
		// The flat chooser is the only surface that can mark a template broken, so guided never
		// offers one.
		if t.Error() != nil {
			continue
		}
		if t.FromRegistry() {
			registryTemplates = append(registryTemplates, t)
		} else {
			curatedNames = append(curatedNames, t.Name())
		}
	}

	cat := catalog.New(curatedNames)
	if cat.Empty() && len(registryTemplates) == 0 {
		if opts.Stdout != nil {
			fmt.Fprintln(opts.Stdout, "Falling back to the full template list.")
		}
		return nil, errFallBackToFlatList
	}
	if cat.Empty() {
		return chooseRegistryTemplate(registryTemplates, opts, sel)
	}

	var choice guidedChoice
	var language string
	if err := ui.SurveyStack(
		func() (err error) {
			choice, err = chooseCloud(cat, registryTemplates, opts, sel)
			return err
		},
		func() (err error) {
			if choice.template != nil {
				return nil
			}
			language, err = chooseLanguage(choice.provider, opts, sel)
			return err
		},
	); err != nil {
		return nil, err
	}
	if choice.template != nil {
		return choice.template, nil
	}

	// Past this point the user has answered the provider and language prompts, so a miss is a broken
	// invariant (the prompts only offer values the catalog can resolve), not a reason to silently fall
	// back to the flat list after having already prompted. Surface it instead.
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

// guidedChoice is the outcome of the cloud prompt: either a provider that still needs a language, or
// a registry template chosen directly.
type guidedChoice struct {
	provider catalog.Provider
	template cmdTemplates.Template
}

// cloudRow is one option in the cloud prompt: a featured provider, the Other expansion, a publishing
// organization's registry templates, or the Browse-all fallback.
type cloudRow struct {
	label    string
	provider catalog.Provider
	registry []cmdTemplates.Template
}

func cloudRows(cat *catalog.Catalog, registryTemplates []cmdTemplates.Template) []cloudRow {
	featured := cat.Featured()
	rows := make([]cloudRow, 0, len(featured)+3)
	for _, p := range featured {
		rows = append(rows, cloudRow{label: p.DisplayName, provider: p})
	}
	rows = append(rows, cloudRow{label: optionOther})
	rows = append(rows, registryRows(registryTemplates)...)
	rows = append(rows, cloudRow{label: optionBrowseAll})
	return rows
}

// registryRows buckets registry templates by publishing organization, one row per org.
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
		rows = append(rows, cloudRow{label: label, registry: group})
	}
	return rows
}

// chooseCloud prompts for a featured cloud, expanding "Other" into the full provider list, an org row
// into that org's registry templates, and "Browse all templates" into the flat-list fallback.
func chooseCloud(
	cat *catalog.Catalog, registryTemplates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (guidedChoice, error) {
	rows := cloudRows(cat, registryTemplates)
	var row cloudRow
	var choice guidedChoice
	err := ui.SurveyStack(
		func() (err error) {
			row, err = pick(sel, "Which cloud would you like to use?", opts, rows,
				func(r cloudRow) string { return r.label })
			if err == nil && row.label == optionBrowseAll {
				return errFallBackToFlatList
			}
			return err
		},
		func() (err error) {
			choice = guidedChoice{}
			switch {
			case row.registry != nil:
				choice.template, err = chooseRegistryTemplate(row.registry, opts, sel)
			case row.label == optionOther:
				choice.provider, err = pick(sel, "Which provider would you like to use?", opts, cat.Others(),
					func(p catalog.Provider) string { return p.DisplayName })
			default:
				choice.provider = row.provider
			}
			return err
		},
	)
	return choice, err
}

func chooseLanguage(provider catalog.Provider, opts display.Options, sel selectFunc) (string, error) {
	language, err := pick(
		sel, "Which language would you like to use?", opts,
		provider.Languages, func(l catalog.Language) string { return l.DisplayName })
	if err != nil {
		return "", err
	}
	return language.ID, nil
}

func chooseRegistryTemplate(
	registryTemplates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	message := fmt.Sprintf("Please choose a template (%d total):", len(registryTemplates))
	return pick(sel, message, opts, registryTemplates, templateLabeler(registryTemplates))
}
