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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func setupGitRepo(ctx context.Context, workDir string, repoArgs *GitRepo) (string, error) {
	cloneOptions := &git.CloneOptions{
		RemoteName: "origin", // be explicit so we can require it in remote refs
		URL:        repoArgs.URL,
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
				return "", fmt.Errorf("unable to use SSH Private Key Path: %w", err)
			}

			cloneOptions.Auth = publicKeys
		}

		// Then we check if the details of a SSH Private Key as passed
		if authDetails.SSHPrivateKey != "" {
			publicKeys, err := ssh.NewPublicKeys("git", []byte(repoArgs.Auth.SSHPrivateKey), repoArgs.Auth.Password)
			if err != nil {
				return "", fmt.Errorf("unable to use SSH Private Key: %w", err)
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

	// *Repository.Clone() will do appropriate fetching given a branch name. We must deal with
	// different varieties, since people have been advised to use these as a workaround while only
	// "refs/heads/<default>" worked.
	//
	// If a reference name is not supplied, then .Clone will fetch all refs (and all objects
	// referenced by those), and checking out a commit later will work as expected.
	if repoArgs.Branch != "" {
		refName := plumbing.ReferenceName(repoArgs.Branch)
		switch {
		case refName.IsRemote(): // e.g., refs/remotes/origin/branch
			shorter := refName.Short() // this gives "origin/branch"
			parts := strings.SplitN(shorter, "/", 2)
			if len(parts) == 2 && parts[0] == "origin" {
				refName = plumbing.NewBranchReferenceName(parts[1])
			} else {
				return "", fmt.Errorf("a remote ref must begin with 'refs/remote/origin/', but got %q", repoArgs.Branch)
			}
		case refName.IsTag(): // looks like `refs/tags/v1.0.0` -- respect this even though the field is `.Branch`
			// nothing to do
		case !refName.IsBranch(): // not a remote, not refs/heads/branch; treat as a simple branch name
			refName = plumbing.NewBranchReferenceName(repoArgs.Branch)
		default:
			// already looks like a full branch name, so use as is
		}
		cloneOptions.ReferenceName = refName
	}

	// Azure DevOps requires multi_ack and multi_ack_detailed capabilities, which go-git doesn't
	// implement. But: it's possible to do a full clone by saying it's _not_ _un_supported, in which
	// case the library happily functions so long as it doesn't _actually_ get a multi_ack packet. See
	// https://github.com/go-git/go-git/blob/v5.5.1/_examples/azure_devops/main.go.
	oldUnsupportedCaps := transport.UnsupportedCapabilities
	// This check is crude, but avoids having another dependency to parse the git URL.
	if strings.Contains(repoArgs.URL, "dev.azure.com") {
		transport.UnsupportedCapabilities = []capability.Capability{
			capability.ThinPack,
		}
	}

	// clone
	repo, err := git.PlainCloneContext(ctx, workDir, false, cloneOptions)
	if err != nil {
		return "", fmt.Errorf("unable to clone repo: %w", err)
	}

	transport.UnsupportedCapabilities = oldUnsupportedCaps

	if repoArgs.CommitHash != "" {
		// checkout commit if specified
		w, err := repo.Worktree()
		if err != nil {
			return "", err
		}

		hash := repoArgs.CommitHash
		err = w.Checkout(&git.CheckoutOptions{
			Hash:  plumbing.NewHash(hash),
			Force: true,
		})
		if err != nil {
			return "", fmt.Errorf("unable to checkout commit: %w", err)
		}
	}

	var relPath string
	if repoArgs.ProjectPath != "" {
		relPath = repoArgs.ProjectPath
	}

	workDir = filepath.Join(workDir, relPath)
	return workDir, nil
}
