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

package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/version"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var ConfigFile string

func GetProjectStackPath(stack backend.Stack) (string, error) {
	if ConfigFile == "" {
		_, path, err := workspace.DetectProjectStackPath(stack.Ref().Name().Q())
		return path, err
	}
	return ConfigFile, nil
}

func LoadProjectStack(project *workspace.Project, stack backend.Stack) (*workspace.ProjectStack, error) {
	return loadProjectStackByReference(project, stack.Ref())
}

func loadProjectStackByReference(
	project *workspace.Project,
	stackRef backend.StackReference,
) (*workspace.ProjectStack, error) {
	if ConfigFile == "" {
		return workspace.DetectProjectStack(stackRef.Name().Q())
	}
	return workspace.LoadProjectStack(project, ConfigFile)
}

func SaveProjectStack(stack backend.Stack, ps *workspace.ProjectStack) error {
	if ConfigFile == "" {
		return workspace.SaveProjectStack(stack.Ref().Name().Q(), ps)
	}
	return ps.Save(ConfigFile)
}

type LoadOption int

const (
	// LoadOnly specifies that we should stop after loading the stack.
	LoadOnly LoadOption = 1 << iota

	// OfferNew is set if we want to allow the user
	// to create a stack if one was not found.
	OfferNew

	// SetCurrent is set if we want to change the current stack
	// once one is found or created.
	SetCurrent
)

// OfferNew reports whether the stackOfferNew flag is set.
func (o LoadOption) OfferNew() bool {
	return o&OfferNew != 0
}

// SetCurrent reports whether the stackSetCurrent flag is set.
func (o LoadOption) SetCurrent() bool {
	return o&SetCurrent != 0
}

// RequireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func RequireStack(ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
	stackName string, lopt LoadOption, opts display.Options,
) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack(ctx, ws, lm, lopt, opts)
	}

	// Try to read the current project
	project, root, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	b, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, opts)
	if err != nil {
		return nil, err
	}

	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}

	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	if stack != nil {
		return stack, err
	}

	// No stack was found.  If we're in a terminal, prompt to create one.
	if lopt.OfferNew() && cmdutil.Interactive() {
		fmt.Printf("The stack '%s' does not exist.\n", stackName)
		fmt.Printf("\n")
		_, err = cmdutil.ReadConsole("If you would like to create this stack now, please press <ENTER>, otherwise " +
			"press ^C")
		if err != nil {
			return nil, err
		}

		return CreateStack(ctx, ws, b, stackRef, root, nil, lopt.SetCurrent(), "")
	}

	return nil, fmt.Errorf("no stack named '%s' found", stackName)
}

func requireCurrentStack(
	ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, lopt LoadOption, opts display.Options,
) (backend.Stack, error) {
	// Try to read the current project
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	// Search for the current stack.
	b, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, opts)
	if err != nil {
		return nil, err
	}
	stack, err := state.CurrentStack(ctx, b)
	if err != nil {
		return nil, err
	} else if stack != nil {
		return stack, nil
	}

	// If no current stack exists, and we are interactive, prompt to select or create one.
	return ChooseStack(ctx, ws, b, lopt, opts)
}

