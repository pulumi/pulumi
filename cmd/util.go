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

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"

	"github.com/golang/glog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
	git "gopkg.in/src-d/go-git.v4"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/gitutil"
	"github.com/pulumi/pulumi/pkg/util/testutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func hasDebugCommands() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_DEBUG_COMMANDS"))
}

func currentBackend(opts backend.DisplayOptions) (backend.Backend, error) {
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return nil, err
	}
	if local.IsLocalBackendURL(creds.Current) {
		return local.New(cmdutil.Diag(), creds.Current)
	}
	return cloud.Login(commandContext(), cmdutil.Diag(), creds.Current, opts)
}

// This is used to control the contents of the tracing header.
var tracingHeader = os.Getenv("PULUMI_TRACING_HEADER")

func commandContext() context.Context {
	ctx := context.Background()
	if cmdutil.IsTracingEnabled() {
		if cmdutil.TracingRootSpan != nil {
			ctx = opentracing.ContextWithSpan(ctx, cmdutil.TracingRootSpan)
		}

		tracingOptions := backend.TracingOptions{
			PropagateSpans: true,
			TracingHeader:  tracingHeader,
		}
		ctx = backend.ContextWithTracingOptions(ctx, tracingOptions)
	}
	return ctx
}

// createStack creates a stack with the given name, and optionally selects it as the current.
func createStack(
	b backend.Backend, stackRef backend.StackReference, opts interface{}, setCurrent bool) (backend.Stack, error) {
	stack, err := b.CreateStack(commandContext(), stackRef, opts)
	if err != nil {
		// If it's a StackAlreadyExistsError, don't wrap it.
		if _, ok := err.(*backend.StackAlreadyExistsError); ok {
			return nil, err
		}
		return nil, errors.Wrapf(err, "could not create stack")
	}

	if setCurrent {
		if err = state.SetCurrentStack(stack.Name().String()); err != nil {
			return nil, err
		}
	}

	return stack, nil
}

// requireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func requireStack(
	stackName string, offerNew bool, opts backend.DisplayOptions, setCurrent bool) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack(offerNew, opts, setCurrent)
	}

	b, err := currentBackend(opts)
	if err != nil {
		return nil, err
	}

	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}

	stack, err := b.GetStack(commandContext(), stackRef)
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

		return createStack(b, stackRef, nil, setCurrent)
	}

	return nil, errors.Errorf("no stack named '%s' found", stackName)
}

func requireCurrentStack(offerNew bool, opts backend.DisplayOptions, setCurrent bool) (backend.Stack, error) {
	// Search for the current stack.
	b, err := currentBackend(opts)
	if err != nil {
		return nil, err
	}
	stack, err := state.CurrentStack(commandContext(), b)
	if err != nil {
		return nil, err
	} else if stack != nil {
		return stack, nil
	}

	// If no current stack exists, and we are interactive, prompt to select or create one.
	return chooseStack(b, offerNew, opts, setCurrent)
}

// chooseStack will prompt the user to choose amongst the full set of stacks in the given backends.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func chooseStack(
	b backend.Backend, offerNew bool, opts backend.DisplayOptions, setCurrent bool) (backend.Stack, error) {
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

	// First create a list and map of stack names.
	var options []string
	stacks := make(map[string]backend.Stack)
	allStacks, err := b.ListStacks(commandContext(), &proj.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "could not query backend for stacks")
	}
	for _, stack := range allStacks {
		name := stack.Name().String()
		options = append(options, name)
		stacks[name] = stack
	}
	sort.Strings(options)

	// If we are offering to create a new stack, add that to the end of the list.
	newOption := "<create a new stack>"
	if offerNew {
		options = append(options, newOption)
	} else if len(options) == 0 {
		// If no options are available, we can't offer a choice!
		return nil, errors.New("this command requires a stack, but there are none")
	}

	// If a stack is already selected, make that the default.
	var current string
	currStack, currErr := state.CurrentStack(commandContext(), b)
	contract.IgnoreError(currErr)
	if currStack != nil {
		current = currStack.Name().String()
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
	message = opts.Color.Colorize(colors.BrightWhite + message + colors.Reset)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
		Default: current,
	}, &option, nil); err != nil {
		return nil, errors.New(chooseStackErr)
	}

	if option == newOption {
		stackName, err := cmdutil.ReadConsole("Please enter your desired stack name")
		if err != nil {
			return nil, err
		}

		stackRef, err := b.ParseStackReference(stackName)
		if err != nil {
			return nil, err
		}

		return createStack(b, stackRef, nil, setCurrent)
	}

	return stacks[option], nil
}

