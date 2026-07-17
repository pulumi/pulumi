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

// publishedTemplate exposes GetPublisher; workspaceTemplate (from pulumi/templates) does not.
type publishedTemplate interface{ GetPublisher() string }

func isRegistryTemplate(t cmdTemplates.Template) bool {
	_, ok := t.(publishedTemplate)
	return ok
}

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

// chooseGuided returns (nil, nil) when the caller should fall back to the flat chooser.
func chooseGuided(
	templates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	byName := make(map[string]cmdTemplates.Template, len(templates))
	var registryTemplates []cmdTemplates.Template
	for _, t := range templates {
		byName[t.Name()] = t
		if isRegistryTemplate(t) {
			registryTemplates = append(registryTemplates, t)
		}
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

	provider, err := chooseProvider(opts, sel)
	if err != nil {
		return nil, err
	}

	language, err := chooseLanguage(provider, opts, sel)
	if err != nil {
		return nil, err
	}

	name, ok := catalog.Resolve(provider.ID, language)
	if !ok {
		return nil, nil
	}
	template, ok := byName[name]
	if !ok {
		return nil, nil
	}
	return template, nil
}

func chooseProvider(opts display.Options, sel selectFunc) (catalog.Provider, error) {
	featured := catalog.Featured()
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

	others := catalog.Others()
	otherOptions := make([]string, 0, len(others))
	otherByDisplayName := make(map[string]catalog.Provider, len(others))
	for _, p := range others {
		otherOptions = append(otherOptions, p.DisplayName)
		otherByDisplayName[p.DisplayName] = p
	}

	answer, err = sel("\rWhich provider would you like to use?\n", otherOptions, opts)
	if err != nil {
		return catalog.Provider{}, err
	}
	return otherByDisplayName[answer], nil
}

func chooseLanguage(provider catalog.Provider, opts display.Options, sel selectFunc) (string, error) {
	options := make([]string, 0, len(provider.Languages))
	byDisplayName := make(map[string]string, len(provider.Languages))
	for _, l := range provider.Languages {
		options = append(options, l.DisplayName)
		byDisplayName[l.DisplayName] = l.ID
	}

	answer, err := sel("\rWhich language would you like to use?\n", options, opts)
	if err != nil {
		return "", err
	}
	return byDisplayName[answer], nil
}

func chooseRegistryTemplate(
	registryTemplates []cmdTemplates.Template, opts display.Options, sel selectFunc,
) (cmdTemplates.Template, error) {
	options := make([]string, 0, len(registryTemplates))
	byDisplayName := make(map[string]cmdTemplates.Template, len(registryTemplates))
	for _, t := range registryTemplates {
		options = append(options, t.DisplayName())
		byDisplayName[t.DisplayName()] = t
	}

	message := fmt.Sprintf("\rPlease choose a template (%d total):\n", len(options))
	answer, err := sel(message, options, opts)
	if err != nil {
		return nil, err
	}
	template, ok := byDisplayName[answer]
	if !ok {
		return nil, fmt.Errorf("no such template: %q", answer)
	}
	return template, nil
}
