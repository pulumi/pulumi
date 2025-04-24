// Copyright 2016-2024, Pulumi Corporation.
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
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"

	survey "github.com/AlecAivazis/survey/v2"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

const (
	noSelection     = "no"
	yesSelection    = "yes"
	refineSelection = "refine"
)

func deriveAIOrTemplate(args newArgs) string {
	if args.aiPrompt != "" || args.aiLanguage != "" {
		return "ai"
	}
	return "template"
}

func shouldPromptForAIOrTemplate(args newArgs, userBackend backend.Backend) bool {
	_, isHTTPBackend := userBackend.(httpstate.Backend)
	return args.aiPrompt == "" &&
		args.aiLanguage == "" &&
		!args.templateMode &&
		isHTTPBackend
}

// Iteratively prompt the user for input, sending their input as a prompt tp Pulumi AI
// Stream the response back to the console, and repeat until the user is done.
func runAINew(
	ctx context.Context,
	args newArgs,
	opts display.Options,
	backend httpstate.Backend,
) (conversationURL string, err error) {
	languageOptions := make([]string, 0, len(httpstate.PulumiAILanguageOptions))
	for _, language := range httpstate.PulumiAILanguageOptions {
		languageOptions = append(languageOptions, language.String())
	}
	var rawLanguageSelect string
	if args.aiLanguage == "" {
		if err = survey.AskOne(&survey.Select{
			Message: "Please select a language for your project:",
			Options: languageOptions,
		}, &rawLanguageSelect, ui.SurveyIcons(opts.Color)); err != nil {
			return "", err
		}
		err = args.aiLanguage.Set(rawLanguageSelect)
		if err != nil {
			return "", err
		}
	}
	var continuePrompt string
	var connectionID string
	var conversationID string
	for continuePrompt, conversationURL, conversationID, connectionID, err = runAINewPromptStep(
		ctx,
		backend,
		opts,
		args,
		"",
		"",
		"",
		"",
	); continuePrompt == refineSelection; continuePrompt,
		conversationURL,
		conversationID,
		connectionID,
		err = runAINewPromptStep(
		ctx,
		backend,
		opts,
		args,
		continuePrompt,
		connectionID,
		conversationURL,
		conversationID,
	) {
		if err != nil {
			return "", err
		}
	}
	return conversationURL, err
}

func sendPromptToPulumiAI(
	ctx context.Context,
	backend httpstate.Backend,
	promptMessage string,
	conversationID string,
	connectionID string,
	language httpstate.PulumiAILanguage,
) (string, string, string, error) {
	pulumiAIURL := env.AIServiceEndpoint.Value()
	if pulumiAIURL == "" {
		pulumiAIURL = "https://www.pulumi.com/ai"
	}
	parsedURL, err := url.Parse(pulumiAIURL)
	if err != nil {
		return "", "", "", err
	}
	requestBody := httpstate.AIPromptRequestBody{
		Language:       language,
		Instructions:   promptMessage,
		ResponseMode:   "code",
		ConversationID: conversationID,
		ConnectionID:   connectionID,
	}
	fmt.Println("Sending prompt to Pulumi AI...")
	res, err := backend.PromptAI(ctx, requestBody)
	if err != nil {
		return "", "", "", err
	}
	defer res.Body.Close()
	reader := bufio.NewReader(res.Body)
	fmt.Println("Pulumi AI response:")
	for {
		chunk, err := reader.ReadByte()
		if err != nil && err.Error() != "EOF" {
			return "", "", "", err
		}
		fmt.Print(string(chunk))
		if err != nil && err.Error() == "EOF" {
			// Add a newline to ensure we don't overwrite the last line of the Pulumi AI response
			fmt.Println()
			break
		}
	}
	conversationID = res.Header.Get("x-conversation-id")
	connectionID = res.Header.Get("x-connection-id")
	projectURL := parsedURL.JoinPath("api", "project", url.PathEscape(conversationID+".zip")).String()
	conversationURL := parsedURL.JoinPath("conversations", conversationID).String()
	fmt.Println("View this conversation at: ", conversationURL)
	return projectURL, connectionID, conversationID, nil
}

