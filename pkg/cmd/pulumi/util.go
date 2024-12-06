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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/version"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Creates a secrets manager for an existing stack, using the stack to pick defaults if necessary and writing any
// changes back to the stack's configuration where applicable.
func createSecretsManagerForExistingStack(
	_ context.Context, ws pkgWorkspace.Context, stack backend.Stack, secretsProvider string,
	rotateSecretsProvider, creatingStack bool,
) error {
	// As part of creating the stack, we also need to configure the secrets provider for the stack.
	// We need to do this configuration step for cases where we will be using with the passphrase
	// secrets provider or one of the cloud-backed secrets providers.  We do not need to do this
	// for the Pulumi Cloud backend secrets provider.
	// we have an explicit flag to rotate the secrets manager ONLY when it's a passphrase!
	isDefaultSecretsProvider := secretsProvider == "" || secretsProvider == "default"

	// If we're creating the stack, it's the default secrets provider, and it's the cloud backend
	// return early to avoid probing for the project and stack config files, which otherwise
	// would fail when creating a stack from a directory that does not have a project file.
	if isDefaultSecretsProvider && creatingStack {
		if _, isCloud := stack.Backend().(httpstate.Backend); isCloud {
			return nil
		}
	}

	project, _, err := ws.ReadProject()
	if err != nil {
		return err
	}
	ps, err := loadProjectStack(project, stack)
	if err != nil {
		return err
	}

	oldConfig := deepcopy.Copy(ps).(*workspace.ProjectStack)
	if isDefaultSecretsProvider {
		_, err = stack.DefaultSecretManager(ps)
	} else if secretsProvider == passphrase.Type {
		_, err = passphrase.NewPromptingPassphraseSecretsManager(ps, rotateSecretsProvider)
	} else {
		// All other non-default secrets providers are handled by the cloud secrets provider which
		// uses a URL schema to identify the provider
		_, err = cloud.NewCloudSecretsManager(ps, secretsProvider, rotateSecretsProvider)
	}
	if err != nil {
		return err
	}

	// Handle if the configuration changed any of EncryptedKey, etc
	if needsSaveProjectStackAfterSecretManger(oldConfig, ps) {
		if err = workspace.SaveProjectStack(stack.Ref().Name().Q(), ps); err != nil {
			return fmt.Errorf("saving stack config: %w", err)
		}
	}

	return nil
}

// Creates a secrets manager for a new stack. If a stack configuration already exists (e.g. the user has created a
// Pulumi.<stack>.yaml file themselves, prior to stack initialisation), try to respect the settings therein. Otherwise,
// fall back to a default defined by the backend that will manage the stack.
func createSecretsManagerForNewStack(
	ws pkgWorkspace.Context,
	b backend.Backend,
	stackRef backend.StackReference,
	secretsProvider string,
) (*workspace.ProjectStack, bool, secrets.Manager, error) {
	var sm secrets.Manager

	// Attempt to read a stack configuration, since it's possible that the user may have supplied one even though the
	// stack has not actually been created yet. If we fail to read one, that's OK -- we'll just create a new one and
	// populate it as we go.
	var ps *workspace.ProjectStack
	project, _, err := ws.ReadProject()
	if err != nil {
		ps = &workspace.ProjectStack{}
	} else {
		ps, err = loadProjectStackByReference(project, stackRef)
		if err != nil {
			ps = &workspace.ProjectStack{}
		}
	}

	oldConfig := deepcopy.Copy(ps).(*workspace.ProjectStack)

	isDefaultSecretsProvider := secretsProvider == "" || secretsProvider == "default"
	if isDefaultSecretsProvider {
		sm, err = b.DefaultSecretManager(ps)
	} else if secretsProvider == passphrase.Type {
		sm, err = passphrase.NewPromptingPassphraseSecretsManager(ps, false /*rotateSecretsProvider*/)
	} else {
		sm, err = cloud.NewCloudSecretsManager(ps, secretsProvider, false /*rotateSecretsProvider*/)
	}
	if err != nil {
		return nil, false, nil, err
	}

	needsSave := needsSaveProjectStackAfterSecretManger(oldConfig, ps)
	return ps, needsSave, sm, err
}

