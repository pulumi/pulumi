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
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

func setupGitRepo(ctx context.Context, workDir string, repoArgs *GitRepo) (string, error) {
	cloneOptions := &git.CloneOptions{
		URL: repoArgs.URL,
	}

	if repoArgs.Auth != nil {

		authDetails := repoArgs.Auth
		// Each of the authentication options are mutually exclusive so let's check that only 1 is specified
		if authDetails.SSHPrivateKeyPath != "" && authDetails.Username != "" ||
			authDetails.PersonalAccessToken != "" && authDetails.Username != "" ||
			authDetails.PersonalAccessToken != "" && authDetails.SSHPrivateKeyPath != "" ||
			authDetails.Username != "" && authDetails.SSHPrivateKey != "" {
			return "", errors.New("please specify one authentication option of `Personal Access Token`, " +
				"`Username\\Password`, `SSH Private Key Path` or `SSH Private Key`")
		}

		// Firstly we will try to check that an SSH Private Key Path has been specified
		if authDetails.SSHPrivateKeyPath != "" {
			publicKeys, err := ssh.NewPublicKeysFromFile("git", repoArgs.Auth.SSHPrivateKeyPath, repoArgs.Auth.Password)
			if err != nil {
				return "", errors.Wrap(err, "unable to use SSH Private Key Path")
			}

			cloneOptions.Auth = publicKeys
		}

		// Then we check if the details of a SSH Private Key as passed
		if authDetails.SSHPrivateKey != "" {
			publicKeys, err := ssh.NewPublicKeys("git", []byte(repoArgs.Auth.SSHPrivateKey), repoArgs.Auth.Password)
			if err != nil {
				return "", errors.Wrap(err, "unable to use SSH Private Key")
			}

			cloneOptions.Auth = publicKeys
		}

		// Then we check to see if a Personal Access Token has been specified
		// the username for use with a PAT can be *anything* but an empty string
		// so we are setting this to `git`
		if authDetails.PersonalAccessToken != "" {
			cloneOptions.Auth = &http.BasicAuth{
				Username: "git",
				Password: repoArgs.Auth.PersonalAccessToken,
			}
		}

		// then we check to see if a username and a password has been specified
		if authDetails.Password != "" && authDetails.Username != "" {
			cloneOptions.Auth = &http.BasicAuth{
				Username: repoArgs.Auth.Username,
				Password: repoArgs.Auth.Password,
			}
		}
	}

	// clone
	repo, err := git.PlainCloneContext(ctx, workDir, false, cloneOptions)
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
