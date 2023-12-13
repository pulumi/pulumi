// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

const (
	noSelection     = "no"
	yesSelection    = "yes"
	refineSelection = "refine"
)

type pulumiAILanguage string

const (
	pulumiAILanguageTypeScript pulumiAILanguage = "TypeScript"
	pulumiAILanguageJavaScript pulumiAILanguage = "JavaScript"
	pulumiAILanguagePython     pulumiAILanguage = "Python"
	pulumiAILanguageGo         pulumiAILanguage = "Go"
	pulumiAILanguageCSharp     pulumiAILanguage = "C#"
	pulumiAILanguageJava       pulumiAILanguage = "Java"
	pulumiAILanguageYAML       pulumiAILanguage = "YAML"
)

var pulumiAILanguageMap map[string]pulumiAILanguage = map[string]pulumiAILanguage{
	"typescript": pulumiAILanguageTypeScript,
	"javascript": pulumiAILanguageJavaScript,
	"python":     pulumiAILanguagePython,
	"go":         pulumiAILanguageGo,
	"c#":         pulumiAILanguageCSharp,
	"java":       pulumiAILanguageJava,
	"yaml":       pulumiAILanguageYAML,
}

func (e *pulumiAILanguage) String() string {
	return string(*e)
}

func (e *pulumiAILanguage) Set(v string) error {
	switch strings.ToLower(v) {
	case "typescript", "javascript", "python", "go", "c#", "java", "yaml":
		*e = pulumiAILanguage(pulumiAILanguageMap[strings.ToLower(v)])
		return nil
	default:
		return errors.New(`must be one of "TypeScript", "JavaScript", "Python", "Go", "C#", "Java", "YAML"`)
	}
}

func (e *pulumiAILanguage) Type() string {
	return "pulumiAILanguage"
}

type pulumiAIModel string

const (
	pulumiAINoModel        pulumiAIModel = ""
	pulumiAIModelGPT3      pulumiAIModel = "gpt-3.5-turbo"
	pulumiAIModelGPT4      pulumiAIModel = "gpt-4"
	pulumiAIModelGPT4Turbo pulumiAIModel = "gpt-4-turbo"
)

func (e *pulumiAIModel) String() string {
	return string(*e)
}

func (e *pulumiAIModel) Set(v string) error {
	switch v {
	case "gpt-3.5-turbo", "gpt-4", "gpt-4-turbo", "":
		*e = pulumiAIModel(v)
		return nil
	default:
		return errors.New(`must be one of "gpt-3.5-turbo", "gpt-4", "gpt-4-turbo"`)
	}
}

func (e *pulumiAIModel) Type() string {
	return "pulumiAIModel"
}

func deriveAIOrTemplate(args newArgs) string {
	if args.aiPrompt != "" || args.aiLanguage != "" || args.aiModel != "" {
		return "ai"
	}
	return "template"
}

func shouldPromptForAIOrTemplate(args newArgs, userBackend backend.Backend) bool {
	return args.aiPrompt == "" && args.aiLanguage == "" && args.aiModel == "" && !args.templateMode && userBackend.Name() == "pulumi.com"
}

type aiPromptRequestBody struct {
	Language       pulumiAILanguage `json:"language"`
	Instructions   string           `json:"instructions"`
	ResponseMode   string           `json:"responseMode"`
	ConversationID string           `json:"conversationId"`
	ConnectionID   string           `json:"connectionId"`
}

type aiPromptWithModelRequestBody struct {
	Language       pulumiAILanguage `json:"language"`
	Instructions   string           `json:"instructions"`
	Model          pulumiAIModel    `json:"model"`
	ResponseMode   string           `json:"responseMode"`
	ConversationID string           `json:"conversationId"`
	ConnectionID   string           `json:"connectionId"`
}

