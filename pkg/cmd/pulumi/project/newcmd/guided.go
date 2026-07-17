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

	survey "github.com/AlecAivazis/survey/v2"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd/catalog"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

const (
	optionOther             = "Other"
	sourcePulumiTemplates   = "Pulumi templates"
	sourceRegistryTemplates = "Registry templates"
)

type selectFunc func(message string, options []string, opts display.Options) (string, error)

func surveySelect(message string, options []string, opts display.Options) (string, error) {
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)
	var answer string
	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  options,
		PageSize: cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: len(options)}),
	}, &answer, ui.SurveyIcons(opts.Color)); err != nil {
		return "", errors.New("no template selected")
	}
	return answer, nil
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

// chooseGuided returns (nil, nil) when the caller should fall back to the flat chooser.
func chooseGuided(
	templates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	byName := make(map[string]cmdTemplates.Template, len(templates))
	var registryTemplates []cmdTemplates.Template
	curatedNames := make([]string, 0, len(templates))
	for _, t := range templates {
		byName[t.Name()] = t
		if t.FromRegistry() {
			registryTemplates = append(registryTemplates, t)
		} else {
			curatedNames = append(curatedNames, t.Name())
		}
	}

	cat := catalog.New(curatedNames)
	if cat.Empty() && len(registryTemplates) == 0 {
		return nil, nil
	}
	if cat.Empty() {
		return chooseRegistryTemplate(registryTemplates, opts, sel)
	}

	if len(registryTemplates) > 0 {
		source, err := sel(
			"\rWhere would you like to start?\n", []string{sourcePulumiTemplates, sourceRegistryTemplates}, opts)
		if err != nil {
			return nil, err
		}
		if source == sourceRegistryTemplates {
			return chooseRegistryTemplate(registryTemplates, opts, sel)
		}
	}

	provider, err := chooseProvider(cat, opts, sel)
	if err != nil {
		return nil, err
	}

	language, err := chooseLanguage(provider, opts, sel)
	if err != nil {
		return nil, err
	}

	// Past this point the user has answered the provider and language prompts, so a miss is a broken
	// invariant (the prompts only offer values the catalog can resolve), not a reason to silently fall
	// back to the flat list after having already prompted. Surface it instead.
	name, ok := cat.Resolve(provider.ID, language)
	if !ok {
		return nil, fmt.Errorf("no template for provider %q and language %q", provider.ID, language)
	}
	template, ok := byName[name]
	if !ok {
		return nil, fmt.Errorf("template %q is missing from the available set", name)
	}
	return template, nil
}

func chooseProvider(cat *catalog.Catalog, opts display.Options, sel selectFunc) (catalog.Provider, error) {
	featured := cat.Featured()
	options := make([]string, 0, len(featured)+1)
	byDisplayName := make(map[string]catalog.Provider, len(featured))
	for _, p := range featured {
		options = append(options, p.DisplayName)
		byDisplayName[p.DisplayName] = p
	}
	options = append(options, optionOther)

	answer, err := sel("\rWhich cloud would you like to use?\n", options, opts)
	if err != nil {
		return catalog.Provider{}, err
	}
	if answer != optionOther {
		return byDisplayName[answer], nil
	}
	return pick(
		sel, "\rWhich provider would you like to use?\n", opts, cat.Others(),
		func(p catalog.Provider) string { return p.DisplayName })
}

func chooseLanguage(provider catalog.Provider, opts display.Options, sel selectFunc) (string, error) {
	language, err := pick(
		sel, "\rWhich language would you like to use?\n", opts,
		provider.Languages, func(l catalog.Language) string { return l.DisplayName })
	if err != nil {
		return "", err
	}
	return language.ID, nil
}

func chooseRegistryTemplate(
	registryTemplates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	message := fmt.Sprintf("\rPlease choose a template (%d total):\n", len(registryTemplates))
	return pick(sel, message, opts, registryTemplates, func(t cmdTemplates.Template) string { return t.DisplayName() })
}