const initialPromptPrompt = `What cloud infrastructure would you like to build?
(try something like "a static website on AWS behind a CDN")`

const refinePromptPrompt = "Tell Pulumi AI how to refine the previous program:"

var errEmptyPrompt = errors.New("prompt cannot be empty")

func isValidAiPrompt(prompt string) error {
	if prompt == "" {
		return errEmptyPrompt
	}
	return nil
}

func promptForAiMessage(prompt promptForValueFunc, opts display.Options, isInitial bool) (string, error) {
	for {
		if isInitial {
			fmt.Println(initialPromptPrompt)
		} else {
			fmt.Println(refinePromptPrompt)
		}
		promptMessage, err := prompt(false, "", "", false, isValidAiPrompt, opts)
		if err != nil {
			if errors.Is(err, errEmptyPrompt) {
				continue
			}

			return promptMessage, err
		}

		return promptMessage, nil
	}
}

func runAINewPromptStep(
	ctx context.Context,
	backend httpstate.Backend,
	opts display.Options,
	args newArgs,
	currentContinueSelection string,
	connectionID string,
	conversationURL string,
	conversationID string,
) (continueSelection string,
	conversationURLReturn string,
	conversationIDReturn string,
	connectionIDReturn string,
	err error,
) {
	var promptMessage string
	if args.aiPrompt == "" || currentContinueSelection != "" {
		isInitial := currentContinueSelection == ""
		promptMessage, err = promptForAiMessage(args.prompt, opts, isInitial)
		if err != nil {
			return "", "", "", "", err
		}
	} else {
		promptMessage = args.aiPrompt
	}
	conversationURLReturn, connectionIDReturn, conversationIDReturn, err = sendPromptToPulumiAI(
		ctx,
		backend,
		promptMessage,
		conversationID,
		connectionID,
		args.aiLanguage,
	)
	if err != nil {
		return "", "", "", "", err
	}
	if connectionID != "" && connectionID != connectionIDReturn {
		connectionIDReturn = connectionID
		fmt.Println("Connection ID changed, please restart the prompt")
	}
	if conversationID != "" && conversationID != conversationIDReturn {
		return "", "", "", "", fmt.Errorf("conversation id %s changed to %s", conversationID, conversationIDReturn)
	}
	continuePromptOptions := []string{
		refineSelection,
		yesSelection,
		noSelection,
	}
	continuePromptOptionsDescriptions := map[string]string{
		refineSelection: "Write a prompt to further refine this program",
		yesSelection:    "Use this program to create the project",
		noSelection:     "Abort the prompt and exit",
	}
	if !args.yes {
		if err := survey.AskOne(&survey.Select{
			Message: "Use this program as a template?",
			Options: continuePromptOptions,
			Description: func(opt string, _ int) string {
				return continuePromptOptionsDescriptions[opt]
			},
		}, &continueSelection, ui.SurveyIcons(opts.Color)); err != nil {
			return "", "", "", "", err
		}
		if continueSelection == noSelection {
			return "", "", "", "", errors.New("aborting prompt")
		}
	} else {
		continueSelection = yesSelection
	}
	return continueSelection, conversationURLReturn, conversationIDReturn, connectionIDReturn, nil
}

// Prompt the user to decide whether they'd like to enter an interactive AI prompt or use a template.
func chooseWithAIOrTemplate(opts display.Options) (string, error) {
	options := []string{
		"template",
		"ai",
	}

	optionsDescriptionMap := map[string]string{
		"template": "Create a new Pulumi project using a template",
		"ai":       "Create a new Pulumi project using Pulumi AI",
	}

	var ai string
	if err := survey.AskOne(&survey.Select{
		Message: "Would you like to create a project from a template or using a Pulumi AI prompt?",
		Options: options,
		Description: func(opt string, _ int) string {
			return optionsDescriptionMap[opt]
		},
	}, &ai, ui.SurveyIcons(opts.Color)); err != nil {
		return "template", err
	}

	return ai, nil
}
