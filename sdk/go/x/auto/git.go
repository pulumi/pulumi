// Copyright 2016-2020, Pulumi Corporation.
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

package auto

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func setupGitRepo(ctx context.Context, workDir string, repoArgs *GitRepo) (string, error) {
	// clone
	repo, err := git.PlainCloneContext(ctx, workDir, false, &git.CloneOptions{URL: repoArgs.URL})
	if err != nil {
		return "", errors.Wrap(err, "unable to clone repo")
	}

	// checkout branch if specified
	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	var hash string
	if repoArgs.CommitHash != "" {
		hash = repoArgs.CommitHash
	}
	var branch string
	if repoArgs.Branch != "" {
		branch = repoArgs.Branch
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash:   plumbing.NewHash(hash),
		Branch: plumbing.ReferenceName(branch),
		Force:  true,
	})
	if err != nil {
		return "", errors.Wrap(err, "unable to checkout branch")
	}

	var relPath string
	if repoArgs.ProjectPath != "" {
		relPath = repoArgs.ProjectPath
	}

	workDir = filepath.Join(workDir, relPath)
	return workDir, nil
}
