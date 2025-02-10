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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

const (
	BrokenTemplateDescription = "(This template is currently broken)"
)

// ChooseTemplate will prompt the user to choose amongst the available templates.
func ChooseTemplate(templates []workspace.Template, opts display.Options) (workspace.Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !opts.IsInteractive {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true

	groups := map[string]*group{}
	toDisplay := make([]templateOrGroup, 0, len(templates))

	for _, t := range templates {
		if t.Group == nil || t.Group.GroupName == "" {
			toDisplay = append(toDisplay, template(t))
			continue
		}

		g, ok := groups[t.Group.GroupName]
		if !ok {
			g = new(group)
			groups[t.Group.GroupName] = g
		}
		g.members = append(g.members, t)
	}

	for _, g := range groups {
		if len(g.members) == 1 {
			toDisplay = append(toDisplay, template(g.members[0]))
			continue
		}
		toDisplay = append(toDisplay, *g)
	}

	options, optionToTemplateMap := templatesToOptionArrayAndMap(toDisplay, func(t templateOrGroup) templateOrGroup { return t })
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
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	if t, ok := optionToTemplateMap[option].(template); ok {
		return workspace.Template(t), nil
	}

	g := optionToTemplateMap[option].(group)

	options, optionToGroupMemberMap := templatesToOptionArrayAndMap(g.members, func(t workspace.Template) template { return template(t) })
	nopts = len(options)
	pageSize = cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: nopts})
	message = fmt.Sprintf("\rPlease choose a specific template (%d total):\n", nopts)
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  options,
		PageSize: pageSize,
	}, &option, ui.SurveyIcons(opts.Color)); err != nil {
		return workspace.Template{}, errors.New(chooseTemplateErr)
	}

	return optionToGroupMemberMap[option], nil
}

type templateOrGroup interface {
	getName() string
	broken() bool
	description() string
}

var (
	_ templateOrGroup = template{}
	_ templateOrGroup = group{}
)

type template workspace.Template

func (t template) getName() string { return t.Name }
func (t template) broken() bool    { return (workspace.Template)(t).Errored() }
func (t template) description() string {
	// If template is broken, indicate it in the project description.
	if t.broken() {
		return pkgWorkspace.ValueOrDefaultProjectDescription("", BrokenTemplateDescription, t.Description)
	}
	return pkgWorkspace.ValueOrDefaultProjectDescription("", t.ProjectDescription, t.Description)
}

type group struct{ members []workspace.Template }

func (g group) getName() string { return g.members[0].Group.GroupName }
func (g group) broken() bool    { return false }
func (g group) description() string {
	for _, m := range g.members {
		if m.Group.GroupDescription != "" {
			return m.Group.GroupDescription
		}
	}
	return ""
}

// templatesToOptionArrayAndMap returns an array of option strings and a map of option strings to templates.
// Each option string is made up of the template name and description with some padding in between.
func templatesToOptionArrayAndMap[T any, U templateOrGroup](templates []T, promote func(T) U) ([]string, map[string]T) {
	// Find the longest name length. Used to add padding between the name and description.
	maxNameLength := 0
	for _, template := range templates {
		template := promote(template)
		if len(template.getName()) > maxNameLength {
			maxNameLength = len(template.getName())
		}
	}

	// Build the array and map.
	var options []string
	var brokenOptions []string
	nameToTemplateMap := make(map[string]T)
	for _, templateT := range templates {
		template := promote(templateT)
		// Create the option string that combines the name, padding, and description.
		desc := template.description()
		option := fmt.Sprintf(fmt.Sprintf("%%%ds    %%s", -maxNameLength), template.getName(), desc)

		nameToTemplateMap[option] = templateT
		if template.broken() {
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