// Iteratively prompt the user for input, sending their input as a prompt tp Pulumi AI
// Stream the response back to the console, and repeat until the user is done.
func runAINew(
	ctx context.Context,
	args newArgs,
	opts display.Options,
	userName string,
) (conversationURL string, err error) {
	languageOptions := []string{
		"TypeScript",
		"JavaScript",
		"Python",
		"Go",
		"C#",
		"Java",
		"YAML",
	}
	if args.aiLanguage == "" {
		if err = survey.AskOne(&survey.Select{
			Message: "Please select a language for your project:",
			Options: languageOptions,
		}, &args.aiLanguage, surveyIcons(opts.Color)); err != nil {
			return "", err
		}
	}
	var continuePrompt string
	var connectionID string
	var conversationID string
	for continuePrompt, conversationURL, conversationID, connectionID, err = runAINewPromptStep(
		opts,
		args.aiLanguage,
		args.aiModel,
		args.aiPrompt,
		args.yes,
		userName,
		"",
		"",
		"",
		"",
	); continuePrompt == refineSelection; continuePrompt,
		conversationURL,
		conversationID,
		connectionID,
		err = runAINewPromptStep(
		opts,
		args.aiLanguage,
		args.aiModel,
		args.aiPrompt,
		args.yes,
		userName,
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
	promptMessage string,
	conversationID string,
	connectionID string,
	userName string,
	language pulumiAILanguage,
	model pulumiAIModel,
) (string, string, string, error) {
	pulumiAIURL := env.AIServiceEndpoint.Value()
	if pulumiAIURL == "" {
		pulumiAIURL = "https://www.pulumi.com/ai"
	}
	parsedURL, err := url.Parse(pulumiAIURL)
	if err != nil {
		return "", "", "", err
	}
	requestPath := parsedURL.JoinPath("api", "chat")
	var requestBody interface{}
	if model != pulumiAINoModel {
		requestBody = aiPromptWithModelRequestBody{
			Language:       language,
			Instructions:   promptMessage,
			Model:          model,
			ResponseMode:   "code",
			ConversationID: conversationID,
			ConnectionID:   connectionID,
		}
	} else {
		requestBody = aiPromptRequestBody{
			Language:       language,
			Instructions:   promptMessage,
			ResponseMode:   "code",
			ConversationID: conversationID,
			ConnectionID:   connectionID,
		}
	}
	marshalledBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", "", "", err
	}
	userDataCookie := http.Cookie{Name: "pulumi_command_line_user_name", Value: userName}
	request, err := http.NewRequest("POST", requestPath.String(), bytes.NewReader(marshalledBody))
	if err != nil {
		return "", "", "", err
	}
	request.AddCookie(&userDataCookie)
	fmt.Println("Sending prompt to Pulumi AI...")
	res, err := http.DefaultClient.Do(request)
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
	projectURL := parsedURL.JoinPath("api", "project", url.PathEscape(fmt.Sprintf("%s.zip", conversationID))).String()
	conversationURL := parsedURL.JoinPath("conversations", conversationID).String()
	fmt.Println("View this conversation at:", conversationURL)
	return projectURL, connectionID, conversationID, nil
}

func runAINewPromptStep(
	opts display.Options,
	language pulumiAILanguage,
	model pulumiAIModel,
	prompt string,
	yes bool,
	userName string,
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
	if prompt == "" || currentContinueSelection != "" {
		if err := survey.AskOne(&survey.Input{
			Message: "Please input your prompt here (\"a static website on AWS behind a CDN\"):\n",
		}, &promptMessage, surveyIcons(opts.Color)); err != nil {
			return "", "", "", "", err
		}
	} else {
		promptMessage = prompt
	}
	conversationURLReturn, connectionIDReturn, conversationIDReturn, err = sendPromptToPulumiAI(
		promptMessage,
		conversationID,
		connectionID,
		userName,
		language,
		model,
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
	if !yes {
		if err := survey.AskOne(&survey.Select{
			Message: "Use this program as a template?",
			Options: continuePromptOptions,
			Description: func(opt string, _ int) string {
				return continuePromptOptionsDescriptions[opt]
			},
		}, &continueSelection, surveyIcons(opts.Color)); err != nil {
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
	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true

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
	}, &ai, surveyIcons(opts.Color)); err != nil {
		return "template", err
	}

	return ai, nil
}
