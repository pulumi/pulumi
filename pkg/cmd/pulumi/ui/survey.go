package ui

import ui "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/ui"

var ErrRetryPromptForValue = ui.ErrRetryPromptForValue

func SurveyIcons(color colors.Colorization) survey.AskOpt {
	return ui.SurveyIcons(color)
}

// Ask multiple survey based questions.
// 
// Ctrl-C will go back in the stack, and valid answers will go forward.
func SurveyStack(interactions ...func() error) error {
	return ui.SurveyStack(interactions...)
}

// PromptForValue prompts the user for a value with a defaultValue preselected. Hitting enter accepts the
// default. If yes is true, defaultValue is returned without prompting. isValidFn is an optional parameter;
// when specified, it will be run to validate that value entered. When this function returns a non nil error
// validation is assumed to have failed and an error is printed. The error returned by isValidFn is also displayed
// to provide information about why the validation failed. A period is appended to this message. `PromptForValue` then
// prompts again.
func PromptForValue(yes bool, valueType string, defaultValue string, secret bool, isValidFn func(string) error, opts display.Options) (string, error) {
	return ui.PromptForValue(yes, valueType, defaultValue, secret, isValidFn, opts)
}

// PromptUserSkippable wraps over promptUser making it skippable through the "yes" parameter
// commonly being the value of the --yes flag used in each command.
// If yes is true, defaultValue is returned without prompting.
func PromptUserSkippable(yes bool, msg string, options []string, defaultOption string, colorization colors.Colorization) string {
	return ui.PromptUserSkippable(yes, msg, options, defaultOption, colorization)
}

// PromptUser prompts the user for a value with a list of options. Hitting enter accepts the
// default.
func PromptUser(msg string, options []string, defaultOption string, colorization colors.Colorization, surveyAskOpts ...survey.AskOpt) string {
	return ui.PromptUser(msg, options, defaultOption, colorization, surveyAskOpts...)
}

// PromptUserMultiSkippable wraps over promptUserMulti making it skippable through the "yes" parameter
// commonly being the value of the --yes flag used in each command.
// If yes is true, defaultValue is returned without prompting.
func PromptUserMultiSkippable(yes bool, msg string, options []string, defaultOptions []string, colorization colors.Colorization) []string {
	return ui.PromptUserMultiSkippable(yes, msg, options, defaultOptions, colorization)
}

// PromptUserMulti prompts the user for a value with a list of options, allowing to select none or multiple options.
// defaultOptions is a set of values to be selected by default.
func PromptUserMulti(msg string, options []string, defaultOptions []string, colorization colors.Colorization) []string {
	return ui.PromptUserMulti(msg, options, defaultOptions, colorization)
}

func ConfirmPrompt(prompt string, name string, opts display.Options) bool {
	return ui.ConfirmPrompt(prompt, name, opts)
}

