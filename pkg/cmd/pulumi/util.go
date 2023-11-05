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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/spf13/pflag"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	git "github.com/go-git/go-git/v5"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/cloud"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/util/tracing"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func hasDebugCommands() bool {
	return env.DebugCommands.Value()
}

func hasExperimentalCommands() bool {
	return env.Experimental.Value()
}

func useLegacyDiff() bool {
	return env.EnableLegacyDiff.Value()
}

func disableProviderPreview() bool {
	return env.DisableProviderPreview.Value()
}

func disableResourceReferences() bool {
	return env.DisableResourceReferences.Value()
}

func disableOutputValues() bool {
	return env.DisableOutputValues.Value()
}

// skipConfirmations returns whether or not confirmation prompts should
// be skipped. This should be used by pass any requirement that a --yes
// parameter has been set for non-interactive scenarios.
//
// This should NOT be used to bypass protections for destructive
// operations, such as those that will fail without a --force parameter.
func skipConfirmations() bool {
	return env.SkipConfirmations.Value()
}

// backendInstance is used to inject a backend mock from tests.
var backendInstance backend.Backend

func isFilestateBackend(opts display.Options) (bool, error) {
	if backendInstance != nil {
		return false, nil
	}

	// Try to read the current project
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return false, err
	}

	url, err := workspace.GetCurrentCloudURL(project)
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return filestate.IsFileStateBackendURL(url), nil
}

func loginToCloud(
	ctx context.Context,
	cloudURL string,
	project *workspace.Project,
	insecure bool,
	opts display.Options,
) (backend.Backend, error) {
	lm := httpstate.NewLoginManager()
	_, err := lm.Login(ctx, cloudURL, insecure, "pulumi", "Pulumi stacks", httpstate.WelcomeUser, true /*current*/, opts)
	if err != nil {
		return nil, err
	}
	return httpstate.New(cmdutil.Diag(), cloudURL, project, insecure)
}

func nonInteractiveCurrentBackend(ctx context.Context, project *workspace.Project) (backend.Backend, error) {
	if backendInstance != nil {
		return backendInstance, nil
	}

	url, err := workspace.GetCurrentCloudURL(project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	if filestate.IsFileStateBackendURL(url) {
		return filestate.New(ctx, cmdutil.Diag(), url, project)
	}

	insecure := workspace.GetCloudInsecure(url)
	_, err = httpstate.NewLoginManager().Current(ctx, url, insecure, true)
	if err != nil {
		return nil, err
	}
	return httpstate.New(cmdutil.Diag(), url, project, insecure)
}

func currentBackend(ctx context.Context, project *workspace.Project, opts display.Options) (backend.Backend, error) {
	if backendInstance != nil {
		return backendInstance, nil
	}

	url, err := workspace.GetCurrentCloudURL(project)
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	if filestate.IsFileStateBackendURL(url) {
		return filestate.New(ctx, cmdutil.Diag(), url, project)
	}

	return loginToCloud(ctx, url, project, workspace.GetCloudInsecure(url), opts)
}

// This is used to control the contents of the tracing header.
var tracingHeader = os.Getenv("PULUMI_TRACING_HEADER")

func commandContext() context.Context {
	ctx := context.Background()
	if cmdutil.IsTracingEnabled() {
		if cmdutil.TracingRootSpan != nil {
			ctx = opentracing.ContextWithSpan(ctx, cmdutil.TracingRootSpan)
		}

		tracingOptions := tracing.Options{
			PropagateSpans: true,
			TracingHeader:  tracingHeader,
		}
		ctx = tracing.ContextWithOptions(ctx, tracingOptions)
	}
	return ctx
}

func createSecretsManager(
	ctx context.Context, stack backend.Stack, secretsProvider string,
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

	project, _, err := readProject()
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
	if needsSaveProjectStackAfterSecretManger(stack, oldConfig, ps) {
		if err = workspace.SaveProjectStack(stack.Ref().Name().Q(), ps); err != nil {
			return fmt.Errorf("saving stack config: %w", err)
		}
	}

	return nil
}

// createStack creates a stack with the given name, and optionally selects it as the current.
func createStack(ctx context.Context,
	b backend.Backend, stackRef backend.StackReference,
	root string, opts *backend.CreateStackOptions, setCurrent bool,
	secretsProvider string,
) (backend.Stack, error) {
	stack, err := b.CreateStack(ctx, stackRef, root, opts)
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

	if err := createSecretsManager(ctx, stack, secretsProvider,
		false /*rotateSecretsManager*/, true /*creatingStack*/); err != nil {
		return nil, err
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
func requireStack(ctx context.Context,
	stackName string, lopt stackLoadOption, opts display.Options,
) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack(ctx, lopt, opts)
	}

	// Try to read the current project
	project, root, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	b, err := currentBackend(ctx, project, opts)
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

		return createStack(ctx, b, stackRef, root, nil, lopt.SetCurrent(), "")
	}

	return nil, fmt.Errorf("no stack named '%s' found", stackName)
}

