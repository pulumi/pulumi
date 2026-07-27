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

package newcmd

import (
	"errors"
	"fmt"
	"net/url"
	"sort"

	"github.com/AlecAivazis/survey/v2/terminal"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
)

const (
	BrokenTemplateDescription = "(This template is currently broken)"
)

// ChooseTemplate will prompt the user to choose amongst the available templates.
func ChooseTemplate(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
	if !opts.IsInteractive {
		return nil, nil
	}

	return chooseTemplateFlat(templates, opts)
}

// guidedChooser walks the user from provider to language to a starter template, falling back to flat
// when the guided flow cannot structure the available templates.
func guidedChooser(sel selectFunc, flat chooseTemplateFunc) chooseTemplateFunc {
	return func(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
		if !opts.IsInteractive {
			return nil, nil
		}

		template, err := chooseGuided(templates, opts, sel)
		switch {
		case errors.Is(err, errFallBackToFlatList):
			return flat(templates, opts)
		case errors.Is(err, terminal.InterruptErr):
			return nil, errors.New("no template selected")
		case err != nil:
			return nil, err
		}
		return template, nil
	}
}

func chooseTemplateFlat(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
	options, optionToTemplateMap := templatesToOptionArrayAndMap(templates)
	message := fmt.Sprintf("Please choose a template (%d total):", len(options))

	option, err := surveySelect(message, options, opts)
	if err != nil {
		return nil, errors.New("no template selected; please use `pulumi new` to choose one")
	}

	return optionToTemplateMap[option], nil
}

// templateLabeler formats each template in the set as its display name padded to the longest name,
// followed by its description (or a broken marker).
func templateLabeler(templates []cmdTemplates.Template) func(cmdTemplates.Template) string {
	maxNameLength := 0
	for _, template := range templates {
		maxNameLength = max(maxNameLength, len(template.DisplayName()))
	}
	return func(template cmdTemplates.Template) string {
		desc := template.Description()
		if template.Error() != nil {
			desc = BrokenTemplateDescription
		}
		return fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), template.DisplayName(), desc)
	}
}

// templatesToOptionArrayAndMap returns an array of option strings and a map of option strings to templates.
// Each option string is made up of the template name and description with some padding in between.
func templatesToOptionArrayAndMap(templates []cmdTemplates.Template) ([]string, map[string]cmdTemplates.Template) {
	label := templateLabeler(templates)

	var options []string
	var brokenOptions []string
	nameToTemplateMap := make(map[string]cmdTemplates.Template)
	for _, template := range templates {
		option := label(template)
		nameToTemplateMap[option] = template
		if template.Error() != nil {
			brokenOptions = append(brokenOptions, option)
		} else {
			options = append(options, option)
		}
	}
	// After sorting the options, add the broken templates to the end
	sort.Strings(options)
	options = append(options, brokenOptions...)

	return options, nameToTemplateMap
}

// sanitizeTemplate strips sensitive data such as credentials and query strings from a template URL.
func sanitizeTemplate(template string) string {
	// If it's a valid URL, strip any credentials and query strings.
	if parsedURL, err := url.Parse(template); err == nil {
		parsedURL.User = nil
		parsedURL.RawQuery = ""
		return parsedURL.String()
	}
	// Otherwise, return the original string.
	return template
}