// createStack creates a stack with the given name, and optionally selects it as the current.
func createStack(ctx context.Context, ws pkgWorkspace.Context,
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

type stackLoadOption int

const (
	// stackLoadOnly specifies that we should stop after loading the stack.
	stackLoadOnly stackLoadOption = 1 << iota

	// stackOfferNew is set if we want to allow the user
	// to create a stack if one was not found.
	stackOfferNew

	// stackSetCurrent is set if we want to change the current stack
	// once one is found or created.
	stackSetCurrent
)

// OfferNew reports whether the stackOfferNew flag is set.
func (o stackLoadOption) OfferNew() bool {
	return o&stackOfferNew != 0
}

// SetCurrent reports whether the stackSetCurrent flag is set.
func (o stackLoadOption) SetCurrent() bool {
	return o&stackSetCurrent != 0
}

// requireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func requireStack(ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
	stackName string, lopt stackLoadOption, opts display.Options,
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

		return createStack(ctx, ws, b, stackRef, root, nil, lopt.SetCurrent(), "")
	}

	return nil, fmt.Errorf("no stack named '%s' found", stackName)
}

func requireCurrentStack(
	ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager, lopt stackLoadOption, opts display.Options,
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
	return chooseStack(ctx, ws, b, lopt, opts)
}

// chooseStack will prompt the user to choose amongst the full set of stacks in the given backend.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func chooseStack(ctx context.Context, ws pkgWorkspace.Context,
	b backend.Backend, lopt stackLoadOption, opts display.Options,
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

		return createStack(ctx, ws, b, stackRef, root, nil, lopt.SetCurrent(), "")
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

// parseAndSaveConfigArray parses the config array and saves it as a config for
// the provided stack.
func parseAndSaveConfigArray(ws pkgWorkspace.Context, s backend.Stack, configArray []string, path bool) error {
	if len(configArray) == 0 {
		return nil
	}
	commandLineConfig, err := parseConfig(configArray, path)
	if err != nil {
		return err
	}

	if err = saveConfig(ws, s, commandLineConfig); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// readProjectForUpdate attempts to detect and read a Pulumi project for the current workspace. If
// the project is successfully detected and read, it is returned along with the path to its
// containing directory, which will be used as the root of the project's Pulumi program. If a
// client address is present, the returned project will always have the runtime set to "client"
// with the address option set to the client address.
func readProjectForUpdate(ws pkgWorkspace.Context, clientAddress string) (*workspace.Project, string, error) {
	proj, root, err := ws.ReadProject()
	if err != nil {
		return nil, "", err
	}
	if clientAddress != "" {
		proj.Runtime = workspace.NewProjectRuntimeInfo("client", map[string]interface{}{
			"address": clientAddress,
		})
	}
	return proj, root, nil
}

// readPolicyProject attempts to detect and read a Pulumi PolicyPack project for the current
// workspace. If the project is successfully detected and read, it is returned along with the path
// to its containing directory, which will be used as the root of the project's Pulumi program.
func readPolicyProject(pwd string) (*workspace.PolicyPackProject, string, string, error) {
	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectPolicyPackPathFrom(pwd)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to find current Pulumi project because of "+
			"an error when searching for the PulumiPolicy.yaml file (searching upwards from %s)"+": %w", pwd, err)
	} else if path == "" {
		return nil, "", "", fmt.Errorf("no PulumiPolicy.yaml project file found (searching upwards from %s)", pwd)
	}
	proj, err := workspace.LoadPolicyPack(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to load Pulumi policy project located at %q: %w", path, err)
	}

	return proj, path, filepath.Dir(path), nil
}

// updateFlagsToOptions ensures that the given update flags represent a valid combination.  If so, an UpdateOptions
// is returned with a nil-error; otherwise, the non-nil error contains information about why the combination is invalid.
func updateFlagsToOptions(interactive, skipPreview, yes, previewOnly bool) (backend.UpdateOptions, error) {
	switch {
	case !interactive && !yes && !skipPreview && !previewOnly:
		return backend.UpdateOptions{},
			errors.New("one of --yes, --skip-preview, or --preview-only must be specified in non-interactive mode")
	case skipPreview && previewOnly:
		return backend.UpdateOptions{},
			errors.New("--skip-preview and --preview-only cannot be used together")
	case yes && previewOnly:
		return backend.UpdateOptions{},
			errors.New("--yes and --preview-only cannot be used together")
	default:
		return backend.UpdateOptions{
			AutoApprove: yes,
			SkipPreview: skipPreview,
			PreviewOnly: previewOnly,
		}, nil
	}
}