func requireCurrentStack(ctx context.Context, lopt stackLoadOption, opts display.Options) (backend.Stack, error) {
	// Try to read the current project
	project, _, err := readProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	// Search for the current stack.
	b, err := currentBackend(ctx, project, opts)
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
	return chooseStack(ctx, b, lopt, opts)
}

// chooseStack will prompt the user to choose amongst the full set of stacks in the given backend.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func chooseStack(ctx context.Context,
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

	proj, root, err := readProject()
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
	}, &option, surveyIcons(opts.Color)); err != nil {
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

		return createStack(ctx, b, stackRef, root, nil, lopt.SetCurrent(), "")
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
func parseAndSaveConfigArray(s backend.Stack, configArray []string, path bool) error {
	if len(configArray) == 0 {
		return nil
	}
	commandLineConfig, err := parseConfig(configArray, path)
	if err != nil {
		return err
	}

	if err = saveConfig(s, commandLineConfig); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

// readProjectForUpdate attempts to detect and read a Pulumi project for the current workspace. If
// the project is successfully detected and read, it is returned along with the path to its
// containing directory, which will be used as the root of the project's Pulumi program. If a
// client address is present, the returned project will always have the runtime set to "client"
// with the address option set to the client address.
func readProjectForUpdate(clientAddress string) (*workspace.Project, string, error) {
	proj, root, err := readProject()
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

// readProject attempts to detect and read a Pulumi project for the current workspace. If the
// project is successfully detected and read, it is returned along with the path to its containing
// directory, which will be used as the root of the project's Pulumi program.
func readProject() (*workspace.Project, string, error) {
	proj, path, err := workspace.DetectProjectAndPath()
	if err != nil {
		return nil, "", err
	}

	return proj, filepath.Dir(path), nil
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

// anyWriter is an io.Writer that will set itself to `true` iff any call to `anyWriter.Write` is made with a
// non-zero-length slice. This can be used to determine whether or not any data was ever written to the writer.
type anyWriter bool

func (w *anyWriter) Write(d []byte) (int, error) {
	if len(d) > 0 {
		*w = true
	}
	return len(d), nil
}

// isGitWorkTreeDirty returns true if the work tree for the current directory's repository is dirty.
func isGitWorkTreeDirty(repoRoot string) (bool, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return false, err
	}

	gitStatusCmd := exec.Command(gitBin, "status", "--porcelain", "-z")
	var anyOutput anyWriter
	var stderr bytes.Buffer
	gitStatusCmd.Dir = repoRoot
	gitStatusCmd.Stdout = &anyOutput
	gitStatusCmd.Stderr = &stderr
	if err = gitStatusCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
		return false, fmt.Errorf("'git status' failed: %w", err)
	}

	return bool(anyOutput), nil
}

// getUpdateMetadata returns an UpdateMetadata object, with optional data about the environment
// performing the update.
func getUpdateMetadata(
	msg, root, execKind, execAgent string, updatePlan bool, env *workspace.Environment, flags *pflag.FlagSet,
) (*backend.UpdateMetadata, error) {
	m := &backend.UpdateMetadata{
		Message:     msg,
		Environment: make(map[string]string),
	}

	addPulumiCLIMetadataToEnvironment(m.Environment, flags, os.Environ)

	if err := addGitMetadata(root, m); err != nil {
		logging.V(3).Infof("errors detecting git metadata: %s", err)
	}

	addCIMetadataToEnvironment(m.Environment)

	addExecutionMetadataToEnvironment(m.Environment, execKind, execAgent)

	addUpdatePlanMetadataToEnvironment(m.Environment, updatePlan)

	addESCMetadataToEnvironment(m.Environment, env)

	return m, nil
}

// addPulumiCLIMetadataToEnvironment enriches updates with metadata to provide context about
// the system, environment, and options that an update is being performed with.
func addPulumiCLIMetadataToEnvironment(env map[string]string, flags *pflag.FlagSet, getEnviron func() []string) {
	// Pulumi Environment Variables (Name and Set/Truthiness Only)

	// Set up a map of all known bool environment variables.
	boolVars := map[string]struct{}{}
	for _, e := range declared.Variables() {
		_, ok := e.Value.(declared.BoolValue)
		if !ok {
			continue
		}
		boolVars[e.Name()] = struct{}{}
	}

	// Add all environment variables that start with "PULUMI_" and their set-ness or truthiness.
	for _, envvar := range getEnviron() {
		if !strings.HasPrefix(envvar, "PULUMI_") {
			// Not a Pulumi environment variable. Skip.
			continue
		}
		name, value, ok := strings.Cut(envvar, "=")
		if !ok {
			// Invalid environment variable. Skip.
			continue
		}
		flag := "set"
		if _, isBoolVar := boolVars[name]; isBoolVar {
			flag = "false"
			if cmdutil.IsTruthy(value) {
				flag = "true"
			}
		}

		// Only store the variable name and set-ness or truthiness not the value.
		env["pulumi.env."+name] = flag
	}

	// System information
	env["pulumi.version"] = version.Version
	env["pulumi.os"] = runtime.GOOS
	env["pulumi.arch"] = runtime.GOARCH

	// Guard against nil pointer dereference.
	if flags == nil {
		// No command was provided. Don't add flags.
		return
	}

	// Pulumi CLI Flags (Name Only)
	flags.Visit(func(f *pflag.Flag) {
		env["pulumi.flag."+f.Name] = func() string {
			truth, err := flags.GetBool(f.Name)
			switch {
			case err != nil:
				return "set" // not a bool flag
			case truth:
				return "true"
			default:
				return "false"
			}
		}()
	})
}

// addGitMetadata populate's the environment metadata bag with Git-related values.
func addGitMetadata(projectRoot string, m *backend.UpdateMetadata) error {
	var allErrors *multierror.Error

	// Gather git-related data as appropriate. (Returns nil, nil if no repo found.)
	repo, err := gitutil.GetGitRepository(projectRoot)
	if err != nil {
		return fmt.Errorf("detecting Git repository: %w", err)
	}
	if repo == nil {
		return nil
	}

	if err := AddGitRemoteMetadataToMap(repo, projectRoot, m.Environment); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	if err := addGitCommitMetadata(repo, projectRoot, m); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	return allErrors.ErrorOrNil()
}

// AddGitRemoteMetadataToMap reads the given git repo and adds its metadata to the given map bag.
func AddGitRemoteMetadataToMap(repo *git.Repository, projectRoot string, env map[string]string) error {
	var allErrors *multierror.Error

	// Get the remote URL for this repo.
	remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")
	if err != nil {
		return fmt.Errorf("detecting Git remote URL: %w", err)
	}
	if remoteURL == "" {
		return nil
	}

	// Check if the remote URL is a GitHub or a GitLab URL.
	if err := addVCSMetadataToEnvironment(remoteURL, env); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	// Add the repository root path.
	tree, err := repo.Worktree()
	if err != nil {
		allErrors = multierror.Append(allErrors, fmt.Errorf("detecting VCS root: %w", err))
	} else {
		rel, err := filepath.Rel(tree.Filesystem.Root(), projectRoot)
		if err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("detecting project root: %w", err))
		} else if !strings.HasPrefix(rel, "..") {
			env[backend.VCSRepoRoot] = filepath.ToSlash(rel)
		}
	}

	return allErrors.ErrorOrNil()
}

