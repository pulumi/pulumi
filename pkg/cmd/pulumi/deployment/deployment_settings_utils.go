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

package deployment

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var stackDeploymentConfigFile string

func loadProjectStackDeployment(stack backend.Stack) (*workspace.ProjectStackDeployment, error) {
	if stackDeploymentConfigFile == "" {
		return workspace.DetectProjectStackDeployment(stack.Ref().Name().Q())
	}
	return workspace.LoadProjectStackDeployment(stackDeploymentConfigFile)
}

func saveProjectStackDeployment(psd *workspace.ProjectStackDeployment, stack backend.Stack) error {
	if stackDeploymentConfigFile == "" {
		return workspace.SaveProjectStackDeployment(stack.Ref().Name().Q(), psd)
	}
	return psd.Save(stackDeploymentConfigFile)
}

type prompts interface {
	PromptUser(msg string, options []string, defaultOption string,
		colorization colors.Colorization,
	) string
	PromptUserSkippable(yes bool, msg string, options []string, defaultOption string,
		colorization colors.Colorization,
	) string
	PromptUserMultiSkippable(yes bool, msg string, options []string, defaultOptions []string,
		colorization colors.Colorization,
	) []string
	PromptForValue(
		yes bool, valueType string, defaultValue string, secret bool,
		isValidFn func(value string) error, opts display.Options,
	) (string, error)
	AskForConfirmation(prompt string, color colors.Colorization, defaultValue bool, yes bool) bool
	Print(prompt string)
}

type promptHandlers struct{}

func (promptHandlers) AskForConfirmation(prompt string, color colors.Colorization, defaultValue bool, yes bool) bool {
	return askForConfirmation(prompt, color, defaultValue, yes)
}

func (promptHandlers) PromptUser(msg string, options []string, defaultOption string,
	colorization colors.Colorization,
) string {
	return ui.PromptUser(msg, options, defaultOption, colorization)
}

func (promptHandlers) PromptUserSkippable(yes bool, msg string, options []string, defaultOption string,
	colorization colors.Colorization,
) string {
	return ui.PromptUserSkippable(yes, msg, options, defaultOption, colorization)
}

func (promptHandlers) PromptUserMultiSkippable(yes bool, msg string, options []string, defaultOptions []string,
	colorization colors.Colorization,
) []string {
	return ui.PromptUserMultiSkippable(yes, msg, options, defaultOptions, colorization)
}

func (promptHandlers) PromptForValue(
	yes bool, valueType string, defaultValue string, secret bool,
	isValidFn func(value string) error, opts display.Options,
) (string, error) {
	return ui.PromptForValue(yes, valueType, defaultValue, secret, isValidFn, opts)
}

func (promptHandlers) Print(prompt string) {
	fmt.Println(prompt)
}

type repoLookup interface {
	GetRootDirectory(wd string) (string, error)
	GetBranchName() string
	RemoteURL() (string, error)
	GetRepoRoot() string
}

func newRepoLookup(wd string) (repoLookup, error) {
	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{DetectDotGit: true})
	switch {
	case errors.Is(err, git.ErrRepositoryNotExists):
		return &noRepoLookupImpl{}, nil
	case err != nil:
		return nil, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	h, err := repo.Head()
	if err != nil {
		return nil, err
	}

	return &repoLookupImpl{
		RepoRoot: worktree.Filesystem.Root(),
		Repo:     repo,
		Head:     h,
	}, nil
}

type repoLookupImpl struct {
	RepoRoot string
	Repo     *git.Repository
	Head     *plumbing.Reference
}

func (r *repoLookupImpl) GetRootDirectory(wd string) (string, error) {
	dir, err := filepath.Rel(r.RepoRoot, wd)
	if err != nil {
		return "", err
	}

	return dir, err
}

func (r *repoLookupImpl) GetBranchName() string {
	if r.Head == nil {
		return ""
	}
	return r.Head.Name().String()
}

func (r *repoLookupImpl) RemoteURL() (string, error) {
	if r.Repo == nil {
		return "", nil
	}
	return gitutil.GetGitRemoteURL(r.Repo, "origin")
}

func (r *repoLookupImpl) GetRepoRoot() string {
	return r.RepoRoot
}

type noRepoLookupImpl struct{}

func (r *noRepoLookupImpl) GetRootDirectory(wd string) (string, error) {
	return ".", nil
}

func (r *noRepoLookupImpl) GetBranchName() string {
	return ""
}

func (r *noRepoLookupImpl) RemoteURL() (string, error) {
	return "", nil
}

func (r *noRepoLookupImpl) GetRepoRoot() string {
	return ""
}

func askForConfirmation(prompt string, color colors.Colorization, defaultValue bool, yes bool) bool {
	def := optNo
	if defaultValue {
		def = optYes
	}
	options := []string{optYes, optNo}
	response := ui.PromptUserSkippable(yes, prompt, options, def, color)
	return response == optYes
}

// ValidateRelativeDirectory ensures a relative path points to a valid directory
func ValidateRelativeDirectory(rootDir string) func(string) error {
	return func(s string) error {
		if rootDir == "" {
			return nil
		}

		dir := filepath.Join(rootDir, filepath.FromSlash(s))
		info, err := os.Stat(dir)

		switch {
		case os.IsNotExist(err):
			return fmt.Errorf("invalid relative path %s", s)
		case err != nil:
			return err
		}

		if !info.IsDir() {
			return fmt.Errorf("invalid relative path %s, is not a directory", s)
		}

		return nil
	}
}

func ValidateGitURL(s string) error {
	_, _, err := gitutil.ParseGitRepoURL(s)

	return err
}

func ValidateShortInputNonEmpty(s string) error {
	if s == "" {
		return errors.New("should not be empty")
	}

	return ValidateShortInput(s)
}

func ValidateShortInput(s string) error {
	const maxTagValueLength = 256

	if len(s) > maxTagValueLength {
		return errors.New("must be 256 characters or less")
	}

	return nil
}
