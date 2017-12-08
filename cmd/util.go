// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
)

// allBackends returns all known backends.  The boolean is true if any are cloud backends.
func allBackends() ([]backend.Backend, bool) {
	// Add all the known backends to the list and query them all.  We always use the local backend,
	// in addition to all of those cloud backends we are currently logged into.
	backends := []backend.Backend{local.New()}
	cloudBackends, err := cloud.CurrentBackends()
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

func getGitHubProjectForOrigin(dir string) (string, string, error) {
	gitRoot, err := fsutil.WalkUp(dir, func(s string) bool { return filepath.Base(s) == ".git" }, nil)
	if err != nil {
		return "", "", errors.Wrap(err, "could not detect git repository")
	}
	if gitRoot == "" {
		return "", "", errors.Errorf("could not locate git repository starting at: %s", dir)
	}

	repo, err := git.NewFilesystemRepository(gitRoot)
	if err != nil {
		return "", "", err
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", errors.Wrap(err, "could not read origin information")
	}

	remoteURL := remote.Config().URL
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
