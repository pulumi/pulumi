// Copyright 2016-2018, Pulumi Corporation.
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
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	opentracing "github.com/opentracing/opentracing-go"

	git "github.com/go-git/go-git/v5"
	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/pkg/v3/util/tracing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func hasDebugCommands() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG_COMMANDS"))
}

func hasExperimentalCommands() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_EXPERIMENTAL"))
}

func useLegacyDiff() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_ENABLE_LEGACY_DIFF"))
}

func disableProviderPreview() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_DISABLE_PROVIDER_PREVIEW"))
}

func disableResourceReferences() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_DISABLE_RESOURCE_REFERENCES"))
}

func disableOutputValues() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_DISABLE_OUTPUT_VALUES"))
}

// skipConfirmations returns whether or not confirmation prompts should
// be skipped. This should be used by pass any requirement that a --yes
// parameter has been set for non-interactive scenarios.
//
// This should NOT be used to bypass protections for destructive
// operations, such as those that will fail without a --force parameter.
func skipConfirmations() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_SKIP_CONFIRMATIONS"))
}

// backendInstance is used to inject a backend mock from tests.
var backendInstance backend.Backend

func isFilestateBackend(opts display.Options) (bool, error) {
	if backendInstance != nil {
		return false, nil
	}

	url, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return false, fmt.Errorf("could not get cloud url: %w", err)
	}

	return filestate.IsFileStateBackendURL(url), nil
}

func nonInteractiveCurrentBackend(ctx context.Context) (backend.Backend, error) {
	if backendInstance != nil {
		return backendInstance, nil
	}

	url, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	if filestate.IsFileStateBackendURL(url) {
		return filestate.New(cmdutil.Diag(), url)
	}
	return httpstate.NewLoginManager().Current(ctx, cmdutil.Diag(), url)
}

func currentBackend(ctx context.Context, opts display.Options) (backend.Backend, error) {
	if backendInstance != nil {
		return backendInstance, nil
	}

	url, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return nil, fmt.Errorf("could not get cloud url: %w", err)
	}

	if filestate.IsFileStateBackendURL(url) {
		return filestate.New(cmdutil.Diag(), url)
	}
	return httpstate.NewLoginManager().Login(ctx, cmdutil.Diag(), url, opts)
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
	rotatePassphraseSecretsProvider, creatingStack bool) error {

	// As part of creating the stack, we also need to configure the secrets provider for the stack.
	// We need to do this configuration step for cases where we will be using with the passphrase
	// secrets provider or one of the cloud-backed secrets providers.  We do not need to do this
	// for the Pulumi service backend secrets provider.
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

	configFile, err := getProjectStackPath(stack)
	if err != nil {
		return err
	}

	if isDefaultSecretsProvider {
		_, err = stack.DefaultSecretManager(configFile)
		return err
	}

	if secretsProvider == passphrase.Type {
		if _, pharseErr := filestate.NewPassphraseSecretsManager(stack.Ref().Name(), configFile,
			rotatePassphraseSecretsProvider); pharseErr != nil {
			return pharseErr
		}
	} else {
		// All other non-default secrets providers are handled by the cloud secrets provider which
		// uses a URL schema to identify the provider
		if _, secretsErr := newCloudSecretsManager(stack.Ref().Name(), configFile, secretsProvider); secretsErr != nil {
			return secretsErr
		}
	}

	return nil
}

// createStack creates a stack with the given name, and optionally selects it as the current.
func createStack(ctx context.Context,
	b backend.Backend, stackRef backend.StackReference, opts interface{}, setCurrent bool,
	secretsProvider string) (backend.Stack, error) {

	stack, err := b.CreateStack(ctx, stackRef, opts)
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

// requireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func requireStack(ctx context.Context,
	stackName string, offerNew bool, opts display.Options, setCurrent bool) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack(ctx, offerNew, opts, setCurrent)
	}

	b, err := currentBackend(ctx, opts)
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
	if offerNew && cmdutil.Interactive() {
		fmt.Printf("The stack '%s' does not exist.\n", stackName)
		fmt.Printf("\n")
		_, err = cmdutil.ReadConsole("If you would like to create this stack now, please press <ENTER>, otherwise " +
			"press ^C")
		if err != nil {
			return nil, err
		}

		return createStack(ctx, b, stackRef, nil, setCurrent, "")
	}

	return nil, fmt.Errorf("no stack named '%s' found", stackName)
}