func addVCSMetadataToEnvironment(remoteURL string, env map[string]string) error {
	// GitLab, Bitbucket, Azure DevOps etc. repo slug if applicable.
	// We don't require a cloud-hosted VCS, so swallow errors.
	vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL)
	if err != nil {
		return fmt.Errorf("detecting VCS project information: %w", err)
	}
	env[backend.VCSRepoOwner] = vcsInfo.Owner
	env[backend.VCSRepoName] = vcsInfo.Repo
	env[backend.VCSRepoKind] = vcsInfo.Kind

	return nil
}

func addGitCommitMetadata(repo *git.Repository, repoRoot string, m *backend.UpdateMetadata) error {
	// When running in a CI/CD environment, the current git repo may be running from a
	// detached HEAD and may not have have the latest commit message. We fall back to
	// CI-system specific environment variables when possible.
	ciVars := ciutil.DetectVars()

	// Commit at HEAD
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("getting repository HEAD: %w", err)
	}

	hash := head.Hash()
	m.Environment[backend.GitHead] = hash.String()
	commit, commitErr := repo.CommitObject(hash)
	if commitErr != nil {
		return fmt.Errorf("getting HEAD commit info: %w", commitErr)
	}

	// If in detached head, will be "HEAD", and fallback to use value from CI/CD system if possible.
	// Otherwise, the value will be like "refs/heads/master".
	headName := head.Name().String()
	if headName == "HEAD" && ciVars.BranchName != "" {
		headName = ciVars.BranchName
	}
	if headName != "HEAD" {
		m.Environment[backend.GitHeadName] = headName
	}

	// If there is no message set manually, default to the Git commit's title.
	msg := strings.TrimSpace(commit.Message)
	if msg == "" && ciVars.CommitMessage != "" {
		msg = ciVars.CommitMessage
	}
	if m.Message == "" {
		m.Message = gitCommitTitle(msg)
	}

	// Store committer and author information.
	m.Environment[backend.GitCommitter] = commit.Committer.Name
	m.Environment[backend.GitCommitterEmail] = commit.Committer.Email
	m.Environment[backend.GitAuthor] = commit.Author.Name
	m.Environment[backend.GitAuthorEmail] = commit.Author.Email

	// If the worktree is dirty, set a bit, as this could be a mistake.
	isDirty, err := isGitWorkTreeDirty(repoRoot)
	if err != nil {
		return fmt.Errorf("checking git worktree dirty state: %w", err)
	}
	m.Environment[backend.GitDirty] = strconv.FormatBool(isDirty)

	return nil
}

