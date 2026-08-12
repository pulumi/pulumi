// Copyright 2026, Pulumi Corporation.
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

package gitutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v6/plumbing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// RetrieveGitFolder downloads the repo to path and returns the full path on disk.
func RetrieveGitFolder(ctx context.Context, rawurl string, path string) (string, error) {
	url, urlPath, err := ParseGitRepoURL(rawurl)
	if err != nil {
		return "", err
	}

	ref, commit, subDirectory, err := GetGitReferenceNameOrHashAndSubDirectory(url, urlPath)
	if err != nil {
		return "", fmt.Errorf("failed to get git ref: %w", err)
	}
	logging.V(10).Infof(
		"Attempting to fetch from %s at commit %s@%s for subdirectory '%s'",
		url, ref, commit, subDirectory)

	if ref != "" {
		// Different reference attempts to cycle through
		// We default to master then main in that order. We need to order them to avoid breaking
		// already existing processes for repos that already have a master and main branch.
		refAttempts := []plumbing.ReferenceName{plumbing.Master, plumbing.NewBranchReferenceName("main")}

		if ref != plumbing.HEAD {
			// If we have a non-default reference, we just use it
			refAttempts = []plumbing.ReferenceName{ref}
		}

		var cloneErrs []error
		for _, ref := range refAttempts {
			// Attempt the clone. If it succeeds, break
			err := GitCloneOrPull(ctx, url, ref, path, true /*shallow*/)
			if err == nil {
				break
			}
			logging.V(10).Infof("Failed to clone %s@%s: %v", url, ref, err)
			cloneErrs = append(cloneErrs, fmt.Errorf("ref '%s': %w", ref, err))
		}
		if len(cloneErrs) == len(refAttempts) {
			return "", fmt.Errorf("failed to clone %s: %w", rawurl, errors.Join(cloneErrs...))
		}
	} else {
		if cloneErr := GitCloneAndCheckoutCommit(ctx, url, commit, path); cloneErr != nil {
			logging.V(10).Infof("Failed to clone %s@%s: %v", url, commit, err)
			return "", fmt.Errorf("failed to clone and checkout %s(%s): %w", url, commit, cloneErr)
		}
	}

	// Verify the sub directory exists.
	fullPath := filepath.Join(path, filepath.FromSlash(subDirectory))
	logging.V(10).Infof("Cloned %s at commit %s@%s to %s", url, ref, commit, fullPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		logging.V(10).Infof("Failed to stat %s after cloning %s: %v", fullPath, url, err)
		return "", err
	}
	if !info.IsDir() {
		logging.V(10).Infof("%s was not a directory after cloning %s: %v", fullPath, url, err)
		return "", fmt.Errorf("%s is not a directory", fullPath)
	}

	return fullPath, nil
}
