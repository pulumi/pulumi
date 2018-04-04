// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	// legacyEnvvar controls whether legacy features are supported or not.  If it is truthy, then features like
	// purely local deployments are still supported.
	legacyEnvvar = "PULUMI_LEGACY"
)

// allBackends returns all known backends.  The boolean is true if any are cloud backends.
func allBackends() ([]backend.Backend, bool, error) {
	// Add all the known backends to the list and query them all.
	var backends []backend.Backend

	// If legacy mode is enabled, add the local backend.
	d := cmdutil.Diag()
	legacy, _ := strconv.ParseBool(os.Getenv(legacyEnvvar))
	if legacy {
		backends = append(backends, local.New(d))
	}

	// Now use all backends we are logged into.  If there are none, and legacy mode is disabled, prompt for login.
	cloudBackends, _, err := cloud.CurrentBackends(d)
	if err != nil {
		// Print the error, but keep going so that the local operations still occur.
		_, fmterr := fmt.Fprintf(os.Stderr, "error: could not obtain current cloud backends: %v", err)
		contract.IgnoreError(fmterr)
	} else if len(cloudBackends) > 0 {
		// We got some cloud backends the user is logged into, huzzah, simply append them.
		for _, be := range cloudBackends {
			backends = append(backends, be)
		}
	} else if !legacy {
		// The user isn't logged in, and we aren't in legacy mode.  We will need to ask them to login.
		fmt.Printf(colors.ColorizeText(
			colors.SpecImportant + "This command requires that you login." + colors.Reset + "\n"))
		be, err := cloud.Login(d, "")
		if err != nil {
			return nil, false, err
		}
		backends = append(backends, be)
	}

	return backends, len(cloudBackends) > 0, nil
}

// createStack creates a stack with the given name, and selects it as the current.
func createStack(b backend.Backend, stackName tokens.QName, opts interface{}) (backend.Stack, error) {
	stack, err := b.CreateStack(stackName, opts)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create stack")
	}

	if err = state.SetCurrentStack(stackName); err != nil {
		return nil, err
	}

	return stack, nil
}

// requireStack will require that a stack exists.  If stackName is blank, the currently selected stack from
// the workspace is returned.  If no stack with either the given name, or a currently selected stack, exists,
// and we are in an interactive terminal, the user will be prompted to create a new stack.
func requireStack(stackName tokens.QName, offerNew bool) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack(offerNew)
	}

	// Search all known backends for this stack.
	bes, _, err := allBackends()
	if err != nil {
		return nil, err
	}
	stack, err := state.Stack(stackName, bes)
	if err != nil {
		return nil, err
	}
	if stack != nil {
		return stack, err
	}

	// No stack was found.  If we're in a terminal, prompt to create one.
	if cmdutil.Interactive() {
		fmt.Printf("The stack '%s' does not exist.\n", stackName)
		fmt.Printf("\n")
		fmt.Printf("It is possible that you aren't logged into the correct cloud where this stack\n")
		fmt.Printf("has been provisioned.  If that is the case, please press ^C and run `pulumi login`\n")
		fmt.Printf("to log into the cloud, and then try this operation again.\n")
		fmt.Printf("\n")
		_, err = cmdutil.ReadConsole("If instead, you would like to create this stack now, please press <ENTER>")
		if err != nil {
			return nil, err
		}

		// TODO[pulumi/pulumi#816]: right now, we assume that we're creating a local stack.  This is a little
		//     inconsistent compared to where we are now, but it's closer to where we think we're going.  As
		//     part of 816, we should tidy this up.
		b := local.New(cmdutil.Diag())
		return createStack(b, stackName, nil)
	}

	return nil, errors.Errorf("no stack named '%s' found; double check that you're logged in", stackName)
}

func requireCurrentStack(offerNew bool) (backend.Stack, error) {
	// Search for the current stack.
	bes, _, err := allBackends()
	if err != nil {
		return nil, err
	}
	stack, err := state.CurrentStack(bes)
	if err != nil {
		return nil, err
	} else if stack != nil {
		return stack, nil
	}

	// If no current stack exists, and we are interactive, prompt to select or create one.
	return chooseStack(bes, offerNew)
}

