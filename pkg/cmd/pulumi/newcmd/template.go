// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

const (
	BrokenTemplateDescription = "(This template is currently broken)"
)

// ChooseTemplate will prompt the user to choose amongst the available templates.
func ChooseTemplate(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !opts.IsInteractive {
		return nil, errors.New(chooseTemplateErr)
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true

	options, optionToTemplateMap := templatesToOptionArrayAndMap(templates)
	nopts := len(options)
	pageSize := cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: nopts})
	message := fmt.Sprintf("\rPlease choose a template (%d total):\n", nopts)
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  options,
		PageSize: pageSize,
	}, &option, ui.SurveyIcons(opts.Color)); err != nil {
		return nil, errors.New(chooseTemplateErr)
	}

	return optionToTemplateMap[option], nil
}

// templatesToOptionArrayAndMap returns an array of option strings and a map of option strings to templates.
// Each option string is made up of the template name and description with some padding in between.
func templatesToOptionArrayAndMap(templates []cmdTemplates.Template) ([]string, map[string]cmdTemplates.Template) {
	// Find the longest name length. Used to add padding between the name and description.
	maxNameLength := 0
	for _, template := range templates {
		if len(template.Name()) > maxNameLength {
			maxNameLength = len(template.Name())
		}
	}

	// Build the array and map.
	var options []string
	var brokenOptions []string
	nameToTemplateMap := make(map[string]cmdTemplates.Template)
	for _, template := range templates {
		projectDescription := template.ProjectDescription()
		// If template is broken, indicate it in the project description.
		if template.Error() != nil {
			projectDescription = BrokenTemplateDescription
		}

		// Create the option string that combines the name, padding, and description.
		desc := pkgWorkspace.ValueOrDefaultProjectDescription("", projectDescription, template.Description())
		option := fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), template.Name(), desc)

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
