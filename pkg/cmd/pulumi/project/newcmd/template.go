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
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
)

const (
	BrokenTemplateDescription = "(This template is currently broken)"
)

var errNoTemplateSelected = errors.New("no template selected; please use `pulumi new` to choose one")

// ChooseTemplate will prompt the user to choose amongst the available templates.
func ChooseTemplate(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
	if !opts.IsInteractive {
		return nil, nil
	}

	template, err := chooseTemplateFromList(sortedForDisplay(templates), opts, surveySelect)
	if err != nil {
		return nil, errNoTemplateSelected
	}
	return template, nil
}

// sortedForDisplay orders templates by display name, broken templates last.
func sortedForDisplay(templates []cmdTemplates.Template) []cmdTemplates.Template {
	sorted := slices.Clone(templates)
	slices.SortStableFunc(sorted, func(a, b cmdTemplates.Template) int {
		aBroken, bBroken := a.Error() != nil, b.Error() != nil
		if aBroken != bBroken {
			if aBroken {
				return 1
			}
			return -1
		}
		return strings.Compare(a.DisplayName(), b.DisplayName())
	})
	return sorted
}

func guidedChooser(sel selectFunc, flat chooseTemplateFunc) chooseTemplateFunc {
	return func(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
		if !opts.IsInteractive {
			return nil, nil
		}

		template, err := chooseGuided(templates, opts, sel)
		switch {
		case errors.Is(err, errFallBackToFlatList):
			fmt.Fprintln(opts.Stdout, "Falling back to the full template list.")
			return flat(templates, opts)
		case errors.Is(err, terminal.InterruptErr):
			return nil, errNoTemplateSelected
		case err != nil:
			return nil, err
		}
		return template, nil
	}
}

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
		return fmt.Sprintf("%-*s    %s", maxNameLength, template.DisplayName(), desc)
	}
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
