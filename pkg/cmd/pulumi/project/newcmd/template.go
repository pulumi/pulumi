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
	return declinedToChoose(chooseTemplateFromList(sortedForDisplay(templates), opts, surveySelect))
}

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

// Only Templates waits on the slowest fetch.
type templateSource interface {
	ProjectTemplates() ([]cmdTemplates.Template, error)
	DatabaseTemplates() ([]cmdTemplates.Template, error)
	VcsTemplateSourceOrgs() []string
	Templates() ([]cmdTemplates.Template, error)
}

func (args newArgs) useGuidedFlow() bool {
	return args.templateNameOrURL == "" && !args.yes && args.interactive
}

func declinedToChoose(
	template cmdTemplates.Template, err error,
) (cmdTemplates.Template, error) {
	if errors.Is(err, terminal.InterruptErr) {
		return nil, errNoTemplateSelected
	}
	return template, err
}

func resolveTemplate(
	src templateSource, args newArgs, opts display.Options, selector selectFunc,
) (cmdTemplates.Template, error) {
	if args.useGuidedFlow() {
		return declinedToChoose(chooseGuidedFromSource(src, opts, selector))
	}
	all, err := src.Templates()
	if err != nil {
		return nil, err
	}
	return declinedToChoose(pickFromSet(all, args.yes, opts, selector))
}

// `--yes` never prompts, and takes no template rather than guessing among several.
func pickFromSet(
	templates []cmdTemplates.Template, yes bool, opts display.Options, selector selectFunc,
) (cmdTemplates.Template, error) {
	switch {
	case len(templates) == 1:
		return templates[0], nil
	case yes:
		return nil, nil
	case len(templates) == 0:
		return nil, errors.New("no templates")
	}
	return chooseTemplateFromList(sortedForDisplay(templates), opts, selector)
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