func requireCurrentStack(
	ctx context.Context, offerNew bool,
	opts display.Options, setCurrent bool) (backend.Stack, error) {
	// Search for the current stack.
	b, err := currentBackend(ctx, opts)
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
	return chooseStack(ctx, b, offerNew, opts, setCurrent)
}

// chooseStack will prompt the user to choose amongst the full set of stacks in the given backend.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func chooseStack(ctx context.Context,
	b backend.Backend, offerNew bool, opts display.Options, setCurrent bool) (backend.Stack, error) {

	// Prepare our error in case we need to issue it.  Bail early if we're not interactive.
	var chooseStackErr string
	if offerNew {
		chooseStackErr = "no stack selected; please use `pulumi stack select` or `pulumi stack init` to choose one"
	} else {
		chooseStackErr = "no stack selected; please use `pulumi stack select` to choose one"
	}
	if !cmdutil.Interactive() {
		return nil, errors.New(chooseStackErr)
	}

	proj, err := workspace.DetectProject()
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

	var options []string
	for _, summary := range allSummaries {
		name := summary.Name().String()
		options = append(options, name)
	}
	sort.Strings(options)

	// If we are offering to create a new stack, add that to the end of the list.
	const newOption = "<create a new stack>"
	if offerNew {
		options = append(options, newOption)
	} else if len(options) == 0 {
		// If no options are available, we can't offer a choice!
		return nil, errors.New("this command requires a stack, but there are none")
	}

	// If a stack is already selected, make that the default.
	var current string
	currStack, currErr := state.CurrentStack(ctx, b)
	contract.IgnoreError(currErr)
	if currStack != nil {
		current = currStack.Ref().String()
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = opts.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a stack"
	if offerNew {
		message += ", or create a new one:"
	} else {
		message += ":"
	}
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	cmdutil.EndKeypadTransmitMode()

	var option string
	if err = survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
		Default: current,
	}, &option, nil); err != nil {
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

		return createStack(ctx, b, stackRef, nil, setCurrent, "")
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
	if setCurrent {
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
	proj, path, err := readProjectWithPath()
	if err != nil {
		return nil, "", err
	}

	return proj, filepath.Dir(path), nil
}

// readProjectWithPath attempts to detect and read a Pulumi project for the current workspace. If
// the project is successfully detected and read, it is returned along with the path to the project
// file, which will be used as the root of the project's Pulumi program.
//
// If a project is not found while searching and no other error occurs, workspace.ErrProjectNotFound
// is returned.
func readProjectWithPath() (*workspace.Project, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectProjectPathFrom(pwd)
	if err != nil {
		return nil, "", err
	}
	proj, err := workspace.LoadProject(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load Pulumi project located at %q: %w", path, err)
	}

	return proj, path, nil
}

// readPolicyProject attempts to detect and read a Pulumi PolicyPack project for the current
// workspace. If the project is successfully detected and read, it is returned along with the path
// to its containing directory, which will be used as the root of the project's Pulumi program.
func readPolicyProject() (*workspace.PolicyPackProject, string, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", "", err
	}

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
func getUpdateMetadata(msg, root, execKind, execAgent string, updatePlan bool) (*backend.UpdateMetadata, error) {
	m := &backend.UpdateMetadata{
		Message:     msg,
		Environment: make(map[string]string),
	}

	if err := addGitMetadata(root, m); err != nil {
		logging.V(3).Infof("errors detecting git metadata: %s", err)
	}

	addCIMetadataToEnvironment(m.Environment)

	addExecutionMetadataToEnvironment(m.Environment, execKind, execAgent)

	addUpdatePlanMetadataToEnvironment(m.Environment, updatePlan)

	return m, nil
}

// addGitMetadata populate's the environment metadata bag with Git-related values.
func addGitMetadata(repoRoot string, m *backend.UpdateMetadata) error {
	var allErrors *multierror.Error

	// Gather git-related data as appropriate. (Returns nil, nil if no repo found.)
	repo, err := gitutil.GetGitRepository(repoRoot)
	if err != nil {
		return fmt.Errorf("detecting Git repository: %w", err)
	}
	if repo == nil {
		return nil
	}

	if err := AddGitRemoteMetadataToMap(repo, m.Environment); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	if err := addGitCommitMetadata(repo, repoRoot, m); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	return allErrors.ErrorOrNil()
}

// AddGitRemoteMetadataToMap reads the given git repo and adds its metadata to the given map bag.
func AddGitRemoteMetadataToMap(repo *git.Repository, env map[string]string) error {
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

type cancellationScope struct {
	context *cancel.Context
	sigint  chan os.Signal
	done    chan bool
}

func (s *cancellationScope) Context() *cancel.Context {
	return s.context
}

func (s *cancellationScope) Close() {
	signal.Stop(s.sigint)
	close(s.sigint)
	<-s.done
}

type cancellationScopeSource int

var cancellationScopes = backend.CancellationScopeSource(cancellationScopeSource(0))

func (cancellationScopeSource) NewScope(events chan<- engine.Event, isPreview bool) backend.CancellationScope {
	cancelContext, cancelSource := cancel.NewContext(context.Background())

	c := &cancellationScope{
		context: cancelContext,
		sigint:  make(chan os.Signal),
		done:    make(chan bool),
	}

	go func() {
		for range c.sigint {
			// If we haven't yet received a SIGINT, call the cancellation func. Otherwise call the termination
			// func.
			if cancelContext.CancelErr() == nil {
				message := "^C received; cancelling. If you would like to terminate immediately, press ^C again.\n"
				if !isPreview {
					message += colors.BrightRed + "Note that terminating immediately may lead to orphaned resources " +
						"and other inconsistencies.\n" + colors.Reset
				}
				engine.NewEvent(engine.StdoutColorEvent, engine.StdoutEventPayload{
					Message: message,
					Color:   colors.Always,
				})

				cancelSource.Cancel()
			} else {
				message := colors.BrightRed + "^C received; terminating" + colors.Reset
				engine.NewEvent(engine.StdoutColorEvent, engine.StdoutEventPayload{
					Message: message,
					Color:   colors.Always,
				})

				cancelSource.Terminate()
			}
		}
		close(c.done)
	}()
	signal.Notify(c.sigint, os.Interrupt)

	return c
}

func makeJSONString(v interface{}) (string, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		return "", err
	}
	return out.String(), nil
}

// printJSON simply prints out some object, formatted as JSON, using standard indentation.
func printJSON(v interface{}) error {
	jsonStr, err := makeJSONString(v)
	if err != nil {
		return err
	}
	fmt.Print(jsonStr)
	return nil
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

	defaultOrg, err := workspace.GetBackendConfigDefaultOrg()
	if err != nil {
		return "", err
	}

	if defaultOrg != "" {
		return fmt.Sprintf("%s/%s", defaultOrg, stackName), nil
	}

	return stackName, nil
}

// we only want to log a secrets decryption for a service backend project
// we will allow any secrets provider to be used (service or self managed)
// we will log the message and not worry about the response. The types
// of messages we will log here will range from single secret decryption events
// to requesting a list of secrets in an individual event e.g. stack export
// the logging event will only happen during the `--show-secrets` path within the cli
func log3rdPartySecretsProviderDecryptionEvent(ctx context.Context, backend backend.Stack,
	secretName, commandName string) {
	if stack, ok := backend.(httpstate.Stack); ok {
		// we only want to do something if this is a service backend
		if be, ok := stack.Backend().(httpstate.Backend); ok {
			client := be.Client()
			if client != nil {
				id := backend.(httpstate.Stack).StackIdentifier()
				// we don't really care if these logging calls fail as they should not stop the execution
				if secretName != "" {
					contract.Assert(commandName == "")
					err := client.Log3rdPartySecretsProviderDecryptionEvent(ctx, id, secretName)
					contract.IgnoreError(err)
				}

				if commandName != "" {
					contract.Assert(secretName == "")
					err := client.LogBulk3rdPartySecretsProviderDecryptionEvent(ctx, id, commandName)
					contract.IgnoreError(err)
				}
			}
		}
	}
}
