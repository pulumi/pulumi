// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	git "gopkg.in/src-d/go-git.v4"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
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

func parseColorization(color string) (colors.Colorization, error) {
	switch color {
	case "auto", "": // default to Always
		return colors.Always, nil
	case "always":
		return colors.Always, nil
	case "never":
		return colors.Never, nil
	case "raw":
		return colors.Raw, nil
	}

	return colors.Never, fmt.Errorf(
		"unsupported color option: '%s'.  Supported values are: auto, always, never, raw", color)
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

// registerUpdateOptionsFlags registers a set of flags which will configure the fields of the provided UpdateOptions.
func registerUpdateOptionsFlags(cmd *cobra.Command, opts *engine.UpdateOptions) {
	cmd.PersistentFlags().StringSliceVar(
		&opts.Analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")

	// We use a custom flag type so that we can accept colorization options as a color.Colorization type.
	cf := colorFlag{
		Output: &opts.Color,
	}
	cmd.PersistentFlags().Var(&cf, "color", "Colorize output. Choices are: always, never, raw, auto")

	cmd.PersistentFlags().BoolVarP(
		&opts.DryRun, "dry-run", "d", false,
		"Don't create/delete resources; just preview the planned operations")
	cmd.PersistentFlags().IntVarP(
		&opts.Parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowReplacementSteps, "show-replacement-steps", true,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&opts.ShowSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&opts.Summary, "summary", false,
		"Only display summarization of resources and operations")
}

// colorFlag is a custom cobra.Command flag to wrap a colors.Colorization value.
type colorFlag struct {
	Output *colors.Colorization
	value  string
}

var _ pflag.Value = &colorFlag{}

func (cf *colorFlag) String() string {
	return cf.value
}

func (cf *colorFlag) Set(v string) error {
	c, err := parseColorization(v)
	if err != nil {
		return err
	}
	*cf.Output = c
	return nil
}

func (cf *colorFlag) Type() string {
	return "color.Colorize"
}