// gitCommitTitle turns a commit message into its title, simply by taking the first line.
func gitCommitTitle(s string) string {
	if ixCR := strings.Index(s, "\r"); ixCR != -1 {
		s = s[:ixCR]
	}
	if ixLF := strings.Index(s, "\n"); ixLF != -1 {
		s = s[:ixLF]
	}
	return s
}

// addCIMetadataToEnvironment populates the environment metadata bag with CI/CD-related values.
func addCIMetadataToEnvironment(env map[string]string) {
	// Add the key/value pair to env, if there actually is a value.
	addIfSet := func(key, val string) {
		if val != "" {
			env[key] = val
		}
	}

	// Use our built-in CI/CD detection logic.
	vars := ciutil.DetectVars()
	if vars.Name == "" {
		return
	}
	env[backend.CISystem] = string(vars.Name)
	addIfSet(backend.CIBuildID, vars.BuildID)
	addIfSet(backend.CIBuildNumer, vars.BuildNumber)
	addIfSet(backend.CIBuildType, vars.BuildType)
	addIfSet(backend.CIBuildURL, vars.BuildURL)
	addIfSet(backend.CIPRHeadSHA, vars.SHA)
	addIfSet(backend.CIPRNumber, vars.PRNumber)
}

// addExecutionMetadataToEnvironment populates the environment metadata bag with execution-related values.
func addExecutionMetadataToEnvironment(env map[string]string, execKind, execAgent string) {
	// this comes from a hidden flag, so we restrict the set of allowed values
	switch execKind {
	case constant.ExecKindAutoInline:
		break
	case constant.ExecKindAutoLocal:
		break
	case constant.ExecKindCLI:
		break
	default:
		execKind = constant.ExecKindCLI
	}
	env[backend.ExecutionKind] = execKind
	if execAgent != "" {
		env[backend.ExecutionAgent] = execAgent
	}
}

// addUpdatePlanMetadataToEnvironment populates the environment metadata bag with update plan related values.
func addUpdatePlanMetadataToEnvironment(env map[string]string, updatePlan bool) {
	env[backend.UpdatePlan] = strconv.FormatBool(updatePlan)
}

// addESCMetadataToEnvironment populates the environment metadata bag with ESC-related metadata.
func addESCMetadataToEnvironment(env map[string]string, stackEnv *workspace.Environment) {
	if imports := stackEnv.Imports(); len(imports) != 0 {
		if bytes, err := json.Marshal(imports); err == nil {
			env[backend.StackEnvironments] = string(bytes)
		}
	}
}

