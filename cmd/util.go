// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// allBackends returns all known backends.  The boolean is true if any are cloud backends.
func allBackends() ([]backend.Backend, bool) {
	// Add all the known backends to the list and query them all.  We always use the local backend,
	// in addition to all of those cloud backends we are currently logged into.
	d := cmdutil.Diag()
	backends := []backend.Backend{local.New(d)}
	cloudBackends, _, err := cloud.CurrentBackends(d)
	if err != nil {
		// Print the error, but keep going so that the local operations still occur.
		_, fmterr := fmt.Fprintf(os.Stderr, "error: could not obtain current cloud backends: %v", err)
		contract.IgnoreError(fmterr)
	} else {
		for _, be := range cloudBackends {
			backends = append(backends, be)
		}
	}
	return backends, len(cloudBackends) > 0
}

func requireStack(stackName tokens.QName) (backend.Stack, error) {
	if stackName == "" {
		return requireCurrentStack()
	}
	bes, _ := allBackends()
	stack, err := state.Stack(stackName, bes)
	if err != nil {
		return nil, err
	} else if stack == nil {
		return nil, errors.Errorf("no stack named '%s' found; double check that you're logged in", stackName)
	}
	return stack, nil
}

func requireCurrentStack() (backend.Stack, error) {
	bes, _ := allBackends()
	stack, err := state.CurrentStack(bes)
	if err != nil {
		return nil, err
	} else if stack == nil {
		return nil, errors.New("no current stack detected; please use `pulumi stack` to `init` or `select` one")
	}
	return stack, nil
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

// readPackage attempts to detect and read the package for the current workspace. If an error occurs, it will be
// printed to Stderr, and the returned value will be nil. If the package is successfully detected and read, it
// is returned along with the path to its containing directory, which will be used as the root of the package's
// Pulumi program.
func readPackage() (*pack.Package, string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	// Now that we got here, we have a path, so we will try to load it.
	pkgpath, err := workspace.DetectPackageFrom(pwd)
	if err != nil {
		return nil, "", errors.Errorf("could not locate a package to load: %v", err)
	} else if pkgpath == "" {
		return nil, "", errors.Errorf("could not find Pulumi.yaml (searching upwards from %s)", pwd)
	}
	pkg, err := pack.Load(pkgpath)
	if err != nil {
		return nil, "", err
	}

	return pkg, filepath.Dir(pkgpath), nil
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
			m.Environment["git.head"] = head.Hash().String()
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
		m.Environment["git.dirty"] = fmt.Sprint(dirty)

		// GitHub repo slug if applicable. We don't require GitHub, so swallow errors.
		ghLogin, ghRepo, err := getGitHubProjectForOriginByRepo(repo)
		if err == nil {
			m.Environment["github.login"] = ghLogin
			m.Environment["github.repo"] = ghRepo
		}
	}

	return m, nil
}
