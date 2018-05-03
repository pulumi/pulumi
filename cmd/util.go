// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/util/cancel"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func currentBackend() (backend.Backend, error) {
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return nil, err
	}
	if local.IsLocalBackendURL(creds.Current) {
		return local.New(cmdutil.Diag(), creds.Current), nil
	}
	return cloud.Login(cmdutil.Diag(), creds.Current)
}

// createStack creates a stack with the given name, and selects it as the current.
func createStack(b backend.Backend, stackRef backend.StackReference, opts interface{}) (backend.Stack, error) {
	stack, err := b.CreateStack(stackRef, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create stack")
	}

	if err = state.SetCurrentStack(stack.Name().String()); err != nil {
		return nil, err
	}

	return stack, nil
}

// requireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func requireStack(stackName string, offerNew bool) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack(offerNew)
	}

	b, err := currentBackend()
	if err != nil {
		return nil, err
	}

	stackRef, err := b.ParseStackReference(stackName)
	if err != nil {
		return nil, err
	}

	stack, err := b.GetStack(stackRef)
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

		return createStack(b, stackRef, nil)
	}

	return nil, errors.Errorf("no stack named '%s' found", stackName)
}

func requireCurrentStack(offerNew bool) (backend.Stack, error) {
	// Search for the current stack.
	b, err := currentBackend()
	if err != nil {
		return nil, err
	}
	stack, err := state.CurrentStack(b)
	if err != nil {
		return nil, err
	} else if stack != nil {
		return stack, nil
	}

	// If no current stack exists, and we are interactive, prompt to select or create one.
	return chooseStack(b, offerNew)
}

// chooseStack will prompt the user to choose amongst the full set of stacks in the given backends.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func chooseStack(b backend.Backend, offerNew bool) (backend.Stack, error) {
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
	allStacks, err := b.ListStacks(&proj.Name)
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
	currStack, currErr := state.CurrentStack(b)
	contract.IgnoreError(currErr)
	if currStack != nil {
		current = currStack.Name().String()
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = colors.ColorizeText(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a stack"
	if offerNew {
		message += ", or create a new one:"
	} else {
		message += ":"
	}
	message = colors.ColorizeText(colors.BrightWhite + message + colors.Reset)

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

		return createStack(b, stackRef, nil)
	}

	return stacks[option], nil
}

func detectOwnerAndName(dir string) (string, string, error) {
	owner, repo, err := getGitHubProjectForOrigin(dir)
	if err == nil {
		return owner, repo, nil
	}

	user, err := user.Current()
	if err != nil {
		return "", "", err
	}

	return user.Username, filepath.Base(dir), nil
}

// getGitRepository returns the git repository by walking up from the provided directory.
// If no repository is found, will return (nil, nil).
func getGitRepository(dir string) (*git.Repository, error) {
	gitRoot, err := fsutil.WalkUp(dir, func(s string) bool { return filepath.Base(s) == ".git" }, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "searching for git repository from %v", dir)
	}
	if gitRoot == "" {
		return nil, nil
	}

	// Open the git repo in the .git folder's parent, not the .git folder itself.
	repo, err := git.PlainOpen(path.Join(gitRoot, ".."))
	if err == git.ErrRepositoryNotExists {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "reading git repository")
	}
	return repo, nil
}

func getGitHubProjectForOrigin(dir string) (string, string, error) {
	repo, err := getGitRepository(dir)
	if repo == nil {
		return "", "", fmt.Errorf("no git repository found from %v", dir)
	}
	if err != nil {
		return "", "", err
	}
	return getGitHubProjectForOriginByRepo(repo)
}

// Returns the GitHub login, and GitHub repo name if the "origin" remote is
// a GitHub URL.
func getGitHubProjectForOriginByRepo(repo *git.Repository) (string, string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", errors.Wrap(err, "could not read origin information")
	}

	remoteURL := ""
	if len(remote.Config().URLs) > 0 {
		remoteURL = remote.Config().URLs[0]
	}
	project := ""

	const GitHubSSHPrefix = "git@github.com:"
	const GitHubHTTPSPrefix = "https://github.com/"
	const GitHubRepositorySuffix = ".git"

	if strings.HasPrefix(remoteURL, GitHubSSHPrefix) {
		project = trimGitRemoteURL(remoteURL, GitHubSSHPrefix, GitHubRepositorySuffix)
	} else if strings.HasPrefix(remoteURL, GitHubHTTPSPrefix) {
		project = trimGitRemoteURL(remoteURL, GitHubHTTPSPrefix, GitHubRepositorySuffix)
	}

	split := strings.Split(project, "/")

	if len(split) != 2 {
		return "", "", errors.Errorf("could not detect GitHub project from url: %v", remote)
	}

	return split[0], split[1], nil
}

func trimGitRemoteURL(url string, prefix string, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(url, prefix), suffix)
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
	if cf.value == "" {
		return colors.Always
	}

	return cf.value
}

// getUpdateMetadata returns an UpdateMetadata object, with optional data about the environment
// performing the update.
func getUpdateMetadata(msg, root string) (backend.UpdateMetadata, error) {
	m := backend.UpdateMetadata{
		Message:     msg,
		Environment: make(map[string]string),
	}

	// Gather git-related data as appropriate. (Returns nil, nil if no repo found.)
	repo, err := getGitRepository(root)
	if err != nil {
		return m, errors.Wrap(err, "looking for git repository")
	}
	if repo != nil && err == nil {
		const gitErrCtx = "reading git repo (%v)" // Message passed with wrapped errors from go-git.

		// Commit at HEAD.
		head, err := repo.Head()
		if err == nil {
			m.Environment[backend.GitHead] = head.Hash().String()
		} else {
			// Ignore "reference not found" in the odd case where the HEAD commit doesn't exist.
			// (git init, but no commits yet.)
			if err != plumbing.ErrReferenceNotFound {
				return m, errors.Wrapf(err, gitErrCtx, "getting head")
			}
		}

		// If the current commit is dirty.
		w, err := repo.Worktree()
		if err != nil {
			return m, errors.Wrapf(err, gitErrCtx, "getting worktree")
		}
		s, err := w.Status()
		if err != nil {
			return m, errors.Wrapf(err, gitErrCtx, "getting worktree status")
		}
		dirty := !s.IsClean()
		m.Environment[backend.GitDirty] = fmt.Sprint(dirty)

		// GitHub repo slug if applicable. We don't require GitHub, so swallow errors.
		ghLogin, ghRepo, err := getGitHubProjectForOriginByRepo(repo)
		if err == nil {
			m.Environment[backend.GitHubLogin] = ghLogin
			m.Environment[backend.GitHubRepo] = ghRepo
		}
	}

	return m, nil
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

// IsInteractive returns true if the environment and command line options indicate we should
// do things interactively
func IsInteractive(cmd *cobra.Command) bool {
	nonInteractive, err := cmd.Flags().GetBool("non-interactive")
	contract.IgnoreError(err)
	return !nonInteractive && terminal.IsTerminal(int(os.Stdout.Fd())) && !isCI()
}