// ChooseStack will prompt the user to choose amongst the full set of stacks in the given backend.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func ChooseStack(ctx context.Context, ws pkgWorkspace.Context,
	b backend.Backend, lopt LoadOption, opts display.Options,
) (backend.Stack, error) {
	// Prepare our error in case we need to issue it.  Bail early if we're not interactive.
	var chooseStackErr string
	if lopt.OfferNew() {
		chooseStackErr = "no stack selected; please use `pulumi stack select` or `pulumi stack init` to choose one"
	} else {
		chooseStackErr = "no stack selected; please use `pulumi stack select` to choose one"
	}
	if !cmdutil.Interactive() {
		return nil, errors.New(chooseStackErr)
	}

	proj, root, err := ws.ReadProject()
	if err != nil {
		return nil, err
	}

	// List stacks as available options.
	project := string(proj.Name)

	var (
		allSummaries []backend.StackSummary
		inContToken  backend.ContinuationToken
	)
	for {
		summaries, outContToken, err := b.ListStacks(ctx, backend.ListStacksFilter{Project: &project}, inContToken)
		if err != nil {
			return nil, fmt.Errorf("could not query backend for stacks: %w", err)
		}

		allSummaries = append(allSummaries, summaries...)

		if outContToken == nil {
			break
		}
		inContToken = outContToken
	}

	options := slice.Prealloc[string](len(allSummaries))
	for _, summary := range allSummaries {
		name := summary.Name().String()
		options = append(options, name)
	}
	sort.Strings(options)

	// If a stack is already selected, make that the default.
	var defaultOption string
	currStack, currErr := state.CurrentStack(ctx, b)
	contract.IgnoreError(currErr)
	if currStack != nil {
		defaultOption = currStack.Ref().String()
	}

	// If we are offering to create a new stack, add that to the end of the list.
	// Otherwise, default to a stack if one exists – otherwise pressing enter will result in
	// the empty string being passed (see https://github.com/go-survey/survey/issues/342).
	const newOption = "<create a new stack>"
	if lopt.OfferNew() {
		options = append(options, newOption)
		// If we're offering the option to make a new stack AND we don't have a default current stack then
		// make the new option the default
		if defaultOption == "" {
			defaultOption = newOption
		}
	} else if len(options) == 0 {
		// If no options are available, we can't offer a choice!
		return nil, errors.New("this command requires a stack, but there are none")
	} else if defaultOption == "" {
		defaultOption = options[0]
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	message := "\rPlease choose a stack"
	if lopt.OfferNew() {
		message += ", or create a new one:"
	} else {
		message += ":"
	}
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	var option string
	if err = survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
		Default: defaultOption,
	}, &option, ui.SurveyIcons(opts.Color)); err != nil {
		return nil, errors.New(chooseStackErr)
	}

	if option == newOption {
		hint := "Please enter your desired stack name"
		if b.SupportsOrganizations() {
			hint += ".\nTo create a stack in an organization, " +
				"use the format <org-name>/<stack-name> (e.g. `acmecorp/dev`)"
		}
		stackName, readErr := cmdutil.ReadConsole(hint)
		if readErr != nil {
			return nil, readErr
		}

		stackRef, parseErr := b.ParseStackReference(stackName)
		if parseErr != nil {
			return nil, parseErr
		}

		return CreateStack(ctx, ws, b, stackRef, root, nil, lopt.SetCurrent(), "")
	}

	// With the stack name selected, look it up from the backend.
	stackRef, err := b.ParseStackReference(option)
	if err != nil {
		return nil, fmt.Errorf("parsing selected stack: %w", err)
	}
	// GetStack may return (nil, nil) if the stack isn't found.
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, fmt.Errorf("getting selected stack: %w", err)
	}
	if stack == nil {
		return nil, fmt.Errorf("no stack named '%s' found", stackRef)
	}

	// If setCurrent is true, we'll persist this choice so it'll be used for future CLI operations.
	if lopt.SetCurrent() {
		if err = state.SetCurrentStack(stackRef.String()); err != nil {
			return nil, err
		}
	}

	return stack, nil
}

// CreateStack creates a stack with the given name, and optionally selects it as the current.
func CreateStack(ctx context.Context, ws pkgWorkspace.Context,
	b backend.Backend, stackRef backend.StackReference,
	root string, opts *backend.CreateStackOptions, setCurrent bool,
	secretsProvider string,
) (backend.Stack, error) {
	ps, needsSave, sm, err := createSecretsManagerForNewStack(ws, b, stackRef, secretsProvider)
	if err != nil {
		return nil, fmt.Errorf("could not create secrets manager for new stack: %w", err)
	}

	// If we have a non-empty secrets manager, we'll send it off to the backend as part of the initial state to be stored
	// for the stack.
	var initialState *apitype.UntypedDeployment
	if sm != nil {
		m := deploy.Manifest{
			Time:    time.Now(),
			Version: version.Version,
			Plugins: nil,
		}
		m.Magic = m.NewMagic()

		d := &apitype.DeploymentV3{
			Manifest: m.Serialize(),
			SecretsProviders: &apitype.SecretsProvidersV1{
				Type:  sm.Type(),
				State: sm.State(),
			},
		}
		dJSON, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("could not serialize initial state for new stack: %w", err)
		}

		initialState = &apitype.UntypedDeployment{
			Version:    3,
			Deployment: dJSON,
		}
	}

	stack, err := b.CreateStack(ctx, stackRef, root, initialState, opts)
	if err != nil {
		// If it's a well-known error, don't wrap it.
		if _, ok := err.(*backend.StackAlreadyExistsError); ok {
			return nil, err
		}
		if _, ok := err.(*backend.OverStackLimitError); ok {
			return nil, err
		}
		return nil, fmt.Errorf("could not create stack: %w", err)
	}

	// Now that we've created the stack, we'll write out any necessary configuration changes.
	if needsSave {
		err = workspace.SaveProjectStack(stack.Ref().Name().Q(), ps)
		if err != nil {
			return nil, fmt.Errorf("saving stack config: %w", err)
		}
	}

	if setCurrent {
		if err = state.SetCurrentStack(stack.Ref().String()); err != nil {
			return nil, err
		}
	}

	return stack, nil
}