// chooseStack will prompt the user to choose amongst the full set of stacks in the given backends.  If offerNew is
// true, then the option to create an entirely new stack is provided and will create one as desired.
func chooseStack(bes []backend.Backend, offerNew bool) (backend.Stack, error) {
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

	// First create a list and map of stack names.
	var options []string
	stacks := make(map[string]backend.Stack)
	for _, be := range bes {
		allStacks, err := be.ListStacks()
		if err != nil {
			return nil, errors.Wrapf(err, "could not query backend for stacks")
		}
		for _, stack := range allStacks {
			name := string(stack.Name())
			options = append(options, name)
			stacks[name] = stack
		}
	}
	sort.Strings(options)

	// If we are offering to create a new stack, add that to the end of the list.
	newOption := "<create a new stack>"
	if offerNew {
		options = append(options, newOption)
	} else if len(options) == 0 {
		// If no options are available, we can't offer a choice!
		return nil, errors.New("this command requires a stack, but there are none; are you logged in?")
	}

	// If a stack is already selected, make that the default.
	var current string
	currStack, currErr := state.CurrentStack(bes)
	contract.IgnoreError(currErr)
	if currStack != nil {
		current = string(currStack.Name())
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

		// TODO[pulumi/pulumi#816]: right now, we assume that we're creating a local stack.  This is a little
		//     inconsistent compared to where we are now, but it's closer to where we think we're going.  As
		//     part of 816, we should tidy this up.
		b := local.New(cmdutil.Diag())
		return createStack(b, tokens.QName(stackName), nil)
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

// upgradeConfigurationFiles does an upgrade to move from the old configuration system (where we had config stored) in
// workspace settings and Pulumi.yaml, to the new system where configuration data is stored in Pulumi.<stack-name>.yaml
func upgradeConfigurationFiles() error {
	bes, _, err := allBackends()
	if err != nil {
		return err
	}

	skippedUpgrade := false
	for _, b := range bes {
		stacks, err := b.ListStacks()
		if err != nil {
			skippedUpgrade = true
			continue
		}

		for _, stack := range stacks {
			stackName := stack.Name()

			// If the new file exists, we can skip upgrading this stack.
			newFile, err := workspace.DetectProjectStackPath(stackName)
			if err != nil {
				return errors.Wrap(err, "upgrade project")
			}

			_, err = os.Stat(newFile)
			if err != nil && !os.IsNotExist(err) {
				return errors.Wrap(err, "upgrading project")
			} else if err == nil {
				// new file was present, skip upgrading this stack.
				continue
			}

			cfg, salt, err := getOldConfiguration(stackName)
			if err != nil {
				return errors.Wrap(err, "upgrading project")
			}

			ps, err := workspace.DetectProjectStack(stackName)
			if err != nil {
				return errors.Wrap(err, "upgrading project")
			}

			ps.Config = cfg
			ps.EncryptionSalt = salt

			if err := workspace.SaveProjectStack(stackName, ps); err != nil {
				return errors.Wrap(err, "upgrading project")
			}

			if err := removeOldConfiguration(stackName); err != nil {
				return errors.Wrap(err, "upgrading project")
			}
		}
	}

	if !skippedUpgrade {
		return removeOldProjectConfiguration()
	}

	return nil
}

// getOldConfiguration reads the configuration for a given stack from the current workspace.  It applies a hierarchy
// of configuration settings based on stack overrides and workspace-wide global settings.  If any of the workspace
// settings had an impact on the values returned, the second return value will be true. It also returns the encryption
// salt used for the stack.
func getOldConfiguration(stackName tokens.QName) (config.Map, string, error) {
	contract.Require(stackName != "", "stackName")

	// Get the workspace and package and get ready to merge their views of the configuration.
	ws, err := workspace.New()
	if err != nil {
		return nil, "", err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return nil, "", err
	}

	// We need to apply workspace and project configuration values in the right order.  Basically, we want to
	// end up taking the most specific settings, where per-stack configuration is more specific than global, and
	// project configuration is more specific than workspace.
	result := make(config.Map)

	// First, apply project-local stack-specific configuration.
	if stack, has := proj.StacksDeprecated[stackName]; has {
		for key, value := range stack.Config {
			result[key] = value
		}
	}

	// Now, apply workspace stack-specific configuration.
	if wsStackConfig, has := ws.Settings().ConfigDeprecated[stackName]; has {
		for key, value := range wsStackConfig {
			if _, has := result[key]; !has {
				result[key] = value
			}
		}
	}

	// Next, take anything from the global settings in our project file.
	for key, value := range proj.ConfigDeprecated {
		if _, has := result[key]; !has {
			result[key] = value
		}
	}

	// Finally, take anything left in the workspace's global configuration.
	if wsGlobalConfig, has := ws.Settings().ConfigDeprecated[""]; has {
		for key, value := range wsGlobalConfig {
			if _, has := result[key]; !has {
				result[key] = value
			}
		}
	}

	// Now, get the encryption key. A stack specific one overrides the global one (global encryption keys were
	// deprecated previously)
	encryptionSalt := proj.EncryptionSaltDeprecated
	if stack, has := proj.StacksDeprecated[stackName]; has && stack.EncryptionSalt != "" {
		encryptionSalt = stack.EncryptionSalt
	}

	return result, encryptionSalt, nil
}

// removeOldConfiguration deletes all configuration information about a stack from both the workspace
// and the project file. It does not touch the newly added Pulumi.<stack-name>.yaml file.
func removeOldConfiguration(stackName tokens.QName) error {
	ws, err := workspace.New()
	if err != nil {
		return err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return err
	}

	delete(proj.StacksDeprecated, stackName)
	delete(ws.Settings().ConfigDeprecated, stackName)

	if err := ws.Save(); err != nil {
		return err
	}

	return workspace.SaveProject(proj)
}

// removeOldProjectConfiguration deletes all project level configuration information from both the workspace and the
// project file.
func removeOldProjectConfiguration() error {
	ws, err := workspace.New()
	if err != nil {
		return err
	}
	proj, err := workspace.DetectProject()
	if err != nil {
		return err
	}

	proj.EncryptionSaltDeprecated = ""
	proj.ConfigDeprecated = nil
	ws.Settings().ConfigDeprecated = nil

	if err := ws.Save(); err != nil {
		return err
	}

	return workspace.SaveProject(proj)
}