// makeJSONString turns the given value into a JSON string.
// If multiline is true, the JSON will be formatted with indentation and a trailing newline.
func makeJSONString(v interface{}, multiline bool) (string, error) {
	var out bytes.Buffer

	// json.Marshal escapes HTML characters, which we don't want,
	// so change that with json.NewEncoder.
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)

	if multiline {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(v); err != nil {
		return "", err
	}

	// json.NewEncoder always adds a trailing newline. Remove it.
	bs := out.Bytes()
	if !multiline {
		if n := len(bs); n > 0 && bs[n-1] == '\n' {
			bs = bs[:n-1]
		}
	}

	return string(bs), nil
}

// printJSON simply prints out some object, formatted as JSON, using standard indentation.
func printJSON(v interface{}) error {
	return fprintJSON(os.Stdout, v)
}

// fprintJSON simply prints out some object, formatted as JSON, using standard indentation.
func fprintJSON(w io.Writer, v interface{}) error {
	jsonStr, err := makeJSONString(v, true /* multi line */)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, jsonStr)
	return err
}

// updateFlagsToOptions ensures that the given update flags represent a valid combination.  If so, an UpdateOptions
// is returned with a nil-error; otherwise, the non-nil error contains information about why the combination is invalid.
func updateFlagsToOptions(interactive, skipPreview, yes bool) (backend.UpdateOptions, error) {
	if !interactive && !yes {
		return backend.UpdateOptions{},
			errors.New("--yes must be passed in non-interactive mode")
	}

	return backend.UpdateOptions{
		AutoApprove: yes,
		SkipPreview: skipPreview,
	}, nil
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

func writePlan(path string, plan *deploy.Plan, enc config.Encrypter, showSecrets bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(f)

	deploymentPlan, err := stack.SerializePlan(plan, enc, showSecrets)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	return encoder.Encode(deploymentPlan)
}

func readPlan(path string, dec config.Decrypter, enc config.Encrypter) (*deploy.Plan, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(f)

	var deploymentPlan apitype.DeploymentPlanV1
	if err := json.NewDecoder(f).Decode(&deploymentPlan); err != nil {
		return nil, err
	}
	return stack.DeserializePlan(deploymentPlan, dec, enc)
}

func buildStackName(stackName string) (string, error) {
	// If we already have a slash (e.g. org/stack, or org/proj/stack) don't add the default org.
	if strings.Contains(stackName, "/") {
		return stackName, nil
	}

	// We never have a project at the point of calling buildStackName (only called from new), so we just pass
	// nil for the project and only check the global settings.
	defaultOrg, err := workspace.GetBackendConfigDefaultOrg(nil)
	if err != nil {
		return "", err
	}

	if defaultOrg != "" {
		return fmt.Sprintf("%s/%s", defaultOrg, stackName), nil
	}

	return stackName, nil
}

// we only want to log a secrets decryption for a Pulumi Cloud backend project
// we will allow any secrets provider to be used (Pulumi Cloud or self managed)
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

func surveyIcons(color colors.Colorization) survey.AskOpt {
	return survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question = survey.Icon{}
		icons.SelectFocus = survey.Icon{Text: color.Colorize(colors.BrightGreen + ">" + colors.Reset)}
	})
}

// Ask multiple survey based questions.
//
// Ctrl-C will go back in the stack, and valid answers will go forward.
func surveyStack(interactions ...func() error) error {
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

func promptUser(msg string, options []string, defaultOption string, colorization colors.Colorization) string {
	prompt := "\b" + colorization.Colorize(colors.SpecPrompt+msg+colors.Reset)
	surveycore.DisableColor = true
	surveyIcons := survey.WithIcons(func(icons *survey.IconSet) {
		icons.Question = survey.Icon{}
		icons.SelectFocus = survey.Icon{Text: colorization.Colorize(colors.BrightGreen + ">" + colors.Reset)}
	})

	var response string
	if err := survey.AskOne(&survey.Select{
		Message: prompt,
		Options: options,
		Default: defaultOption,
	}, &response, surveyIcons); err != nil {
		return ""
	}
	return response
}

func printTable(table cmdutil.Table, opts *cmdutil.TableRenderOptions) {
	fmt.Print(renderTable(table, opts))
}

func renderTable(table cmdutil.Table, opts *cmdutil.TableRenderOptions) string {
	if opts == nil {
		opts = &cmdutil.TableRenderOptions{}
	}
	if len(opts.HeaderStyle) == 0 {
		style := make([]colors.Color, len(table.Headers))
		for i := range style {
			style[i] = colors.SpecHeadline
		}
		opts.HeaderStyle = style
	}
	if opts.Color == "" {
		opts.Color = cmdutil.GetGlobalColorization()
	}
	return table.Render(opts)
}
