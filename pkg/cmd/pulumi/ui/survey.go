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

package ui

import (
	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func SurveyIcons(color colors.Colorization) survey.AskOpt {
	return survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question = survey.Icon{}
		icons.SelectFocus = survey.Icon{Text: color.Colorize(colors.BrightGreen + ">" + colors.Reset)}
	})
}

// Ask multiple survey based questions.
//
// Ctrl-C will go back in the stack, and valid answers will go forward.
func SurveyStack(interactions ...func() error) error {
	for i := 0; i < len(interactions); {
		err := interactions[i]()
		switch err {
		// No error, so go to the next interaction.
		case nil:
			i++
		// We have received an interrupt, so go back to the previous interaction.
		case terminal.InterruptErr:
			// If we have received in interrupt at the beginning of the stack,
			// the user has asked to go back to before the stack. We can't do
			// that, so we just return the interrupt.
			if i == 0 {
				return err
			}
			i--
		// We have received an unexpected error, so return it.
		default:
			return err
		}
	}
	return nil
}

// PromptUserSkippable wraps over promptUser making it skippable through the "yes" parameter
// commonly being the value of the --yes flag used in each command.
// If yes is true, defaultValue is returned without prompting.
func PromptUserSkippable(yes bool, msg string, options []string, defaultOption string,
	colorization colors.Colorization,
) string {
	if yes {
		return defaultOption
	}
	return PromptUser(msg, options, defaultOption, colorization)
}

// PromptUser prompts the user for a value with a list of options. Hitting enter accepts the
// default.
func PromptUser(
	msg string,
	options []string,
	defaultOption string,
	colorization colors.Colorization,
	surveyAskOpts ...survey.AskOpt,
) string {
	prompt := "\b" + colorization.Colorize(colors.SpecPrompt+msg+colors.Reset)
	surveycore.DisableColor = true

	allSurveyAskOpts := append(
		surveyAskOpts,
		survey.WithIcons(func(icons *survey.IconSet) {
			icons.Question = survey.Icon{}
			icons.SelectFocus = survey.Icon{Text: colorization.Colorize(colors.BrightGreen + ">" + colors.Reset)}
		}),
	)

	var response string
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: options,
		Default: defaultOption,
	}, &response, allSurveyAskOpts...); err != nil {
		return ""
	}
	return response
}

// PromptUserMultiSkippable wraps over promptUserMulti making it skippable through the "yes" parameter
// commonly being the value of the --yes flag used in each command.
// If yes is true, defaultValue is returned without prompting.
func PromptUserMultiSkippable(yes bool, msg string, options []string, defaultOptions []string,
	colorization colors.Colorization,
) []string {
	if yes {
		return defaultOptions
	}
	return PromptUserMulti(msg, options, defaultOptions, colorization)
}

// PromptUserMulti prompts the user for a value with a list of options, allowing to select none or multiple options.
// defaultOptions is a set of values to be selected by default.
func PromptUserMulti(msg string, options []string, defaultOptions []string, colorization colors.Colorization) []string {
	confirmationHint := " (use enter to accept the current selection)"

	prompt := "\b" + colorization.Colorize(colors.SpecPrompt+msg+colors.Reset) + confirmationHint

	surveycore.DisableColor = true
	surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question = survey.Icon{}
		icons.SelectFocus = survey.Icon{Text: colorization.Colorize(colors.BrightGreen + ">" + colors.Reset)}
	})

	var response []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message: prompt,
		Options: options,
		Default: defaultOptions,
	}, &response, surveyIcons); err != nil {
		return []string{}
	}
	return response
}
