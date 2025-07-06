// Copyright 2022-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auto

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This takes the unusual step of testing an unexported func. The rationale is to be able to test
// git code in isolation; testing the user of the unexported func (NewLocalWorkspace) drags in lots
// of other factors.

func TestGitClone(t *testing.T) {
	t.Parallel()

	// This makes a git repo to clone from, so to avoid relying on something at GitHub that could
	// change or be inaccessible.
	tmpDir := t.TempDir()
	originDir := filepath.Join(tmpDir, "origin")

	origin, err := git.PlainInit(originDir, false)
	require.NoError(t, err)
	w, err := origin.Worktree()
	require.NoError(t, err)
	nondefaultHead, err := w.Commit("nondefault branch", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "testo",
			Email: "testo@example.com",
		},
		AllowEmptyCommits: true,
	})
	require.NoError(t, err)

	// The following sets up some tags and branches: with `default` becoming the "default" branch
	// when cloning, since it's left as the HEAD of the repo.

	require.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("nondefault"),
		Create: true,
	}))

	// tag the nondefault head so we can test getting a tag too
	_, err = origin.CreateTag("v0.0.1", nondefaultHead, nil)
	require.NoError(t, err)

	// make a branch with slashes in it, so that can be tested too
	require.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("branch/with/slashes"),
		Create: true,
	}))

	require.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("default"),
		Create: true,
	}))
	defaultHead, err := w.Commit("default branch", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "testo",
			Email: "testo@example.com",
		},
		AllowEmptyCommits: true,
	})
	require.NoError(t, err)

	type testcase struct {
		branchName    string
		commitHash    string
		testName      string // use when supplying a hash, for a stable name
		expectedHead  plumbing.Hash
		expectedError string
	}

	for _, tc := range []testcase{
		{branchName: "default", expectedHead: defaultHead},
		{branchName: "nondefault", expectedHead: nondefaultHead},
		{branchName: "branch/with/slashes", expectedHead: nondefaultHead},
		// https://github.com/pulumi/pulumi-kubernetes-operator/issues/103#issuecomment-1107891475
		// advises using `refs/heads/<default>` for the default, and `refs/remotes/origin/<branch>`
		// for a non-default branch -- so we can expect all these varieties to be in use.
		{branchName: "refs/heads/default", expectedHead: defaultHead},
		{branchName: "refs/heads/nondefault", expectedHead: nondefaultHead},
		{branchName: "refs/heads/branch/with/slashes", expectedHead: nondefaultHead},

		{branchName: "refs/remotes/origin/default", expectedHead: defaultHead},
		{branchName: "refs/remotes/origin/nondefault", expectedHead: nondefaultHead},
		{branchName: "refs/remotes/origin/branch/with/slashes", expectedHead: nondefaultHead},
		// try the special tag case
		{branchName: "refs/tags/v0.0.1", expectedHead: nondefaultHead},
		// ask specifically for the commit hash
		{testName: "head of default as hash", commitHash: defaultHead.String(), expectedHead: defaultHead},
		{testName: "head of nondefault as hash", commitHash: nondefaultHead.String(), expectedHead: nondefaultHead},
	} {
		tc := tc
		if tc.testName == "" {
			tc.testName = tc.branchName
		}
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			repo := &GitRepo{
				URL:        originDir,
				Branch:     tc.branchName,
				CommitHash: tc.commitHash,
			}

			//nolint:usetesting // Need to use a specific location for the tmp dir
			tmp, err := os.MkdirTemp(tmpDir, "testcase") // i.e., under the tmp dir from earlier
			require.NoError(t, err)

			_, err = setupGitRepo(context.Background(), tmp, repo)
			require.NoError(t, err)

			r, err := git.PlainOpen(tmp)
			require.NoError(t, err)
			head, err := r.Head()
			require.NoError(t, err)
			assert.Equal(t, tc.expectedHead, head.Hash())
		})
	}

	// test that these result in errors
	for _, tc := range []testcase{
		{
			testName:      "simple branch doesn't exist",
			branchName:    "doesnotexist",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:      "full branch doesn't exist",
			branchName:    "refs/heads/doesnotexist",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:      "malformed branch name",
			branchName:    "refs/notathing/default",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:      "simple tag name won't work",
			branchName:    "v1.0.0",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:   "wrong remote",
			branchName: "refs/remotes/upstream/default",
			expectedError: "a remote ref must begin with 'refs/remote/origin/', " +
				"but got \"refs/remotes/upstream/default\"",
		},
	} {
		tc := tc
		if tc.testName == "" {
			tc.testName = tc.branchName
		}
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			repo := &GitRepo{
				URL:        originDir,
				Branch:     tc.branchName,
				CommitHash: tc.commitHash,
			}

			//nolint:usetesting // Need to use a specific location for the tmp dir
			tmp, err := os.MkdirTemp(tmpDir, "testcase") // i.e., under the tmp dir from earlier
			require.NoError(t, err)

			_, err = setupGitRepo(context.Background(), tmp, repo)
			assert.EqualError(t, err, tc.expectedError)
		})
	}
}