func checkDeploymentVersionError(err error, stackName string) error {
	switch err {
	case stack.ErrDeploymentSchemaVersionTooOld:
		return fmt.Errorf("the stack '%s' is too old to be used by this version of the Pulumi CLI",
			stackName)
	case stack.ErrDeploymentSchemaVersionTooNew:
		return fmt.Errorf("the stack '%s' is newer than what this version of the Pulumi CLI understands. "+
			"Please update your version of the Pulumi CLI", stackName)
	}
	return fmt.Errorf("could not deserialize deployment: %w", err)
}

func getRefreshOption(proj *workspace.Project, refresh string) (bool, error) {
	// we want to check for an explicit --refresh or a --refresh=true or --refresh=false
	// refresh is assigned the empty string by default to distinguish the difference between
	// when the user actually interacted with the cli argument (`NoOptDefVal`)
	// and the default functionality today
	if refresh != "" {
		refreshDetails, boolErr := strconv.ParseBool(refresh)
		if boolErr != nil {
			// the user has passed a --refresh but with a random value that we don't support
			return false, errors.New("unable to determine value for --refresh")
		}
		return refreshDetails, nil
	}

	// the user has not specifically passed an argument on the cli to refresh but has set a Project option to refresh
	if proj.Options != nil && proj.Options.Refresh == "always" {
		return true, nil
	}

	// the default functionality right now is to always skip a refresh
	return false, nil
}

// we only want to log a secrets decryption for a Pulumi Cloud backend project
// we will allow any secrets provider to be used (Pulumi Cloud or passphrase/cloud/etc)
// we will log the message and not worry about the response. The types
// of messages we will log here will range from single secret decryption events
// to requesting a list of secrets in an individual event e.g. stack export
// the logging event will only happen during the `--show-secrets` path within the cli
func log3rdPartySecretsProviderDecryptionEvent(ctx context.Context, backend backend.Stack,
	secretName, commandName string,
) {
	if stack, ok := backend.(httpstate.Stack); ok {
		// we only want to do something if this is a Pulumi Cloud backend
		if be, ok := stack.Backend().(httpstate.Backend); ok {
			client := be.Client()
			if client != nil {
				id := backend.(httpstate.Stack).StackIdentifier()
				// we don't really care if these logging calls fail as they should not stop the execution
				if secretName != "" {
					contract.Assertf(commandName == "", "Command name must be empty if secret name is set")
					err := client.Log3rdPartySecretsProviderDecryptionEvent(ctx, id, secretName)
					contract.IgnoreError(err)
				}

				if commandName != "" {
					contract.Assertf(secretName == "", "Secret name must be empty if command name is set")
					err := client.LogBulk3rdPartySecretsProviderDecryptionEvent(ctx, id, commandName)
					contract.IgnoreError(err)
				}
			}
		}
	}
}

func installPolicyPackDependencies(ctx context.Context, root string, proj *workspace.PolicyPackProject) error {
	span := opentracing.SpanFromContext(ctx)
	// Bit of a hack here. Creating a plugin context requires a "program project", but we've only got a
	// policy project. Ideally we should be able to make a plugin context without any related project. But
	// fow now this works.
	projinfo := &engine.Projinfo{Proj: &workspace.Project{
		Main:    proj.Main,
		Runtime: proj.Runtime,
	}, Root: root}
	_, main, pluginCtx, err := engine.ProjectInfoContext(
		projinfo,
		nil,
		cmdutil.Diag(),
		cmdutil.Diag(),
		nil,
		false,
		span,
		nil,
	)
	if err != nil {
		return err
	}
	defer pluginCtx.Close()

	programInfo := plugin.NewProgramInfo(pluginCtx.Root, pluginCtx.Pwd, main, proj.Runtime.Options())
	lang, err := pluginCtx.Host.LanguageRuntime(proj.Runtime.Name(), programInfo)
	if err != nil {
		return fmt.Errorf("failed to load language plugin %s: %w", proj.Runtime.Name(), err)
	}

	if err = lang.InstallDependencies(plugin.InstallDependenciesRequest{Info: programInfo}); err != nil {
		return fmt.Errorf("installing dependencies failed: %w", err)
	}

	return nil
}

// Format a non-nil error that indicates some arguments are missing for a
// non-interactive session.
func missingNonInteractiveArg(args ...string) error {
	switch len(args) {
	case 0:
		panic("cannot create an error message for missing zero args")
	case 1:
		return fmt.Errorf("Must supply <%s> unless pulumi is run interactively", args[0])
	default:
		for i, s := range args {
			args[i] = "<" + s + ">"
		}
		return fmt.Errorf("Must supply %s and %s unless pulumi is run interactively",
			strings.Join(args[:len(args)-1], ", "), args[len(args)-1])
	}
}
