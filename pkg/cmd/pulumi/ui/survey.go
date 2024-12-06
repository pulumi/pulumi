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
	"errors"
	"fmt"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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

// PromptForValue prompts the user for a value with a defaultValue preselected. Hitting enter accepts the
// default. If yes is true, defaultValue is returned without prompting. isValidFn is an optional parameter;
// when specified, it will be run to validate that value entered. When this function returns a non nil error
// validation is assumed to have failed and an error is printed. The error returned by isValidFn is also displayed
// to provide information about why the validation failed. A period is appended to this message. `PromptForValue` then
// prompts again.
func PromptForValue(
	yes bool, valueType string, defaultValue string, secret bool,
	isValidFn func(value string) error, opts display.Options,
) (string, error) {
	var value string
	for {
		// If we are auto-accepting the default (--yes), just set it and move on to validating.
		// Otherwise, prompt the user interactively for a value.
		if yes {
			value = defaultValue
		} else {
			var prompt string
			if valueType == "" && defaultValue == "" {
				// No need to print anything, this is a full line / blank prompt.
			} else if defaultValue == "" {
				prompt = opts.Color.Colorize(
					fmt.Sprintf("%s%s%s", colors.SpecPrompt, valueType, colors.Reset))
			} else {
				defaultValuePrompt := defaultValue
				if secret {
					defaultValuePrompt = "[secret]"
				}

				prompt = opts.Color.Colorize(
					fmt.Sprintf("%s%s%s (%s)", colors.SpecPrompt, valueType, colors.Reset, defaultValuePrompt))
			}

			// Read the value.
			var err error
			if secret {
				value, err = cmdutil.ReadConsoleNoEcho(prompt)
				if err != nil {
					return "", err
				}
			} else {
				value, err = cmdutil.ReadConsole(prompt)
				if err != nil {
					return "", err
				}
			}
			value = strings.TrimSpace(value)

			// If the user simply hit ENTER, choose the default value.
			if value == "" {
				value = defaultValue
			}
		}

		// Ensure the resulting value is valid; note that we even validate the default, since sometimes
		// we will have invalid default values, like "" for the project name.
		if isValidFn != nil {
			validationError := isValidFn(value)
			if validationError == ErrRetryPromptForValue {
				continue
			} else if validationError != nil {
				// If validation failed, let the user know. If interactive, we will print the error and
				// prompt the user again; otherwise, in the case of --yes, we fail and report an error.
				var err error
				if valueType == "" {
					err = fmt.Errorf("Sorry, '%s' is not valid: %w", value, validationError)
				} else {
					err = fmt.Errorf("Sorry, '%s' is not a valid %s: %w", value, valueType, validationError)
				}
				if yes {
					return "", err
				}
				fmt.Printf("%s\n", err)
				continue
			}
		}

		break
	}

	return value, nil
}

var ErrRetryPromptForValue = errors.New("Try again")

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