// readProject attempts to detect and read the project for the current workspace. If an error occurs, it will be
// printed to Stderr, and the returned value will be nil. If the project is successfully detected and read, it
// is returned along with the path to its containing directory, which will be used as the root of the project's
// Pulumi program.
func readProject() (*workspace.Project, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	// Now that we got here, we have a path, so we will try to load it.
	path, err := workspace.DetectProjectPathFrom(pwd)
	if err != nil {
		return nil, "", errors.Wrapf(err,
			"could not locate Pulumi.yaml project file (searching upwards from %s)", pwd)
	} else if path == "" {
		return nil, "", errors.Errorf(
			"no Pulumi.yaml project file found (searching upwards from %s)", pwd)
	}
	proj, err := workspace.LoadProject(path)
	if err != nil {
		return nil, "", err
	}

	return proj, filepath.Dir(path), nil
}

type colorFlag struct {
	value colors.Colorization
}

func (cf *colorFlag) String() string {
	return string(cf.Colorization())
}

func (cf *colorFlag) Set(value string) error {
	switch value {
	case "always":
		cf.value = colors.Always
	case "never":
		cf.value = colors.Never
	case "raw":
		cf.value = colors.Raw
	// Backwards compat for old flag values.
	case "auto":
		cf.value = colors.Always
	default:
		return errors.Errorf("unsupported color option: '%s'.  Supported values are: always, never, raw", value)
	}

	return nil
}

func (cf *colorFlag) Type() string {
	return "colors.Colorization"
}

func (cf *colorFlag) Colorization() colors.Colorization {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return colors.Never
	}

	if cf.value == "" {
		return colors.Always
	}

	return cf.value
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
func isGitWorkTreeDirty() (bool, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return false, err
	}

	// nolint: gas
	gitStatusCmd := exec.Command(gitBin, "status", "--porcelain", "-z")
	var anyOutput anyWriter
	var stderr bytes.Buffer
	gitStatusCmd.Stdout = &anyOutput
	gitStatusCmd.Stderr = &stderr
	if err = gitStatusCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
		return false, errors.Wrapf(err, "'git status' failed")
	}

	return bool(anyOutput), nil
}

// getUpdateMetadata returns an UpdateMetadata object, with optional data about the environment
// performing the update.
func getUpdateMetadata(msg, root string) (backend.UpdateMetadata, error) {
	m := backend.UpdateMetadata{
		Message:     msg,
		Environment: make(map[string]string),
	}

	if err := addGitMetadataToEnvironment(root, m.Environment); err != nil {
		glog.V(3).Infof("errors detecting git metadata: %s", err)
	}
	addCIMetadataToEnvironment(m.Environment)

	return m, nil
}

// addGitMetadataToEnvironment populate's the environment metadata bag with Git-related values.
func addGitMetadataToEnvironment(repoRoot string, env map[string]string) error {
	var allErrors *multierror.Error

	// Gather git-related data as appropriate. (Returns nil, nil if no repo found.)
	repo, err := gitutil.GetGitRepository(repoRoot)
	if err != nil {
		return errors.Wrapf(err, "detecting Git repository")
	}
	if repo == nil {
		return nil
	}

	if err := addGitHubMetadataToEnvironment(repo, env); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	if err := addGitCommitMetadataToEnvironment(repo, env); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	return allErrors.ErrorOrNil()
}

func addGitHubMetadataToEnvironment(repo *git.Repository, env map[string]string) error {
	// GitHub repo slug if applicable. We don't require GitHub, so swallow errors.
	ghLogin, ghRepo, err := gitutil.GetGitHubProjectForOriginByRepo(repo)
	if err != nil {
		return errors.Wrap(err, "detecting GitHub project information")
	}
	env[backend.GitHubLogin] = ghLogin
	env[backend.GitHubRepo] = ghRepo

	return nil
}

func addGitCommitMetadataToEnvironment(repo *git.Repository, env map[string]string) error {
	// Commit at HEAD
	head, err := repo.Head()
	if err != nil {
		return errors.Wrap(err, "getting repository HEAD")
	}

	hash := head.Hash()
	env[backend.GitHead] = hash.String()
	commit, commitErr := repo.CommitObject(hash)
	if commitErr != nil {
		return errors.Wrap(commitErr, "getting HEAD commit info")
	}
	env[backend.GitCommitter] = commit.Committer.Name
	env[backend.GitCommitterEmail] = commit.Committer.Email
	env[backend.GitAuthor] = commit.Author.Name
	env[backend.GitAuthorEmail] = commit.Author.Email

	isDirty, err := isGitWorkTreeDirty()
	if err != nil {
		return errors.Wrapf(err, "checking git worktree dirty state")
	}
	env[backend.GitDirty] = fmt.Sprint(isDirty)

	return nil
}

// addCIMetadataToEnvironment populate's the environment metadata bag with CI/CD-related values.
func addCIMetadataToEnvironment(env map[string]string) {
	// Check if running on Travis CI. See:
	// https://docs.travis-ci.com/user/environment-variables/
	if os.Getenv("TRAVIS") == "true" {
		env[backend.CISystem] = "travis-ci"

		// Pass build-related information.
		env[backend.CIBuildID] = os.Getenv("TRAVIS_JOB_ID")
		env[backend.CIBuildType] = os.Getenv("TRAVIS_EVENT_TYPE")
		// Travis doesn't set a build URL in its environment, see:
		// https://github.com/travis-ci/travis-ci/issues/8935

		// Pass pull request-specific vales as appropriate.
		if sha := os.Getenv("TRAVIS_PULL_REQUEST_SHA"); sha != "" {
			env[backend.CIPRHeadSHA] = sha
		}
	}
}

type cancellationScope struct {
	context *cancel.Context
	sigint  chan os.Signal
}

func (s *cancellationScope) Context() *cancel.Context {
	return s.context
}

func (s *cancellationScope) Close() {
	signal.Stop(s.sigint)
	close(s.sigint)
}

type cancellationScopeSource int

var cancellationScopes = backend.CancellationScopeSource(cancellationScopeSource(0))

func (cancellationScopeSource) NewScope(events chan<- engine.Event, isPreview bool) backend.CancellationScope {
	cancelContext, cancelSource := cancel.NewContext(context.Background())

	c := &cancellationScope{
		context: cancelContext,
		sigint:  make(chan os.Signal),
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
				events <- engine.Event{
					Type: engine.StdoutColorEvent,
					Payload: engine.StdoutEventPayload{
						Message: message,
						Color:   colors.Always,
					},
				}

				cancelSource.Cancel()
			} else {
				message := colors.BrightRed + "^C received; terminating" + colors.Reset
				events <- engine.Event{
					Type: engine.StdoutColorEvent,
					Payload: engine.StdoutEventPayload{
						Message: message,
						Color:   colors.Always,
					},
				}

				cancelSource.Terminate()
			}
		}
	}()
	signal.Notify(c.sigint, os.Interrupt)

	return c
}

// isInteractive returns true if the environment and command line options indicate we should
// do things interactively
func isInteractive(nonInteractive bool) bool {
	return !nonInteractive && terminal.IsTerminal(int(os.Stdout.Fd())) && !testutil.IsCI()
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
